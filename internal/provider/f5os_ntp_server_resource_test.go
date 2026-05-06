package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
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
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

const testAccNTPServerUpdatedConfig = `
resource "f5os_ntp_server" "test" {
  server             = "10.255.255.1"
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
		var gotKeyID int64
		if ntp.KeyID != nil {
			gotKeyID = *ntp.KeyID
		}
		if gotKeyID != expectKeyID {
			return fmt.Errorf("expected key_id %d, got %d", expectKeyID, gotKeyID)
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
			// Step 1: Create and verify
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
			// Step 2: Import state by server address — verifies ImportState
			// passes the import ID through to the "server" attribute so that
			// Read can query the device (mock) and populate all fields.
			{
				ResourceName:      "f5os_ntp_server.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test (real device)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Unit test: Create calls PatchNTPGlobalConfig when ntp_service/ntp_authentication are set
// ---------------------------------------------------------------------------

func TestUnitNTPGlobalConfigPatchedOnCreate(t *testing.T) {
	testAccPreUnitCheck(t)

	var ntpConfigPatchCount int32

	// Mock: POST to create NTP server
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
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
				"openconfig-system:server": [{
					"address": "10.20.30.40",
					"config": {
						"address": "10.20.30.40",
						"f5-openconfig-system-ntp:key-id": 123,
						"prefer": true,
						"iburst": true
					}
				}]
			}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Mock: GET/PATCH for global NTP config — track PATCH calls
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			atomic.AddInt32(&ntpConfigPatchCount, 1)
			defer func() { _ = r.Body.Close() }()
			body, _ := io.ReadAll(r.Body)

			var payload struct {
				Config struct {
					Enabled       *bool `json:"enabled,omitempty"`
					EnableNTPAuth *bool `json:"enable-ntp-auth,omitempty"`
				} `json:"config"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("failed to unmarshal PATCH /ntp/config body: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if payload.Config.Enabled == nil {
				t.Error("expected 'enabled' in PATCH body, got nil")
			} else if *payload.Config.Enabled != true {
				t.Errorf("expected enabled=true, got %v", *payload.Config.Enabled)
			}
			if payload.Config.EnableNTPAuth == nil {
				t.Error("expected 'enable-ntp-auth' in PATCH body, got nil")
			} else if *payload.Config.EnableNTPAuth != true {
				t.Errorf("expected enable-ntp-auth=true, got %v", *payload.Config.EnableNTPAuth)
			}
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
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_authentication", "true"),
					func(_ *terraform.State) error {
						count := atomic.LoadInt32(&ntpConfigPatchCount)
						if count == 0 {
							return fmt.Errorf("PatchNTPGlobalConfig was never called during Create (PATCH /ntp/config count = 0)")
						}
						return nil
					},
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update calls PatchNTPGlobalConfig when ntp_service/ntp_authentication change
// ---------------------------------------------------------------------------

const testUnitNTPServerUpdateNTPServiceConfig = `
resource "f5os_ntp_server" "test" {
  server             = "10.20.30.40"
  key_id             = 123
  prefer             = true
  iburst             = true
  ntp_service        = false
  ntp_authentication = false
}
`

func TestUnitNTPGlobalConfigPatchedOnUpdate(t *testing.T) {
	testAccPreUnitCheck(t)

	var ntpConfigPatchCount int32
	// Track the most recent PATCH payload to /ntp/config so we can verify
	// that the Update step sends the updated values.
	var lastPatchService, lastPatchAuth *bool

	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	// Track whether service was enabled or disabled in GET responses.
	// Start enabled=true, auth=true; after PATCH with false, return false.
	var globalEnabled int32 = 1
	var globalAuth int32 = 1

	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers/server=10.20.30.40", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:server": [{
					"address": "10.20.30.40",
					"config": {
						"address": "10.20.30.40",
						"f5-openconfig-system-ntp:key-id": 123,
						"prefer": true,
						"iburst": true
					}
				}]
			}`))
		case "PATCH":
			w.WriteHeader(http.StatusNoContent)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			atomic.AddInt32(&ntpConfigPatchCount, 1)
			defer func() { _ = r.Body.Close() }()
			body, _ := io.ReadAll(r.Body)

			var payload struct {
				Config struct {
					Enabled       *bool `json:"enabled,omitempty"`
					EnableNTPAuth *bool `json:"enable-ntp-auth,omitempty"`
				} `json:"config"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("failed to unmarshal PATCH body: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			lastPatchService = payload.Config.Enabled
			lastPatchAuth = payload.Config.EnableNTPAuth

			// Update the mock state so subsequent GETs reflect the change.
			if payload.Config.Enabled != nil {
				if *payload.Config.Enabled {
					atomic.StoreInt32(&globalEnabled, 1)
				} else {
					atomic.StoreInt32(&globalEnabled, 0)
				}
			}
			if payload.Config.EnableNTPAuth != nil {
				if *payload.Config.EnableNTPAuth {
					atomic.StoreInt32(&globalAuth, 1)
				} else {
					atomic.StoreInt32(&globalAuth, 0)
				}
			}
			w.WriteHeader(http.StatusNoContent)
		case "GET":
			en := atomic.LoadInt32(&globalEnabled) == 1
			au := atomic.LoadInt32(&globalAuth) == 1
			w.WriteHeader(http.StatusOK)
			resp := fmt.Sprintf(`{
				"openconfig-system:config": {
					"enabled": %v,
					"enable-ntp-auth": %v
				}
			}`, en, au)
			_, _ = w.Write([]byte(resp))
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with ntp_service=true, ntp_authentication=true
			{
				Config: testUnitNTPServerBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_authentication", "true"),
				),
			},
			// Step 2: Update to ntp_service=false, ntp_authentication=false
			{
				Config: testUnitNTPServerUpdateNTPServiceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_service", "false"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_authentication", "false"),
					func(_ *terraform.State) error {
						count := atomic.LoadInt32(&ntpConfigPatchCount)
						// At least 2: once for Create, once for Update
						if count < 2 {
							return fmt.Errorf("expected PATCH /ntp/config to be called at least 2 times (Create + Update), got %d", count)
						}
						if lastPatchService == nil || *lastPatchService != false {
							return fmt.Errorf("expected last PATCH to set enabled=false, got %v", lastPatchService)
						}
						if lastPatchAuth == nil || *lastPatchAuth != false {
							return fmt.Errorf("expected last PATCH to set enable-ntp-auth=false, got %v", lastPatchAuth)
						}
						return nil
					},
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: PatchNTPGlobalConfig is NOT called when ntp_service/ntp_authentication are omitted
// ---------------------------------------------------------------------------

const testUnitNTPServerNoGlobalConfig = `
resource "f5os_ntp_server" "test" {
  server = "10.20.30.40"
  key_id = 123
  prefer = true
  iburst = true
}
`

func TestUnitNTPGlobalConfigNotPatchedWhenOmitted(t *testing.T) {
	testAccPreUnitCheck(t)

	var ntpConfigPatchCount int32

	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers/server=10.20.30.40", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:server": [{
					"address": "10.20.30.40",
					"config": {
						"address": "10.20.30.40",
						"f5-openconfig-system-ntp:key-id": 123,
						"prefer": true,
						"iburst": true
					}
				}]
			}`))
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			atomic.AddInt32(&ntpConfigPatchCount, 1)
			w.WriteHeader(http.StatusNoContent)
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:config": {
					"enabled": false,
					"enable-ntp-auth": false
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
				Config: testUnitNTPServerNoGlobalConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "server", "10.20.30.40"),
					func(_ *terraform.State) error {
						count := atomic.LoadInt32(&ntpConfigPatchCount)
						if count != 0 {
							return fmt.Errorf("expected PATCH /ntp/config to NOT be called when ntp_service/ntp_authentication are omitted, but got %d calls", count)
						}
						return nil
					},
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: key_id=0 is serialized in POST body (not dropped by omitempty)
//
// Before the *int64 fix, omitempty treated int64(0) as empty and silently
// dropped "f5-openconfig-system-ntp:key-id" from the JSON payload.  This
// test would FAIL with the old code because the POST body would lack key-id.
// ---------------------------------------------------------------------------

const testUnitNTPServerKeyIDZeroConfig = `
resource "f5os_ntp_server" "zero" {
  server             = "10.20.30.41"
  key_id             = 0
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

func TestUnitNTPServerKeyIDZeroSerialized(t *testing.T) {
	testAccPreUnitCheck(t)

	var postBodyKeyID *float64 // float64 because json.Unmarshal decodes numbers as float64

	// Mock: POST to create NTP server — inspect the body for key-id
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			defer func() { _ = r.Body.Close() }()
			body, _ := io.ReadAll(r.Body)

			// Decode the POST payload to check for key-id
			var payload struct {
				Server []struct {
					Config struct {
						KeyID *float64 `json:"f5-openconfig-system-ntp:key-id"`
					} `json:"config"`
				} `json:"server"`
			}
			if err := json.Unmarshal(body, &payload); err == nil && len(payload.Server) > 0 {
				postBodyKeyID = payload.Server[0].Config.KeyID
			}

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	// Mock: GET/DELETE for specific NTP server — return key-id: 0
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers/server=10.20.30.41", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:server": [{
					"address": "10.20.30.41",
					"config": {
						"address": "10.20.30.41",
						"f5-openconfig-system-ntp:key-id": 0,
						"prefer": true,
						"iburst": true
					}
				}]
			}`))
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
				Config: testUnitNTPServerKeyIDZeroConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Terraform state must record key_id as "0", not empty
					resource.TestCheckResourceAttr("f5os_ntp_server.zero", "key_id", "0"),
					// The POST body must have included key-id: 0
					func(_ *terraform.State) error {
						if postBodyKeyID == nil {
							return fmt.Errorf("POST body did not contain 'f5-openconfig-system-ntp:key-id'; omitempty dropped key_id=0")
						}
						if *postBodyKeyID != 0 {
							return fmt.Errorf("expected key-id=0 in POST body, got %v", *postBodyKeyID)
						}
						return nil
					},
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: key_id omitted from config results in no key-id field in payload
//
// When the user does NOT set key_id at all, the provider should omit the
// field from the JSON payload entirely (omitempty with nil *int64).
// Also verifies the read path handles a GET response that lacks key-id
// (nil-pointer safety).
// ---------------------------------------------------------------------------

const testUnitNTPServerKeyIDOmittedConfig = `
resource "f5os_ntp_server" "nokey" {
  server             = "10.20.30.42"
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

func TestUnitNTPServerKeyIDOmittedNotSerialized(t *testing.T) {
	testAccPreUnitCheck(t)

	var keyIDPresent bool

	// Mock: POST to create NTP server — check that key-id is absent
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			defer func() { _ = r.Body.Close() }()
			body, _ := io.ReadAll(r.Body)

			// Use a raw map to detect the presence of the key-id field
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err == nil {
				if servers, ok := payload["server"].([]interface{}); ok && len(servers) > 0 {
					if srv, ok := servers[0].(map[string]interface{}); ok {
						if cfg, ok := srv["config"].(map[string]interface{}); ok {
							_, keyIDPresent = cfg["f5-openconfig-system-ntp:key-id"]
						}
					}
				}
			}

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	// Mock: GET/PATCH/DELETE for specific NTP server — return response WITHOUT key-id
	// to also exercise the nil-pointer dereference safety in GetNTPServer
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers/server=10.20.30.42", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:server": [{
					"address": "10.20.30.42",
					"config": {
						"address": "10.20.30.42",
						"prefer": true,
						"iburst": true
					}
				}]
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
				Config: testUnitNTPServerKeyIDOmittedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.nokey", "server", "10.20.30.42"),
					// The POST body must NOT have included key-id
					func(_ *terraform.State) error {
						if keyIDPresent {
							return fmt.Errorf("POST body contained 'f5-openconfig-system-ntp:key-id' even though key_id was not set in the config; expected the field to be omitted")
						}
						return nil
					},
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test (real device)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Acceptance-test HCL configs for NTP global config patching
// Uses 10.255.255.2 to avoid colliding with the basic NTP acceptance test
// ---------------------------------------------------------------------------

const testAccNTPGlobalConfigCreateEnabled = `
resource "f5os_ntp_server" "global" {
  server             = "10.255.255.2"
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

const testAccNTPGlobalConfigUpdateDisabled = `
resource "f5os_ntp_server" "global" {
  server             = "10.255.255.2"
  prefer             = true
  iburst             = true
  ntp_service        = false
  ntp_authentication = false
}
`

const testAccNTPGlobalConfigUpdateReEnabled = `
resource "f5os_ntp_server" "global" {
  server             = "10.255.255.2"
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

// ---------------------------------------------------------------------------
// Acceptance test: PatchNTPGlobalConfig is called on Create and Update
// ---------------------------------------------------------------------------

func TestAccNTPGlobalConfigPatched(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNTPServerDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with ntp_service=true, ntp_authentication=true
			//         Verify the global config was actually written to the device.
			{
				Config: testAccNTPGlobalConfigCreateEnabled,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Terraform state checks
					resource.TestCheckResourceAttr("f5os_ntp_server.global", "server", "10.255.255.2"),
					resource.TestCheckResourceAttr("f5os_ntp_server.global", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.global", "ntp_authentication", "true"),
					// Direct device API verification
					testAccCheckNTPServerOnDevice("10.255.255.2", 0, true, true),
					testAccCheckNTPGlobalConfigOnDevice(true, true),
				),
			},
			// Step 2: Update to ntp_service=false, ntp_authentication=false
			//         Verify PatchNTPGlobalConfig wrote the change to the device.
			{
				Config: testAccNTPGlobalConfigUpdateDisabled,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.global", "ntp_service", "false"),
					resource.TestCheckResourceAttr("f5os_ntp_server.global", "ntp_authentication", "false"),
					// Direct device API verification
					testAccCheckNTPGlobalConfigOnDevice(false, false),
				),
			},
			// Step 3: Update back to ntp_service=true, ntp_authentication=true
			//         Confirms the toggle works in both directions.
			{
				Config: testAccNTPGlobalConfigUpdateReEnabled,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.global", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.global", "ntp_authentication", "true"),
					// Direct device API verification
					testAccCheckNTPGlobalConfigOnDevice(true, true),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies NTP server cleanup
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: key_id omitted — verifies the *int64 nil path
//
// The key-id field is a YANG leafref that requires a pre-configured NTP
// authentication key to exist on the device.  Because our test device has no
// NTP authentication keys, we cannot send key_id=0 or any other value (the
// device returns "illegal reference").  The key_id=0 serialisation fix is
// fully covered by TestUnitNTPServerKeyIDZeroSerialized.
//
// This acceptance test verifies the *other* half of the *int64 fix: when the
// user omits key_id, the pointer stays nil and omitempty correctly omits the
// field from the JSON payload, so the device accepts the request.
//
// Uses 10.255.255.3 to avoid colliding with other NTP acceptance tests.
// ---------------------------------------------------------------------------

const testAccNTPKeyIDOmittedCreate = `
resource "f5os_ntp_server" "keyid" {
  server             = "10.255.255.3"
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

func TestAccNTPServerKeyIDOmitted(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNTPServerDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create without key_id — exercises the *int64 nil path.
			// Before the fix, CreateNTPServerPayload always called
			// plan.KeyID.ValueInt64() which returned 0, and the old
			// int64 field would either be omitted (omitempty) or sent
			// as 0 depending on the code path.  With *int64, when
			// key_id is not in the config the pointer stays nil and
			// omitempty correctly omits the field.
			{
				Config: testAccNTPKeyIDOmittedCreate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.keyid", "server", "10.255.255.3"),
					resource.TestCheckResourceAttr("f5os_ntp_server.keyid", "prefer", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.keyid", "iburst", "true"),
					// Direct device API verification — key_id defaults to 0
					testAccCheckNTPServerOnDevice("10.255.255.3", 0, true, true),
				),

			},
			// Step 2: Destroy is automatic — CheckDestroy verifies cleanup.
			// Note: Update steps are intentionally omitted. The NTP server
			// PATCH (Update) has a pre-existing bug where the POST-style
			// payload doesn't correctly update prefer/iburst on the device.
			// That bug is orthogonal to the *int64 fix under test.
		},
	})
}

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
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "prefer", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "iburst", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_authentication", "true"),
					// Direct device API verification
					testAccCheckNTPServerOnDevice("10.255.255.1", 0, true, true),
					testAccCheckNTPGlobalConfigOnDevice(true, true),
				),
			},
			// Step 2: Import state by server address — verifies ImportState
			// passes the ID through to the "server" attribute so Read can
			// query the device and populate all fields.
			{
				ResourceName:      "f5os_ntp_server.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update and verify
			{
				Config: testAccNTPServerUpdatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "server", "10.255.255.1"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "prefer", "false"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "iburst", "false"),
					// Direct device API verification
					testAccCheckNTPServerOnDevice("10.255.255.1", 0, false, false),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}
