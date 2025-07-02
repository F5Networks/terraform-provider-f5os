package provider

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// can you please write the unit test and acceptance test cases for the ntp server resource?
// fix all the issues in the unit test case.
// Test configurations
const testAccF5osNTPServerBasicConfig = `
resource "f5os_ntp_server" "test" {
  server             = "10.20.30.40"
  key_id             = 123
  prefer             = true
  iburst             = true
  ntp_service        = true
  ntp_authentication = true
}
`

const testAccF5osNTPServerUpdatedConfig = `
resource "f5os_ntp_server" "test" {
  server             = "10.20.30.40"
  key_id             = 456
  prefer             = false
  iburst             = false
  ntp_service        = true
  ntp_authentication = true
}
`

func TestUnitF5osNTPServerResource(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock server interactions for NTP server endpoints
	mux.HandleFunc("/restconf/data/openconfig-system:system/ntp/openconfig-system:servers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}
	})

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

}

// TestAccF5osNTPServerResource runs acceptance tests for the NTP Server resource
func TestAccF5osNTPServerResource(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless TF_ACC set")
	}

	// Check if we can run the test
	cmd := exec.Command("ping", "-c", "1", os.Getenv("F5OS_HOST"))
	err := cmd.Run()
	if err != nil {
		t.Skipf("Unable to reach %s: %s", os.Getenv("F5OS_HOST"), err)
	} else {
		t.Logf("Connected to %s", os.Getenv("F5OS_HOST"))
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Step 1: Test basic configuration
				Config: testAccF5osNTPServerBasicConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "server", "10.20.30.40"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "key_id", "123"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "prefer", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "iburst", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_authentication", "true"),
				),
			},
			{
				// Step 2: Test resource import
				ResourceName:      "f5os_ntp_server.test",
				ImportState:       true,
				ImportStateVerify: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "server", "10.20.30.40"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "key_id", "123"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "prefer", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "iburst", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_service", "true"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "ntp_authentication", "true"),
				),
			},
			{
				// Step 3: Test resource update
				Config: testAccF5osNTPServerUpdatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "server", "10.20.30.40"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "key_id", "456"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "prefer", "false"),
					resource.TestCheckResourceAttr("f5os_ntp_server.test", "iburst", "false"),
				),
			},
			{
				// Step 4: Test resource destroy
				Config: `
                resource "f5os_ntp_server" "test" {
                    # Empty config to trigger destroy
                }
            `,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Use the custom TestCheckNoResourceExists to validate destroy
					checkNoResourceExists("f5os_ntp_server.test"),
				),
			},
		},
	})
}

func checkNoResourceExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// Look for the resource in the state
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			// Resource not found in state, which is what we want
			return nil
		}
		// If resource exists in state, it should be marked as tainted/destroyed
		if rs.Primary.ID == "" {
			return nil
		}
		return fmt.Errorf("resource %s still exists", name)
	}
}
