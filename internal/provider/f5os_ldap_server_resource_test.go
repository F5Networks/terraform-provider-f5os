package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

func TestUnitLdapServerCreateGetUpdate(t *testing.T) {
	testAccPreUnitCheck(t)
	t.Logf("Server URL: %s", server.URL)

	// Mock handler for creating an LDAP server
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=ldap-group1/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// Mock handler for reading/updating/deleting an LDAP server
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=ldap-group1/servers/server=192.0.2.1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"openconfig-system:server": [
					{
						"address": "192.0.2.1",
						"f5-openconfig-aaa-ldap:ldap": {
							"address": "192.0.2.1",
							"f5-openconfig-aaa-ldap:auth-port": 636,
							"f5-openconfig-aaa-ldap:type": "ldaps"
						}
					}
				]
			}`)
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	// Create client using the test helper that reads from environment variables
	// which were set to point to our mock server by testAccPreUnitCheck
	client, err := newTestClientFromEnv()
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Test Create
	port := int64(636)
	err = client.CreateLdapServer("ldap-group1", "192.0.2.1", &port, "ldaps")
	assert.NoError(t, err)

	// Test Get
	config, err := client.GetLdapServer("ldap-group1", "192.0.2.1")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "192.0.2.1", config.Address)
	assert.NotNil(t, config.AuthPort)
	assert.Equal(t, int64(636), *config.AuthPort)
	assert.Equal(t, "ldaps", config.Type)

	// Test Update
	newPort := int64(389)
	err = client.UpdateLdapServer("ldap-group1", "192.0.2.1", &newPort, "ldap")
	assert.NoError(t, err)

	// Test Delete
	err = client.DeleteLdapServer("ldap-group1", "192.0.2.1")
	assert.NoError(t, err)
}

func TestUnitLdapServerCreateError(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock handler returning 500 error on create
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=error-group/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"error": "server error"}`)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	defer teardown()

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	// Attempt create that should fail
	port := int64(636)
	err = client.CreateLdapServer("error-group", "192.0.2.100", &port, "ldaps")
	assert.Error(t, err, "expected error on HTTP 500")
}

func TestUnitLdapServerGetNotFound(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock handler returning 404 for get
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=missing-group/servers/server=192.0.2.100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	defer teardown()

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	// Attempt get that should fail
	config, err := client.GetLdapServer("missing-group", "192.0.2.100")
	assert.Error(t, err, "expected error on HTTP 404")
	assert.Nil(t, config, "expected nil config on error")
}

func TestUnitLdapServerDeleteError(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock handler returning 500 on delete
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=del-error-group/servers/server=192.0.2.100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	defer teardown()

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	// Attempt delete that should fail
	err = client.DeleteLdapServer("del-error-group", "192.0.2.100")
	assert.Error(t, err, "expected error on HTTP 500")
}

func TestUnitLdapServerUpdateError(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock handler returning 400 on update
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=upd-error-group/servers/server=192.0.2.100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	defer teardown()

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	// Attempt update that should fail
	port := int64(389)
	err = client.UpdateLdapServer("upd-error-group", "192.0.2.100", &port, "ldap")
	assert.Error(t, err, "expected error on HTTP 400")
}

func TestUnitLdapServerCreateMinimalConfig(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock handler accepting minimal config (no port/type)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=minimal-group/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// Mock handler for get returning minimal config
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=minimal-group/servers/server=192.0.2.50", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"openconfig-system:server": [
					{
						"address": "192.0.2.50",
						"f5-openconfig-aaa-ldap:ldap": {
							"address": "192.0.2.50"
						}
					}
				]
			}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	// Create with nil port and type (minimal config)
	err = client.CreateLdapServer("minimal-group", "192.0.2.50", nil, "")
	assert.NoError(t, err)

	// Get should succeed and return address only
	config, err := client.GetLdapServer("minimal-group", "192.0.2.50")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "192.0.2.50", config.Address)
	assert.Nil(t, config.AuthPort, "expected nil port for minimal config")
	assert.Empty(t, config.Type, "expected empty type for minimal config")
}

func TestUnitLdapServerSpecialCharactersInGroupName(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock handler for group name with special characters (URL encoded)
	groupNameEncoded := "ldap-group%2Bspecial"
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group="+groupNameEncoded+"/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group="+groupNameEncoded+"/servers/server=192.0.2.75", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"openconfig-system:server": [
					{
						"address": "192.0.2.75",
						"f5-openconfig-aaa-ldap:ldap": {
							"address": "192.0.2.75",
							"f5-openconfig-aaa-ldap:auth-port": 389,
							"f5-openconfig-aaa-ldap:type": "ldap"
						}
					}
				]
			}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	// Create with group name containing special characters
	port := int64(389)
	err = client.CreateLdapServer("ldap-group+special", "192.0.2.75", &port, "ldap")
	assert.NoError(t, err, "should handle special characters with URL encoding")

	// Get should also work with special characters
	config, err := client.GetLdapServer("ldap-group+special", "192.0.2.75")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "192.0.2.75", config.Address)
}

func TestUnitLdapServerMultipleServersInGroup(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock handler for creating servers in same group
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=multi-group/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// Mock handlers for multiple servers
	for _, addr := range []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"} {
		addr := addr // capture for closure
		mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=multi-group/servers/server="+addr, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{
					"openconfig-system:server": [
						{
							"address": "%s",
							"f5-openconfig-aaa-ldap:ldap": {
								"address": "%s",
								"f5-openconfig-aaa-ldap:auth-port": 636,
								"f5-openconfig-aaa-ldap:type": "ldaps"
							}
						}
					]
				}`, addr, addr)
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		})
	}

	defer teardown()

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	// Create multiple servers in same group
	port := int64(636)
	addresses := []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"}
	for _, addr := range addresses {
		err = client.CreateLdapServer("multi-group", addr, &port, "ldaps")
		assert.NoError(t, err, "should create server %s", addr)

		// Verify each can be read
		config, err := client.GetLdapServer("multi-group", addr)
		assert.NoError(t, err)
		assert.Equal(t, addr, config.Address)
	}
}

func TestUnitLdapServerUpdatePreservesAddress(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=update-group/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// Mock handler verifying address doesn't change during update
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=update-group/servers/server=192.0.2.99", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"openconfig-system:server": [
					{
						"address": "192.0.2.99",
						"f5-openconfig-aaa-ldap:ldap": {
							"address": "192.0.2.99",
							"f5-openconfig-aaa-ldap:auth-port": 389,
							"f5-openconfig-aaa-ldap:type": "ldap"
						}
					}
				]
			}`)
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	// Create initial
	port1 := int64(636)
	err = client.CreateLdapServer("update-group", "192.0.2.99", &port1, "ldaps")
	assert.NoError(t, err)

	// Update port and type
	port2 := int64(389)
	err = client.UpdateLdapServer("update-group", "192.0.2.99", &port2, "ldap")
	assert.NoError(t, err)

	// Verify address still matches
	config, err := client.GetLdapServer("update-group", "192.0.2.99")
	assert.NoError(t, err)
	assert.Equal(t, "192.0.2.99", config.Address, "address must not change during update")
}

func testAccLdapServerGroupName() string {
	if g := os.Getenv("F5OS_LDAP_SERVER_GROUP"); g != "" {
		return g
	}
	return "tf_acc_test_ldap_group"
}

func testAccLdapServerResourceConfig(name string, port int) string {
	return fmt.Sprintf(`
resource "f5os_ldap_server" "%s" {
    server_group = "%s"
    address      = "192.0.2.10"
    auth_port    = %d
    type         = "ldap"
}
`, name, testAccLdapServerGroupName(), port)
}

func testAccLdapServerResourceConfigUpdated(name string, port int) string {
	return fmt.Sprintf(`
resource "f5os_ldap_server" "%s" {
	server_group = "%s"
	address      = "192.0.2.10"
	auth_port    = %d
	type         = "ldaps"
}
`, name, testAccLdapServerGroupName(), port)
}

// ensureTestServerGroup ensures the named LDAP server-group exists using
// the provider API. If the group did not exist and was created by this helper,
// the returned cleanup function will delete it. If the group already existed
// before calling this helper the cleanup is a no-op.
func ensureTestServerGroup(t *testing.T, groupName string) func() {
	t.Helper()

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Logf("ensureTestServerGroup: could not create client: %v", err)
		return func() {}
	}

	created, err := EnsureLdapServerGroupCreated(client, groupName)
	if err != nil {
		t.Fatalf("ensureTestServerGroup: failed to ensure server-group %q via API: %v", groupName, err)
	}
	if created {
		t.Logf("ensureTestServerGroup: server-group %q created by test; will be destroyed in cleanup", groupName)
		return func() {
			if err := DeleteServerGroup(client, groupName); err != nil {
				t.Logf("ensureTestServerGroup cleanup: failed to delete server-group %q: %v", groupName, err)
			}
		}
	}
	return func() {}
}

func TestAccLdapServerLifecycle(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	groupName := testAccLdapServerGroupName()
	// Prefer API-based setup/cleanup for server-group so tests can run without SSH/CLI
	cleanup := ensureTestServerGroup(t, groupName)
	t.Cleanup(cleanup)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccLdapServerResourceConfig("acc_test", 1389),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ldap_server.acc_test", "server_group", groupName),
					resource.TestCheckResourceAttr("f5os_ldap_server.acc_test", "address", "192.0.2.10"),
					resource.TestCheckResourceAttr("f5os_ldap_server.acc_test", "auth_port", "1389"),
					resource.TestCheckResourceAttr("f5os_ldap_server.acc_test", "type", "ldap"),
					func(s *terraform.State) error {
						client, err := newTestClientFromEnv()
						if err != nil {
							return err
						}
						srv, err := client.GetLdapServer(groupName, "192.0.2.10")
						if err != nil {
							return err
						}
						if srv.AuthPort == nil || *srv.AuthPort != 1389 {
							return fmt.Errorf("expected port 1389 on device, got %v", srv.AuthPort)
						}
						return nil
					},
				),
			},
			// Step 2: Import
			{
				ResourceName:      "f5os_ldap_server.acc_test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:192.0.2.10", groupName),
				ImportStateVerify: true,
			},
			// Step 3: Update type
			{
				Config: testAccLdapServerResourceConfigUpdated("acc_test", 1389),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ldap_server.acc_test", "type", "ldaps"),
				),
			},
			// Step 4: Add another server
			{
				Config: testAccLdapServerResourceConfigTwoServers(groupName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ldap_server.s1", "address", "192.0.2.10"),
					resource.TestCheckResourceAttr("f5os_ldap_server.s2", "address", "192.0.2.11"),
				),
			},
			// Step 5: Update servers
			{
				Config: testAccLdapServerResourceConfigTwoServersUpdated(groupName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ldap_server.s1", "auth_port", "1636"),
					resource.TestCheckResourceAttr("f5os_ldap_server.s2", "type", "ldaps"),
				),
			},
			// Step 6: Destroy is automatic
		},
	})
}

// func testAccLdapServerResourceConfigTwoServers(groupName string) string {
// 	return fmt.Sprintf(`
// resource "f5os_ldap_server" "s1" {
// 	server_group = "%s"
// 	address      = "192.0.2.10"
// 	auth_port    = 1389
// 	type         = "ldap"
// }
// resource "f5os_ldap_server" "s2" {
// 	server_group = "%s"
// 	address      = "192.0.2.11"
// 	auth_port    = 1389
// 	type         = "ldap"
// }
// `, groupName, groupName)
// }

// func testAccLdapServerResourceConfigTwoServersUpdated(groupName string) string {
// 	return fmt.Sprintf(`
// resource "f5os_ldap_server" "s1" {
// 	server_group = "%s"
// 	address      = "192.0.2.10"
// 	auth_port    = 1636
// 	type         = "ldap"
// }
// resource "f5os_ldap_server" "s2" {
// 	server_group = "%s"
// 	address      = "192.0.2.11"
// 	auth_port    = 1389
// 	type         = "ldaps"
// }
// `, groupName, groupName)
// }

func testAccLdapServerResourceConfigTwoServers(groupName string) string {
	return fmt.Sprintf(`
resource "f5os_ldap_server" "s1" {
	server_group = "%s"
	address      = "192.0.2.10"
	auth_port    = 1389
	type         = "ldap"
}
resource "f5os_ldap_server" "s2" {
	server_group = "%s"
	address      = "192.0.2.11"
	auth_port    = 1389
	type         = "ldap"
}
`, groupName, groupName)
}

func testAccLdapServerResourceConfigTwoServersUpdated(groupName string) string {
	return fmt.Sprintf(`
resource "f5os_ldap_server" "s1" {
	server_group = "%s"
	address      = "192.0.2.10"
	auth_port    = 1636
	type         = "ldap"
}
resource "f5os_ldap_server" "s2" {
	server_group = "%s"
	address      = "192.0.2.11"
	auth_port    = 1389
	type         = "ldaps"
}
`, groupName, groupName)
}

func TestAccLdapServerResourceImport(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	groupName := testAccLdapServerGroupName()
	cleanup := ensureTestServerGroup(t, groupName)
	t.Cleanup(cleanup)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLdapServerResourceConfig("test", 389),
			},
			{

				ResourceName:      "f5os_ldap_server.test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:192.0.2.10", groupName),
				ImportStateVerify: true,
			},
		},
	})
}

func TestUnitLdapServerImportState(t *testing.T) {
	t.Run("create then import", func(t *testing.T) {
		testAccPreUnitCheck(t)
		defer teardown()

		// Mock: create endpoint
		mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=ldap_servers/servers", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		})

		// Mock: read/delete endpoint
		mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=ldap_servers/servers/server=192.0.2.10", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{
					"openconfig-system:server": [{
						"address": "192.0.2.10",
						"f5-openconfig-aaa-ldap:ldap": {
							"address": "192.0.2.10",
							"f5-openconfig-aaa-ldap:auth-port": 389,
							"f5-openconfig-aaa-ldap:type": "ldap"
						}
					}]
				}`)
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		})

		resource.Test(t, resource.TestCase{
			IsUnitTest:               true,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: `
resource "f5os_ldap_server" "test" {
	server_group = "ldap_servers"
	address      = "192.0.2.10"
	auth_port    = 389
	type         = "ldap"
}
`,
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr("f5os_ldap_server.test", "server_group", "ldap_servers"),
						resource.TestCheckResourceAttr("f5os_ldap_server.test", "address", "192.0.2.10"),
					),
				},
				{
					ResourceName:      "f5os_ldap_server.test",
					ImportState:       true,
					ImportStateId:     "ldap_servers:192.0.2.10",
					ImportStateVerify: true,
				},
			},
		})
	})

	t.Run("invalid composite ID format raises error", func(t *testing.T) {
		testAccPreUnitCheck(t)
		defer teardown()

		// Mock: create endpoint (needed for the initial Config step)
		mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=ldap_servers/servers", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		})

		mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=ldap_servers/servers/server=192.0.2.20", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yang-data+json")
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{
					"openconfig-system:server": [{
						"address": "192.0.2.20",
						"f5-openconfig-aaa-ldap:ldap": {
							"address": "192.0.2.20",
							"f5-openconfig-aaa-ldap:auth-port": 389,
							"f5-openconfig-aaa-ldap:type": "ldap"
						}
					}]
				}`)
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		})

		resource.Test(t, resource.TestCase{
			IsUnitTest:               true,
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: `
resource "f5os_ldap_server" "test" {
	server_group = "ldap_servers"
	address      = "192.0.2.20"
	auth_port    = 389
	type         = "ldap"
}
`,
				},
				{
					ResourceName:  "f5os_ldap_server.test",
					ImportState:   true,
					ImportStateId: "invalid-id-without-colon",
					ExpectError:   regexp.MustCompile(`Invalid import ID format`),
				},
			},
		})
	})
}

// TestUnitLdapServerCreateWirePayloadStructure validates the exact wire payload
// sent to the F5OS RESTCONF endpoint, ensuring auth-port and type are placed in
// the augmented f5-openconfig-aaa-ldap:ldap/config container and NOT in openconfig-system:server/config.
func TestUnitLdapServerCreateWirePayloadStructure(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	var capturedBody map[string]interface{}
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=payload-group/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	port := int64(1389)
	err = client.CreateLdapServer("payload-group", "10.171.125.61", &port, "ldap")
	assert.NoError(t, err)

	// Verify top-level structure
	serverList, ok := capturedBody["openconfig-system:server"].([]interface{})
	assert.True(t, ok, "expected openconfig-system:server list in payload")
	assert.Len(t, serverList, 1)

	entry := serverList[0].(map[string]interface{})
	assert.Equal(t, "10.171.125.61", entry["address"])

	// OpenConfig config container must contain address and MUST NOT contain auth-port or type
	ocConfig, ok := entry["config"].(map[string]interface{})
	assert.True(t, ok, "expected config map in server entry")
	assert.Equal(t, "10.171.125.61", ocConfig["address"])
	assert.NotContains(t, ocConfig, "f5-openconfig-aaa-ldap:auth-port")
	assert.NotContains(t, ocConfig, "auth-port")
	assert.NotContains(t, ocConfig, "type")

	// LDAP augmented container must hold auth-port and type under its config
	ldapContainer, ok := entry["f5-openconfig-aaa-ldap:ldap"].(map[string]interface{})
	assert.True(t, ok, "expected f5-openconfig-aaa-ldap:ldap container")
	ldapConfig, ok := ldapContainer["config"].(map[string]interface{})
	assert.True(t, ok, "expected config inside f5-openconfig-aaa-ldap:ldap")
	assert.Equal(t, float64(1389), ldapConfig["auth-port"])
	assert.Equal(t, "ldap", ldapConfig["type"])
}

// TestUnitLdapServerCreateAdoptsExistingServer tests that when Create receives
// "object already exists" from POST, it falls back to PATCH and adopts the server.
func TestUnitLdapServerCreateAdoptsExistingServer(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	patchCalled := false

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=existing-group/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"data-exists","error-message":"object already exists"}]}}`)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=existing-group/servers/server=10.171.125.61", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		switch r.Method {
		case http.MethodPatch:
			patchCalled = true
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"openconfig-system:server": [{
					"address": "10.171.125.61",
					"f5-openconfig-aaa-ldap:ldap": {
						"address": "10.171.125.61",
						"f5-openconfig-aaa-ldap:auth-port": 1389,
						"f5-openconfig-aaa-ldap:type": "ldap"
					}
				}]
			}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_ldap_server" "test" {
	server_group = "existing-group"
	address      = "10.171.125.61"
	auth_port    = 1389
	type         = "ldap"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_ldap_server.test", "server_group", "existing-group"),
					resource.TestCheckResourceAttr("f5os_ldap_server.test", "address", "10.171.125.61"),
					resource.TestCheckResourceAttr("f5os_ldap_server.test", "auth_port", "1389"),
					resource.TestCheckResourceAttr("f5os_ldap_server.test", "type", "ldap"),
					func(s *terraform.State) error {
						if !patchCalled {
							return fmt.Errorf("expected PATCH to be called during adoption of existing server")
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitLdapServerDeleteEmptyAddressProtection tests that Delete handles
// an empty address gracefully without dispatching an invalid DELETE server= request.
func TestUnitLdapServerDeleteEmptyAddressProtection(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	deleteCalled := false

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=del-group/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=del-group/servers/server=10.171.125.61", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{
				"openconfig-system:server": [{
					"address": "10.171.125.61",
					"f5-openconfig-aaa-ldap:ldap": {
						"address": "10.171.125.61",
						"f5-openconfig-aaa-ldap:auth-port": 389,
						"f5-openconfig-aaa-ldap:type": "ldap"
					}
				}]
			}`)
		case http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// Direct check on Delete with empty address in model:
	// Verify it extracts address from state.ID
	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	res := &LdapServerResource{client: client}
	ctx := context.Background()

	var resSchema fwresource.SchemaResponse
	res.Schema(ctx, fwresource.SchemaRequest{}, &resSchema)

	ldapServerState := func(model LdapServerResourceModel) tfsdk.State {
		state := tfsdk.State{
			Schema: resSchema.Schema,
			Raw:    tftypes.NewValue(resSchema.Schema.Type().TerraformType(ctx), nil),
		}
		if diagnostics := state.Set(ctx, &model); diagnostics.HasError() {
			t.Fatalf("failed to create Terraform state: %v", diagnostics)
		}
		return state
	}

	// Case 1: address is empty, ID has "del-group:10.171.125.61" -> successfully deletes
	req1 := fwresource.DeleteRequest{
		State: ldapServerState(LdapServerResourceModel{
			ID:          types.StringValue("del-group:10.171.125.61"),
			ServerGroup: types.StringValue("del-group"),
			Address:     types.StringValue(""),
		}),
	}
	resp1 := &fwresource.DeleteResponse{}
	res.Delete(ctx, req1, resp1)
	assert.False(t, resp1.Diagnostics.HasError())
	assert.True(t, deleteCalled, "expected Delete to recover address from ID and call DELETE")

	// Case 2: address is empty, ID is empty -> safely returns without error
	deleteCalled = false
	req2 := fwresource.DeleteRequest{
		State: ldapServerState(LdapServerResourceModel{
			ID:          types.StringValue(""),
			ServerGroup: types.StringValue("del-group"),
			Address:     types.StringValue(""),
		}),
	}
	resp2 := &fwresource.DeleteResponse{}
	res.Delete(ctx, req2, resp2)
	assert.False(t, resp2.Diagnostics.HasError())
	assert.False(t, deleteCalled, "expected Delete to safely skip without calling DELETE")
}

// TestUnitLdapServerReadNotFoundRemovesFromState tests that when an LDAP server
// is missing on the device during Read, Read calls resp.State.RemoveResource
// rather than returning an error.
func TestUnitLdapServerReadNotFoundRemovesFromState(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=test-group/servers/server=10.171.125.99", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"data-missing","error-message":"data missing"}]}}`)
	})

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	res := &LdapServerResource{client: client}
	ctx := context.Background()

	var resSchema fwresource.SchemaResponse
	res.Schema(ctx, fwresource.SchemaRequest{}, &resSchema)

	initialState := tfsdk.State{
		Schema: resSchema.Schema,
		Raw:    tftypes.NewValue(resSchema.Schema.Type().TerraformType(ctx), nil),
	}
	model := LdapServerResourceModel{
		ID:          types.StringValue("test-group:10.171.125.99"),
		ServerGroup: types.StringValue("test-group"),
		Address:     types.StringValue("10.171.125.99"),
	}
	_ = initialState.Set(ctx, &model)

	readReq := fwresource.ReadRequest{State: initialState}
	readResp := &fwresource.ReadResponse{State: initialState}
	res.Read(ctx, readReq, readResp)

	assert.False(t, readResp.Diagnostics.HasError(), "Read should not return error when resource is not found")
	assert.True(t, readResp.State.Raw.IsNull(), "Read should remove resource from state (null state)")
}

// TestUnitLdapServerReadStripsTypePrefix verifies Read correctly strips the
// "f5-openconfig-aaa-ldap:" prefix from Type when the API response includes it.
func TestUnitLdapServerReadStripsTypePrefix(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/server-groups/server-group=test-group/servers/server=192.0.2.1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{                                                                                                                                                                                                                
			"openconfig-system:server": [                                                                                                                                                                                                         
			{                                                                                                                                                                                                                                   
				"address": "192.0.2.1",                                                                                                                                                                                                           
				"f5-openconfig-aaa-ldap:ldap": {                                                                                                                                                                                                  
				"address": "192.0.2.1",                                                                                                                                                                                                         
				"f5-openconfig-aaa-ldap:auth-port": 636,                                                                                                                                                                                        
				"f5-openconfig-aaa-ldap:type": "f5-openconfig-aaa-ldap:ldaps"                                                                                                                                                                   
			}                                                                                                                                                                                                                                   
			}                                                                                                                                                                                                                                     
		]                                                                                                                                                                                                                                       
		}`)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	client, err := newTestClientFromEnv()
	assert.NoError(t, err)

	config, err := client.GetLdapServer("test-group", "192.0.2.1")
	assert.NoError(t, err)
	assert.Equal(t, "ldaps", config.Type)
}
