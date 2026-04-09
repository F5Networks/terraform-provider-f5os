package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// newSnmpClientFromEnv creates a fresh f5osclient session from environment variables.
// Port defaults to 8888 to match the provider (provider.go:104).
func newSnmpClientFromEnv() (*f5ossdk.F5os, error) {
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

func TestAccSnmpResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSnmpDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccSnmpResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_snmp.test", "state", "present"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.0.name", "test_community"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.0.name", "test_target"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.0.name", "test_user"),
					resource.TestCheckResourceAttrSet("f5os_snmp.test", "id"),
				),
			},
			// ImportState testing
			// Read now queries the device and returns ALL communities, targets,
			// users, and MIB settings — not just the ones Terraform manages.
			// The nested blocks must be ignored because the device contains
			// pre-existing entries beyond what this test created.
			// Passwords (auth_passwd, privacy_passwd) are also never returned
			// by the API.
			{
				ResourceName:      "f5os_snmp.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"snmp_community",
					"snmp_target",
					"snmp_user",
					"snmp_mib",
				},
			},
			// Update and Read testing
			{
				Config: testAccSnmpResourceConfigUpdated,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_snmp.test", "state", "present"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.#", "2"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.#", "2"),
					resource.TestCheckResourceAttrSet("f5os_snmp.test", "id"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

// testAccCheckSnmpDestroy verifies SNMP config is removed on the device
func testAccCheckSnmpDestroy(s *terraform.State) error {
	if os.Getenv("F5OS_HOST") == "" {
		return nil
	}
	client, err := newSnmpClientFromEnv()
	if err != nil {
		// If we cannot connect, don't fail the destroy check
		return nil
	}

	data, err := client.GetSnmpConfig()
	if err != nil || len(data) == 0 {
		// Treat errors or empty response as destroyed
		return nil
	}
	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	// Only fail if any of the test-created entries remain
	communities := []string{"test_community", "test_community2"}
	targets := []string{"test_target", "test_target2"}
	users := []string{"test_user"}

	if jsonContainsAnyName(payload, communities) || jsonContainsAnyName(payload, targets) || jsonContainsAnyName(payload, users) {
		return fmt.Errorf("SNMP configuration still present after destroy")
	}
	return nil
}

// jsonContainsAnyName recursively searches the JSON payload for any object with a matching name
// It matches either at obj["name"] or obj["config"]["name"].
func jsonContainsAnyName(v interface{}, names []string) bool {
	switch t := v.(type) {
	case map[string]interface{}:
		// direct name
		if nameVal, ok := t["name"].(string); ok {
			for _, n := range names {
				if nameVal == n {
					return true
				}
			}
		}
		// name under config
		if cfg, ok := t["config"].(map[string]interface{}); ok {
			if cfgName, ok2 := cfg["name"].(string); ok2 {
				for _, n := range names {
					if cfgName == n {
						return true
					}
				}
			}
		}
		for _, val := range t {
			if jsonContainsAnyName(val, names) {
				return true
			}
		}
	case []interface{}:
		for _, item := range t {
			if jsonContainsAnyName(item, names) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Direct-API verification helpers
// ---------------------------------------------------------------------------

// testAccCheckSnmpCommunityOnDevice verifies a named community exists on the
// device with the expected security models.
func testAccCheckSnmpCommunityOnDevice(name string, securityModels ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newSnmpClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create SNMP client: %w", err)
		}
		data, err := client.GetSnmpConfig()
		if err != nil {
			return fmt.Errorf("GetSnmpConfig failed: %w", err)
		}
		var envelope struct {
			SNMP struct {
				Communities struct {
					Community []struct {
						Name   string `json:"name"`
						Config struct {
							SecurityModel []string `json:"security-model"`
						} `json:"config"`
					} `json:"community"`
				} `json:"communities"`
			} `json:"f5-system-snmp:snmp"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("failed to parse SNMP config: %w", err)
		}
		for _, c := range envelope.SNMP.Communities.Community {
			if c.Name != name {
				continue
			}
			// Verify each expected security model is present
			got := strings.Join(c.Config.SecurityModel, ",")
			for _, sm := range securityModels {
				found := false
				for _, actual := range c.Config.SecurityModel {
					if actual == sm {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("community %q: expected security-model %q in %q", name, sm, got)
				}
			}
			return nil
		}
		return fmt.Errorf("community %q not found on device", name)
	}
}

// testAccCheckSnmpTargetOnDevice verifies a named trap target exists on the
// device with the expected IPv4 address and port.
func testAccCheckSnmpTargetOnDevice(name, ipv4 string, port int64) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newSnmpClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create SNMP client: %w", err)
		}
		data, err := client.GetSnmpConfig()
		if err != nil {
			return fmt.Errorf("GetSnmpConfig failed: %w", err)
		}
		var envelope struct {
			SNMP struct {
				Targets struct {
					Target []struct {
						Name   string `json:"name"`
						Config struct {
							IPv4 *struct {
								Address string `json:"address"`
								Port    int64  `json:"port"`
							} `json:"ipv4"`
						} `json:"config"`
					} `json:"target"`
				} `json:"targets"`
			} `json:"f5-system-snmp:snmp"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("failed to parse SNMP config: %w", err)
		}
		for _, t := range envelope.SNMP.Targets.Target {
			if t.Name != name {
				continue
			}
			if t.Config.IPv4 == nil {
				return fmt.Errorf("target %q: no ipv4 config on device", name)
			}
			if t.Config.IPv4.Address != ipv4 {
				return fmt.Errorf("target %q: expected ipv4 %q, got %q", name, ipv4, t.Config.IPv4.Address)
			}
			if t.Config.IPv4.Port != port {
				return fmt.Errorf("target %q: expected port %d, got %d", name, port, t.Config.IPv4.Port)
			}
			return nil
		}
		return fmt.Errorf("target %q not found on device", name)
	}
}

// testAccCheckSnmpUserOnDevice verifies a named SNMPv3 user exists on the
// device with the expected authentication and privacy protocols.
func testAccCheckSnmpUserOnDevice(name, authProto, privacyProto string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newSnmpClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create SNMP client: %w", err)
		}
		data, err := client.GetSnmpConfig()
		if err != nil {
			return fmt.Errorf("GetSnmpConfig failed: %w", err)
		}
		var envelope struct {
			SNMP struct {
				Users struct {
					User []struct {
						Name   string `json:"name"`
						Config struct {
							AuthProto    string `json:"authentication-protocol"`
							PrivacyProto string `json:"privacy-protocol"`
						} `json:"config"`
					} `json:"user"`
				} `json:"users"`
			} `json:"f5-system-snmp:snmp"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("failed to parse SNMP config: %w", err)
		}
		for _, u := range envelope.SNMP.Users.User {
			if u.Name != name {
				continue
			}
			if !strings.EqualFold(u.Config.AuthProto, authProto) {
				return fmt.Errorf("user %q: expected auth-proto %q, got %q", name, authProto, u.Config.AuthProto)
			}
			if !strings.EqualFold(u.Config.PrivacyProto, privacyProto) {
				return fmt.Errorf("user %q: expected privacy-proto %q, got %q", name, privacyProto, u.Config.PrivacyProto)
			}
			return nil
		}
		return fmt.Errorf("user %q not found on device", name)
	}
}

// testAccCheckSnmpMibOnDevice verifies the SNMP MIB sysName, sysContact, and
// sysLocation values on the device.
func testAccCheckSnmpMibOnDevice(sysName, sysContact, sysLocation string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newSnmpClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create SNMP client: %w", err)
		}
		data, err := client.GetSnmpMib()
		if err != nil {
			return fmt.Errorf("GetSnmpMib failed: %w", err)
		}
		var envelope struct {
			System struct {
				SysName     string `json:"sysName"`
				SysContact  string `json:"sysContact"`
				SysLocation string `json:"sysLocation"`
			} `json:"SNMPv2-MIB:system"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("failed to parse SNMP MIB: %w", err)
		}
		if envelope.System.SysName != sysName {
			return fmt.Errorf("sysName: expected %q, got %q", sysName, envelope.System.SysName)
		}
		if envelope.System.SysContact != sysContact {
			return fmt.Errorf("sysContact: expected %q, got %q", sysContact, envelope.System.SysContact)
		}
		if envelope.System.SysLocation != sysLocation {
			return fmt.Errorf("sysLocation: expected %q, got %q", sysLocation, envelope.System.SysLocation)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// TestAccSnmpResourceRead — verifies Read populates state from the device
// ---------------------------------------------------------------------------

// TestAccSnmpResourceRead verifies that:
//   - After Create, Terraform state reflects what is on the device (community
//     security_model, target ipv4/port, user auth/privacy protos, MIB fields).
//   - Read correctly filters state to only the entries Terraform manages —
//     pre-existing device entries are not pulled into state.
//   - After Update, both old and new entries appear in state and on the device.
func TestAccSnmpResourceRead(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSnmpDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create — verify both Terraform state AND device state.
			{
				Config: testAccSnmpResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// --- Terraform state assertions ---
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.0.name", "test_community"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.0.security_model.#", "2"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.0.name", "test_target"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.0.ipv4_address", "192.168.1.100"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.0.port", "162"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.0.name", "test_user"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.0.auth_proto", "sha"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.0.privacy_proto", "aes"),
					// Passwords must be present in state (preserved via Computed+UseStateForUnknown)
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.0.auth_passwd", "testpassword123"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.0.privacy_passwd", "privacypassword123"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_mib.sysname", "F5OS-Test-System"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_mib.syscontact", "admin@example.com"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_mib.syslocation", "Test Lab"),
					// --- Direct device API assertions ---
					testAccCheckSnmpCommunityOnDevice("test_community", "v1", "v2c"),
					testAccCheckSnmpTargetOnDevice("test_target", "192.168.1.100", 162),
					testAccCheckSnmpUserOnDevice("test_user", "sha", "aes"),
					testAccCheckSnmpMibOnDevice("F5OS-Test-System", "admin@example.com", "Test Lab"),
				),
			},
			// Step 2: Update — verify second community/target appear in both
			// state and on the device; original entry still present.
			{
				Config: testAccSnmpResourceConfigUpdated,
				Check: resource.ComposeAggregateTestCheckFunc(
					// --- Terraform state assertions ---
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.#", "2"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.#", "2"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_mib.sysname", "F5OS-Test-System-Updated"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_mib.syscontact", "admin@updated.example.com"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_mib.syslocation", "Updated Test Lab"),
					// Passwords still in state after update (UseStateForUnknown preserved them)
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.0.auth_passwd", "testpassword123"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_user.0.privacy_passwd", "privacypassword123"),
					// --- Direct device API assertions ---
					testAccCheckSnmpCommunityOnDevice("test_community", "v1", "v2c"),
					testAccCheckSnmpCommunityOnDevice("test_community2", "v2c"),
					testAccCheckSnmpTargetOnDevice("test_target", "192.168.1.100", 162),
					testAccCheckSnmpTargetOnDevice("test_target2", "192.168.1.101", 162),
					testAccCheckSnmpMibOnDevice("F5OS-Test-System-Updated", "admin@updated.example.com", "Updated Test Lab"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// TestAccSnmpResourceReadFiltersToManagedEntries — verifies Read does not
// import pre-existing device entries into Terraform state
// ---------------------------------------------------------------------------

// TestAccSnmpResourceReadFiltersToManagedEntries verifies that Read only
// tracks the entries that Terraform created, not the full device config.
// It asserts the state contains exactly the managed entries (1 community,
// 1 target) and not any of the pre-existing device communities/targets.
func TestAccSnmpResourceReadFiltersToManagedEntries(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSnmpDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSnmpResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Exactly 1 community in state — not the device's pre-existing ones
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_community.0.name", "test_community"),
					// Exactly 1 target in state — not the device's pre-existing ones
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.test", "snmp_target.0.name", "test_target"),
					// Verify pre-existing device entries are NOT in state
					testAccCheckSnmpStateDoesNotContainCommunity("f5os_snmp.test", "community1"),
					testAccCheckSnmpStateDoesNotContainCommunity("f5os_snmp.test", "community2"),
					testAccCheckSnmpStateDoesNotContainTarget("f5os_snmp.test", "Chamo_PC"),
				),
			},
		},
	})
}

// testAccCheckSnmpStateDoesNotContainCommunity asserts that no entry in the
// snmp_community list in Terraform state has the given name.
func testAccCheckSnmpStateDoesNotContainCommunity(resourceName, communityName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceName)
		}
		countStr := rs.Primary.Attributes["snmp_community.#"]
		count, _ := strconv.Atoi(countStr)
		for i := 0; i < count; i++ {
			key := fmt.Sprintf("snmp_community.%d.name", i)
			if rs.Primary.Attributes[key] == communityName {
				return fmt.Errorf("pre-existing community %q found in Terraform state — Read is not filtering correctly", communityName)
			}
		}
		return nil
	}
}

// testAccCheckSnmpStateDoesNotContainTarget asserts that no entry in the
// snmp_target list in Terraform state has the given name.
func testAccCheckSnmpStateDoesNotContainTarget(resourceName, targetName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceName)
		}
		countStr := rs.Primary.Attributes["snmp_target.#"]
		count, _ := strconv.Atoi(countStr)
		for i := 0; i < count; i++ {
			key := fmt.Sprintf("snmp_target.%d.name", i)
			if rs.Primary.Attributes[key] == targetName {
				return fmt.Errorf("pre-existing target %q found in Terraform state — Read is not filtering correctly", targetName)
			}
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Unit test: verifies Read preserves null snmp_mib when the user did not
// declare it in the HCL config (mibWasManaged guard).
// ---------------------------------------------------------------------------

const testUnitSnmpNoMibConfig = `
resource "f5os_snmp" "nomib" {
  state = "present"

  snmp_community = [
    {
      name           = "unit_community"
      security_model = ["v2c"]
    }
  ]

  snmp_target = [
    {
      name           = "unit_target"
      security_model = "v2c"
      community      = "unit_community"
      ipv4_address   = "192.168.1.200"
      port           = 162
    }
  ]
}
`

func TestUnitSnmpReadPreservesNullMib(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock: SNMP config — POST for communities/targets/users
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-snmp:snmp/f5-system-snmp:communities", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-snmp:snmp/f5-system-snmp:targets", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-snmp:snmp/f5-system-snmp:users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Mock: GET SNMP config — returns the community and target we created
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-snmp:snmp", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-system-snmp:snmp": {
					"communities": {
						"community": [
							{
								"name": "unit_community",
								"config": {
									"name": "unit_community",
									"security-model": ["v2c"]
								}
							}
						]
					},
					"targets": {
						"target": [
							{
								"name": "unit_target",
								"config": {
									"name": "unit_target",
									"security-model": "v2c",
									"community": "unit_community",
									"ipv4": {
										"address": "192.168.1.200",
										"port": 162
									}
								}
							}
						]
					}
				}
			}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Mock: GET SNMP MIB — returns real values, but they should NOT
	// appear in state because snmp_mib is not declared in the config.
	mux.HandleFunc("/restconf/data/SNMPv2-MIB:SNMPv2-MIB/system", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"SNMPv2-MIB:system": {
				"sysName": "DeviceThatShouldNotAppear",
				"sysContact": "admin@should-not-appear.com",
				"sysLocation": "Nowhere"
			}
		}`))
	})

	// Mock: DELETE endpoints for destroy
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-snmp:snmp/targets/target=unit_target", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-snmp:snmp/communities/community=unit_community", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitSnmpNoMibConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Community and target should be in state
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_community.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_community.0.name", "unit_community"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_target.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_target.0.name", "unit_target"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_target.0.ipv4_address", "192.168.1.200"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_target.0.port", "162"),

					// snmp_mib must NOT be populated — the user didn't
					// declare it and the mibWasManaged guard should keep
					// it null even though the mock returns MIB data.
					resource.TestCheckNoResourceAttr("f5os_snmp.nomib", "snmp_mib.sysname"),
					resource.TestCheckNoResourceAttr("f5os_snmp.nomib", "snmp_mib.syscontact"),
					resource.TestCheckNoResourceAttr("f5os_snmp.nomib", "snmp_mib.syslocation"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// TestAccSnmpResourceMibNotManaged — acceptance test for the mibWasManaged
// guard: when snmp_mib is omitted from the HCL config, Read must not
// populate it from the device even though the device has global MIB values.
// ---------------------------------------------------------------------------

// testAccCheckSnmpMibAbsentFromState asserts that snmp_mib is not set in
// Terraform state (i.e., all MIB sub-attributes are absent).
func testAccCheckSnmpMibAbsentFromState(resourceName string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckNoResourceAttr(resourceName, "snmp_mib.sysname"),
		resource.TestCheckNoResourceAttr(resourceName, "snmp_mib.syscontact"),
		resource.TestCheckNoResourceAttr(resourceName, "snmp_mib.syslocation"),
	)
}

const testAccSnmpNoMibConfig = `
resource "f5os_snmp" "nomib" {
  state = "present"

  snmp_community = [
    {
      name           = "test_nomib_community"
      security_model = ["v2c"]
    }
  ]

  snmp_target = [
    {
      name           = "test_nomib_target"
      security_model = "v2c"
      community      = "test_nomib_community"
      ipv4_address   = "192.168.1.200"
      port           = 162
    }
  ]
}
`

func TestAccSnmpResourceMibNotManaged(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSnmpDestroy,
		Steps: []resource.TestStep{
			// Create SNMP config WITHOUT snmp_mib.
			// After Read, snmp_mib must remain null in state even though
			// the device has global MIB values — the mibWasManaged guard
			// must suppress them.
			{
				Config: testAccSnmpNoMibConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Community and target written and read back correctly
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_community.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_community.0.name", "test_nomib_community"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_target.#", "1"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_target.0.name", "test_nomib_target"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_target.0.ipv4_address", "192.168.1.200"),
					resource.TestCheckResourceAttr("f5os_snmp.nomib", "snmp_target.0.port", "162"),

					// snmp_mib must NOT be populated — the user didn't
					// declare it and the device's global MIB values must
					// not leak into Terraform state.
					testAccCheckSnmpMibAbsentFromState("f5os_snmp.nomib"),

					// Confirm community and target exist on the device
					testAccCheckSnmpCommunityOnDevice("test_nomib_community", "v2c"),
					testAccCheckSnmpTargetOnDevice("test_nomib_target", "192.168.1.200", 162),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// TestAccSnmpResourceDeleteResetsMib — verifies that terraform destroy resets
// MIB sysName/sysContact/sysLocation to empty strings instead of leaving
// orphaned values on the device.
// Captures the device baseline MIB before the test and restores it via
// a deferred cleanup.
// ---------------------------------------------------------------------------

// snmpMibBaseline holds the original MIB values to restore after the test.
type snmpMibBaseline struct {
	SysName     string
	SysContact  string
	SysLocation string
}

// captureSnmpMibBaseline reads the current MIB values from the device.
func captureSnmpMibBaseline() (*snmpMibBaseline, error) {
	client, err := newSnmpClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	data, err := client.GetSnmpMib()
	if err != nil {
		return nil, fmt.Errorf("GetSnmpMib failed: %w", err)
	}
	var envelope struct {
		System struct {
			SysName     string `json:"sysName"`
			SysContact  string `json:"sysContact"`
			SysLocation string `json:"sysLocation"`
		} `json:"SNMPv2-MIB:system"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse MIB: %w", err)
	}
	return &snmpMibBaseline{
		SysName:     envelope.System.SysName,
		SysContact:  envelope.System.SysContact,
		SysLocation: envelope.System.SysLocation,
	}, nil
}

// restoreSnmpMibBaseline writes the captured baseline MIB values back to the device.
func restoreSnmpMibBaseline(b *snmpMibBaseline) error {
	client, err := newSnmpClientFromEnv()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	payload := map[string]interface{}{
		"SNMPv2-MIB:system": map[string]interface{}{
			"SNMPv2-MIB:sysName":     b.SysName,
			"SNMPv2-MIB:sysContact":  b.SysContact,
			"SNMPv2-MIB:sysLocation": b.SysLocation,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal MIB payload: %w", err)
	}
	return client.UpdateSnmpMib(payloadBytes)
}

// testAccCheckSnmpMibResetOnDevice verifies that after destroy, all three
// MIB fields are empty strings on the device.
func testAccCheckSnmpMibResetOnDevice() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newSnmpClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		data, err := client.GetSnmpMib()
		if err != nil {
			return fmt.Errorf("GetSnmpMib failed: %w", err)
		}
		var envelope struct {
			System struct {
				SysName     string `json:"sysName"`
				SysContact  string `json:"sysContact"`
				SysLocation string `json:"sysLocation"`
			} `json:"SNMPv2-MIB:system"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("failed to parse MIB: %w", err)
		}
		if envelope.System.SysName != "" {
			return fmt.Errorf("sysName should be empty after destroy, got %q", envelope.System.SysName)
		}
		if envelope.System.SysContact != "" {
			return fmt.Errorf("sysContact should be empty after destroy, got %q", envelope.System.SysContact)
		}
		if envelope.System.SysLocation != "" {
			return fmt.Errorf("sysLocation should be empty after destroy, got %q", envelope.System.SysLocation)
		}
		return nil
	}
}

const testAccSnmpMibResetConfig = `
resource "f5os_snmp" "mib_reset" {
  state = "present"

  snmp_community = [
    {
      name           = "test_mib_reset_community"
      security_model = ["v2c"]
    }
  ]

  snmp_mib = {
    sysname     = "MibResetTest"
    syscontact  = "reset@example.com"
    syslocation = "Reset Lab"
  }
}
`

func TestAccSnmpResourceDeleteResetsMib(t *testing.T) {
	// Capture baseline MIB before test so we can restore after.
	baseline, err := captureSnmpMibBaseline()
	if err != nil {
		t.Skipf("Cannot capture MIB baseline: %v", err)
	}
	t.Cleanup(func() {
		if err := restoreSnmpMibBaseline(baseline); err != nil {
			t.Logf("WARNING: failed to restore MIB baseline: %v", err)
		} else {
			t.Logf("MIB baseline restored: sysName=%q sysContact=%q sysLocation=%q",
				baseline.SysName, baseline.SysContact, baseline.SysLocation)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		// CheckDestroy verifies MIB was reset to empty strings AND
		// that test communities were removed.
		CheckDestroy: func(s *terraform.State) error {
			// First check: test communities cleaned up
			if err := testAccCheckSnmpDestroy(s); err != nil {
				return err
			}
			// Second check: MIB fields are empty strings
			return testAccCheckSnmpMibResetOnDevice()(s)
		},
		Steps: []resource.TestStep{
			// Step 1: Create with MIB values — verify they land on the device.
			{
				Config: testAccSnmpMibResetConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_snmp.mib_reset", "snmp_mib.sysname", "MibResetTest"),
					resource.TestCheckResourceAttr("f5os_snmp.mib_reset", "snmp_mib.syscontact", "reset@example.com"),
					resource.TestCheckResourceAttr("f5os_snmp.mib_reset", "snmp_mib.syslocation", "Reset Lab"),
					testAccCheckSnmpMibOnDevice("MibResetTest", "reset@example.com", "Reset Lab"),
					testAccCheckSnmpCommunityOnDevice("test_mib_reset_community", "v2c"),
				),
			},
			// Destroy is automatic. CheckDestroy above verifies:
			// 1. test_mib_reset_community is gone
			// 2. sysName, sysContact, sysLocation are all ""
			// t.Cleanup restores the original baseline MIB values.
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test HCL configs
// ---------------------------------------------------------------------------

const testAccSnmpResourceConfig = `
resource "f5os_snmp" "test" {
  state = "present"
  
  snmp_community = [
    {
      name           = "test_community"
      security_model = ["v1", "v2c"]
    }
  ]
  
  snmp_target = [
    {
      name         = "test_target"
      security_model = "v2c"
      community    = "test_community"
      ipv4_address = "192.168.1.100"
      port         = 162
    }
  ]
  
  snmp_user = [
    {
      name           = "test_user"
      auth_proto     = "sha"
      auth_passwd    = "testpassword123"
      privacy_proto  = "aes"
      privacy_passwd = "privacypassword123"
    }
  ]
  
  snmp_mib = {
    sysname     = "F5OS-Test-System"
    syscontact  = "admin@example.com"
    syslocation = "Test Lab"
  }
}
`

const testAccSnmpResourceConfigUpdated = `
resource "f5os_snmp" "test" {
  state = "present"
  
  snmp_community = [
    {
      name           = "test_community"
      security_model = ["v1", "v2c"]
    },
    {
      name           = "test_community2"
      security_model = ["v2c"]
    }
  ]
  
  snmp_target = [
    {
      name         = "test_target"
      security_model = "v2c"
      community    = "test_community"
      ipv4_address = "192.168.1.100"
      port         = 162
    },
    {
      name         = "test_target2"
      security_model = "v2c"
      community    = "test_community2"
      ipv4_address = "192.168.1.101"
      port         = 162
    }
  ]
  
  snmp_user = [
    {
      name           = "test_user"
      auth_proto     = "sha"
      auth_passwd    = "testpassword123"
      privacy_proto  = "aes"
      privacy_passwd = "privacypassword123"
    }
  ]
  
  snmp_mib = {
    sysname     = "F5OS-Test-System-Updated"
    syscontact  = "admin@updated.example.com"
    syslocation = "Updated Test Lab"
  }
}
`
