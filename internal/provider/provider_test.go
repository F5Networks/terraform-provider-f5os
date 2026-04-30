package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
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
		"F5OS_HOST":     os.Getenv("F5OS_HOST"),
		"F5OS_USERNAME": os.Getenv("F5OS_USERNAME"),
		"F5OS_PASSWORD": os.Getenv("F5OS_PASSWORD"),
	}
	setup()
	_ = os.Setenv("F5OS_HOST", server.URL)
	_ = os.Setenv("F5OS_USERNAME", "testuser")
	_ = os.Setenv("F5OS_PASSWORD", "testpass")
}

func setup() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)
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
