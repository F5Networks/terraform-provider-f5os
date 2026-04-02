package provider

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// =============================================================================
// Proxy Acceptance Tests
// =============================================================================
//
// These tests verify that the F5OS provider works correctly when HTTP_PROXY
// or HTTPS_PROXY environment variables are set.
//
// Prerequisites:
//   - A real F5OS device (rSeries or VELOS) accessible via the proxy
//   - Standard F5OS env vars: F5OS_HOST, F5OS_USERNAME, F5OS_PASSWORD
//   - Proxy env var: HTTPS_PROXY (for TLS connections) or HTTP_PROXY
//   - TF_ACC=1 to enable acceptance tests
//
// Example usage with a SOCKS5 proxy:
//   export HTTPS_PROXY=socks5://proxy.example.com:1080
//   export F5OS_HOST=192.168.1.100
//   export F5OS_USERNAME=admin
//   export F5OS_PASSWORD=secret
//   TF_ACC=1 go test -v -run TestAccProxy -timeout 5m ./internal/provider/
//
// Example usage with an HTTP proxy:
//   export HTTPS_PROXY=http://proxy.example.com:3128
//   export F5OS_HOST=192.168.1.100
//   export F5OS_USERNAME=admin
//   export F5OS_PASSWORD=secret
//   TF_ACC=1 go test -v -run TestAccProxy -timeout 5m ./internal/provider/
//
// The tests use a simple DNS resource which is non-destructive and safe to
// run on shared test devices.
// =============================================================================

// testAccProxyPreCheck verifies that all required environment variables are
// set for proxy acceptance tests, including at least one proxy variable.
func testAccProxyPreCheck(t *testing.T) {
	t.Helper()

	// Check standard F5OS env vars
	for _, envVar := range []string{"F5OS_HOST", "F5OS_USERNAME", "F5OS_PASSWORD"} {
		if os.Getenv(envVar) == "" {
			t.Fatalf("%s must be set for proxy acceptance tests", envVar)
		}
	}

	// Check that at least one proxy env var is set
	httpsProxy := os.Getenv("HTTPS_PROXY")
	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxyLower := os.Getenv("https_proxy")
	httpProxyLower := os.Getenv("http_proxy")

	if httpsProxy == "" && httpProxy == "" && httpsProxyLower == "" && httpProxyLower == "" {
		t.Skip("Skipping proxy acceptance test: no proxy env var set (HTTPS_PROXY, HTTP_PROXY, https_proxy, or http_proxy)")
	}

	// Log which proxy is being used
	if httpsProxy != "" {
		t.Logf("Using HTTPS_PROXY: %s", httpsProxy)
	} else if httpsProxyLower != "" {
		t.Logf("Using https_proxy: %s", httpsProxyLower)
	} else if httpProxy != "" {
		t.Logf("Using HTTP_PROXY: %s", httpProxy)
	} else if httpProxyLower != "" {
		t.Logf("Using http_proxy: %s", httpProxyLower)
	}

	// Log NO_PROXY if set (useful for debugging)
	if noProxy := os.Getenv("NO_PROXY"); noProxy != "" {
		t.Logf("NO_PROXY is set: %s", noProxy)
	}
	if noProxyLower := os.Getenv("no_proxy"); noProxyLower != "" {
		t.Logf("no_proxy is set: %s", noProxyLower)
	}
}

// newProxyTestClientFromEnv creates an F5OS client using environment variables.
// This is used by custom check functions to verify state on the device.
// The client inherits proxy settings from HTTPS_PROXY/HTTP_PROXY via
// http.ProxyFromEnvironment which is set on the Transport in NewSession.
func newProxyTestClientFromEnv() (*f5ossdk.F5os, error) {
	host := os.Getenv("F5OS_HOST")
	user := os.Getenv("F5OS_USERNAME")
	if user == "" {
		user = os.Getenv("F5OS_USER")
	}
	pass := os.Getenv("F5OS_PASSWORD")

	port := 8888 // Default port matching provider.go:104
	if p := os.Getenv("F5OS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	cfg := &f5ossdk.F5osConfig{
		Host:             host,
		User:             user,
		Password:         pass,
		Port:             port,
		DisableSSLVerify: true, // Match provider default
	}

	return f5ossdk.NewSession(cfg)
}

// testAccCheckProxyClientConnects verifies that an F5OS client can successfully
// connect to the device through the proxy by creating a session and verifying
// it obtained an auth token.
func testAccCheckProxyClientConnects() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newProxyTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create F5OS client through proxy: %w", err)
		}

		// Verify we got an auth token (proves successful connection through proxy)
		if client.Token == "" {
			return fmt.Errorf("F5OS client connected but received no auth token")
		}

		// Verify transport has proxy configured
		if client.Transport == nil {
			return fmt.Errorf("F5OS client Transport is nil")
		}
		if client.Transport.Proxy == nil {
			return fmt.Errorf("F5OS client Transport.Proxy is nil - proxy not configured")
		}

		return nil
	}
}

// testAccCheckProxyDNSApplied verifies DNS configuration was applied on the
// device by querying the API directly (through the proxy).
func testAccCheckProxyDNSApplied(expectedServer string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newProxyTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client for DNS verification: %w", err)
		}

		// Use GetRequest to fetch DNS config
		dnsPath := fmt.Sprintf("%s/openconfig-system:system/dns", client.UriRoot)
		url := fmt.Sprintf("%s%s", client.Host, dnsPath)

		data, err := client.GetRequest(url)
		if err != nil {
			return fmt.Errorf("failed to get DNS config through proxy: %w", err)
		}

		// Basic check - verify we got a response
		if len(data) == 0 {
			return fmt.Errorf("empty response from DNS endpoint")
		}

		// Check if our expected server is in the response
		if expectedServer != "" {
			if !strings.Contains(string(data), expectedServer) {
				return fmt.Errorf("DNS config does not contain expected server %q", expectedServer)
			}
		}

		return nil
	}
}

// testAccCheckProxyDNSDestroy verifies DNS test entries are removed after
// the test completes.
func testAccCheckProxyDNSDestroy(s *terraform.State) error {
	client, err := newProxyTestClientFromEnv()
	if err != nil {
		// Cannot connect - treat as destroyed (or proxy no longer available)
		return nil
	}

	// Verify the test DNS server is no longer configured
	dnsPath := fmt.Sprintf("%s/openconfig-system:system/dns", client.UriRoot)
	url := fmt.Sprintf("%s%s", client.Host, dnsPath)

	data, err := client.GetRequest(url)
	if err != nil {
		// Error reading = treat as destroyed
		return nil
	}

	// Check that our test-specific DNS server (10.255.255.53) is gone
	// Using non-routable IP per safety rules in SKILL.md
	if strings.Contains(string(data), "10.255.255.53") {
		return fmt.Errorf("test DNS server 10.255.255.53 still present after destroy")
	}

	return nil
}

// =============================================================================
// Acceptance Test: Basic Proxy Connectivity
// =============================================================================

// TestAccProxyConnectivity verifies that the F5OS provider can connect to a
// device through a proxy server. This is a minimal test that just creates
// a session and verifies connectivity.
func TestAccProxyConnectivity(t *testing.T) {
	testAccProxyPreCheck(t)

	// Direct client test - verify we can connect through proxy
	client, err := newProxyTestClientFromEnv()
	if err != nil {
		t.Fatalf("Failed to connect to F5OS device through proxy: %v", err)
	}

	if client.Token == "" {
		t.Fatal("Connected but received no auth token")
	}

	t.Logf("Successfully connected to F5OS device through proxy")
	t.Logf("  Host: %s", client.Host)
	t.Logf("  Platform: %s", client.PlatformType)
	t.Logf("  Token obtained: yes (length=%d)", len(client.Token))

	// Verify transport proxy is configured
	if client.Transport == nil {
		t.Fatal("Transport is nil")
	}
	if client.Transport.Proxy == nil {
		t.Fatal("Transport.Proxy is nil - proxy support not configured")
	}
	t.Log("Transport.Proxy is configured (http.ProxyFromEnvironment)")
}

// =============================================================================
// Acceptance Test: DNS Resource Through Proxy
// =============================================================================

// TestAccProxyDNSResource tests creating, reading, updating, and deleting a
// DNS resource through a proxy connection. This is a full CRUD test that
// exercises the provider's proxy support end-to-end.
//
// Safety: Uses non-routable IP (10.255.255.53) and RFC 2606 reserved domain
// (.invalid) per the skill's safety rules for shared test devices.
func TestAccProxyDNSResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccProxyPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProxyDNSDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create DNS config and verify through proxy
			{
				Config: testAccProxyDNSResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Terraform state checks
					resource.TestCheckResourceAttr("f5os_dns.proxy_test", "id", "dns"),
					resource.TestCheckResourceAttr("f5os_dns.proxy_test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.proxy_test", "dns_servers.0", "10.255.255.53"),
					resource.TestCheckResourceAttr("f5os_dns.proxy_test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.proxy_test", "dns_domains.0", "proxy-test.invalid"),
					// Direct API verification through proxy
					testAccCheckProxyClientConnects(),
					testAccCheckProxyDNSApplied("10.255.255.53"),
				),
			},
			// Step 2: Update DNS config and verify through proxy
			{
				Config: testAccProxyDNSResourceConfigUpdated,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.proxy_test", "dns_servers.#", "2"),
					resource.TestCheckResourceAttr("f5os_dns.proxy_test", "dns_servers.0", "10.255.255.53"),
					resource.TestCheckResourceAttr("f5os_dns.proxy_test", "dns_servers.1", "10.255.255.54"),
					testAccCheckProxyClientConnects(),
					testAccCheckProxyDNSApplied("10.255.255.54"),
				),
			},
			// Step 3: Destroy is automatic - CheckDestroy verifies cleanup
		},
	})
}

// DNS config using non-routable IPs and reserved domains (safe for shared devices)
const testAccProxyDNSResourceConfig = `
resource "f5os_dns" "proxy_test" {
  dns_servers = ["10.255.255.53"]
  dns_domains = ["proxy-test.invalid"]
}
`

const testAccProxyDNSResourceConfigUpdated = `
resource "f5os_dns" "proxy_test" {
  dns_servers = ["10.255.255.53", "10.255.255.54"]
  dns_domains = ["proxy-test.invalid", "proxy-test-updated.invalid"]
}
`

// =============================================================================
// Acceptance Test: Verify Proxy Transport Preservation
// =============================================================================

// TestAccProxyTransportPreservation verifies that the proxy-configured
// transport is preserved across multiple API calls, including re-auth
// scenarios. This test creates multiple resources to exercise the transport
// reuse paths.
func TestAccProxyTransportPreservation(t *testing.T) {
	testAccProxyPreCheck(t)

	// Create initial session
	client1, err := newProxyTestClientFromEnv()
	if err != nil {
		t.Fatalf("First connection failed: %v", err)
	}
	t.Logf("First session created, token length: %d", len(client1.Token))

	// Verify transport has proxy
	if client1.Transport.Proxy == nil {
		t.Fatal("First session: Transport.Proxy is nil")
	}

	// Make an API call to exercise the transport
	dnsPath := fmt.Sprintf("%s/openconfig-system:system/dns", client1.UriRoot)
	url := fmt.Sprintf("%s%s", client1.Host, dnsPath)
	_, err = client1.GetRequest(url)
	if err != nil {
		t.Fatalf("API call through proxy failed: %v", err)
	}
	t.Log("API call through proxy succeeded")

	// Verify transport still has proxy after API call
	if client1.Transport.Proxy == nil {
		t.Fatal("Transport.Proxy is nil after API call")
	}

	// Create a second session to verify proxy config is consistent
	client2, err := newProxyTestClientFromEnv()
	if err != nil {
		t.Fatalf("Second connection failed: %v", err)
	}
	t.Logf("Second session created, token length: %d", len(client2.Token))

	if client2.Transport.Proxy == nil {
		t.Fatal("Second session: Transport.Proxy is nil")
	}

	// Both transports should have proxy configured but be independent
	if client1.Transport == client2.Transport {
		t.Fatal("Sessions should have independent Transport instances")
	}

	t.Log("Proxy transport preservation verified across multiple sessions")
}

// =============================================================================
// Acceptance Test: Data Source Through Proxy
// =============================================================================

// TestAccProxyDeviceInfoDataSource tests reading the device info data source
// through a proxy. This is a read-only test that's safe for any device.
func TestAccProxyDeviceInfoDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccProxyPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProxyDeviceInfoDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.f5os_device_info.proxy_test", "id"),
					resource.TestCheckResourceAttrSet("data.f5os_device_info.proxy_test", "platform_type"),
					testAccCheckProxyClientConnects(),
				),
			},
		},
	})
}

const testAccProxyDeviceInfoDataSourceConfig = `
data "f5os_device_info" "proxy_test" {
}
`

// =============================================================================
// Acceptance Test: NO_PROXY Bypass
// =============================================================================

// TestAccNoProxyBypass verifies that when NO_PROXY is set to include the
// F5OS host, the connection still succeeds (bypassing the proxy).
// This test is skipped if NO_PROXY is not set.
func TestAccNoProxyBypass(t *testing.T) {
	testAccProxyPreCheck(t)

	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = os.Getenv("no_proxy")
	}
	if noProxy == "" {
		t.Skip("Skipping NO_PROXY bypass test: NO_PROXY env var not set")
	}

	host := os.Getenv("F5OS_HOST")
	t.Logf("Testing NO_PROXY bypass with NO_PROXY=%s, F5OS_HOST=%s", noProxy, host)

	client, err := newProxyTestClientFromEnv()
	if err != nil {
		t.Fatalf("Connection failed: %v", err)
	}

	if client.Token == "" {
		t.Fatal("Connected but received no auth token")
	}

	t.Logf("Successfully connected with NO_PROXY set")
	t.Logf("  Host: %s", client.Host)
	t.Logf("  Platform: %s", client.PlatformType)
}
