package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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

func testAccLdapServerResourceConfig(name string, port int) string {
	return fmt.Sprintf(`
resource "f5os_ldap_server" "%s" {
	server_group = "ldap_servers"
	address      = "192.168.1.%d"
	auth_port    = %d
	type         = "ldap"
}
`, name, port, port)
}

func TestAccLdapServerResourceImport(t *testing.T) {
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
				ImportStateId:     "ldap_servers:192.168.1.389",
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
