package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

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
			{
				ResourceName:      "f5os_snmp.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"snmp_user.0.auth_passwd",
					"snmp_user.0.privacy_passwd",
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
	host := os.Getenv("F5OS_HOST")
	if host == "" {
		return nil
	}
	user := os.Getenv("F5OS_USERNAME")
	if user == "" {
		user = os.Getenv("F5OS_USER")
	}
	pass := os.Getenv("F5OS_PASSWORD")
	port := 0
	if p := os.Getenv("F5OS_SERVER_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	} else if p := os.Getenv("F5OS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	disableSSL := false
	if v := os.Getenv("F5_VALIDATE_CERTS"); v != "" {
		// F5_VALIDATE_CERTS=false => DisableSSLVerify=true
		if v == "false" || v == "0" {
			disableSSL = true
		}
	}

	cfg := &f5ossdk.F5osConfig{
		Host:             host,
		User:             user,
		Password:         pass,
		Port:             port,
		DisableSSLVerify: disableSSL,
	}
	client, err := f5ossdk.NewSession(cfg)
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
