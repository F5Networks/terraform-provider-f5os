package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

const (
// providerConfig is a shared configuration to combine with the actual
// test configuration so the HashiCups client is properly configured.
// It is also possible to use the HASHICUPS_ environment variables instead,
// such as updating the Makefile and running the testing through that tool.
// providerConfig = “
// f5osURI = "https://localhost:60155"
// f5osURI = "http://192.168.10.10:8888"
)

var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// server is a test HTTP server used to provide mock API responses
	server *httptest.Server

	// savedEnv holds the original F5OS env vars so teardown() can restore
	// them after unit tests that overwrite them with mock-server values.
	savedEnv map[string]string
)

var (
	// testAccProtoV6ProviderFactories are used to instantiate a provider during
	// acceptance testing. The factory function will be invoked for every Terraform
	// CLI command executed to create a provider server to which the CLI can
	// reattach.
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"f5os": providerserver.NewProtocol6WithError(New("devel")()),
	}
)

func testAccPreCheck(t *testing.T) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
	for _, s := range [...]string{"F5OS_HOST", "F5OS_USERNAME", "F5OS_PASSWORD"} {
		if os.Getenv(s) == "" {
			t.Fatal("F5OS_HOST, F5OS_USERNAME and F5OS_PASSWORD are required for tests.")
			return
		}
	}
}

func testAccPreUnitCheck(t *testing.T) {
	// Save original env vars so teardown() can restore them. This prevents
	// unit tests from polluting F5OS_HOST for acceptance tests that run
	// later in the same process.
	savedEnv = map[string]string{
		"F5OS_HOST":          os.Getenv("F5OS_HOST"),
		"F5OS_USERNAME":      os.Getenv("F5OS_USERNAME"),
		"F5OS_PASSWORD":      os.Getenv("F5OS_PASSWORD"),
		"F5OS_POLL_INTERVAL": os.Getenv("F5OS_POLL_INTERVAL"),
	}
	setup()
	_ = os.Setenv("F5OS_HOST", server.URL)
	_ = os.Setenv("F5OS_USERNAME", "testuser")
	_ = os.Setenv("F5OS_PASSWORD", "testpass")
	// Use a very short poll interval in unit tests to avoid the 20-second
	// sleeps that the real client uses between polling iterations.
	_ = os.Setenv("F5OS_POLL_INTERVAL", "1ms")
}

// unitTestLoginURI is the RESTCONF path NewSession uses to authenticate.
// Kept in sync with the client's uriLogin constant.
const unitTestLoginURI = "/restconf/data/openconfig-system:system/aaa"

func setup() {
	// test server
	mux = http.NewServeMux()
	// Wrap the mux so the login endpoint always yields an X-Auth-Token.
	// NewSession rejects a login response without a token, but many unit
	// tests only register their resource-specific data endpoints (or a
	// handler on the login path itself, for a different purpose such as a
	// token-lifetime PATCH) and rely on a successful session handshake.
	//
	// For the login path we pre-set a default X-Auth-Token before invoking
	// the mux. A registered handler can still overwrite the header, but if
	// it does not (the common case), the token is present so the session is
	// created. If no handler is registered for the login path at all, we
	// answer it directly.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == unitTestLoginURI {
			w.Header().Set("X-Auth-Token", "test-token")
			if _, pattern := mux.Handler(r); pattern == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
	server = httptest.NewServer(handler)
}

func teardown() {
	server.Close()
	// Restore original env vars so acceptance tests that run later in the
	// same process connect to the real device, not the (now-closed) mock.
	if savedEnv != nil {
		for k, v := range savedEnv {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
		savedEnv = nil
	}
}

// loadFixtureBytes returns the entire contents of the given file as a byte slice
func loadFixtureBytes(path string) []byte {
	contents, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// loadFixtureString returns the entire contents of the given file as a string
func loadFixtureString(path string) string {
	return string(loadFixtureBytes(path))
}

// testClientCache caches f5osclient sessions keyed by the connection
// parameters (host|user|port) so that repeated calls to newTestClientFromEnv
// during a test run reuse a single authenticated session instead of logging in
// again for every check function.
//
// Each acceptance test spins up many custom check functions, and each one used
// to call f5ossdk.NewSession — a full login + setPlatformType handshake. On
// devices with a strict AAA login policy (e.g. F5OS 2.0.0) that burst of logins
// trips the auth rate-limiter and check functions start getting 401
// access-denied. Reusing one session per connection eliminates the churn.
var (
	testClientCacheMu sync.Mutex
	testClientCache   = map[string]*f5ossdk.F5os{}
)

// newTestClientFromEnv returns an f5osclient session built from the standard
// F5OS_HOST / F5OS_USERNAME (or F5OS_USER) / F5OS_PASSWORD / F5OS_PORT
// environment variables. Port defaults to 8888 to match the provider.
//
// The session is cached and reused across calls with the same connection
// parameters to avoid re-authenticating on every acceptance-test check
// function, which can trip the device's auth rate-limiter.
//
// Use this in acceptance-test check functions that need an independent client
// to verify device state outside of the Terraform resource lifecycle.
func newTestClientFromEnv() (*f5ossdk.F5os, error) {
	host := os.Getenv("F5OS_HOST")
	user := os.Getenv("F5OS_USERNAME")
	if user == "" {
		user = os.Getenv("F5OS_USER")
	}
	pass := os.Getenv("F5OS_PASSWORD")
	port := 8888
	if p := os.Getenv("F5OS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	key := fmt.Sprintf("%s|%s|%d", host, user, port)

	testClientCacheMu.Lock()
	defer testClientCacheMu.Unlock()

	if client, ok := testClientCache[key]; ok && client != nil {
		return client, nil
	}

	cfg := &f5ossdk.F5osConfig{
		Host:             host,
		User:             user,
		Password:         pass,
		Port:             port,
		DisableSSLVerify: true,
	}
	client, err := f5ossdk.NewSession(cfg)
	if err != nil {
		return nil, err
	}
	testClientCache[key] = client
	return client, nil
}

// testAccPreCheckPlatformRSeries creates a throwaway f5osclient session to
// detect the device's platform type and skips the test if it is not rSeries.
// Use this in PreCheck for acceptance tests that assume an rSeries target.
func testAccPreCheckPlatformRSeries(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("testAccPreCheckPlatformRSeries: failed to create session: %s", err)
	}
	// PlatformType for rSeries is the model name (e.g. "r5900", "r12800-DS").
	// Skip if the device is a VELOS partition or controller.
	if client.PlatformType == "Velos Partition" || client.PlatformType == "Velos Controller" {
		t.Skipf("skipping: test requires rSeries but device is %q", client.PlatformType)
	}
}

// setupMockPlatformVersion registers handlers on the shared mux that make
// NewSession detect an rSeries platform running the specified F5OS version.
// Call this after testAccPreUnitCheck(t) and before registering test-specific
// handlers. Any mocked test that needs version-gated behavior (e.g., v1.7+
// password policy fields, v1.8+ TLS SAN) should use this helper.
func setupMockPlatformVersion(m *http.ServeMux, version string) {
	// Handler 1: Return an rSeries platform component list so
	// setPlatformType() detects "rSeries Platform" and calls setPlatformVersion().
	m.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_components_rseries.json"))
	})

	// Handler 2: Return the specified version so setPlatformVersion() sets
	// client.PlatformVersion to the value we want.
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-system-image:image/state/install", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-system-image:install":{"install-os-version":"%s","install-status":"success"}}`, version)
	})
}

// TestSessionCacheConcurrencyDedupe exercises the session-cache
// "double-check" pattern used in Configure by simulating concurrent
// callers that attempt to get-or-create a session for the same cache
// key. The test does not call the networked NewSession path; it
// simulates creation to avoid external dependencies while validating
// the concurrency semantics (only one final cache entry and all
// callers receive the same pointer).
func TestSessionCacheConcurrencyDedupe(t *testing.T) {
	const goroutines = 20

	// Ensure a clean starting state.
	sessionCacheMu.Lock()
	sessionCache = map[string]*f5ossdk.F5os{}
	sessionCacheMu.Unlock()

	host := "unit-test-host"
	port := 8888
	user := "tester"
	password := "pw"
	disableSSL := false
	headers := map[string]string{}

	cacheKey := sessionCacheKey(host, port, user, password, disableSSL, headers)

	results := make(chan *f5ossdk.F5os, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			// Read phase (fast, uses the mutex as in Configure).
			sessionCacheMu.Lock()
			client := sessionCache[cacheKey]
			sessionCacheMu.Unlock()

			if client == nil {
				// Simulate the expensive creation path (NewSession)
				// without performing network I/O. Sleep briefly to
				// increase the chance of interleaving between
				// goroutines.
				time.Sleep(5 * time.Millisecond)
				created := &f5ossdk.F5os{
					Host:     host,
					User:     user,
					Password: password,
				}

				// Double-check + store, matching the provider's pattern.
				sessionCacheMu.Lock()
				if existing, ok := sessionCache[cacheKey]; ok {
					client = existing
				} else {
					sessionCache[cacheKey] = created
					client = created
				}
				sessionCacheMu.Unlock()
			}

			results <- client
		}()
	}

	wg.Wait()
	close(results)

	// Verify all goroutines received the same pointer and the cache
	// has one entry.
	var first *f5ossdk.F5os
	for c := range results {
		if first == nil {
			first = c
			continue
		}
		if c != first {
			t.Fatalf("concurrent callers received different client pointers: %p vs %p", first, c)
		}
	}

	sessionCacheMu.Lock()
	if len(sessionCache) != 1 {
		t.Fatalf("expected 1 entry in sessionCache, got %d", len(sessionCache))
	}
	if sessionCache[cacheKey] != first {
		t.Fatalf("sessionCache entry does not match returned clients: cache=%p returned=%p", sessionCache[cacheKey], first)
	}
	sessionCacheMu.Unlock()
}
