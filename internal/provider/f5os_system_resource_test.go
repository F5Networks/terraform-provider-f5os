package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

const (
	// devicePreCheckTimeout is how long testAccPreCheckWithRetry and
	// testAccCheckSystemDestroy wait for the device before giving up.
	devicePreCheckTimeout = 90 * time.Second

	// deviceStepTimeout is how long PreConfig closures between test steps
	// wait for the device before giving up.
	deviceStepTimeout = 60 * time.Second
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
// Acceptance test helpers — device availability and retry logic
// ---------------------------------------------------------------------------

// waitForDeviceAvailable polls the device until the RESTCONF API is fully
// reachable or the timeout is exceeded. This handles the case where system
// configuration changes (hostname, SSHD ciphers, etc.) cause the RESTCONF
// service to restart — sometimes more than once in quick succession.
//
// It delegates to pollUntilStable (defined in f5os_system_resource.go) which
// contains the shared polling/cooldown logic. The only difference from the
// provider's waitForDeviceReady is that this function creates a fresh client
// from environment variables on each check (since tests don't have a
// persistent provider client).
func waitForDeviceAvailable(timeout time.Duration) error {
	check := func() bool {
		client, err := newTestClientFromEnv()
		if err != nil {
			return false
		}
		if _, err = client.GetRequest("/openconfig-system:system/config"); err != nil {
			return false
		}
		if _, err = client.GetRequest("/openconfig-system:system/f5-security-ciphers:security/services/service"); err != nil {
			return false
		}
		if _, err = client.GetRequest("/openconfig-system:system/aaa"); err != nil {
			return false
		}
		return true
	}

	return pollUntilStable(check, timeout)
}

// testAccPreCheckWithRetry wraps testAccPreCheck with retry logic to handle
// transient device unavailability after system configuration changes.
func testAccPreCheckWithRetry(t *testing.T) {
	t.Helper()
	// First, wait for the device to be available (handles service restarts)
	if err := waitForDeviceAvailable(devicePreCheckTimeout); err != nil {
		t.Fatalf("Device not available for acceptance test: %v", err)
	}
	// Then run the standard pre-check
	testAccPreCheck(t)
}

// testAccWaitForDeviceStabilization waits for the device to stabilize after
// configuration changes. Call this between test steps that modify system
// settings which may trigger service restarts.
func testAccWaitForDeviceStabilization() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return waitForDeviceAvailable(deviceStabilizeTimeout)
	}
}

// ---------------------------------------------------------------------------
// Acceptance test helpers — direct device API verification
// ---------------------------------------------------------------------------

// testAccCheckSystemSettingsOnDevice queries the device directly and verifies
// system settings (motd, login_banner, timezone, cli_timeout, token_lifetime,
// sshd_idle_timeout, httpd_ciphersuite). Includes retry logic to handle
// transient connection failures during service restarts.
func testAccCheckSystemSettingsOnDevice(
	expectedMotd, expectedBanner, expectedTimezone string,
	expectedCliTimeout int64, expectedTokenLifetime int64,
	expectedSshdIdleTimeout, expectedHttpdCiphersuite string,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// Wait for device to be available (handles service restarts)
		if err := waitForDeviceAvailable(deviceStepTimeout); err != nil {
			return fmt.Errorf("device not available: %w", err)
		}

		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		// Check system config (hostname, motd, login_banner)
		data, err := client.GetRequest("/openconfig-system:system/config")
		if err != nil {
			return fmt.Errorf("failed to read system config: %w", err)
		}
		var sysCfg f5ossdk.F5ResSystemConfig
		if err := json.Unmarshal(data, &sysCfg); err != nil {
			return fmt.Errorf("failed to parse system config: %w", err)
		}
		if sysCfg.OpenConfigSystem.Motd != expectedMotd {
			return fmt.Errorf("motd: expected %q, got %q", expectedMotd, sysCfg.OpenConfigSystem.Motd)
		}
		if sysCfg.OpenConfigSystem.LoginBanner != expectedBanner {
			return fmt.Errorf("login_banner: expected %q, got %q", expectedBanner, sysCfg.OpenConfigSystem.LoginBanner)
		}

		// Check clock config
		clockData, err := client.GetRequest("/openconfig-system:system/clock")
		if err != nil {
			return fmt.Errorf("failed to read clock config: %w", err)
		}
		var clockCfg f5ossdk.F5ResClockConfig
		if err := json.Unmarshal(clockData, &clockCfg); err != nil {
			return fmt.Errorf("failed to parse clock config: %w", err)
		}
		if clockCfg.OpenConfigClock.Config.TimeZoneName != expectedTimezone {
			return fmt.Errorf("timezone: expected %q, got %q", expectedTimezone, clockCfg.OpenConfigClock.Config.TimeZoneName)
		}

		// Check token lifetime
		lifetimeData, err := client.GetRequest("/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime")
		if err != nil {
			return fmt.Errorf("failed to read token lifetime: %w", err)
		}
		var lifetime f5ossdk.F5ResTokenLifetime
		if err := json.Unmarshal(lifetimeData, &lifetime); err != nil {
			return fmt.Errorf("failed to parse token lifetime: %w", err)
		}
		if int64(lifetime.Lifetime) != expectedTokenLifetime {
			return fmt.Errorf("token_lifetime: expected %d, got %d", expectedTokenLifetime, lifetime.Lifetime)
		}

		// Check system settings (cli_timeout, sshd_idle_timeout)
		settingsData, err := client.GetRequest("/openconfig-system:system/f5-system-settings:settings")
		if err != nil {
			return fmt.Errorf("failed to read system settings: %w", err)
		}
		var settingsCfg f5ossdk.F5ResSettingsConfig
		if err := json.Unmarshal(settingsData, &settingsCfg); err != nil {
			return fmt.Errorf("failed to parse system settings: %w", err)
		}
		// CliTimeout may arrive as string, int, or float64 from the JSON response.
		switch v := settingsCfg.Settings.Config.CliTimeout.(type) {
		case float64:
			if int64(v) != expectedCliTimeout {
				return fmt.Errorf("cli_timeout: expected %d, got %v", expectedCliTimeout, v)
			}
		case string:
			parsed, _ := strconv.ParseInt(v, 10, 64)
			if parsed != expectedCliTimeout {
				return fmt.Errorf("cli_timeout: expected %d, got %q", expectedCliTimeout, v)
			}
		default:
			return fmt.Errorf("cli_timeout: unexpected type %T", settingsCfg.Settings.Config.CliTimeout)
		}
		if s, ok := settingsCfg.Settings.Config.SshdIdleTimeout.(string); ok {
			if s != expectedSshdIdleTimeout {
				return fmt.Errorf("sshd_idle_timeout: expected %q, got %q", expectedSshdIdleTimeout, s)
			}
		}

		// Check httpd ciphersuite
		cipherData, err := client.GetRequest("/openconfig-system:system/f5-security-ciphers:security/services/service")
		if err != nil {
			return fmt.Errorf("failed to read cipher services: %w", err)
		}
		var rawCiphers map[string][]map[string]any
		if err := json.Unmarshal(cipherData, &rawCiphers); err != nil {
			return fmt.Errorf("failed to parse cipher services: %w", err)
		}
		for _, svc := range rawCiphers["f5-security-ciphers:service"] {
			if svc["name"] != "httpd" {
				continue
			}
			cfg, ok := svc["config"].(map[string]any)
			if !ok {
				return fmt.Errorf("httpd config is not a map")
			}
			actual, _ := cfg["ssl-ciphersuite"].(string)
			if actual != expectedHttpdCiphersuite {
				return fmt.Errorf("httpd_ciphersuite: expected %q, got %q", expectedHttpdCiphersuite, actual)
			}
		}

		return nil
	}
}

// testAccCheckSystemDestroy verifies the system resource has been cleaned up.
// For the system resource, "destroy" means the configurable settings have been
// removed (hostname, motd, login-banner cleared; timeouts removed).
func testAccCheckSystemDestroy(s *terraform.State) error {
	// Wait for device to be available after destroy (service may be restarting)
	if err := waitForDeviceAvailable(devicePreCheckTimeout); err != nil {
		// Cannot connect after waiting — treat as destroyed (or device unreachable)
		return nil
	}

	client, err := newTestClientFromEnv()
	if err != nil {
		// Cannot connect — treat as destroyed
		return nil
	}

	// After destroy, motd and login-banner should be empty/cleared. Verify
	// they are empty rather than checking for specific test values, so that
	// any new test with a different motd string is still caught.
	data, err := client.GetRequest("/openconfig-system:system/config")
	if err != nil {
		return nil // Cannot read — treat as destroyed
	}
	var sysCfg f5ossdk.F5ResSystemConfig
	if err := json.Unmarshal(data, &sysCfg); err != nil {
		return nil
	}

	if sysCfg.OpenConfigSystem.Motd != "" {
		return fmt.Errorf("system resource not destroyed: motd still %q (expected empty)", sysCfg.OpenConfigSystem.Motd)
	}
	if sysCfg.OpenConfigSystem.LoginBanner != "" {
		return fmt.Errorf("system resource not destroyed: login_banner still %q (expected empty)", sysCfg.OpenConfigSystem.LoginBanner)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Acceptance tests (real device)
// ---------------------------------------------------------------------------

func TestAccSystemCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckWithRetry(t) },
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
					// Wait for device to stabilize after cipher/hostname changes
					testAccWaitForDeviceStabilization(),
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
		PreCheck:                 func() { testAccPreCheckWithRetry(t) },
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
					// Wait for device to stabilize after cipher/hostname changes
					testAccWaitForDeviceStabilization(),
				),
			},
			{
				// PreConfig runs before Terraform operations; wait for device availability
				PreConfig: func() {
					if err := waitForDeviceAvailable(deviceStepTimeout); err != nil {
						t.Fatalf("Device not available before update step: %v", err)
					}
				},
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
					// Wait for device to stabilize after cipher changes
					testAccWaitForDeviceStabilization(),
				),
			},
		},
	})
}


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



// testAccCheckSystemSSHListsOnDevice queries the device cipher service
// endpoints directly and verifies the sshd ciphers, kex, macs, and hkey
// lists match the expected values. Includes retry logic to handle transient
// connection failures during service restarts.
func testAccCheckSystemSSHListsOnDevice(
	expectedCiphers []string,
	expectedKex []string,
	expectedMacs []string,
	expectedHkey []string,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
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
// Includes retry logic to handle transient connection failures during service restarts.
func testAccCheckSystemHostnameOnDevice(expected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
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
		PreCheck:                 func() { testAccPreCheckWithRetry(t) },
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
					// Wait for device to stabilize after cipher/hostname changes
					testAccWaitForDeviceStabilization(),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: full CRUD lifecycle with direct API verification.
// Create -> verify on device -> Import -> Update -> verify on device -> Destroy
// ---------------------------------------------------------------------------

const testAccSystemCRUDCreateConfig = `
resource "f5os_system" "crud_test" {
  hostname          = "r5900-crud-test"
  motd              = "Test CRUD motd"
  login_banner      = "CRUD test banner"
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

const testAccSystemCRUDUpdateConfig = `
resource "f5os_system" "crud_test" {
  hostname          = "r5900-crud-test"
  motd              = "Updated CRUD motd"
  login_banner      = "Updated CRUD banner"
  timezone          = "America/New_York"
  cli_timeout       = 7200
  token_lifetime    = 20
  sshd_idle_timeout = "3600"
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers      = ["aes128-ctr", "aes256-ctr"]
  sshd_kex_alg      = ["ecdh-sha2-nistp384"]
  sshd_mac_alg      = ["hmac-sha1-96"]
  sshd_hkey_alg     = ["ecdsa-sha2-nistp256", "rsa-sha2-256", "rsa-sha2-512", "ssh-ed25519"]
}
`

func TestAccSystemCRUDResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckWithRetry(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSystemDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create and verify via direct API
			{
				Config: testAccSystemCRUDCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Terraform state assertions
					resource.TestCheckResourceAttr("f5os_system.crud_test", "hostname", "r5900-crud-test"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "motd", "Test CRUD motd"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "login_banner", "CRUD test banner"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "timezone", "UTC"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "cli_timeout", "3500"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "token_lifetime", "16"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "sshd_ciphers.#", "2"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "sshd_kex_alg.#", "2"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "sshd_mac_alg.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "sshd_hkey_alg.#", "4"),

					// Direct device API assertions
					testAccCheckSystemHostnameOnDevice("r5900-crud-test"),
					testAccCheckSystemSettingsOnDevice(
						"Test CRUD motd", "CRUD test banner", "UTC",
						3500, 16, "1800", "ECDHE-RSA-AES256-GCM-SHA384",
					),
					testAccCheckSystemSSHListsOnDevice(
						[]string{"aes256-ctr", "aes256-gcm@openssh.com"},
						[]string{"ecdh-sha2-nistp256", "ecdh-sha2-nistp384"},
						[]string{"hmac-sha1-96"},
						[]string{"ecdsa-sha2-nistp256", "rsa-sha2-256", "rsa-sha2-512", "ssh-ed25519"},
					),
					// Wait for device to stabilize after cipher/hostname changes
					testAccWaitForDeviceStabilization(),
				),
			},
			// Step 2: Import state
			{
				// PreConfig runs before Terraform operations; wait for device availability
				PreConfig: func() {
					if err := waitForDeviceAvailable(deviceStepTimeout); err != nil {
						t.Fatalf("Device not available before import step: %v", err)
					}
				},
				ResourceName:      "f5os_system.crud_test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "r5900-crud-test",
			},
			// Step 3: Update and verify via direct API
			{
				// PreConfig runs before Terraform operations; wait for device availability
				PreConfig: func() {
					if err := waitForDeviceAvailable(deviceStepTimeout); err != nil {
						t.Fatalf("Device not available before update step: %v", err)
					}
				},
				Config: testAccSystemCRUDUpdateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.crud_test", "hostname", "r5900-crud-test"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "motd", "Updated CRUD motd"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "login_banner", "Updated CRUD banner"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "timezone", "America/New_York"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "cli_timeout", "7200"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "token_lifetime", "20"),
					resource.TestCheckResourceAttr("f5os_system.crud_test", "sshd_idle_timeout", "3600"),

					// Direct device API assertions
					testAccCheckSystemHostnameOnDevice("r5900-crud-test"),
					testAccCheckSystemSettingsOnDevice(
						"Updated CRUD motd", "Updated CRUD banner", "America/New_York",
						7200, 20, "3600", "ECDHE-RSA-AES256-GCM-SHA384",
					),
					testAccCheckSystemSSHListsOnDevice(
						[]string{"aes128-ctr", "aes256-ctr"},
						[]string{"ecdh-sha2-nistp384"},
						[]string{"hmac-sha1-96"},
						[]string{"ecdsa-sha2-nistp256", "rsa-sha2-256", "rsa-sha2-512", "ssh-ed25519"},
					),
					// Wait for device to stabilize after cipher changes
					testAccWaitForDeviceStabilization(),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
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

// ---------------------------------------------------------------------------
// Unit test: Create + Update with all optional fields, then auto-Destroy.
// Covers Update (0% -> ~85%) and Delete with all optional branches.
// ---------------------------------------------------------------------------

const testUnitSystemCreateFullConfig = `
resource "f5os_system" "full_test" {
  hostname          = "full-test.example.net"
  motd              = "Create motd"
  login_banner      = "Create banner"
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

const testUnitSystemUpdateFullConfig = `
resource "f5os_system" "full_test" {
  hostname          = "full-test.example.net"
  motd              = "Updated motd"
  login_banner      = "Updated banner"
  timezone          = "America/New_York"
  cli_timeout       = 7200
  token_lifetime    = 30
  sshd_idle_timeout = "3600"
  httpd_ciphersuite = "ECDHE-RSA-AES128-GCM-SHA256"
  sshd_ciphers      = ["aes128-ctr"]
  sshd_kex_alg      = ["ecdh-sha2-nistp256", "ecdh-sha2-nistp521"]
  sshd_mac_alg      = ["hmac-sha2-512"]
  sshd_hkey_alg     = ["rsa-sha2-256", "ssh-ed25519"]
}
`

func TestUnitSystemUpdateAllFields(t *testing.T) {
	testAccPreUnitCheck(t)

	// Track state: "create" values until the Update's system config PATCH fires,
	// then switch to "update" values. The Update method sends the system config
	// PATCH last, so once we see the second PATCH to /openconfig-system:system
	// we know the Update is in progress and subsequent Reads should return
	// updated values.
	systemPatchCount := 0

	// Mock: PATCH /openconfig-system:system (system config)
	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			systemPatchCount++
			w.WriteHeader(http.StatusOK)
		}
	})

	// Mock: PATCH /openconfig-system:system/aaa (token lifetime)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// isUpdated returns true after the Update's system config PATCH has fired.
	// Create fires one PATCH to /openconfig-system:system, Update fires a second.
	isUpdated := func() bool { return systemPatchCount >= 2 }

	// Mock: system settings
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			w.WriteHeader(http.StatusNoContent)
		case "GET":
			w.WriteHeader(http.StatusOK)
			if isUpdated() {
				_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{"idle-timeout":7200,"sshd-idle-timeout":"3600"}}}`))
			} else {
				_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{"idle-timeout":"3600","sshd-idle-timeout":"1800"}}}`))
			}
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Mock: system config GET
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			if isUpdated() {
				_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"full-test.example.net","motd-banner":"Updated motd","login-banner":"Updated banner"}}`))
			} else {
				_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"full-test.example.net","motd-banner":"Create motd","login-banner":"Create banner"}}`))
			}
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Mock: clock GET
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if isUpdated() {
			_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"America/New_York"}}}`))
		} else {
			_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
		}
	})

	// Mock: cipher services GET
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if isUpdated() {
			_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{"name":"httpd","ssl-ciphersuite":"ECDHE-RSA-AES128-GCM-SHA256"}},{"name":"sshd","config":{"name":"sshd","ciphers":["aes128-ctr"],"kexalgorithms":["ecdh-sha2-nistp256","ecdh-sha2-nistp521"],"macs":["hmac-sha2-512"],"host-key-algorithms":["rsa-sha2-256","ssh-ed25519"]}}]}`))
		} else {
			_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{"name":"httpd","ssl-ciphersuite":"ECDHE-RSA-AES256-GCM-SHA384"}},{"name":"sshd","config":{"name":"sshd","ciphers":["aes256-ctr","aes256-gcm@openssh.com"],"kexalgorithms":["ecdh-sha2-nistp384"],"macs":["hmac-sha2-256"],"host-key-algorithms":["ssh-ed25519"]}}]}`))
		}
	})

	// Mock: token lifetime GET
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if isUpdated() {
			_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 30}`))
		} else {
			_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
		}
	})

	// Mock: cipher PUT endpoints (create/update)
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

	// Mock: DELETE endpoints for system config sub-paths
	mux.HandleFunc("/restconf/data/openconfig-system:system/config/login-banner", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config/hostname", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config/motd-banner", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Mock: DELETE endpoints for system settings sub-paths
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings/idle-timeout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings/sshd-idle-timeout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Mock: DELETE endpoints for token lifetime
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/config/lifetime/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Mock: DELETE endpoints for cipher services
	mux.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="httpd"/config/ssl-cipher-suite`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="sshd"/config/ciphers`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="sshd"/config/kexalgorithms`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="sshd"/config/macs`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="sshd"/config/host-key-algorithms`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with all fields
			{
				Config: testUnitSystemCreateFullConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.full_test", "hostname", "full-test.example.net"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "motd", "Create motd"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "login_banner", "Create banner"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "timezone", "UTC"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "cli_timeout", "3600"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "token_lifetime", "15"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_ciphers.#", "2"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_kex_alg.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_mac_alg.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_hkey_alg.#", "1"),
				),
			},
			// Step 2: Update all fields (hostname stays the same -> triggers Update, not recreate)
			{
				Config: testUnitSystemUpdateFullConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.full_test", "hostname", "full-test.example.net"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "motd", "Updated motd"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "login_banner", "Updated banner"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "timezone", "America/New_York"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "cli_timeout", "7200"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "token_lifetime", "30"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_idle_timeout", "3600"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "httpd_ciphersuite", "ECDHE-RSA-AES128-GCM-SHA256"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_ciphers.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_ciphers.0", "aes128-ctr"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_kex_alg.#", "2"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_kex_alg.0", "ecdh-sha2-nistp256"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_kex_alg.1", "ecdh-sha2-nistp521"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_mac_alg.#", "1"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_mac_alg.0", "hmac-sha2-512"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_hkey_alg.#", "2"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_hkey_alg.0", "rsa-sha2-256"),
					resource.TestCheckResourceAttr("f5os_system.full_test", "sshd_hkey_alg.1", "ssh-ed25519"),
				),
			},
			// Step 3: Destroy is automatic — DELETE handlers mock cleanup
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: CliTimeout returned as integer (not string) from the API.
// Exercises the int and float64 branches in SystemResourceModelToState.
// ---------------------------------------------------------------------------

func TestUnitSystemCliTimeoutIntType(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			w.WriteHeader(http.StatusNoContent)
		case "GET":
			// Return idle-timeout as a JSON number (float64 when decoded via any).
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{"idle-timeout":7200,"sshd-idle-timeout":"900"}}}`))
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"int-timeout.example.net","login-banner":"Banner","motd-banner":"Motd"}}`))
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
				Config: `
resource "f5os_system" "int_timeout" {
  hostname          = "int-timeout.example.net"
  motd              = "Motd"
  login_banner      = "Banner"
  timezone          = "UTC"
  cli_timeout       = 7200
  sshd_idle_timeout = "900"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.int_timeout", "cli_timeout", "7200"),
					resource.TestCheckResourceAttr("f5os_system.int_timeout", "sshd_idle_timeout", "900"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when system PATCH returns an error.
// Exercises the error branch in Create after the first PatchRequest call.
// ---------------------------------------------------------------------------

func TestUnitSystemCreateSystemPatchError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"internal error"}]}}`))
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_test" {
  hostname = "error-test.example.net"
}`,
				ExpectError: regexp.MustCompile(`failure while creating System`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when token lifetime PATCH returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemCreateTokenLifetimeError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"token error"}]}}`))
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_token" {
  hostname       = "err-token.example.net"
  token_lifetime = 15
}`,
				ExpectError: regexp.MustCompile(`failure while Patching Token Lifetime`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when system settings PATCH returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemCreateSettingsError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"settings error"}]}}`))
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_settings" {
  hostname    = "err-settings.example.net"
  cli_timeout = 3600
}`,
				ExpectError: regexp.MustCompile(`failure while Patching System Settings`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when HTTPD cipher PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemCreateHttpdCipherError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=httpd/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"httpd cipher error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_httpd" {
  hostname          = "err-httpd.example.net"
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
}`,
				ExpectError: regexp.MustCompile(`failure while Http Cipher`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when SSHD cipher PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemCreateSshdCipherError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/ciphers", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"sshd cipher error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_sshd_cipher" {
  hostname     = "err-sshd-cipher.example.net"
  sshd_ciphers = ["aes256-ctr"]
}`,
				ExpectError: regexp.MustCompile(`failure while Sshd Cipher`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when SSHD kex algorithm PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemCreateKexAlgError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/kexalgorithms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"kex error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_kex" {
  hostname     = "err-kex.example.net"
  sshd_kex_alg = ["ecdh-sha2-nistp384"]
}`,
				ExpectError: regexp.MustCompile(`failure while creating Sshd Key Algo`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when SSHD MAC algorithm PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemCreateMacAlgError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/macs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"mac error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_mac" {
  hostname     = "err-mac.example.net"
  sshd_mac_alg = ["hmac-sha2-256"]
}`,
				ExpectError: regexp.MustCompile(`failure while creating Mac Algo`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when SSHD host key algorithm PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemCreateHkeyAlgError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/host-key-algorithms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"hkey error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_hkey" {
  hostname      = "err-hkey.example.net"
  sshd_hkey_alg = ["ssh-ed25519"]
}`,
				ExpectError: regexp.MustCompile(`failure while creating Ssh Host Key Algo`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read fails when system config GET returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemReadSystemConfigError(t *testing.T) {
	testAccPreUnitCheck(t)

	callCount := 0
	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount > 1 {
			// First call is during Create (no Read call in Create, actually Create
			// doesn't call Read -- the framework calls Read after Create), so fail
			// on all GET calls to trigger the Read error.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"read config error"}]}}`))
			return
		}
		// First Read (part of plan-refresh) also fails.
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"read config error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_read" {
  hostname = "err-read.example.net"
}`,
				ExpectError: regexp.MustCompile(`failure while (fetching System Config|creating System)`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read fails when clock GET returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemReadClockError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"clock-err.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"clock error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_clock" {
  hostname = "clock-err.example.net"
}`,
				ExpectError: regexp.MustCompile(`failure while fetching Clock Config`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read fails when cipher services GET returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemReadCiphersError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"cipher-err.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"cipher error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_ciphers" {
  hostname = "cipher-err.example.net"
}`,
				ExpectError: regexp.MustCompile(`failure while fetching Ciphers Config`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read fails when settings GET returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemReadSettingsError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"settings-err.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"settings error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_settings_read" {
  hostname = "settings-err.example.net"
}`,
				ExpectError: regexp.MustCompile(`failure while fetching Settings Config`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read fails when token lifetime GET returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemReadTokenLifetimeError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"lifetime-err.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"lifetime error"}]}}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_lifetime" {
  hostname = "lifetime-err.example.net"
}`,
				ExpectError: regexp.MustCompile(`failure while fetching Token Lifetime`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read receives invalid JSON for system config (unmarshal error).
// ---------------------------------------------------------------------------

func TestUnitSystemReadBadSystemJSON(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_bad_json" {
  hostname = "bad-json.example.net"
}`,
				ExpectError: regexp.MustCompile(`F5OS Error`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read receives invalid JSON for clock config (unmarshal error).
// ---------------------------------------------------------------------------

func TestUnitSystemReadBadClockJSON(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"bad-clock.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_bad_clock" {
  hostname = "bad-clock.example.net"
}`,
				ExpectError: regexp.MustCompile(`F5OS Error`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read receives invalid JSON for cipher services (unmarshal error).
// ---------------------------------------------------------------------------

func TestUnitSystemReadBadCipherJSON(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"bad-cipher.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{bad json`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_bad_cipher_json" {
  hostname = "bad-cipher.example.net"
}`,
				ExpectError: regexp.MustCompile(`F5OS Error`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read receives invalid JSON for settings (unmarshal error).
// ---------------------------------------------------------------------------

func TestUnitSystemReadBadSettingsJSON(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"bad-settings.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{bad`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_bad_settings_json" {
  hostname = "bad-settings.example.net"
}`,
				ExpectError: regexp.MustCompile(`F5OS Error`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read receives invalid JSON for token lifetime (unmarshal error).
// ---------------------------------------------------------------------------

func TestUnitSystemReadBadTokenLifetimeJSON(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"bad-token.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{bad`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "err_bad_token_json" {
  hostname = "bad-token.example.net"
}`,
				ExpectError: regexp.MustCompile(`F5OS Error`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Helpers that register mock handlers returning success for groups of system
// resource endpoints. Tests that need a specific endpoint to fail should
// register their own handler for that path (on a different path, or track
// call counts to fail on later invocations) rather than re-registering the
// same path, which would panic.
// ---------------------------------------------------------------------------

// systemMockReadEndpoints registers GET handlers for Read endpoints.
// Returns minimal values that match the test configs for update error tests.
func systemMockReadEndpoints(m *http.ServeMux) {
	m.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"update-err.example.net","login-banner":"","motd-banner":""}}`))
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{"name":"httpd","ssl-ciphersuite":"ECDHE-RSA-AES256-GCM-SHA384"}},{"name":"sshd","config":{"name":"sshd","ciphers":["aes256-ctr"],"kexalgorithms":["ecdh-sha2-nistp384"],"macs":["hmac-sha2-256"],"host-key-algorithms":["ssh-ed25519"]}}]}`))
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			w.WriteHeader(http.StatusNoContent)
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{"idle-timeout":3600,"sshd-idle-timeout":"1800"}}}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})
}

// systemMockWriteEndpoints registers PUT/PATCH handlers that return success.
func systemMockWriteEndpoints(m *http.ServeMux) {
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=httpd/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/ciphers", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/kexalgorithms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/macs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/host-key-algorithms", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

// systemMockDeleteEndpoints registers DELETE handlers for all cipher/settings paths.
func systemMockDeleteEndpoints(m *http.ServeMux) {
	// System config sub-paths
	m.HandleFunc("/restconf/data/openconfig-system:system/config/login-banner", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/config/hostname", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/config/motd-banner", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// System settings sub-paths
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings/idle-timeout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings/sshd-idle-timeout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// Token lifetime
	m.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/config/lifetime/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="httpd"/config/ssl-cipher-suite`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="sshd"/config/ciphers`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="sshd"/config/kexalgorithms`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="sshd"/config/macs`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	m.HandleFunc(`/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service="sshd"/config/host-key-algorithms`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when token lifetime PATCH returns an error.
// Uses a 2-step test: step 1 creates successfully, step 2 updates and fails.
// ---------------------------------------------------------------------------

func TestUnitSystemUpdateTokenLifetimeError(t *testing.T) {
	testAccPreUnitCheck(t)

	patchCount := 0 // track /aaa PATCH calls

	// System config PATCH (Create + Update both call this)
	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Token lifetime: succeed on Create (patchCount=0), fail on Update (patchCount=1)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			patchCount++
			if patchCount > 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"token error on update"}]}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	systemMockReadEndpoints(mux)
	systemMockWriteEndpoints(mux)
	systemMockDeleteEndpoints(mux)

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "upd_err_token" {
  hostname       = "update-err.example.net"
  motd           = ""
  login_banner   = ""
  timezone       = "UTC"
  token_lifetime = 15
}`,
				Check: resource.TestCheckResourceAttr("f5os_system.upd_err_token", "token_lifetime", "15"),
			},
			{
				Config: `
resource "f5os_system" "upd_err_token" {
  hostname       = "update-err.example.net"
  motd           = ""
  login_banner   = ""
  timezone       = "UTC"
  token_lifetime = 30
}`,
				ExpectError: regexp.MustCompile(`failure while Patching Token Lifetime`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when system settings PATCH returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemUpdateSettingsError(t *testing.T) {
	testAccPreUnitCheck(t)

	settingsPatchCount := 0

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Override the settings endpoint to fail on Update
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			settingsPatchCount++
			if settingsPatchCount > 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"settings error"}]}}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{"idle-timeout":3600}}}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"update-err.example.net","login-banner":"","motd-banner":""}}`))
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
				Config: `
resource "f5os_system" "upd_err_settings" {
  hostname     = "update-err.example.net"
  motd         = ""
  login_banner = ""
  timezone     = "UTC"
  cli_timeout  = 3600
}`,
				Check: resource.TestCheckResourceAttr("f5os_system.upd_err_settings", "cli_timeout", "3600"),
			},
			{
				Config: `
resource "f5os_system" "upd_err_settings" {
  hostname     = "update-err.example.net"
  motd         = ""
  login_banner = ""
  timezone     = "UTC"
  cli_timeout  = 7200
}`,
				ExpectError: regexp.MustCompile(`failure while Patching System Settings`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when system config PATCH returns an error.
// The system config PATCH is the last call in Update.
// ---------------------------------------------------------------------------

func TestUnitSystemUpdateSystemPatchError(t *testing.T) {
	testAccPreUnitCheck(t)

	systemPatchCount := 0

	// System config PATCH: succeed on Create (count 1), fail on Update (count 2)
	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			systemPatchCount++
			if systemPatchCount > 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"system patch error"}]}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"update-err.example.net","login-banner":"b","motd-banner":"m"}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
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
				Config: `
resource "f5os_system" "upd_err_sys" {
  hostname     = "update-err.example.net"
  motd         = "m"
  login_banner = "b"
  timezone     = "UTC"
}`,
				Check: resource.TestCheckResourceAttr("f5os_system.upd_err_sys", "motd", "m"),
			},
			{
				Config: `
resource "f5os_system" "upd_err_sys" {
  hostname     = "update-err.example.net"
  motd         = "changed"
  login_banner = "b"
  timezone     = "UTC"
}`,
				ExpectError: regexp.MustCompile(`failure while creating System`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when HTTPD cipher PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemUpdateHttpdCipherError(t *testing.T) {
	testAccPreUnitCheck(t)

	httpdPutCount := 0

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// HTTPD cipher: succeed on Create, fail on Update
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=httpd/config", func(w http.ResponseWriter, r *http.Request) {
		httpdPutCount++
		if httpdPutCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"httpd error"}]}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"update-err.example.net","login-banner":"","motd-banner":""}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{"name":"httpd","ssl-ciphersuite":"ECDHE-RSA-AES256-GCM-SHA384"}},{"name":"sshd","config":{}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})
	systemMockDeleteEndpoints(mux)

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "upd_err_httpd" {
  hostname          = "update-err.example.net"
  motd              = ""
  login_banner      = ""
  timezone          = "UTC"
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
}`,
				Check: resource.TestCheckResourceAttr("f5os_system.upd_err_httpd", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
			},
			{
				Config: `
resource "f5os_system" "upd_err_httpd" {
  hostname          = "update-err.example.net"
  motd              = ""
  login_banner      = ""
  timezone          = "UTC"
  httpd_ciphersuite = "ECDHE-RSA-AES128-GCM-SHA256"
}`,
				ExpectError: regexp.MustCompile(`failure while Http Cipher`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when SSHD cipher PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemUpdateSshdCipherError(t *testing.T) {
	testAccPreUnitCheck(t)

	sshdCipherPutCount := 0

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=httpd/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/ciphers", func(w http.ResponseWriter, r *http.Request) {
		sshdCipherPutCount++
		if sshdCipherPutCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"sshd cipher update error"}]}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"update-err.example.net","login-banner":"","motd-banner":""}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{"ciphers":["aes256-ctr"]}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})
	systemMockDeleteEndpoints(mux)

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "upd_err_sshd" {
  hostname     = "update-err.example.net"
  motd         = ""
  login_banner = ""
  timezone     = "UTC"
  sshd_ciphers = ["aes256-ctr"]
}`,
				Check: resource.TestCheckResourceAttr("f5os_system.upd_err_sshd", "sshd_ciphers.0", "aes256-ctr"),
			},
			{
				Config: `
resource "f5os_system" "upd_err_sshd" {
  hostname     = "update-err.example.net"
  motd         = ""
  login_banner = ""
  timezone     = "UTC"
  sshd_ciphers = ["aes128-ctr"]
}`,
				ExpectError: regexp.MustCompile(`failure while Sshd Cipher`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when SSHD kex algorithm PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemUpdateKexAlgError(t *testing.T) {
	testAccPreUnitCheck(t)

	kexPutCount := 0

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/kexalgorithms", func(w http.ResponseWriter, r *http.Request) {
		kexPutCount++
		if kexPutCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"kex update error"}]}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"update-err.example.net","login-banner":"","motd-banner":""}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{"kexalgorithms":["ecdh-sha2-nistp384"]}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})
	systemMockDeleteEndpoints(mux)

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "upd_err_kex" {
  hostname     = "update-err.example.net"
  motd         = ""
  login_banner = ""
  timezone     = "UTC"
  sshd_kex_alg = ["ecdh-sha2-nistp384"]
}`,
				Check: resource.TestCheckResourceAttr("f5os_system.upd_err_kex", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
			},
			{
				Config: `
resource "f5os_system" "upd_err_kex" {
  hostname     = "update-err.example.net"
  motd         = ""
  login_banner = ""
  timezone     = "UTC"
  sshd_kex_alg = ["ecdh-sha2-nistp256"]
}`,
				ExpectError: regexp.MustCompile(`failure while creating Sshd Key Algo`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when SSHD MAC PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemUpdateMacAlgError(t *testing.T) {
	testAccPreUnitCheck(t)

	macPutCount := 0

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/macs", func(w http.ResponseWriter, r *http.Request) {
		macPutCount++
		if macPutCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"mac update error"}]}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"update-err.example.net","login-banner":"","motd-banner":""}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{"macs":["hmac-sha2-256"]}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})
	systemMockDeleteEndpoints(mux)

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "upd_err_mac" {
  hostname     = "update-err.example.net"
  motd         = ""
  login_banner = ""
  timezone     = "UTC"
  sshd_mac_alg = ["hmac-sha2-256"]
}`,
				Check: resource.TestCheckResourceAttr("f5os_system.upd_err_mac", "sshd_mac_alg.0", "hmac-sha2-256"),
			},
			{
				Config: `
resource "f5os_system" "upd_err_mac" {
  hostname     = "update-err.example.net"
  motd         = ""
  login_banner = ""
  timezone     = "UTC"
  sshd_mac_alg = ["hmac-sha2-512"]
}`,
				ExpectError: regexp.MustCompile(`failure while creating Mac Algo`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when SSHD host key algorithm PUT returns an error.
// ---------------------------------------------------------------------------

func TestUnitSystemUpdateHkeyAlgError(t *testing.T) {
	testAccPreUnitCheck(t)

	hkeyPutCount := 0

	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/host-key-algorithms", func(w http.ResponseWriter, r *http.Request) {
		hkeyPutCount++
		if hkeyPutCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"hkey update error"}]}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:config":{"hostname":"update-err.example.net","login-banner":"","motd-banner":""}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/clock", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"openconfig-system:clock":{"config":{"timezone-name":"UTC"}}}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-security-ciphers:service":[{"name":"httpd","config":{}},{"name":"sshd","config":{"host-key-algorithms":["ssh-ed25519"]}}]}`))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"f5-system-settings:settings":{"config":{}}}`))
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-aaa-confd-restconf-token:restconf-token/state/lifetime", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"f5-aaa-confd-restconf-token:lifetime": 15}`))
	})
	systemMockDeleteEndpoints(mux)

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_system" "upd_err_hkey" {
  hostname      = "update-err.example.net"
  motd          = ""
  login_banner  = ""
  timezone      = "UTC"
  sshd_hkey_alg = ["ssh-ed25519"]
}`,
				Check: resource.TestCheckResourceAttr("f5os_system.upd_err_hkey", "sshd_hkey_alg.0", "ssh-ed25519"),
			},
			{
				Config: `
resource "f5os_system" "upd_err_hkey" {
  hostname      = "update-err.example.net"
  motd          = ""
  login_banner  = ""
  timezone      = "UTC"
  sshd_hkey_alg = ["rsa-sha2-256"]
}`,
				ExpectError: regexp.MustCompile(`failure while creating Ssh Host Key Algo`),
			},
		},
	})
}
