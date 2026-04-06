package provider

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// ---------------------------------------------------------------------------
// Unit-test HCL configs (use 10.20.30.40 — only hits mock server)
// ---------------------------------------------------------------------------

const testUnitNTPServerBasicConfig = `
resource "f5os_ntp_server" "test" {
  server             = "10.20.30.40"
  key_id             = 123
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

// ---------------------------------------------------------------------------
// Acceptance-test HCL configs (use non-routable 10.255.255.x per safety rules)
// ---------------------------------------------------------------------------

const testAccNTPServerBasicConfig = `
resource "f5os_ntp_server" "test" {
  server             = "10.255.255.1"
  key_id             = 0
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

const testAccNTPServerUpdatedConfig = `
resource "f5os_ntp_server" "test" {
  server             = "10.255.255.1"
  key_id             = 0
  prefer             = false
  iburst             = false
  ntp_service        = true
  ntp_authentication = true
}
`

// ---------------------------------------------------------------------------
// Helper: create a fresh F5OS client from env vars (port defaults to 8888)
// ---------------------------------------------------------------------------

func newNtpClientFromEnv() (*f5ossdk.F5os, error) {
	host := os.Getenv("F5OS_HOST")
	user := os.Getenv("F5OS_USERNAME")
	if user == "" {
		user = os.Getenv("F5OS_USER")
	}
	pass := os.Getenv("F5OS_PASSWORD")
	port := 8888 // Must default to 8888 to match the provider
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
		DisableSSLVerify: true,
	}
	return f5ossdk.NewSession(cfg)
}

// ---------------------------------------------------------------------------
// Direct API verification: check NTP server exists on device with expected values
// ---------------------------------------------------------------------------

func testAccCheckNTPServerOnDevice(server string, expectKeyID int64, expectPrefer, expectIBurst bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newNtpClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create F5OS client: %w", err)
		}
		ntp, err := client.GetNTPServer(server)
		if err != nil {
			return fmt.Errorf("failed to read NTP server %s from device: %w", server, err)
		}
		if ntp.Address != server {
			return fmt.Errorf("expected NTP server address %q, got %q", server, ntp.Address)
		}
		if ntp.KeyID != expectKeyID {
			return fmt.Errorf("expected key_id %d, got %d", expectKeyID, ntp.KeyID)
		}
		if ntp.Prefer != expectPrefer {
			return fmt.Errorf("expected prefer=%v, got %v", expectPrefer, ntp.Prefer)
		}
		if ntp.IBurst != expectIBurst {
			return fmt.Errorf("expected iburst=%v, got %v", expectIBurst, ntp.IBurst)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Direct API verification: check NTP global config on device
// ---------------------------------------------------------------------------

func testAccCheckNTPGlobalConfigOnDevice(expectService, expectAuth bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newNtpClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create F5OS client: %w", err)
		}
		service, auth, err := client.GetNTPGlobalConfig()
		if err != nil {
			return fmt.Errorf("failed to read NTP global config from device: %w", err)
		}
		if service != expectService {
			return fmt.Errorf("expected ntp_service=%v on device, got %v", expectService, service)
		}
		if auth != expectAuth {
			return fmt.Errorf("expected ntp_authentication=%v on device, got %v", expectAuth, auth)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// CheckDestroy: verify test NTP server was removed from device
// ---------------------------------------------------------------------------

func testAccCheckNTPServerDestroy(s *terraform.State) error {
	client, err := newNtpClientFromEnv()
	if err != nil {
		// Cannot connect — treat as destroyed
		return nil
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_ntp_server" {
			continue
		}
		server := rs.Primary.Attributes["server"]
		if server == "" {
			continue
		}
		ntp, err := client.GetNTPServer(server)
		if err == nil && ntp != nil {
			return fmt.Errorf("NTP server %s still exists on device after destroy", server)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Unit test (mock server)
// ---------------------------------------------------------------------------

func TestUnitF5osNTPServerResource(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock: POST to create NTP server
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	// Mock: GET/PATCH/DELETE for specific NTP server
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers/server=10.20.30.40", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:server": [
					{
						"address": "10.20.30.40",
						"config": {
							"address": "10.20.30.40",
							"f5-openconfig-system-ntp:key-id": 123,
							"prefer": true,
							"iburst": true
						}
					}
				]
			}`))
		case "PATCH":
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Mock: GET/PATCH for global NTP config
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			w.WriteHeader(http.StatusNoContent)
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:config": {
					"enabled": true,
					"enable-ntp-auth": true
				}
			}`))
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitNTPServerBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "server", "10.20.30.40"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "key_id", "123"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "prefer", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "iburst", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_authentication", "true"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test (real device)
// ---------------------------------------------------------------------------

func TestAccF5osNTPServerResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNTPServerDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create and verify
			{
				Config: testAccNTPServerBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Terraform state checks
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "server", "10.255.255.1"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "key_id", "0"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "prefer", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "iburst", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_authentication", "true"),
					// Direct device API verification
					testAccCheckNTPServerOnDevice("10.255.255.1", 0, true, true),
					testAccCheckNTPGlobalConfigOnDevice(true, true),
				),
			},
			// Step 2: Update and verify
			{
				Config: testAccNTPServerUpdatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "server", "10.255.255.1"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "key_id", "0"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "prefer", "false"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "iburst", "false"),
					// Direct device API verification
					testAccCheckNTPServerOnDevice("10.255.255.1", 0, false, false),
				),
			},
			// Step 3: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}
