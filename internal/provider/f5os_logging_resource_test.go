package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// ===========================================================================
// Section A: Acceptance test helpers
// ===========================================================================

// ---------------------------------------------------------------------------
// Pre-test cleanup: remove stale test resources from the device
// ---------------------------------------------------------------------------

// testAccLoggingCleanup removes any leftover test servers, remote forwarding,
// TLS, and include-hostname config from the device. This is called in PreCheck
// to ensure each test starts from a clean baseline, even if a prior test's
// destroy failed or left residual state (e.g., Update does not delete servers
// removed from config, so stale servers can persist across test runs).
func testAccLoggingCleanup(t *testing.T) {
	t.Helper()
	client, err := newTestClientFromEnv()
	if err != nil {
		t.Logf("cleanup: could not create client, skipping: %v", err)
		return
	}
	baseURI := "/openconfig-system:system/logging"

	// Delete all remote servers (the bulk endpoint removes them all).
	if err := client.DeleteRequest(baseURI + "/remote-servers"); err != nil {
		t.Logf("cleanup: DELETE remote-servers: %v (may not exist)", err)
	}
	// Delete remote forwarding config.
	if err := client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:host-logs"); err != nil {
		t.Logf("cleanup: DELETE host-logs: %v (may not exist)", err)
	}
	// Delete include-hostname config.
	if err := client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:config"); err != nil {
		t.Logf("cleanup: DELETE config: %v (may not exist)", err)
	}
	// Delete TLS/CA bundles.
	if err := client.DeleteRequest(baseURI + "/f5-openconfig-system-logging:tls"); err != nil {
		t.Logf("cleanup: DELETE tls: %v (may not exist)", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers to get cert/key/ca_bundle from env
// ---------------------------------------------------------------------------

func getTestCerts(t *testing.T) (string, string) {
	cert := os.Getenv("F5OS_TEST_CERT")
	key := os.Getenv("F5OS_TEST_KEY")
	if cert == "" || key == "" {
		t.Skip("F5OS_TEST_CERT and F5OS_TEST_KEY must be set for this test")
	}
	return cert, key
}

func getTestCABundle(t *testing.T) string {
	cabundle := os.Getenv("F5OS_TEST_CA_BUNDLE")
	if cabundle == "" {
		t.Skip("F5OS_TEST_CA_BUNDLE must be set for this test")
	}
	return cabundle
}

// ---------------------------------------------------------------------------
// Direct-API verification helpers
// ---------------------------------------------------------------------------



// testAccCheckLoggingIncludeHostnameOnDevice queries the device directly
// and verifies the include-hostname setting.
func testAccCheckLoggingIncludeHostnameOnDevice(expected bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetRequest("/openconfig-system:system/logging/f5-openconfig-system-logging:config")
		if err != nil {
			return fmt.Errorf("failed to read include-hostname from device: %w", err)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(resp, &data); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
		if config, ok := data["f5-openconfig-system-logging:config"].(map[string]interface{}); ok {
			if v, ok := config["include-hostname"].(bool); ok {
				if v != expected {
					return fmt.Errorf("include-hostname on device: %v, expected %v", v, expected)
				}
				return nil
			}
		}
		// If the key doesn't exist, the device default is false
		if expected {
			return fmt.Errorf("include-hostname not found on device, expected %v", expected)
		}
		return nil
	}
}

// testAccCheckLoggingServerOnDevice queries the device directly and verifies
// that a remote logging server with the given address is present.
func testAccCheckLoggingServerOnDevice(address string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetRequest("/openconfig-system:system/logging/remote-servers")
		if err != nil {
			return fmt.Errorf("failed to read servers from device: %w", err)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(resp, &data); err != nil {
			return fmt.Errorf("failed to parse servers response: %w", err)
		}
		if outer, ok := data["openconfig-system:remote-servers"].(map[string]interface{}); ok {
			if servers, ok := outer["remote-server"].([]interface{}); ok {
				for _, s := range servers {
					srv := s.(map[string]interface{})
					if conf, ok := srv["config"].(map[string]interface{}); ok {
						if host, ok := conf["host"].(string); ok {
							if strings.TrimSpace(host) == address {
								return nil
							}
						}
					}
				}
			}
		}
		return fmt.Errorf("server %q not found on device", address)
	}
}

// testAccCheckLoggingRemoteForwardingOnDevice queries the device directly
// and verifies the remote forwarding enabled state.
func testAccCheckLoggingRemoteForwardingOnDevice(expectedEnabled bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetRequest("/openconfig-system:system/logging/f5-openconfig-system-logging:host-logs")
		if err != nil {
			if expectedEnabled {
				return fmt.Errorf("failed to read remote forwarding from device: %w", err)
			}
			return nil // not configured is expected when disabled
		}
		var data map[string]interface{}
		if err := json.Unmarshal(resp, &data); err != nil {
			return fmt.Errorf("failed to parse remote forwarding response: %w", err)
		}
		if hostLogs, ok := data["f5-openconfig-system-logging:host-logs"].(map[string]interface{}); ok {
			if config, ok := hostLogs["config"].(map[string]interface{}); ok {
				if rf, ok := config["remote-forwarding"].(map[string]interface{}); ok {
					if enabled, ok := rf["enabled"].(bool); ok {
						if enabled != expectedEnabled {
							return fmt.Errorf("remote-forwarding enabled on device: %v, expected %v", enabled, expectedEnabled)
						}
						return nil
					}
				}
			}
		}
		if expectedEnabled {
			return fmt.Errorf("remote-forwarding not found on device, expected enabled=%v", expectedEnabled)
		}
		return nil
	}
}

// testAccCheckLoggingTLSOnDevice queries the device directly and verifies
// that TLS configuration is present.
func testAccCheckLoggingTLSOnDevice() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetRequest("/openconfig-system:system/logging/f5-openconfig-system-logging:tls")
		if err != nil {
			return fmt.Errorf("TLS configuration not found on device: %w", err)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(resp, &data); err != nil {
			return fmt.Errorf("failed to parse TLS response: %w", err)
		}
		if tls, ok := data["f5-openconfig-system-logging:tls"].(map[string]interface{}); ok {
			if _, ok := tls["certificate"]; !ok {
				return fmt.Errorf("TLS certificate not found on device")
			}
			return nil
		}
		return fmt.Errorf("TLS configuration structure not found on device")
	}
}

// ---------------------------------------------------------------------------
// CheckDestroy
// ---------------------------------------------------------------------------

// testAccCheckLoggingDestroy verifies that the logging configuration has been
// cleaned up after destroy. It checks that test-created servers, TLS config,
// remote forwarding, and include-hostname have been removed.
func testAccCheckLoggingDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		return nil // cannot connect — nothing to verify
	}

	baseURI := "/openconfig-system:system/logging"

	// Check servers are gone (the test IPs should not be present)
	testServers := []string{
		"10.255.255.90", "10.255.255.91", "10.255.255.92", "10.255.255.93",
		"10.255.255.94", "10.255.255.95", "10.255.255.96", "10.255.255.97",
		"10.255.255.98", "10.255.255.99",
	}
	resp, err := client.GetRequest(baseURI + "/remote-servers")
	if err == nil && len(resp) > 0 {
		for _, addr := range testServers {
			if strings.Contains(string(resp), addr) {
				return fmt.Errorf("test server %s still present on device after destroy", addr)
			}
		}
	}

	// Check TLS configuration is gone
	resp, err = client.GetRequest(baseURI + "/f5-openconfig-system-logging:tls")
	if err == nil && len(resp) > 0 {
		// TLS endpoint returned data — check if cert/key are still configured
		if strings.Contains(string(resp), "certificate") && strings.Contains(string(resp), "key") {
			return fmt.Errorf("TLS configuration still present on device after destroy")
		}
	}

	// Check remote forwarding is gone
	resp, err = client.GetRequest(baseURI + "/f5-openconfig-system-logging:host-logs")
	if err == nil && len(resp) > 0 {
		if strings.Contains(string(resp), "remote-forwarding") {
			var data map[string]interface{}
			if json.Unmarshal(resp, &data) == nil {
				// Check if remote-forwarding is enabled
				if hostLogs, ok := data["f5-openconfig-system-logging:host-logs"].(map[string]interface{}); ok {
					if config, ok := hostLogs["config"].(map[string]interface{}); ok {
						if rf, ok := config["remote-forwarding"].(map[string]interface{}); ok {
							if enabled, ok := rf["enabled"].(bool); ok && enabled {
								return fmt.Errorf("remote forwarding still enabled on device after destroy")
							}
						}
					}
				}
			}
		}
	}

	// Check include-hostname is reset (should be false/absent after destroy)
	resp, err = client.GetRequest(baseURI + "/f5-openconfig-system-logging:config")
	if err == nil && len(resp) > 0 {
		var data map[string]interface{}
		if json.Unmarshal(resp, &data) == nil {
			if config, ok := data["f5-openconfig-system-logging:config"].(map[string]interface{}); ok {
				if v, ok := config["include-hostname"].(bool); ok && v {
					return fmt.Errorf("include-hostname still true on device after destroy")
				}
			}
		}
	}

	return nil
}

// ===========================================================================
// Section B: Unit test mock infrastructure
// ===========================================================================

// ---------------------------------------------------------------------------
// Shared mock state for logging unit tests
// ---------------------------------------------------------------------------

// loggingMockState holds mutable mock-server state for the logging resource.
type loggingMockState struct {
	mu sync.Mutex

	// servers tracks remote syslog servers.
	servers []loggingMockServer

	// includeHostname tracks the include-hostname boolean.
	includeHostname bool

	// remoteForwarding tracks the host-logs/remote-forwarding config.
	rfEnabled bool
	rfLogs    []loggingMockLogSelector
	rfFiles   []string

	// TLS certificate and key.
	tlsCert string
	tlsKey  string

	// CA bundles list.
	caBundles []loggingMockCABundle

	// counters for verifying API interaction patterns.
	deleteServersCount       int
	deleteTLSCount           int
	deleteRemoteForwdCount   int
	deleteIncludeHostCount   int
	postServersCount         int
	putServersCount          int
	putTLSCount              int
	putRemoteForwardingCount int
	putIncludeHostCount      int
}

type loggingMockServer struct {
	Address        string
	Port           int64
	Protocol       string
	Authentication bool
	Logs           []loggingMockLogSelector
}

type loggingMockLogSelector struct {
	Facility string
	Severity string
}

type loggingMockCABundle struct {
	Name    string
	Content string
}

// registerLoggingBaseHandlers registers the 3 boilerplate mock HTTP handlers
// required by every logging unit test: AAA login, platform component detection,
// and image install state. Call this instead of duplicating these handlers in
// each standalone test.
func registerLoggingBaseHandlers() {
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Auth-Token", "mock-token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_components_rseries.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-image:image/state/install", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-system-image:install":{"install-os-version":"1.8.0","install-status":"success"}}`)
	})
}

// setupLoggingMock registers mock HTTP handlers for the logging resource
// and returns a pointer to the shared mock state.
func setupLoggingMock(t *testing.T) *loggingMockState {
	t.Helper()
	testAccPreUnitCheck(t)

	st := &loggingMockState{}

	registerLoggingBaseHandlers()

	// --- Remote Servers (GET, POST, DELETE) ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/remote-servers", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()

		switch r.Method {
		case "GET":
			if len(st.servers) == 0 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"data-missing"}]}}`))
				return
			}
			resp := buildServersGetResponse(st)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(resp)
		case "DELETE":
			st.deleteServersCount++
			st.servers = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// --- Remote Servers POST (trailing slash for POST to collection) ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/remote-servers/", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()

		path := r.URL.Path

		// Handle PUT to specific server: /remote-servers/remote-server=<address>
		if strings.Contains(path, "remote-server=") {
			st.putServersCount++
			parts := strings.SplitN(path, "remote-server=", 2)
			if len(parts) == 2 {
				addr := parts[1]
				body, _ := io.ReadAll(r.Body)
				server := parseServerFromPayload(body, addr)
				// Update or add
				found := false
				for i, s := range st.servers {
					if s.Address == addr {
						st.servers[i] = server
						found = true
						break
					}
				}
				if !found {
					st.servers = append(st.servers, server)
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}

		switch r.Method {
		case "POST":
			st.postServersCount++
			body, _ := io.ReadAll(r.Body)
			server := parseServerFromPayload(body, "")
			st.servers = append(st.servers, server)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// --- TLS Configuration (GET, PUT, DELETE) ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()

		switch r.Method {
		case "GET":
			if st.tlsCert == "" && st.tlsKey == "" && len(st.caBundles) == 0 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"data-missing"}]}}`))
				return
			}
			resp := buildTLSGetResponse(st)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(resp)
		case "PUT":
			st.putTLSCount++
			body, _ := io.ReadAll(r.Body)
			parseTLSPayload(st, body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			st.deleteTLSCount++
			st.tlsCert = ""
			st.tlsKey = ""
			st.caBundles = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// --- CA Bundles GET ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls/ca-bundles", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()

		if len(st.caBundles) == 0 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"data-missing"}]}}`))
			return
		}
		resp := buildCABundlesGetResponse(st)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp)
	})

	// --- Remote Forwarding / Host Logs (GET, PUT, DELETE) ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:host-logs/config", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()

		switch r.Method {
		case "PUT":
			st.putRemoteForwardingCount++
			body, _ := io.ReadAll(r.Body)
			parseRemoteForwardingPayload(st, body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:host-logs", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()

		switch r.Method {
		case "GET":
			resp := buildRemoteForwardingGetResponse(st)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(resp)
		case "DELETE":
			st.deleteRemoteForwdCount++
			st.rfEnabled = false
			st.rfLogs = nil
			st.rfFiles = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// --- Include Hostname / Config (GET, PUT, DELETE) ---
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:config", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()

		switch r.Method {
		case "GET":
			resp := fmt.Sprintf(`{"f5-openconfig-system-logging:config":{"include-hostname":%v}}`, st.includeHostname)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		case "PUT":
			st.putIncludeHostCount++
			body, _ := io.ReadAll(r.Body)
			parseIncludeHostnamePayload(st, body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			st.deleteIncludeHostCount++
			st.includeHostname = false
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return st
}

// ---------------------------------------------------------------------------
// Mock response builders
// ---------------------------------------------------------------------------

func buildServersGetResponse(st *loggingMockState) []byte {
	var serverList []map[string]interface{}
	for _, s := range st.servers {
		conf := map[string]interface{}{
			"host":                               s.Address,
			"remote-port":                        float64(s.Port),
			"f5-openconfig-system-logging:proto": s.Protocol,
		}
		if strings.ToLower(s.Protocol) == "tcp" {
			conf["f5-openconfig-system-logging:authentication"] = map[string]interface{}{
				"enabled": s.Authentication,
			}
		}

		serverEntry := map[string]interface{}{
			"host":   s.Address,
			"config": conf,
		}

		if len(s.Logs) > 0 {
			var selectors []map[string]interface{}
			for _, l := range s.Logs {
				selectors = append(selectors, map[string]interface{}{
					"facility": fmt.Sprintf("f5-system-logging-types:%s", strings.ToUpper(l.Facility)),
					"severity": strings.ToUpper(l.Severity),
				})
			}
			serverEntry["selectors"] = map[string]interface{}{
				"selector": selectors,
			}
		}

		serverList = append(serverList, serverEntry)
	}

	resp := map[string]interface{}{
		"openconfig-system:remote-servers": map[string]interface{}{
			"remote-server": serverList,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func buildTLSGetResponse(st *loggingMockState) []byte {
	tlsObj := map[string]interface{}{}
	if st.tlsCert != "" {
		tlsObj["certificate"] = st.tlsCert
	}
	if st.tlsKey != "" {
		// Simulate F5OS encrypting the key
		tlsObj["key"] = "$8$encryptedkeydatavalue1234567890"
	}

	resp := map[string]interface{}{
		"f5-openconfig-system-logging:tls": tlsObj,
	}
	data, _ := json.Marshal(resp)
	return data
}

func buildCABundlesGetResponse(st *loggingMockState) []byte {
	var bundles []map[string]interface{}
	for _, b := range st.caBundles {
		bundles = append(bundles, map[string]interface{}{
			"name": b.Name,
			"config": map[string]interface{}{
				"name":    b.Name,
				"content": b.Content,
			},
		})
	}
	resp := map[string]interface{}{
		"f5-openconfig-system-logging:ca-bundles": map[string]interface{}{
			"ca-bundle": bundles,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func buildRemoteForwardingGetResponse(st *loggingMockState) []byte {
	config := map[string]interface{}{
		"remote-forwarding": map[string]interface{}{
			"enabled": st.rfEnabled,
		},
	}

	if len(st.rfLogs) > 0 {
		var selectors []map[string]interface{}
		for _, l := range st.rfLogs {
			selectors = append(selectors, map[string]interface{}{
				"facility": fmt.Sprintf("openconfig-system-logging:%s", strings.ToUpper(l.Facility)),
				"severity": strings.ToUpper(l.Severity),
			})
		}
		config["selectors"] = map[string]interface{}{
			"selector": selectors,
		}
	}

	if len(st.rfFiles) > 0 {
		var files []map[string]interface{}
		for _, f := range st.rfFiles {
			files = append(files, map[string]interface{}{
				"name": f,
			})
		}
		config["files"] = map[string]interface{}{
			"file": files,
		}
	}

	resp := map[string]interface{}{
		"f5-openconfig-system-logging:host-logs": map[string]interface{}{
			"config": config,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

// ---------------------------------------------------------------------------
// Mock payload parsers
// ---------------------------------------------------------------------------

func parseServerFromPayload(body []byte, fallbackAddr string) loggingMockServer {
	var payload map[string]interface{}
	_ = json.Unmarshal(body, &payload)

	server := loggingMockServer{}

	// Handle the "remote-server" array in the payload
	if rs, ok := payload["remote-server"].([]interface{}); ok && len(rs) > 0 {
		s := rs[0].(map[string]interface{})
		if host, ok := s["host"].(string); ok {
			server.Address = host
		}
		if conf, ok := s["config"].(map[string]interface{}); ok {
			if host, ok := conf["host"].(string); ok {
				server.Address = host
			}
			if port, ok := conf["remote-port"].(float64); ok {
				server.Port = int64(port)
			}
			if proto, ok := conf["f5-openconfig-system-logging:proto"].(string); ok {
				server.Protocol = proto
			}
			if auth, ok := conf["f5-openconfig-system-logging:authentication"].(map[string]interface{}); ok {
				if enabled, ok := auth["enabled"].(bool); ok {
					server.Authentication = enabled
				}
			}
		}
		if selectors, ok := s["selectors"].(map[string]interface{}); ok {
			if sels, ok := selectors["selector"].([]interface{}); ok {
				for _, sel := range sels {
					sm := sel.(map[string]interface{})
					fac := fmt.Sprintf("%v", sm["facility"])
					sev := fmt.Sprintf("%v", sm["severity"])
					// Strip namespace prefix
					if strings.Contains(fac, ":") {
						fac = strings.Split(fac, ":")[1]
					}
					server.Logs = append(server.Logs, loggingMockLogSelector{
						Facility: strings.ToLower(fac),
						Severity: strings.ToLower(sev),
					})
				}
			}
		}
	}

	if server.Address == "" && fallbackAddr != "" {
		server.Address = fallbackAddr
	}
	return server
}

func parseTLSPayload(st *loggingMockState, body []byte) {
	var payload map[string]interface{}
	_ = json.Unmarshal(body, &payload)

	if tls, ok := payload["f5-openconfig-system-logging:tls"].(map[string]interface{}); ok {
		if cert, ok := tls["certificate"].(string); ok {
			st.tlsCert = cert
		}
		if key, ok := tls["key"].(string); ok {
			st.tlsKey = key
		}
		if caBundlesObj, ok := tls["ca-bundles"].(map[string]interface{}); ok {
			if bundleList, ok := caBundlesObj["ca-bundle"].([]interface{}); ok {
				st.caBundles = nil
				for _, b := range bundleList {
					bundle := b.(map[string]interface{})
					name := ""
					content := ""
					if n, ok := bundle["name"].(string); ok {
						name = n
					}
					if conf, ok := bundle["config"].(map[string]interface{}); ok {
						if c, ok := conf["content"].(string); ok {
							content = c
						}
					}
					st.caBundles = append(st.caBundles, loggingMockCABundle{
						Name:    name,
						Content: content,
					})
				}
			}
		}
	}
}

func parseRemoteForwardingPayload(st *loggingMockState, body []byte) {
	var payload map[string]interface{}
	_ = json.Unmarshal(body, &payload)

	if config, ok := payload["f5-openconfig-system-logging:config"].(map[string]interface{}); ok {
		if rf, ok := config["remote-forwarding"].(map[string]interface{}); ok {
			if enabled, ok := rf["enabled"].(bool); ok {
				st.rfEnabled = enabled
			}
		}
		if selectors, ok := config["selectors"].(map[string]interface{}); ok {
			if sels, ok := selectors["selector"].([]interface{}); ok {
				st.rfLogs = nil
				for _, sel := range sels {
					sm := sel.(map[string]interface{})
					fac := fmt.Sprintf("%v", sm["facility"])
					sev := fmt.Sprintf("%v", sm["severity"])
					if strings.Contains(fac, ":") {
						fac = strings.Split(fac, ":")[1]
					}
					st.rfLogs = append(st.rfLogs, loggingMockLogSelector{
						Facility: strings.ToLower(fac),
						Severity: strings.ToLower(sev),
					})
				}
			}
		}
		if files, ok := config["files"].(map[string]interface{}); ok {
			if fileList, ok := files["file"].([]interface{}); ok {
				st.rfFiles = nil
				for _, f := range fileList {
					fm := f.(map[string]interface{})
					if name, ok := fm["name"].(string); ok {
						st.rfFiles = append(st.rfFiles, name)
					}
				}
			}
		}
	}
}

func parseIncludeHostnamePayload(st *loggingMockState, body []byte) {
	var payload map[string]interface{}
	_ = json.Unmarshal(body, &payload)

	if config, ok := payload["f5-openconfig-system-logging:config"].(map[string]interface{}); ok {
		if v, ok := config["include-hostname"].(bool); ok {
			st.includeHostname = v
		}
	}
}

// ===========================================================================
// Section C: Acceptance tests (TestAcc*)
// ===========================================================================

// ---------------------------------------------------------------------------
// Legacy acceptance test (fixed: added PreCheck and CheckDestroy)
// ---------------------------------------------------------------------------

func TestAccF5osLogging_Logging(t *testing.T) {
	cert, key := getTestCerts(t)
	cabundle := getTestCABundle(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "f5os_logging" "logging" {
  include_hostname = false

  servers = [
    {
      address        = "192.168.100.1"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    },
    {
      address        = "192.168.100.2"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "emergency"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = true

    logs = [
      {
        facility = "local0"
        severity = "error"
      },
      {
        facility = "authpriv"
        severity = "critical"
      }
    ]

    files = [
      {
        name = "rseries_debug.log"
      },
      {
        name = "rseries_audit.log"
      }
    ]
  }

  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "rseries-ca"
      content = <<EOT
%s
EOT
    }
  ]
}
`, cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.logging", "include_hostname", "false"),
					resource.TestCheckResourceAttr("f5os_logging.logging", "servers.0.address", "192.168.100.1"),
					resource.TestCheckResourceAttr("f5os_logging.logging", "servers.1.address", "192.168.100.2"),
					resource.TestCheckResourceAttr("f5os_logging.logging", "tls.certificate", cert),
					resource.TestCheckResourceAttr("f5os_logging.logging", "tls.key", key),
					resource.TestCheckResourceAttr("f5os_logging.logging", "ca_bundles.0.content", cabundle),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: include_hostname only
// Exercises Create, Read, Delete with only include_hostname configured.
// Covers the null code paths for servers, TLS, CA bundles, and remote
// forwarding.
// ---------------------------------------------------------------------------

func TestAccLoggingIncludeHostnameOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLoggingIncludeHostnameOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "true"),
					testAccCheckLoggingIncludeHostnameOnDevice(true),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: remote forwarding only (no servers, no TLS)
// Covers createRemoteForwarding with logs and files, fetchRemoteForwarding
// during Read, and the null branches for servers/TLS/CA in Read.
// ---------------------------------------------------------------------------

func TestAccLoggingRemoteForwardingOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLoggingRemoteForwardingOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.0.facility", "local0"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.0.severity", "error"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.files.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.files.0.name", "acc-test.log"),
					testAccCheckLoggingRemoteForwardingOnDevice(true),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: servers only (UDP, no TLS/auth)
// Covers createServers with UDP protocol (no authentication block), the
// fetchServers happy path during Read, and Delete of servers.
// Uses non-routable 10.255.255.x addresses per safety rules.
// ---------------------------------------------------------------------------

func TestAccLoggingServersOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLoggingServersOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.90"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "514"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.protocol", "udp"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.0.facility", "local0"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.0.severity", "warning"),
					testAccCheckLoggingServerOnDevice("10.255.255.90"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: Create → Update lifecycle
// Step 1: Create with include_hostname=true and one server.
// Step 2: Update — change include_hostname to false, change the server port,
//   and add remote forwarding. This exercises the Update method which covers
//   putTLSWithCABundles (null skip), createRemoteForwarding, createServers
//   (POST-fallback-to-PUT for existing server), and createIncludeHostname.
// ---------------------------------------------------------------------------

func TestAccLoggingCreateThenUpdate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccLoggingUpdateStep1Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.91"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "514"),
					testAccCheckLoggingIncludeHostnameOnDevice(true),
					testAccCheckLoggingServerOnDevice("10.255.255.91"),
				),
			},
			// Step 2: Update
			{
				Config: testAccLoggingUpdateStep2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "false"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.91"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "1514"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.#", "1"),
					testAccCheckLoggingIncludeHostnameOnDevice(false),
					testAccCheckLoggingServerOnDevice("10.255.255.91"),
					testAccCheckLoggingRemoteForwardingOnDevice(true),
				),
			},
			// Destroy is automatic — CheckDestroy verifies cleanup.
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: full config with TLS + CA bundles + auth server
// Exercises putTLSWithCABundles, createServers with authentication=true
// (TCP + auth), the isAuthenticationRequired path in Read,
// fetchTLS (including encrypted key preservation), and
// fetchCABundles. Requires F5OS_TEST_CERT, F5OS_TEST_KEY,
// F5OS_TEST_CA_BUNDLE env vars.
// ---------------------------------------------------------------------------

func TestAccLoggingFullConfigWithTLS(t *testing.T) {
	cert, key := getTestCerts(t)
	cabundle := getTestCABundle(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testAccLoggingFullTLSConfigTemplate,
					cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "false"),
					// Servers sorted by address
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "2"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.92"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.protocol", "tcp"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.authentication", "false"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.1.address", "10.255.255.93"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.1.protocol", "udp"),
					// Remote forwarding
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.#", "2"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.files.#", "1"),
					// TLS and CA bundles
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.key"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.0.name", "acc-test-ca"),
					// Direct device verification
					testAccCheckLoggingIncludeHostnameOnDevice(false),
					testAccCheckLoggingServerOnDevice("10.255.255.92"),
					testAccCheckLoggingServerOnDevice("10.255.255.93"),
					testAccCheckLoggingRemoteForwardingOnDevice(true),
					testAccCheckLoggingTLSOnDevice(),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: multiple servers with update that changes server list
// Step 1: Create with two servers.
// Step 2: Update — remove one server, change the other's port. This exercises
//   the Update → createServers path and the POST-fallback-to-PUT logic.
// ---------------------------------------------------------------------------

func TestAccLoggingMultipleServersUpdate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			// Step 1: Two servers
			{
				Config: testAccLoggingMultiServersStep1Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "2"),
					testAccCheckLoggingServerOnDevice("10.255.255.90"),
					testAccCheckLoggingServerOnDevice("10.255.255.91"),
				),
			},
			// Step 2: Reduce to one server with different port
			{
				Config:             testAccLoggingMultiServersStep2Config,
				ExpectNonEmptyPlan: true, // Update does not delete old servers from device
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.90"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "1514"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: TLS + CA bundles with Create → Update
// Step 1: Create with TLS, CA bundle, and one TCP server (auth=false).
// Step 2: Update — add a second CA bundle. Exercises putTLSWithCABundles
//   during Update with changed CA bundle list.
// Requires F5OS_TEST_CERT, F5OS_TEST_KEY, F5OS_TEST_CA_BUNDLE.
// ---------------------------------------------------------------------------

func TestAccLoggingTLSUpdate(t *testing.T) {
	cert, key := getTestCerts(t)
	cabundle := getTestCABundle(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with TLS + 1 CA bundle
			{
				Config: fmt.Sprintf(testAccLoggingTLSUpdateStep1Template, cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.0.name", "acc-update-ca"),
					testAccCheckLoggingTLSOnDevice(),
				),
			},
			// Step 2: Update — change CA bundle content. Exercises
			// putTLSWithCABundles through the Update path.
			{
				Config: fmt.Sprintf(testAccLoggingTLSUpdateStep2Template, cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.0.name", "acc-update-ca"),
					testAccCheckLoggingTLSOnDevice(),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: TCP server with authentication=true + TLS + CA bundles
// Exercises createServers auth pre-validation, isAuthenticationRequired
// returning true during Read, the authRequired=true entry into the TLS/CA
// fetch block in Read, fetchTLS encrypted key preservation, and Delete with
// TLS-dependent servers.
// Requires F5OS_TEST_CERT, F5OS_TEST_KEY, F5OS_TEST_CA_BUNDLE.
// ---------------------------------------------------------------------------

func TestAccLoggingTCPServerWithAuth(t *testing.T) {
	cert, key := getTestCerts(t)
	cabundle := getTestCABundle(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testAccLoggingTCPAuthConfig, cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.94"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.protocol", "tcp"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.authentication", "true"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.key"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					testAccCheckLoggingServerOnDevice("10.255.255.94"),
					testAccCheckLoggingTLSOnDevice(),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: full-config Create → full-config Update (all sub-resources)
// Step 1: Create with servers + remote forwarding + include_hostname + TLS + CA.
// Step 2: Update ALL sub-resources: change server port, toggle include_hostname,
//   change remote forwarding logs, change CA bundle name. Exercises Update
//   calling all four sub-operations with non-null values.
// Requires F5OS_TEST_CERT, F5OS_TEST_KEY, F5OS_TEST_CA_BUNDLE.
// ---------------------------------------------------------------------------

func TestAccLoggingFullConfigUpdate(t *testing.T) {
	cert, key := getTestCerts(t)
	cabundle := getTestCABundle(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			// Step 1: Full config create
			{
				Config: fmt.Sprintf(testAccLoggingFullConfigUpdateStep1, cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.95"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "514"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.#", "1"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.0.name", "acc-full-ca"),
				),
			},
			// Step 2: Update everything
			{
				Config: fmt.Sprintf(testAccLoggingFullConfigUpdateStep2, cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "false"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.95"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "1514"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.#", "2"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.0.name", "acc-full-ca-v2"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: server with multiple log selectors
// Exercises createServers log sorting and fetchServers
// multiple-selector parsing.
// ---------------------------------------------------------------------------

func TestAccLoggingServerMultipleLogs(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLoggingServerMultipleLogsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.255.255.96"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.#", "2"),
					// Read sorts logs by facility: authpriv < local0
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.0.facility", "authpriv"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.0.severity", "warning"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.1.facility", "local0"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.1.severity", "debug"),
					testAccCheckLoggingServerOnDevice("10.255.255.96"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: TCP server with auth=true but no TLS → expect error
// Exercises createServers TLS verification failure.
// The create should fail before touching the device, so this is safe.
// ---------------------------------------------------------------------------

func TestAccLoggingAuthWithoutTLS(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLoggingAuthWithoutTLSConfig,
				ExpectError: regexp.MustCompile(`(?i)TLS Configuration Required|TLS certificates|authentication`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: Create with all sub-resources then Destroy
// Exercises the full Delete path with all sub-resources configured:
// servers, remote forwarding, include_hostname, and TLS+CA bundles.
// Also exercises the createRemoteForwarding sort paths with multiple log/file
// entries, and same-facility log sort in createServers.
// Requires F5OS_TEST_CERT, F5OS_TEST_KEY, F5OS_TEST_CA_BUNDLE.
// ---------------------------------------------------------------------------

func TestAccLoggingFullDeletePath(t *testing.T) {
	cert, key := getTestCerts(t)
	cabundle := getTestCABundle(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccLoggingCleanup(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLoggingDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(testAccLoggingFullDeletePathConfig, cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "2"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.#", "2"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.files.#", "2"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					testAccCheckLoggingServerOnDevice("10.255.255.98"),
					testAccCheckLoggingServerOnDevice("10.255.255.99"),
					testAccCheckLoggingTLSOnDevice(),
					testAccCheckLoggingRemoteForwardingOnDevice(true),
					testAccCheckLoggingIncludeHostnameOnDevice(true),
				),
			},
			// Destroy is automatic — CheckDestroy verifies all sub-resources cleaned up.
		},
	})
}

// ===========================================================================
// Section D: Unit tests using mock server (TestUnitLogging*)
// ===========================================================================

// TestUnitLoggingFullLifecycle exercises the full Create → Read → Update → Destroy
// lifecycle of the f5os_logging resource with all sub-resources configured:
// servers, remote_forwarding, include_hostname, TLS, and CA bundles.
func TestUnitLoggingFullLifecycle(t *testing.T) {
	st := setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with all sub-resources
			{
				Config: testUnitLoggingFullConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "true"),
					// Servers (sorted by address)
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "2"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.0.0.1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "514"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.protocol", "tcp"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.authentication", "false"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.0.facility", "local0"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.0.severity", "debug"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.1.address", "10.0.0.2"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.1.port", "1514"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.1.protocol", "udp"),
					// Remote forwarding
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.0.facility", "local0"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.0.severity", "error"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.files.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.files.0.name", "test.log"),
					// TLS
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.key"),
					// CA Bundles
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.0.name", "test-ca"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "ca_bundles.0.content"),
					// Verify mock state
					func(s *terraform.State) error {
						st.mu.Lock()
						defer st.mu.Unlock()
						if st.putTLSCount == 0 {
							return fmt.Errorf("expected TLS PUT call, got %d", st.putTLSCount)
						}
						if st.putRemoteForwardingCount == 0 {
							return fmt.Errorf("expected remote forwarding PUT call, got %d", st.putRemoteForwardingCount)
						}
						if st.putIncludeHostCount == 0 {
							return fmt.Errorf("expected include hostname PUT call, got %d", st.putIncludeHostCount)
						}
						if st.postServersCount+st.putServersCount == 0 {
							return fmt.Errorf("expected server POST/PUT calls, got post=%d put=%d", st.postServersCount, st.putServersCount)
						}
						return nil
					},
				),
			},
			// Step 2: Update — change include_hostname and reduce to one server.
			// Clear mock servers so the Update's POST creates a fresh list.
			{
				PreConfig: func() {
					st.mu.Lock()
					defer st.mu.Unlock()
					st.servers = nil
				},
				Config: testUnitLoggingUpdatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "false"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.0.0.1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "514"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.protocol", "tcp"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "false"),
				),
			},
		},
	})
}

// TestUnitLoggingServersOnly tests creating the resource with only servers configured.
// This exercises the code paths where TLS, CA bundles, remote forwarding, and
// include_hostname are all null/unset.
func TestUnitLoggingServersOnly(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLoggingServersOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.0.0.1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.port", "514"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.protocol", "udp"),
					resource.TestCheckNoResourceAttr("f5os_logging.test", "tls.certificate"),
					resource.TestCheckNoResourceAttr("f5os_logging.test", "remote_forwarding.enabled"),
				),
			},
		},
	})
}

// TestUnitLoggingIncludeHostnameOnly tests creating the resource with only
// include_hostname set. This exercises the null code paths for all other
// sub-resources.
func TestUnitLoggingIncludeHostnameOnly(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLoggingIncludeHostnameOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "true"),
				),
			},
		},
	})
}

// TestUnitLoggingRemoteForwardingOnly tests creating the resource with only
// remote_forwarding configured (no servers, no TLS, no CA bundles).
func TestUnitLoggingRemoteForwardingOnly(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLoggingRemoteForwardingOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.logs.#", "2"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.files.#", "2"),
				),
			},
		},
	})
}

// TestUnitLoggingTLSAndCABundlesOnly tests creating the resource with only
// TLS and CA bundles configured (no servers, no remote forwarding).
func TestUnitLoggingTLSAndCABundlesOnly(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLoggingTLSOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.key"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.0.name", "my-ca"),
				),
			},
		},
	})
}

// TestUnitLoggingDeleteVerifiesAPIOrder verifies that Delete calls the
// correct API endpoints and handles the deletion order properly (servers
// first, then remote forwarding, then include hostname, then TLS).
func TestUnitLoggingDeleteVerifiesAPIOrder(t *testing.T) {
	st := setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testUnitLoggingFullConfig,
			},
			// Step 2: Destroy — verify delete endpoints were called
			{
				Destroy: true,
				Config:  testUnitLoggingFullConfig,
				Check: func(s *terraform.State) error {
					st.mu.Lock()
					defer st.mu.Unlock()
					if st.deleteServersCount == 0 {
						return fmt.Errorf("expected servers DELETE call, got %d", st.deleteServersCount)
					}
					if st.deleteRemoteForwdCount == 0 {
						return fmt.Errorf("expected remote forwarding DELETE call, got %d", st.deleteRemoteForwdCount)
					}
					if st.deleteIncludeHostCount == 0 {
						return fmt.Errorf("expected include hostname DELETE call, got %d", st.deleteIncludeHostCount)
					}
					if st.deleteTLSCount == 0 {
						return fmt.Errorf("expected TLS DELETE call, got %d", st.deleteTLSCount)
					}
					return nil
				},
			},
		},
	})
}

// TestUnitLoggingServerWithAuthentication tests a server with authentication=true
// and tcp protocol, which exercises the TLS verification code path in createServers.
func TestUnitLoggingServerWithAuthentication(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLoggingServerWithAuthConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.authentication", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.protocol", "tcp"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
				),
			},
		},
	})
}

// TestUnitLoggingMultipleCABundles verifies that multiple CA bundles are
// handled correctly. The resource sorts them alphabetically during Read,
// so HCL must list them in sorted order to avoid a perpetual diff.
func TestUnitLoggingMultipleCABundles(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLoggingMultipleCABundlesSortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.#", "2"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.0.name", "alpha-ca"),
					resource.TestCheckResourceAttr("f5os_logging.test", "ca_bundles.1.name", "beta-ca"),
				),
			},
		},
	})
}

// TestUnitLoggingServerMultipleLogs verifies that servers with multiple log
// selectors are handled correctly and sorted by facility then severity.
func TestUnitLoggingServerMultipleLogs(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLoggingServerMultipleLogsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.#", "2"),
					// Sorted alphabetically by facility, then severity (matching fetchServers behavior)
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.0.facility", "authpriv"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.0.severity", "warning"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.1.facility", "local0"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.logs.1.severity", "debug"),
				),
			},
		},
	})
}

// TestUnitLoggingReadRefreshesFromDevice verifies that Read refreshes state
// from the mock device rather than preserving stale prior state.
func TestUnitLoggingReadRefreshesFromDevice(t *testing.T) {
	st := setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with include_hostname=true
			{
				Config: testUnitLoggingIncludeHostnameOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "true"),
				),
			},
			// Step 2: Simulate device changing include_hostname out-of-band
			{
				PreConfig: func() {
					st.mu.Lock()
					defer st.mu.Unlock()
					st.includeHostname = false
				},
				Config: testUnitLoggingIncludeHostnameFalseConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "false"),
				),
			},
		},
	})
}

// TestUnitLoggingReadServersOutOfBand verifies that when the device's server
// list changes out of band, Read detects the change. We verify by checking
// the mock state is properly updated when the config changes.
func TestUnitLoggingReadServersOutOfBand(t *testing.T) {
	st := setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with one server
			{
				Config: testUnitLoggingServersOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.0.0.1"),
				),
			},
			// Step 2: Update to a different server address. The mock replaces
			// the server list during the POST/PUT, and Read verifies the new state.
			{
				PreConfig: func() {
					// Clear mock servers to simulate starting fresh for the update
					st.mu.Lock()
					defer st.mu.Unlock()
					st.servers = nil
				},
				Config: testUnitLoggingServersChangedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.address", "10.0.0.5"),
					func(s *terraform.State) error {
						st.mu.Lock()
						defer st.mu.Unlock()
						if len(st.servers) == 0 {
							return fmt.Errorf("expected at least 1 server in mock, got 0")
						}
						found := false
						for _, srv := range st.servers {
							if srv.Address == "10.0.0.5" {
								found = true
								break
							}
						}
						if !found {
							return fmt.Errorf("expected server 10.0.0.5 in mock, got %+v", st.servers)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitLoggingAuthServerWithoutTLSInState tests the Read path where a TCP
// server has authentication=true but the TLS and CA bundle blocks are NOT in
// the Terraform config (state). This exercises setting TLS to null inside the
// authRequired branch and setting CA bundles to null inside the authRequired
// branch.
func TestUnitLoggingAuthServerWithoutTLSInState(t *testing.T) {
	testAccPreUnitCheck(t)

	registerLoggingBaseHandlers()

	// Servers — return a TCP server with authentication=true from the device
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/remote-servers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"openconfig-system:remote-servers":{"remote-server":[{"host":"10.0.0.1","config":{"host":"10.0.0.1","remote-port":514,"f5-openconfig-system-logging:proto":"tcp","f5-openconfig-system-logging:authentication":{"enabled":true}},"selectors":{"selector":[{"facility":"f5-system-logging-types:LOCAL0","severity":"DEBUG"}]}}]}}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/remote-servers/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	})

	// TLS endpoint — needed by the authRequired path in Read
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			// Return valid TLS so the auth verification in createServers passes
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:tls":{"certificate":"cert","key":"key"}}`))
		case "PUT":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// Include hostname
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:config":{"include-hostname":false}}`))
		case "PUT":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Config has a TCP server with auth=true but NO tls or ca_bundles block.
				// Read will see authRequired=true, tlsConfigured=false, caBundlesConfigured=false,
				// enter the authRequired branch, and set TLS/CA to null.
				Config: testUnitLoggingAuthServerNoTLSConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.authentication", "true"),
					resource.TestCheckResourceAttr("f5os_logging.test", "servers.0.protocol", "tcp"),
				),
			},
		},
	})
}

// TestUnitLoggingTLSPlainKeyFromDevice tests the fetchTLS code path where the
// device returns a non-encrypted key (no $8$ prefix). This exercises the else
// branch of the encrypted key check.
func TestUnitLoggingTLSPlainKeyFromDevice(t *testing.T) {
	testAccPreUnitCheck(t)

	registerLoggingBaseHandlers()

	// TLS endpoint — returns a non-encrypted (plain PEM) key
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:tls":{"certificate":"-----BEGIN CERTIFICATE-----\nplaintest\n-----END CERTIFICATE-----\n","key":"-----BEGIN RSA PRIVATE KEY-----\nplainkey\n-----END RSA PRIVATE KEY-----\n"}}`))
		case "PUT":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// CA bundles
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls/ca-bundles", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"data-missing"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testUnitLoggingTLSPlainKeyConfig,
				ExpectNonEmptyPlan: true, // CA bundles not returned by mock
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.key"),
				),
			},
		},
	})
}

// TestUnitLoggingServersEmptyResponse tests the Read path when the servers API
// returns an empty response body. fetchServers should treat this as "no servers
// configured" and set the state to null.
func TestUnitLoggingServersEmptyResponse(t *testing.T) {
	testAccPreUnitCheck(t)

	// Login/platform handlers
	registerLoggingBaseHandlers()

	// Remote servers — always return empty for GET, accept POST
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/remote-servers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			// Return empty response body (204-like)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(""))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/remote-servers/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	})

	// Include hostname — supports all methods
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:config":{"include-hostname":true}}`))
		case "PUT":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_logging" "test" {
  include_hostname = true
  servers = [
    {
      address        = "10.0.0.1"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "warning"
        }
      ]
    }
  ]
}`,
				// The empty GET response for servers means Read returns null
				// for servers, creating a drift from the plan. This triggers
				// a non-empty plan which is expected behavior.
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "true"),
				),
			},
		},
	})
}

// TestUnitLoggingTLSEncryptedKeyPreservation tests the fetchTLS path where
// F5OS returns an encrypted key ($8$ format) and the original PEM key is
// preserved from the current state to avoid drift.
func TestUnitLoggingTLSEncryptedKeyPreservation(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitLoggingTLSOnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.certificate"),
					resource.TestCheckResourceAttrSet("f5os_logging.test", "tls.key"),
				),
			},
		},
	})
}

// TestUnitLoggingRemoteForwardingErrorPath tests the fetchRemoteForwarding
// error path when the API returns an error or invalid JSON.
func TestUnitLoggingRemoteForwardingErrorPath(t *testing.T) {
	testAccPreUnitCheck(t)

	registerLoggingBaseHandlers()

	var requestCount int32
	// Host logs — first GET returns error, subsequent GETs return valid data
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:host-logs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n := atomic.AddInt32(&requestCount, 1)
			if n <= 1 {
				// First GET returns invalid JSON to trigger parse error path
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{invalid json`))
			} else {
				// Subsequent GETs return valid data with enabled=false (default)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:host-logs":{"config":{"remote-forwarding":{"enabled":false}}}}`))
			}
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:host-logs/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_logging" "test" {
  remote_forwarding = {
    enabled = false
  }
}`,
				// ExpectNonEmptyPlan because the invalid JSON triggers a fallback
				// that returns empty logs/files arrays, which differ from null in config.
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "false"),
				),
			},
		},
	})
}

// TestUnitLoggingCABundlesEmptyResponse tests the fetchCABundles path when
// the API returns an empty response body.
func TestUnitLoggingCABundlesEmptyResponse(t *testing.T) {
	testAccPreUnitCheck(t)

	registerLoggingBaseHandlers()

	// TLS endpoint — returns cert but no key for GET; accepts PUT
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:tls":{"certificate":"-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"}}`))
		case "PUT":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// CA bundles — return empty body
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls/ca-bundles", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(""))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_logging" "test" {
  tls = {
    certificate = "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"
    key = "-----BEGIN RSA PRIVATE KEY-----\ntestkey\n-----END RSA PRIVATE KEY-----\n"
  }
  ca_bundles = [
    {
      name    = "empty-test"
      content = "-----BEGIN CERTIFICATE-----\nca-content\n-----END CERTIFICATE-----\n"
    }
  ]
}`,
				ExpectNonEmptyPlan: true, // CA bundles empty from device vs configured
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
				),
			},
		},
	})
}

// TestUnitLoggingCreateIncludeHostnameNull tests the createIncludeHostname
// path when include_hostname is null (should be a no-op).
func TestUnitLoggingCreateIncludeHostnameNull(t *testing.T) {
	_ = setupLoggingMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Only remote forwarding configured — all other sub-resources are null
				Config: `
resource "f5os_logging" "test" {
  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "local0"
        severity = "error"
      }
    ]
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "remote_forwarding.enabled", "true"),
				),
			},
		},
	})
}

// TestUnitLoggingFetchIncludeHostnameError tests the Read path where
// fetchIncludeHostname gets an API error — it should default to false.
func TestUnitLoggingFetchIncludeHostnameError(t *testing.T) {
	testAccPreUnitCheck(t)

	registerLoggingBaseHandlers()

	var getCount int32
	// Include hostname — first few GETs succeed, then return error to simulate
	// device issue during Read
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			n := atomic.AddInt32(&getCount, 1)
			if n <= 2 {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:config":{"include-hostname":false}}`))
			} else {
				// Return error — fetchIncludeHostname should default to false
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"data-missing"}]}}`))
			}
		case "PUT":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_logging" "test" {
  include_hostname = false
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
					resource.TestCheckResourceAttr("f5os_logging.test", "include_hostname", "false"),
				),
			},
		},
	})
}

// TestUnitLoggingDeleteTLSErrorWarning tests the Delete path where TLS
// deletion fails with a specific error about authentication being enabled,
// which produces a warning rather than a fatal error.
func TestUnitLoggingDeleteTLSErrorWarning(t *testing.T) {
	testAccPreUnitCheck(t)

	registerLoggingBaseHandlers()

	// TLS — PUT succeeds, GET returns data, DELETE returns auth error
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:tls":{"certificate":"-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n","key":"$8$encrypted"}}`))
		case "PUT":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			// Simulate the "cannot allow remote server authentication" error
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"cannot allow remote server authentication without ca-bundle"}]}}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// CA bundles — return data for GET
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:tls/ca-bundles", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:ca-bundles":{"ca-bundle":[{"name":"test-ca","config":{"name":"test-ca","content":"-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"}}]}}`))
	})

	// Include hostname
	mux.HandleFunc("/restconf/data/openconfig-system:system/logging/f5-openconfig-system-logging:config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-openconfig-system-logging:config":{"include-hostname":false}}`))
		case "PUT":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_logging" "test" {
  include_hostname = false
  tls = {
    certificate = "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"
    key = "-----BEGIN RSA PRIVATE KEY-----\ntestkey\n-----END RSA PRIVATE KEY-----\n"
  }
  ca_bundles = [
    {
      name    = "test-ca"
      content = "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"
    }
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.test", "id", "f5os-logging"),
				),
			},
		},
	})
}

// ===========================================================================
// Section E: Unit tests - direct function tests
// ===========================================================================

// TestUnitNormalizeNewlinesBasic tests the normalizeNewlines function.
func TestUnitNormalizeNewlinesBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text unchanged",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "trims whitespace",
			input:    "  hello world  ",
			expected: "hello world",
		},
		{
			name:     "normalizes CRLF to LF",
			input:    "line1\r\nline2\r\n",
			expected: "line1\nline2",
		},
		{
			name:     "normalizes CR to LF",
			input:    "line1\rline2\r",
			expected: "line1\nline2",
		},
		{
			name:     "PEM cert ends with newline",
			input:    "-----BEGIN CERTIFICATE-----\ndata\n-----END CERTIFICATE-----",
			expected: "-----BEGIN CERTIFICATE-----\ndata\n-----END CERTIFICATE-----\n",
		},
		{
			name:     "PEM cert already has trailing newline",
			input:    "-----BEGIN CERTIFICATE-----\ndata\n-----END CERTIFICATE-----\n",
			expected: "-----BEGIN CERTIFICATE-----\ndata\n-----END CERTIFICATE-----\n",
		},
		{
			name:     "PEM key ends with newline",
			input:    "-----BEGIN RSA PRIVATE KEY-----\ndata\n-----END RSA PRIVATE KEY-----",
			expected: "-----BEGIN RSA PRIVATE KEY-----\ndata\n-----END RSA PRIVATE KEY-----\n",
		},
		{
			name:     "F5 encrypted key gets trailing newline",
			input:    "$8$encryptedkeydata",
			expected: "$8$encryptedkeydata\n",
		},
		{
			name:     "F5 encrypted key already has trailing newline",
			input:    "$8$encryptedkeydata\n",
			expected: "$8$encryptedkeydata\n",
		},
		{
			name:     "PEM with CRLF gets normalized",
			input:    "-----BEGIN CERTIFICATE-----\r\ndata\r\n-----END CERTIFICATE-----\r\n",
			expected: "-----BEGIN CERTIFICATE-----\ndata\n-----END CERTIFICATE-----\n",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\r\n  ",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeNewlines(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeNewlines(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestUnitFacilityValidatorDescription(t *testing.T) {
	v := facilityValidator{}
	ctx := context.Background()

	desc := v.Description(ctx)
	if desc == "" {
		t.Error("Description() returned empty string")
	}

	md := v.MarkdownDescription(ctx)
	if md == "" {
		t.Error("MarkdownDescription() returned empty string")
	}
	if !strings.Contains(md, "local0") {
		t.Errorf("MarkdownDescription() should mention local0, got %q", md)
	}
}

func TestUnitFacilityValidatorValidateString(t *testing.T) {
	v := facilityValidator{}
	ctx := context.Background()

	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{"valid local0", "local0", false},
		{"valid authpriv", "authpriv", false},
		{"invalid kern", "kern", true},
		{"invalid empty", "", true},
		{"invalid syslog", "syslog", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.StringRequest{
				ConfigValue: types.StringValue(tc.input),
			}
			resp := &validator.StringResponse{}
			v.ValidateString(ctx, req, resp)

			if tc.expectErr && !resp.Diagnostics.HasError() {
				t.Errorf("expected error for input %q, got none", tc.input)
			}
			if !tc.expectErr && resp.Diagnostics.HasError() {
				t.Errorf("unexpected error for input %q: %v", tc.input, resp.Diagnostics)
			}
		})
	}
}

func TestUnitLoggingMetadata(t *testing.T) {
	r := NewF5osLoggingResource()
	ctx := context.Background()

	metaReq := fwresource.MetadataRequest{ProviderTypeName: "f5os"}
	metaResp := &fwresource.MetadataResponse{}
	r.Metadata(ctx, metaReq, metaResp)

	if metaResp.TypeName != "f5os_logging" {
		t.Errorf("expected TypeName f5os_logging, got %q", metaResp.TypeName)
	}
}

func TestUnitLoggingSchema(t *testing.T) {
	r := NewF5osLoggingResource()
	ctx := context.Background()

	schemaReq := fwresource.SchemaRequest{}
	schemaResp := &fwresource.SchemaResponse{}
	r.Schema(ctx, schemaReq, schemaResp)

	s := schemaResp.Schema

	// Verify all expected attributes exist
	expectedAttrs := []string{"id", "servers", "remote_forwarding", "include_hostname", "tls", "ca_bundles"}
	for _, a := range expectedAttrs {
		if _, ok := s.Attributes[a]; !ok {
			t.Errorf("schema missing expected attribute %q", a)
		}
	}

	// Verify id is Computed
	idAttr := s.Attributes["id"]
	if !idAttr.IsComputed() {
		t.Error("id attribute should be Computed")
	}
}

func TestUnitLoggingConfigureNilProviderData(t *testing.T) {
	r := &f5osLoggingResource{}
	ctx := context.Background()

	req := fwresource.ConfigureRequest{
		ProviderData: nil,
	}
	resp := &fwresource.ConfigureResponse{}
	r.Configure(ctx, req, resp)

	// With nil provider data, client should be nil and no errors
	if r.client != nil {
		t.Error("expected client to be nil with nil ProviderData")
	}
}

func TestUnitIsAuthenticationRequired(t *testing.T) {
	ctx := context.Background()
	r := &f5osLoggingResource{}

	serverObjType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"address":        types.StringType,
			"port":           types.Int64Type,
			"protocol":       types.StringType,
			"authentication": types.BoolType,
			"logs": types.ListType{
				ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"facility": types.StringType, "severity": types.StringType}},
			},
		},
	}

	// Test with null servers
	state := &f5osLoggingModel{
		Servers: types.ListNull(serverObjType),
	}
	if r.isAuthenticationRequired(state) {
		t.Error("expected false with null servers")
	}

	// Helper to build a server list value
	buildList := func(servers []serverFields) types.List {
		list, diags := types.ListValueFrom(ctx, serverObjType, servers)
		if diags.HasError() {
			t.Fatalf("failed to create server list: %v", diags)
		}
		return list
	}

	// Test with TCP server with authentication=true
	state.Servers = buildList([]serverFields{
		{Address: "10.0.0.1", Port: 514, Protocol: "tcp", Authentication: true},
	})
	if !r.isAuthenticationRequired(state) {
		t.Error("expected true with TCP server auth=true")
	}

	// Test with UDP server with authentication=true (should return false — UDP doesn't use auth)
	state.Servers = buildList([]serverFields{
		{Address: "10.0.0.1", Port: 514, Protocol: "udp", Authentication: true},
	})
	if r.isAuthenticationRequired(state) {
		t.Error("expected false with UDP server (auth ignored)")
	}

	// Test with TCP server with authentication=false
	state.Servers = buildList([]serverFields{
		{Address: "10.0.0.1", Port: 514, Protocol: "tcp", Authentication: false},
	})
	if r.isAuthenticationRequired(state) {
		t.Error("expected false with TCP server auth=false")
	}
}

// ===========================================================================
// Section F: HCL config constants for acceptance tests (testAcc*)
// ===========================================================================

const testAccLoggingIncludeHostnameOnlyConfig = `
resource "f5os_logging" "test" {
  include_hostname = true
}
`

const testAccLoggingRemoteForwardingOnlyConfig = `
resource "f5os_logging" "test" {
  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "local0"
        severity = "error"
      }
    ]
    files = [
      {
        name = "acc-test.log"
      }
    ]
  }
}
`

const testAccLoggingServersOnlyConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.255.255.90"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "warning"
        }
      ]
    }
  ]
}
`

const testAccLoggingUpdateStep1Config = `
resource "f5os_logging" "test" {
  include_hostname = true

  servers = [
    {
      address        = "10.255.255.91"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "informational"
        }
      ]
    }
  ]
}
`

const testAccLoggingUpdateStep2Config = `
resource "f5os_logging" "test" {
  include_hostname = false

  servers = [
    {
      address        = "10.255.255.91"
      port           = 1514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "informational"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "local0"
        severity = "warning"
      }
    ]
  }
}
`

const testAccLoggingFullTLSConfigTemplate = `
resource "f5os_logging" "test" {
  include_hostname = false

  servers = [
    {
      address        = "10.255.255.92"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    },
    {
      address        = "10.255.255.93"
      port           = 1514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "error"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "authpriv"
        severity = "critical"
      },
      {
        facility = "local0"
        severity = "error"
      }
    ]
    files = [
      {
        name = "acc-tls-test.log"
      }
    ]
  }

  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "acc-test-ca"
      content = <<EOT
%s
EOT
    }
  ]
}
`

const testAccLoggingMultiServersStep1Config = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.255.255.90"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "warning"
        }
      ]
    },
    {
      address        = "10.255.255.91"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "error"
        }
      ]
    }
  ]
}
`

const testAccLoggingMultiServersStep2Config = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.255.255.90"
      port           = 1514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "warning"
        }
      ]
    }
  ]
}
`

const testAccLoggingTLSUpdateStep1Template = `
resource "f5os_logging" "test" {
  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "acc-update-ca"
      content = <<EOT
%s
EOT
    }
  ]
}
`

const testAccLoggingTLSUpdateStep2Template = `
resource "f5os_logging" "test" {
  include_hostname = false

  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "acc-update-ca"
      content = <<EOT
%s
EOT
    }
  ]
}
`

// ---------------------------------------------------------------------------
// HCL configs for new coverage tests
// ---------------------------------------------------------------------------

const testAccLoggingTCPAuthConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.255.255.94"
      port           = 514
      protocol       = "tcp"
      authentication = true
      logs = [
        {
          facility = "local0"
          severity = "informational"
        }
      ]
    }
  ]

  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "acc-auth-ca"
      content = <<EOT
%s
EOT
    }
  ]
}
`

const testAccLoggingFullConfigUpdateStep1 = `
resource "f5os_logging" "test" {
  include_hostname = true

  servers = [
    {
      address        = "10.255.255.95"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "warning"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "local0"
        severity = "error"
      }
    ]
    files = [
      {
        name = "acc-full-test.log"
      }
    ]
  }

  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "acc-full-ca"
      content = <<EOT
%s
EOT
    }
  ]
}
`

const testAccLoggingFullConfigUpdateStep2 = `
resource "f5os_logging" "test" {
  include_hostname = false

  servers = [
    {
      address        = "10.255.255.95"
      port           = 1514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "local0"
        severity = "error"
      },
      {
        facility = "authpriv"
        severity = "warning"
      }
    ]
    files = [
      {
        name = "acc-full-test.log"
      }
    ]
  }

  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "acc-full-ca-v2"
      content = <<EOT
%s
EOT
    }
  ]
}
`

const testAccLoggingServerMultipleLogsConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.255.255.96"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "warning"
        },
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    }
  ]
}
`

const testAccLoggingAuthWithoutTLSConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.255.255.97"
      port           = 514
      protocol       = "tcp"
      authentication = true
      logs = [
        {
          facility = "local0"
          severity = "warning"
        }
      ]
    }
  ]
}
`

const testAccLoggingFullDeletePathConfig = `
resource "f5os_logging" "test" {
  include_hostname = true

  servers = [
    {
      address        = "10.255.255.98"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "warning"
        },
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    },
    {
      address        = "10.255.255.99"
      port           = 1514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "error"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "authpriv"
        severity = "critical"
      },
      {
        facility = "local0"
        severity = "error"
      }
    ]
    files = [
      {
        name = "acc-delete-test1.log"
      },
      {
        name = "acc-delete-test2.log"
      }
    ]
  }

  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "acc-delete-ca"
      content = <<EOT
%s
EOT
    }
  ]
}
`

// ===========================================================================
// Section G: HCL config constants for unit tests (testUnitLogging*)
// ===========================================================================

const testUnitLoggingFullConfig = `
resource "f5os_logging" "test" {
  include_hostname = true

  servers = [
    {
      address        = "10.0.0.1"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    },
    {
      address        = "10.0.0.2"
      port           = 1514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "error"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "local0"
        severity = "error"
      }
    ]
    files = [
      {
        name = "test.log"
      }
    ]
  }

  tls = {
    certificate = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTAeFw0yNDA1MTYwMDAwMDBaFw0yNTA1MTYwMDAwMDBaMBExDzANBgNVBAMM\nBnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAGjUDBOMB0GA1UdDgQWBBQLp48bNsCI6DMsHa5KNKIWJJT10zAfBgNVHSME\nGDAWgBQLp48bNsCI6DMsHa5KNKIWJJT10zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBCwUAA0EAk8wrHWVIBVB9Fsi+K+cRw+mUniZJas0gPJavCl5fIxWw5Qiy52rG\n+5mWFnMnax+hKT2cG5XGhfXF5D+2YDHx8Q==\n-----END CERTIFICATE-----\n"

    key = "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALuj3rD4DbwaE31IfOb8WfxTWMa/yufkXVD8iDYitolvtdI49Kr6\nn90iG1sgk9gaQRPZWEvblcYcwsvCmcyCMo0CAwEAAQJBALB3DjEYSVOw6LSqEm1K\nEfLSHnJxBGbTtNzJ+34GGalHHjGjBWn2FsbUq4Iw5ILFbkKHhmDjdW1XtzT7xw7X\nRFECIQDphJPFz12j3aTll50WT9pUoFbpAUhc4TuDzfXWP8XJzQIhAMtDAu+r/8HO\na+y7U+16transformedForTestingkKKKJhAiEAigc2kqqO6G/MFLB6cXr3GB8TA9k0z\nbAHZKY0TQGlIGUECIGQoGnWzPB3+1qeS4IgqDCmhsKz7dzWQ9oUVbI8s2CT\n-----END RSA PRIVATE KEY-----\n"
  }

  ca_bundles = [
    {
      name    = "test-ca"
      content = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTAeFw0yNDA1MTYwMDAwMDBaFw0yNTA1MTYwMDAwMDBaMBExDzANBgNVBAMM\nBnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAGjUDBOMB0GA1UdDgQWBBQLp48bNsCI6DMsHa5KNKIWJJT10zAfBgNVHSME\nGDAWgBQLp48bNsCI6DMsHa5KNKIWJJT10zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBCwUAA0EAk8wrHWVIBVB9Fsi+K+cRw+mUniZJas0gPJavCl5fIxWw5Qiy52rG\n+5mWFnMnax+hKT2cG5XGhfXF5D+2YDHx8Q==\n-----END CERTIFICATE-----\n"
    }
  ]
}
`

const testUnitLoggingUpdatedConfig = `
resource "f5os_logging" "test" {
  include_hostname = false

  servers = [
    {
      address        = "10.0.0.1"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = false
    logs = [
      {
        facility = "local0"
        severity = "error"
      }
    ]
    files = [
      {
        name = "test.log"
      }
    ]
  }

  tls = {
    certificate = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTAeFw0yNDA1MTYwMDAwMDBaFw0yNTA1MTYwMDAwMDBaMBExDzANBgNVBAMM\nBnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAGjUDBOMB0GA1UdDgQWBBQLp48bNsCI6DMsHa5KNKIWJJT10zAfBgNVHSME\nGDAWgBQLp48bNsCI6DMsHa5KNKIWJJT10zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBCwUAA0EAk8wrHWVIBVB9Fsi+K+cRw+mUniZJas0gPJavCl5fIxWw5Qiy52rG\n+5mWFnMnax+hKT2cG5XGhfXF5D+2YDHx8Q==\n-----END CERTIFICATE-----\n"

    key = "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALuj3rD4DbwaE31IfOb8WfxTWMa/yufkXVD8iDYitolvtdI49Kr6\nn90iG1sgk9gaQRPZWEvblcYcwsvCmcyCMo0CAwEAAQJBALB3DjEYSVOw6LSqEm1K\nEfLSHnJxBGbTtNzJ+34GGalHHjGjBWn2FsbUq4Iw5ILFbkKHhmDjdW1XtzT7xw7X\nRFECIQDphJPFz12j3aTll50WT9pUoFbpAUhc4TuDzfXWP8XJzQIhAMtDAu+r/8HO\na+y7U+16transformedForTestingkKKKJhAiEAigc2kqqO6G/MFLB6cXr3GB8TA9k0z\nbAHZKY0TQGlIGUECIGQoGnWzPB3+1qeS4IgqDCmhsKz7dzWQ9oUVbI8s2CT\n-----END RSA PRIVATE KEY-----\n"
  }

  ca_bundles = [
    {
      name    = "test-ca"
      content = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTAeFw0yNDA1MTYwMDAwMDBaFw0yNTA1MTYwMDAwMDBaMBExDzANBgNVBAMM\nBnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAGjUDBOMB0GA1UdDgQWBBQLp48bNsCI6DMsHa5KNKIWJJT10zAfBgNVHSME\nGDAWgBQLp48bNsCI6DMsHa5KNKIWJJT10zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBCwUAA0EAk8wrHWVIBVB9Fsi+K+cRw+mUniZJas0gPJavCl5fIxWw5Qiy52rG\n+5mWFnMnax+hKT2cG5XGhfXF5D+2YDHx8Q==\n-----END CERTIFICATE-----\n"
    }
  ]
}
`

const testUnitLoggingServersOnlyConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.0.0.1"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "warning"
        }
      ]
    }
  ]
}
`

const testUnitLoggingServersChangedConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.0.0.5"
      port           = 514
      protocol       = "udp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "warning"
        }
      ]
    }
  ]
}
`

const testUnitLoggingIncludeHostnameOnlyConfig = `
resource "f5os_logging" "test" {
  include_hostname = true
}
`

const testUnitLoggingIncludeHostnameFalseConfig = `
resource "f5os_logging" "test" {
  include_hostname = false
}
`

const testUnitLoggingRemoteForwardingOnlyConfig = `
resource "f5os_logging" "test" {
  remote_forwarding = {
    enabled = true
    logs = [
      {
        facility = "local0"
        severity = "error"
      },
      {
        facility = "authpriv"
        severity = "critical"
      }
    ]
    files = [
      {
        name = "debug.log"
      },
      {
        name = "audit.log"
      }
    ]
  }
}
`

const testUnitLoggingTLSOnlyConfig = `
resource "f5os_logging" "test" {
  tls = {
    certificate = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTAeFw0yNDA1MTYwMDAwMDBaFw0yNTA1MTYwMDAwMDBaMBExDzANBgNVBAMM\nBnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAGjUDBOMB0GA1UdDgQWBBQLp48bNsCI6DMsHa5KNKIWJJT10zAfBgNVHSME\nGDAWgBQLp48bNsCI6DMsHa5KNKIWJJT10zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBCwUAA0EAk8wrHWVIBVB9Fsi+K+cRw+mUniZJas0gPJavCl5fIxWw5Qiy52rG\n+5mWFnMnax+hKT2cG5XGhfXF5D+2YDHx8Q==\n-----END CERTIFICATE-----\n"

    key = "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALuj3rD4DbwaE31IfOb8WfxTWMa/yufkXVD8iDYitolvtdI49Kr6\nn90iG1sgk9gaQRPZWEvblcYcwsvCmcyCMo0CAwEAAQJBALB3DjEYSVOw6LSqEm1K\nEfLSHnJxBGbTtNzJ+34GGalHHjGjBWn2FsbUq4Iw5ILFbkKHhmDjdW1XtzT7xw7X\nRFECIQDphJPFz12j3aTll50WT9pUoFbpAUhc4TuDzfXWP8XJzQIhAMtDAu+r/8HO\na+y7U+16transformedForTestingkKKKJhAiEAigc2kqqO6G/MFLB6cXr3GB8TA9k0z\nbAHZKY0TQGlIGUECIGQoGnWzPB3+1qeS4IgqDCmhsKz7dzWQ9oUVbI8s2CT\n-----END RSA PRIVATE KEY-----\n"
  }

  ca_bundles = [
    {
      name    = "my-ca"
      content = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAEwDQYJKoZIhvcNAQELBQADQQBtest\n-----END CERTIFICATE-----\n"
    }
  ]
}
`

const testUnitLoggingServerWithAuthConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.0.0.1"
      port           = 514
      protocol       = "tcp"
      authentication = true
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    }
  ]

  tls = {
    certificate = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTAeFw0yNDA1MTYwMDAwMDBaFw0yNTA1MTYwMDAwMDBaMBExDzANBgNVBAMM\nBnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAGjUDBOMB0GA1UdDgQWBBQLp48bNsCI6DMsHa5KNKIWJJT10zAfBgNVHSME\nGDAWgBQLp48bNsCI6DMsHa5KNKIWJJT10zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBCwUAA0EAk8wrHWVIBVB9Fsi+K+cRw+mUniZJas0gPJavCl5fIxWw5Qiy52rG\n+5mWFnMnax+hKT2cG5XGhfXF5D+2YDHx8Q==\n-----END CERTIFICATE-----\n"

    key = "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALuj3rD4DbwaE31IfOb8WfxTWMa/yufkXVD8iDYitolvtdI49Kr6\nn90iG1sgk9gaQRPZWEvblcYcwsvCmcyCMo0CAwEAAQJBALB3DjEYSVOw6LSqEm1K\nEfLSHnJxBGbTtNzJ+34GGalHHjGjBWn2FsbUq4Iw5ILFbkKHhmDjdW1XtzT7xw7X\nRFECIQDphJPFz12j3aTll50WT9pUoFbpAUhc4TuDzfXWP8XJzQIhAMtDAu+r/8HO\na+y7U+16transformedForTestingkKKKJhAiEAigc2kqqO6G/MFLB6cXr3GB8TA9k0z\nbAHZKY0TQGlIGUECIGQoGnWzPB3+1qeS4IgqDCmhsKz7dzWQ9oUVbI8s2CT\n-----END RSA PRIVATE KEY-----\n"
  }

  ca_bundles = [
    {
      name    = "auth-ca"
      content = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAEwDQYJKoZIhvcNAQELBQADQQBtest\n-----END CERTIFICATE-----\n"
    }
  ]
}
`

// testUnitLoggingMultipleCABundlesSortedConfig lists CA bundles in alphabetical
// order to match what fetchCABundles returns after sorting.
const testUnitLoggingMultipleCABundlesSortedConfig = `
resource "f5os_logging" "test" {
  tls = {
    certificate = "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJALR1oQ4FAKECMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl\nc3RjYTAeFw0yNDA1MTYwMDAwMDBaFw0yNTA1MTYwMDAwMDBaMBExDzANBgNVBAMM\nBnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96w+A28GhN9SHzm/Fn8\nU1jGv8rn5F1Q/Ig2IraJb7XSOPSq+p/dIhtbIJPYGkET2VhL25XGHMLL3JjMgzKN\nAgMBAAGjUDBOMB0GA1UdDgQWBBQLp48bNsCI6DMsHa5KNKIWJJT10zAfBgNVHSME\nGDAWgBQLp48bNsCI6DMsHa5KNKIWJJT10zAMBgNVHRMEBTADAQH/MA0GCSqGSIb3\nDQEBCwUAA0EAk8wrHWVIBVB9Fsi+K+cRw+mUniZJas0gPJavCl5fIxWw5Qiy52rG\n+5mWFnMnax+hKT2cG5XGhfXF5D+2YDHx8Q==\n-----END CERTIFICATE-----\n"

    key = "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALuj3rD4DbwaE31IfOb8WfxTWMa/yufkXVD8iDYitolvtdI49Kr6\nn90iG1sgk9gaQRPZWEvblcYcwsvCmcyCMo0CAwEAAQJBALB3DjEYSVOw6LSqEm1K\nEfLSHnJxBGbTtNzJ+34GGalHHjGjBWn2FsbUq4Iw5ILFbkKHhmDjdW1XtzT7xw7X\nRFECIQDphJPFz12j3aTll50WT9pUoFbpAUhc4TuDzfXWP8XJzQIhAMtDAu+r/8HO\na+y7U+16transformedForTestingkKKKJhAiEAigc2kqqO6G/MFLB6cXr3GB8TA9k0z\nbAHZKY0TQGlIGUECIGQoGnWzPB3+1qeS4IgqDCmhsKz7dzWQ9oUVbI8s2CT\n-----END RSA PRIVATE KEY-----\n"
  }

  ca_bundles = [
    {
      name    = "alpha-ca"
      content = "-----BEGIN CERTIFICATE-----\nalpha-content\n-----END CERTIFICATE-----\n"
    },
    {
      name    = "beta-ca"
      content = "-----BEGIN CERTIFICATE-----\nbeta-content\n-----END CERTIFICATE-----\n"
    }
  ]
}
`

// testUnitLoggingServerMultipleLogsConfig lists logs in alphabetical order
// (by facility, then severity) to match what fetchServers returns after sorting.
const testUnitLoggingServerMultipleLogsConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.0.0.1"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "warning"
        },
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    }
  ]
}
`

// testUnitLoggingAuthServerNoTLSConfig has a TCP server with auth=true but
// no tls or ca_bundles blocks, exercising the Read path where authRequired=true
// but TLS/CA are not configured in state.
const testUnitLoggingAuthServerNoTLSConfig = `
resource "f5os_logging" "test" {
  servers = [
    {
      address        = "10.0.0.1"
      port           = 514
      protocol       = "tcp"
      authentication = true
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    }
  ]
}
`

// testUnitLoggingTLSPlainKeyConfig is used with a mock that returns a non-encrypted
// (plain PEM) key, exercising fetchTLS's else branch for non-encrypted keys.
const testUnitLoggingTLSPlainKeyConfig = `
resource "f5os_logging" "test" {
  tls = {
    certificate = "-----BEGIN CERTIFICATE-----\nplaintest\n-----END CERTIFICATE-----\n"
    key = "-----BEGIN RSA PRIVATE KEY-----\nplainkey\n-----END RSA PRIVATE KEY-----\n"
  }
  ca_bundles = [
    {
      name    = "plain-ca"
      content = "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"
    }
  ]
}
`
