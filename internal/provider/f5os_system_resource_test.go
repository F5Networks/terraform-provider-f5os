package provider

import (
	"encoding/json"
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
// Unit test: verifies Read populates sshd_ciphers, sshd_kex_alg, sshd_mac_alg,
// sshd_hkey_alg from the device API response (not from stale state).
// ---------------------------------------------------------------------------

const testUnitSystemCreateConfig = `
resource "f5os_system" "test" {
  hostname          = "unit-test-host"
  motd              = "unit test motd"
  login_banner      = "unit test banner"
  timezone          = "UTC"
  cli_timeout       = 3600
  token_lifetime    = 15
  sshd_idle_timeout = "1800"
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers      = ["aes256-ctr", "aes256-gcm@openssh.com"]
  sshd_kex_alg      = ["ecdh-sha2-nistp384"]
  sshd_mac_alg      = ["hmac-sha2-256"]
  sshd_hkey_alg     = ["ssh-ed25519"]
}
`

func TestUnitSystemReadPopulatesSSHLists(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock: PATCH /openconfig-system:system (Create/Update system config)
	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Mock: PATCH /openconfig-system:system/aaa (token lifetime)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Mock: PATCH /openconfig-system:system/f5-system-settings:settings
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			w.WriteHeader(http.StatusNoContent)
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-system-settings:settings": {
					"config": {
						"idle-timeout": "3600",
						"sshd-idle-timeout": "1800"
					}
				}
			}`))
		}
	})

	// Mock: GET /openconfig-system:system/config (Read hostname/motd/login-banner)
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"openconfig-system:config": {
				"hostname": "unit-test-host",
				"motd-banner": "unit test motd",
				"login-banner": "unit test banner"
			}
		}`))
	})

	// Mock: GET /openconfig-system:system/clock (Read timezone)
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"openconfig-system:clock": {
				"config": {
					"timezone-name": "UTC"
				}
			}
		}`))
	})

	// Mock: GET /openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})

	// Mock: cipher services — returns BOTH httpd and sshd blocks.
	// The sshd block values are what Read should populate into state.
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-security-ciphers:service": [
				{
					"name": "httpd",
					"config": {
						"name": "httpd",
						"ssl-ciphersuite": "ECDHE-RSA-AES256-GCM-SHA384"
					}
				},
				{
					"name": "sshd",
					"config": {
						"name": "sshd",
						"ciphers": ["aes256-ctr", "aes256-gcm@openssh.com"],
						"kexalgorithms": ["ecdh-sha2-nistp384"],
						"macs": ["hmac-sha2-256"],
						"host-key-algorithms": ["ssh-ed25519"]
					}
				}
			]
		}`))
	})

	// Mock: PUT endpoints for cipher/SSH config (Create writes)
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=httpd/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/ciphers", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/kexalgorithms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/macs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/host-key-algorithms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitSystemCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Scalar attributes — confirm Read populated them
					resource.TestCheckResourceAttr("f5os_system.test", "hostname", "unit-test-host"),
					resource.TestCheckResourceAttr("f5os_system.test", "motd", "unit test motd"),
					resource.TestCheckResourceAttr("f5os_system.test", "login_banner", "unit test banner"),
					resource.TestCheckResourceAttr("f5os_system.test", "timezone", "UTC"),
					resource.TestCheckResourceAttr("f5os_system.test", "cli_timeout", "3600"),
					resource.TestCheckResourceAttr("f5os_system.test", "token_lifetime", "15"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.test", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),

					// SSH list attributes — the fix under test.
					// These must be populated FROM the mock API response,
					// not preserved from old state. Before the fix,
					// ElementsAs was used backwards and these would be
					// empty/stale after Read.
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_ciphers.#", "2"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_ciphers.1", "aes256-gcm@openssh.com"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_kex_alg.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_mac_alg.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_mac_alg.0", "hmac-sha2-256"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_hkey_alg.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.test", "sshd_hkey_alg.0", "ssh-ed25519"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests (real device)
// ---------------------------------------------------------------------------

func TestAccSystemCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccSystemCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Todays weather is great!"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Welcome to the system."),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "UTC"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3600"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "15"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.1", "aes256-gcm@openssh.com"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_mac_alg.0", "hmac-sha1-96"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_hkey_alg.0", "ssh-rsa"),
				),
			},
			// ImportState testing
			// {
			// 	ResourceName:      "f5os_system.system_settings",
			// 	ImportState:       true,
			// 	ImportStateVerify: true,
			// },
		},
	})
}

func TestAccSystemUpdateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccSystemCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Todays weather is great!"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Welcome to the system."),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "UTC"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3600"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "15"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.1", "aes256-gcm@openssh.com"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_mac_alg.0", "hmac-sha1-96"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_hkey_alg.0", "ssh-rsa"),
				),
			},
			{
				Config: testAccSystemUpdateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Todays weather is great Update!"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Welcome to the updated system."),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "Poland"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3500"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "16"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.1", "ecdh-sha2-nistp521"),
				),
			},
		},
	})
}

// func TestAccSystemCreateUnitTC1Resource(t *testing.T) {
// 	testAccPreUnitCheck(t)

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method == "GET" {
// 			assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
// 			w.Header().Set("Content-Type", "application/yang-data+json")
// 			w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
// 			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
// 		}
// 		if r.Method == "PATCH" {
// 			w.WriteHeader(http.StatusOK)
// 			_, _ = fmt.Fprintf(w, ``)
// 		}
// 		count++
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusOK)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=httpd/config", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/ciphers", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/kexalgorithms", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/macs", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/host-key-algorithms", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusOK)
// 		_, _ = fmt.Fprintf(w, `{
// 			"openconfig-system:config": {
// 				"hostname": "system.example.net",
// 				"login-banner": "Welcome to the system.",
// 				"motd-banner": "Todays weather is great!"
// 			}
// 		}`,
// 		)
// 	})
// 	defer teardown()
// 	resource.Test(t, resource.TestCase{
// 		IsUnitTest:               true,
// 		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
// 		Steps: []resource.TestStep{
// 			// Read testing
// 			{
// 				Config: testAccSystemCreateResourceConfig,
// 				Check: resource.ComposeAggregateTestCheckFunc(
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system.example.net"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Todays weather is great!"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Welcome to the system."),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "UTC"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3600"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "15"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_idle_timeout", "1800"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.1", "aes256-gcm@openssh.com"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_mac_alg.0", "hmac-sha1-96"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_hkey_alg.0", "ssh-rsa"),
// 				),
// 			},
// 		},
// 	})
// }

const testAccSystemCreateResourceConfig = `
resource "f5os_system" "system_settings" {
  hostname = "system.example.net"
  motd = "Todays weather is great!"
  login_banner = "Welcome to the system."
  timezone = "UTC"
  cli_timeout = 3600
  token_lifetime = 15
  sshd_idle_timeout = 1800
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers = ["aes256-ctr", "aes256-gcm@openssh.com"]
  sshd_kex_alg = ["ecdh-sha2-nistp384"]
  sshd_mac_alg = ["hmac-sha1-96"]
  sshd_hkey_alg = ["ssh-rsa"]
}`

const testAccSystemUpdateResourceConfig = `
resource "f5os_system" "system_settings" {
  hostname = "system.example.net"
  motd = "Todays weather is great Update!"
  login_banner = "Welcome to the updated system."
  timezone = "Poland"
  cli_timeout = 3500
  token_lifetime = 16
  sshd_idle_timeout = 1800
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers = ["aes256-ctr"]
  sshd_kex_alg = ["ecdh-sha2-nistp384", "ecdh-sha2-nistp521"]
  sshd_mac_alg = ["hmac-sha1-96"]
  sshd_hkey_alg = ["ssh-rsa"]
}`

// ---------------------------------------------------------------------------
// Acceptance test helpers
// ---------------------------------------------------------------------------

// newSystemClientFromEnv creates a fresh f5osclient session from environment
// variables. Port defaults to 8888 to match the provider.
func newSystemClientFromEnv() (*f5ossdk.F5os, error) {
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
	cfg := &f5ossdk.F5osConfig{
		Host:             host,
		User:             user,
		Password:         pass,
		Port:             port,
		DisableSSLVerify: true,
	}
	return f5ossdk.NewSession(cfg)
}

// testAccCheckSystemSSHListsOnDevice queries the device cipher service
// endpoints directly and verifies the sshd ciphers, kex, macs, and hkey
// lists match the expected values.
func testAccCheckSystemSSHListsOnDevice(
	expectedCiphers []string,
	expectedKex []string,
	expectedMacs []string,
	expectedHkey []string,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newSystemClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		data, err := client.GetRequest("/openconfig-system:system/f5-security-ciphers:security/services/service")
		if err != nil {
			return fmt.Errorf("failed to read cipher services: %w", err)
		}

		var raw map[string][]map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("failed to parse cipher services: %w", err)
		}

		for _, svc := range raw["f5-security-ciphers:service"] {
			if svc["name"] != "sshd" {
				continue
			}
			cfg, ok := svc["config"].(map[string]any)
			if !ok {
				return fmt.Errorf("sshd config is not a map")
			}

			if err := compareStringList(cfg, "ciphers", expectedCiphers); err != nil {
				return err
			}
			if err := compareStringList(cfg, "kexalgorithms", expectedKex); err != nil {
				return err
			}
			if err := compareStringList(cfg, "macs", expectedMacs); err != nil {
				return err
			}
			if err := compareStringList(cfg, "host-key-algorithms", expectedHkey); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("sshd service not found on device")
	}
}

// compareStringList compares a JSON array field against expected string values.
func compareStringList(cfg map[string]any, key string, expected []string) error {
	raw, ok := cfg[key]
	if !ok || raw == nil {
		if len(expected) == 0 {
			return nil
		}
		return fmt.Errorf("device %s: expected %v, got (absent)", key, expected)
	}
	arr, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("device %s: expected array, got %T", key, raw)
	}
	if len(arr) != len(expected) {
		return fmt.Errorf("device %s: expected %d entries, got %d", key, len(expected), len(arr))
	}
	for i, v := range arr {
		s, _ := v.(string)
		if s != expected[i] {
			return fmt.Errorf("device %s[%d]: expected %q, got %q", key, i, expected[i], s)
		}
	}
	return nil
}

// testAccCheckSystemHostnameOnDevice verifies the hostname on the device.
func testAccCheckSystemHostnameOnDevice(expected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newSystemClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		data, err := client.GetRequest("/openconfig-system:system/config")
		if err != nil {
			return fmt.Errorf("failed to read system config: %w", err)
		}
		var resp f5ossdk.F5ResSystemConfig
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("failed to parse system config: %w", err)
		}
		if resp.OpenConfigSystem.Hostname != expected {
			return fmt.Errorf("hostname: expected %q, got %q", expected, resp.OpenConfigSystem.Hostname)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Acceptance test: verifies Read populates SSH lists from the device after
// Create, using direct API verification alongside Terraform state checks.
// ---------------------------------------------------------------------------

// Uses a safe cipher configuration that preserves device connectivity:
// - sshd_ciphers includes aes256-ctr + aes256-gcm (broadly supported)
// - sshd_kex_alg includes ecdh-sha2-nistp256 + nistp384 (broadly supported)
// - sshd_hkey_alg includes all 4 baseline algorithms (no reduction)
// - sshd_mac_alg includes hmac-sha1-96 (matches baseline)
const testAccSystemReadSSHListsConfig = `
resource "f5os_system" "read_test" {
  hostname          = "r5900-read-test"
  motd              = "Read SSH lists test"
  login_banner      = "Read test banner"
  timezone          = "UTC"
  cli_timeout       = 3500
  token_lifetime    = 16
  sshd_idle_timeout = "1800"
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers      = ["aes256-ctr", "aes256-gcm@openssh.com"]
  sshd_kex_alg      = ["ecdh-sha2-nistp256", "ecdh-sha2-nistp384"]
  sshd_mac_alg      = ["hmac-sha1-96"]
  sshd_hkey_alg     = ["ecdsa-sha2-nistp256", "rsa-sha2-256", "rsa-sha2-512", "ssh-ed25519"]
}
`

func TestAccSystemReadPopulatesSSHLists(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemReadSSHListsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// --- Terraform state assertions ---
					resource.TestCheckResourceAttr("f5os_system.read_test", "hostname", "r5900-read-test"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "motd", "Read SSH lists test"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "login_banner", "Read test banner"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "timezone", "UTC"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "cli_timeout", "3500"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "token_lifetime", "16"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),

					// SSH lists — the fix under test. Before the fix
					// these would be stale because ElementsAs was used
					// backwards. After the fix, Read populates them from
					// the device via types.ListValueFrom.
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_ciphers.#", "2"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_ciphers.1", "aes256-gcm@openssh.com"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_kex_alg.#", "2"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_kex_alg.0", "ecdh-sha2-nistp256"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_kex_alg.1", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_mac_alg.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_mac_alg.0", "hmac-sha1-96"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_hkey_alg.#", "4"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_hkey_alg.0", "ecdsa-sha2-nistp256"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_hkey_alg.1", "rsa-sha2-256"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_hkey_alg.2", "rsa-sha2-512"),
					resource.TestCheckResourceAttr("f5os_system.read_test", "sshd_hkey_alg.3", "ssh-ed25519"),

					// --- Direct device API assertions ---
					testAccCheckSystemHostnameOnDevice("r5900-read-test"),
					testAccCheckSystemSSHListsOnDevice(
						[]string{"aes256-ctr", "aes256-gcm@openssh.com"},
						[]string{"ecdh-sha2-nistp256", "ecdh-sha2-nistp384"},
						[]string{"hmac-sha1-96"},
						[]string{"ecdsa-sha2-nistp256", "rsa-sha2-256", "rsa-sha2-512", "ssh-ed25519"},
					),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: verifies no panic when SshdIdleTimeout is nil (never configured)
// ---------------------------------------------------------------------------

func TestUnitSystemCreateNoSshdIdleTimeoutTC3Resource(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
		}
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
		}
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"system-no-sshd.example.net","login-banner":"Welcome.","motd-banner":"Hello"}}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{}}]}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemCreateNoSshdIdleTimeoutConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system-no-sshd.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "hostname", "system-no-sshd.example.net"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "sshd_idle_timeout"),
				),
			},
		},
	})
}

const testAccSystemCreateNoSshdIdleTimeoutConfig = `
resource "f5os_system" "system_settings" {
  hostname = "system-no-sshd.example.net"
  motd = "Hello"
  login_banner = "Welcome."
  timezone = "UTC"
}`

// ---------------------------------------------------------------------------
// Unit test: verifies SshdIdleTimeout triggers a settings PATCH when configured
// ---------------------------------------------------------------------------

func TestUnitSystemSshdIdleTimeoutTriggersPatch(t *testing.T) {
	testAccPreUnitCheck(t)

	var settingsPatchCalled bool

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
		}
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			settingsPatchCalled = true
			w.WriteHeader(http.StatusNoContent)
		}
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{"sshd-idle-timeout":"1800"}}}`))
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"sshd-patch-test.example.net","login-banner":"Welcome.","motd-banner":"Hello"}}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{}}]}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemSshdIdleTimeoutConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "sshd-patch-test.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_idle_timeout", "1800"),
					func(s *terraform.State) error {
						if !settingsPatchCalled {
							return fmt.Errorf("expected PATCH to /f5-system-settings:settings but it was not called")
						}
						return nil
					},
				),
			},
		},
	})
}

const testAccSystemSshdIdleTimeoutConfig = `
resource "f5os_system" "system_settings" {
  hostname          = "sshd-patch-test.example.net"
  motd              = "Hello"
  login_banner      = "Welcome."
  timezone          = "UTC"
  sshd_idle_timeout = 1800
}`

// ---------------------------------------------------------------------------
// Unit test: import populates all optional fields from device
// ---------------------------------------------------------------------------

func TestUnitSystemImportPopulatesOptionalFields(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
		}
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{"idle-timeout":3600,"sshd-idle-timeout":"1800"}}}`))
		}
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"import-test.example.net","login-banner":"Imported banner","motd-banner":"Imported motd"}}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"America/Los_Angeles"}}}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{"ssl-ciphersuite":"ECDHE-RSA-AES256-GCM-SHA384"}},{"name":"sshd","config":{"ciphers":["aes256-ctr"],"kexalgorithms":["ecdh-sha2-nistp384"],"macs":["hmac-sha2-256"],"host-key-algorithms":["ssh-ed25519"]}}]}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=httpd/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/ciphers", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/kexalgorithms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/macs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/host-key-algorithms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 20}`))
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the resource so it exists in state
			{
				Config: testAccSystemImportConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "hostname", "import-test.example.net"),
				),
			},
			// Step 2: Import and verify all optional fields are populated
			{
				ResourceName:      "f5os_system.system_settings",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "import-test.example.net",
				Check: resource.ComposeAggregateTestCheckFunc(
					// Required fields
					resource.TestCheckResourceAttr("f5os_system.system_settings", "hostname", "import-test.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Imported motd"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Imported banner"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "America/Los_Angeles"),
					// Optional fields — must be populated from device during import
					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3600"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "20"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_mac_alg.0", "hmac-sha2-256"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_hkey_alg.0", "ssh-ed25519"),
				),
			},
		},
	})
}

const testAccSystemImportConfig = `
resource "f5os_system" "system_settings" {
  hostname          = "import-test.example.net"
  motd              = "Imported motd"
  login_banner      = "Imported banner"
  timezone          = "America/Los_Angeles"
  cli_timeout       = 3600
  token_lifetime    = 20
  sshd_idle_timeout = 1800
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers      = ["aes256-ctr"]
  sshd_kex_alg      = ["ecdh-sha2-nistp384"]
  sshd_mac_alg      = ["hmac-sha2-256"]
  sshd_hkey_alg     = ["ssh-ed25519"]
}`

// ---------------------------------------------------------------------------
// Unit test: minimal config with no optional fields — verifies no errors
// during Create, Read, and Delete when optional attributes are omitted.
// ---------------------------------------------------------------------------

func TestUnitSystemMinimalConfigNoOptionalFields(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/yang-data+json")
			w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
		}
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
		}
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusNoContent)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"minimal-test.example.net","login-banner":"Welcome.","motd-banner":"Hello"}}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{}}]}`))
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSystemMinimalConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "minimal-test.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "hostname", "minimal-test.example.net"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "cli_timeout"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "token_lifetime"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "sshd_idle_timeout"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "httpd_ciphersuite"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "sshd_ciphers"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "sshd_kex_alg"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "sshd_mac_alg"),
					resource.TestCheckNoResourceAttr("f5os_system.system_settings", "sshd_hkey_alg"),
				),
			},
		},
	})
}

const testAccSystemMinimalConfig = `
resource "f5os_system" "system_settings" {
  hostname     = "minimal-test.example.net"
  motd         = "Hello"
  login_banner = "Welcome."
  timezone     = "UTC"
}`
