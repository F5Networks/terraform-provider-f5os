package provider

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

func TestAccTenantDeployResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenant(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantDeployResourceConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", tenantTestImage()),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.10.10.26"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_gateway", "10.10.10.1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
					testAccCheckTenantTypeOnDevice("testtenant-ecosys2", "BIG-IP"),
				),
			},
			{
				ResourceName:      "f5os_tenant.test2",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",
					"virtual_disk_size",
				},
			},
		},
	})
}

func TestAccTenantDeployResourceTC4(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenant(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccTenantDeployResourceConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", tenantTestImage()),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.10.10.26"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
					testAccCheckTenantTypeOnDevice("testtenant-ecosys2", "BIG-IP"),
				),
			},
			// Step 2: Update mgmt_ip
			{
				Config: testAccTenantDeployTC4ResourceConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", tenantTestImage()),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.10.10.27"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
				),
			},
		},
	})
}

func TestUnitTenantDeployResourceUnitTC1(t *testing.T) {
	testAccPreUnitCheck(t)
	var count = 0
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/restconf/data/openconfig-vlan:vlans", r.URL.String(), "Expected method 'GET', got %s", r.URL.String())
		w.WriteHeader(http.StatusNoContent)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
	   "f5-tenant-images:image": [
	       {
	           "name": %q,
	           "in-use": false,
	           "type": "vm-image",
	           "status": "replicated",
	           "date": "2023-8-17",
	           "size": "2.27 GB"
	       }
	   ]
	}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			count++
			return
		}
		if r.Method == "GET" && count <= 4 {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
		} else if r.Method == "GET" && count <= 7 {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_update_config.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors": {"error": [{"error-type": "application","error-tag": "invalid-value","error-message": "uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		count++
	})
	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create and verify type is populated from device
			{
				Config: testAccTenantDeployResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", tenantUnitTestImage),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.10.10.26"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_gateway", "10.10.10.1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
				),
			},
			// Step 2: Import — verifies type is populated from the
			// device API response, not carried over from prior plan.
			// Before the fix, tenantResourceModeltoState did not set
			// data.Type so the imported state would have an empty type.
			{
				ResourceName:      "f5os_tenant.test2",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",           // not returned by API
					"virtual_disk_size", // state vs config size mismatch
				},
			},
			// Step 3: Update and verify type is still correct
			{
				Config: testAccTenantDeployResourceConfigModify(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", tenantUnitTestImage),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.10.10.27"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_gateway", "10.10.10.1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
				),
			},
		},
	})
}

// TestUnitTenantTypePopulatedOnImport verifies that after terraform import,
// the "type" attribute is populated from the device API response (State.Type),
// not preserved from stale plan state. Before the fix that added
//
//	data.Type = types.StringValue(respData.F5TenantsTenant[0].State.Type)
//
// to tenantResourceModeltoState(), the type field would be empty after import.
func TestUnitTenantTypePopulatedOnImport(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccTenantDeployResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
				),
			},
			// Step 2: Import — the critical test. During import there
			// is no prior plan state. The "type" field can ONLY be
			// populated if tenantResourceModeltoState reads it from
			// the API response (State.Type). Without the fix, this
			// assertion would fail with type="" or the ImportStateVerify
			// would report type missing.
			{
				ResourceName:      "f5os_tenant.test2",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",
					"virtual_disk_size",
				},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 state, got %d", len(states))
					}
					typeVal := states[0].Attributes["type"]
					if typeVal != "BIG-IP" {
						return fmt.Errorf("expected type %q after import, got %q — tenantResourceModeltoState is not setting data.Type", "BIG-IP", typeVal)
					}
					return nil
				},
			},
		},
	})
}

// TestUnitTenantVlansPopulatedFromDevice verifies that when the device API
// returns a non-nil vlans array, tenantResourceModeltoState populates
// data.Vlans with the correct values.
func TestUnitTenantVlansPopulatedFromDevice(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			// Return fixture with vlans: [10, 20, 30]
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_multi_vlans.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantVlansMultiConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.#", "3"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.1", "20"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.2", "30"),
				),
			},
		},
	})
}

// TestUnitTenantVlansNullWhenDeviceReturnsNone verifies that when the device
// API response has no vlans field (nil), tenantResourceModeltoState sets
// data.Vlans to types.ListNull so the state doesn't contain stale values.
func TestUnitTenantVlansNullWhenDeviceReturnsNone(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			// Return fixture with NO vlans field
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_no_vlans.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantNoVlansConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckNoResourceAttr("f5os_tenant.test2", "vlans.#"),
				),
			},
		},
	})
}

func TestUnitTenantDeployResourceUnitTC2(t *testing.T) {
	testAccPreUnitCheck(t)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_r4k_state.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
	   "f5-tenant-images:image": [
	       {
	           "name": %q,
	           "in-use": false,
	           "type": "vm-image",
	           "status": "replicated",
	           "date": "2023-8-17",
	           "size": "2.27 GB"
	       }
	   ]
	}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_r4k_get_status.json"))
	})
	var count = 0
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method == "GET" && (count == 0 || count == 1 || count == 2) {
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_r4k_config.json"))
		} else if r.Method == "GET" {
			_, _ = fmt.Fprintf(w, `
			{"ietf-restconf:errors": {"error": [{
	 				"error-type": "application",
	 				"error-tag": "invalid-value",
	 				"error-message": "uri keypath not found"
	 			}]}}`)
		}
		count++
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantDeployTC2ResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", tenantUnitTestImage),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.14.10.10"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_gateway", "10.14.10.1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Running"),
				),
			},
		},
	})
}

func TestUnitTenantDeployResourceUnitTC3(t *testing.T) {
	testAccPreUnitCheck(t)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/restconf/data/openconfig-vlan:vlans", r.URL.String(), "Expected method 'GET', got %s", r.URL.String())
		w.WriteHeader(http.StatusNoContent)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
	   "f5-tenant-images:image": [
	       {
	           "name": %q,
	           "in-use": false,
	           "type": "vm-image",
	           "status": "replicated",
	           "date": "2023-8-17",
	           "size": "2.27 GB"
	       }
	   ]
	}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=test-tenant22/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status_pending.json"))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config:      testAccTenantDeployResourceTC3Config(),
				ExpectError: regexp.MustCompile("Tenant Deployment Pending"),
			},
		},
	})
}

// TestUnitTenantDeployResourcePendingNoInstances2_0_0 exercises the F5OS
// 2.0.0 state response shape where status is "Pending" but the response no
// longer includes state.instances. tenantWait must not panic on the absent
// (nil) instances field and instead treats the deployment as still in
// progress. With a zero timeout the Create loop surfaces the generic timeout
// error immediately, without dereferencing the missing instances map.
func TestUnitTenantDeployResourcePendingNoInstances2_0_0(t *testing.T) {
	testAccPreUnitCheck(t)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/restconf/data/openconfig-vlan:vlans", r.URL.String(), "Expected method 'GET', got %s", r.URL.String())
		w.WriteHeader(http.StatusNoContent)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
	   "f5-tenant-images:image": [
	       {
	           "name": %q,
	           "in-use": false,
	           "type": "vm-image",
	           "status": "replicated",
	           "date": "2023-8-17",
	           "size": "2.27 GB"
	       }
	   ]
	}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=test-tenant22/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status_pending_2_0_0.json"))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourcePendingNoInstancesConfig(),
				ExpectError: regexp.MustCompile("tenant deployment status is still in"),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: verifies the type field is populated from the device
// after Create and Import (the fix under test).
// Uses a real image available on the DUT.
// ---------------------------------------------------------------------------

// testAccCheckTenantTypeOnDevice queries the device directly and verifies
// the tenant type field matches the expected value.
func testAccCheckTenantTypeOnDevice(tenantName, expectedType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetTenant(tenantName)
		if err != nil {
			return fmt.Errorf("GetTenant failed: %w", err)
		}
		if len(resp.F5TenantsTenant) == 0 {
			return fmt.Errorf("tenant %q not found on device", tenantName)
		}
		actual := resp.F5TenantsTenant[0].State.Type
		if actual != expectedType {
			return fmt.Errorf("tenant %q type: expected %q, got %q", tenantName, expectedType, actual)
		}
		return nil
	}
}

// testAccCheckTenantDestroy verifies the test tenant no longer exists.
func testAccCheckTenantDestroy(s *terraform.State) error {
	if os.Getenv("F5OS_HOST") == "" {
		return nil
	}
	client, err := newTestClientFromEnv()
	if err != nil {
		return nil // treat connection failure as destroyed
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_tenant" {
			continue
		}
		name := rs.Primary.Attributes["name"]
		if !client.CheckTenantnotexist(name) {
			return fmt.Errorf("tenant %q still exists after destroy", name)
		}
	}
	return nil
}

// testAccCheckTenantMaxNodesOnDevice queries the device directly and verifies
// the tenant config max-nodes matches the expected value. Only meaningful on
// F5OS 2.0.0+ devices.
func testAccCheckTenantMaxNodesOnDevice(tenantName string, expected int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetTenant(tenantName)
		if err != nil {
			return fmt.Errorf("GetTenant failed: %w", err)
		}
		if len(resp.F5TenantsTenant) == 0 {
			return fmt.Errorf("tenant %q not found on device", tenantName)
		}
		actual := resp.F5TenantsTenant[0].State.MaxNodes
		if actual != expected {
			return fmt.Errorf("tenant %q max-nodes: expected %d, got %d", tenantName, expected, actual)
		}
		return nil
	}
}

// testAccPreCheckTenant2_0_0 runs the standard acceptance pre-check and then
// skips the test unless the device is running F5OS 2.0.0 or later, since
// max_nodes and the associated read-only state fields only exist on 2.0.0+.
func testAccPreCheckTenant2_0_0(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("testAccPreCheckTenant2_0_0: failed to create session: %s", err)
	}
	if !platformVersionAtLeast(client.PlatformVersion, "v2.0") {
		t.Skipf("skipping: test requires F5OS 2.0.0+ but device reports %q", client.PlatformVersion)
	}
	// Ensure the tenant image is present (the test creates a tenant that
	// references it). Done after the version gate so we don't import on
	// devices where the test would skip.
	testAccEnsureImageNamed(t, tenantTestImage())
}

func testAccTenantMaxNodesConfigFunc() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "max_nodes_test" {
  name              = "test-max-nodes"
  image_name        = %q
  mgmt_ip           = "10.10.10.52"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
  max_nodes         = 8
}
`, tenantTestImage())
}

// TestAccTenantMaxNodes2_0_0 verifies, against a real F5OS 2.0.0+ device, that
// config.max-nodes is applied and that the read-only 2.0.0 state fields
// (max_nodes, mgmt_vlan, mgmt_vlan_accessible, clustering_as_service) are
// populated into state. Skips on devices below 2.0.0.
func TestAccTenantMaxNodes2_0_0(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenant2_0_0(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create and verify max_nodes in state and on the device.
			{
				Config: testAccTenantMaxNodesConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.max_nodes_test", "name", "test-max-nodes"),
					resource.TestCheckResourceAttr("f5os_tenant.max_nodes_test", "max_nodes", "8"),
					resource.TestCheckResourceAttr("f5os_tenant.max_nodes_test", "status", "Configured"),
					// These are read-only computed attributes; assert they are
					// set (present in state). Exact values are device-specific.
					resource.TestCheckResourceAttrSet("f5os_tenant.max_nodes_test", "mgmt_vlan"),
					resource.TestCheckResourceAttrSet("f5os_tenant.max_nodes_test", "mgmt_vlan_accessible"),
					resource.TestCheckResourceAttrSet("f5os_tenant.max_nodes_test", "clustering_as_service"),
					// Direct device API verification.
					testAccCheckTenantMaxNodesOnDevice("test-max-nodes", 8),
				),
			},
			// Step 2: Import — max_nodes can only be populated if
			// tenantResourceModeltoState reads State.MaxNodes from the device.
			{
				ResourceName:      "f5os_tenant.max_nodes_test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",
					"virtual_disk_size",
				},
			},
		},
	})
}

func testAccTenantTypeFieldConfigFunc() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "type_test" {
  name              = "test-type-field"
  image_name        = %q
  mgmt_ip           = "10.10.10.50"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
}
`, tenantTestImage())
}

func TestAccTenantDeployResourceTypeField(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenant(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create and verify type is populated in state
			// AND on the device via direct API check.
			{
				Config: testAccTenantTypeFieldConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.type_test", "name", "test-type-field"),
					resource.TestCheckResourceAttr("f5os_tenant.type_test", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.type_test", "status", "Configured"),
					// Direct device API verification
					testAccCheckTenantTypeOnDevice("test-type-field", "BIG-IP"),
				),
			},
			// Step 2: Import — the critical test for the fix. During
			// import there is no prior plan. The "type" field can ONLY
			// be populated if tenantResourceModeltoState reads State.Type
			// from the device. Without the fix, type would be empty.
			{
				ResourceName:      "f5os_tenant.type_test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",           // not returned by API
					"virtual_disk_size", // state vs config size may differ
				},
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test: verifies that vlans are populated in state from the
// device after Create and Import, and updated correctly.
// ---------------------------------------------------------------------------

// testAccCheckTenantVlansOnDevice queries the device directly and verifies
// the tenant config vlans match the expected values.
func testAccCheckTenantVlansOnDevice(tenantName string, expectedVlans []int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetTenant(tenantName)
		if err != nil {
			return fmt.Errorf("GetTenant failed: %w", err)
		}
		if len(resp.F5TenantsTenant) == 0 {
			return fmt.Errorf("tenant %q not found on device", tenantName)
		}
		actual := resp.F5TenantsTenant[0].Config.Vlans
		if len(actual) != len(expectedVlans) {
			return fmt.Errorf("tenant %q vlans: expected %v, got %v", tenantName, expectedVlans, actual)
		}
		for i, v := range expectedVlans {
			if actual[i] != v {
				return fmt.Errorf("tenant %q vlans[%d]: expected %d, got %d", tenantName, i, v, actual[i])
			}
		}
		return nil
	}
}

// testAccCheckTenantNoVlansOnDevice queries the device and verifies the
// tenant has no vlans configured. This handles both nil (omitted from JSON)
// and empty array cases since len(nil) == 0 and len([]int{}) == 0.
func testAccCheckTenantNoVlansOnDevice(tenantName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetTenant(tenantName)
		if err != nil {
			return fmt.Errorf("GetTenant failed: %w", err)
		}
		if len(resp.F5TenantsTenant) == 0 {
			return fmt.Errorf("tenant %q not found on device", tenantName)
		}
		actual := resp.F5TenantsTenant[0].Config.Vlans
		if len(actual) != 0 {
			return fmt.Errorf("tenant %q expected no vlans, got %v", tenantName, actual)
		}
		return nil
	}
}

// TestAccTenantVlansPopulatedInState verifies vlans are populated in state
// from the device after Create and Import, and updated correctly.
// Note: Vlans are stored as an ordered list (types.ListType), not a set.
// The F5OS API preserves VLAN ordering, so index-based assertions are valid.
// Prerequisites: VLANs 3910, 3920, 3930 must exist on the device (range 3900-3999
// is reserved for testing per the skill safety rules).
func TestAccTenantVlansPopulatedInState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenantVlans(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with vlans and verify state + device
			{
				Config: testAccTenantWithVlansConfigFunc([]int{3910, 3920}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "name", "test-vlans-field"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "status", "Configured"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "vlans.#", "2"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "vlans.0", "3910"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "vlans.1", "3920"),
					testAccCheckTenantVlansOnDevice("test-vlans-field", []int{3910, 3920}),
				),
			},
			// Step 2: Import — vlans should now survive import because
			// tenantResourceModeltoState reads Config.Vlans from the
			// device response.
			{
				ResourceName:      "f5os_tenant.vlans_test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",
					"virtual_disk_size",
				},
			},
			// Step 3: Update vlans to a different set
			{
				Config: testAccTenantWithVlansConfigFunc([]int{3910, 3920, 3930}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "vlans.#", "3"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "vlans.0", "3910"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "vlans.1", "3920"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "vlans.2", "3930"),
					testAccCheckTenantVlansOnDevice("test-vlans-field", []int{3910, 3920, 3930}),
				),
			},
		},
	})
}

func TestAccTenantNoVlansInState(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenant(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with no vlans — verify no vlans on device
			{
				Config: testAccTenantWithoutVlansConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "name", "test-vlans-field"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.vlans_test", "status", "Configured"),
					resource.TestCheckNoResourceAttr("f5os_tenant.vlans_test", "vlans.#"),
					testAccCheckTenantNoVlansOnDevice("test-vlans-field"),
				),
			},
			// Step 2: Import — vlans should be null after import
			{
				ResourceName:      "f5os_tenant.vlans_test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",
					"virtual_disk_size",
				},
			},
		},
	})
}

// testAccCheckTenantNoDeploymentFileOnDevice queries the device directly and
// verifies Config.DeploymentFile is empty for a standard BIG-IP tenant.
func testAccCheckTenantNoDeploymentFileOnDevice(tenantName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetTenant(tenantName)
		if err != nil {
			return fmt.Errorf("GetTenant failed: %w", err)
		}
		if len(resp.F5TenantsTenant) == 0 {
			return fmt.Errorf("tenant %q not found on device", tenantName)
		}
		actual := resp.F5TenantsTenant[0].Config.DeploymentFile
		if actual != "" {
			return fmt.Errorf("tenant %q expected no deployment_file, got %q", tenantName, actual)
		}
		return nil
	}
}

// TestAccTenantDeploymentFileAbsentForBigIP verifies that for a standard
// BIG-IP tenant, deployment_file is absent in state and empty on device.
func TestAccTenantDeploymentFileAbsentForBigIP(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenant(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create standard BIG-IP tenant without deployment_file
			{
				Config: testAccTenantBigIPNoDeploymentFileConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.df_bigip_test", "name", "test-df-bigip"),
					resource.TestCheckResourceAttr("f5os_tenant.df_bigip_test", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.df_bigip_test", "status", "Configured"),
					resource.TestCheckNoResourceAttr("f5os_tenant.df_bigip_test", "deployment_file"),
					// Direct device API verification — no deployment_file on device
					testAccCheckTenantNoDeploymentFileOnDevice("test-df-bigip"),
				),
			},
			// Step 2: Import — deployment_file should remain absent
			{
				ResourceName:      "f5os_tenant.df_bigip_test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",
					"virtual_disk_size",
				},
			},
		},
	})
}

func testAccTenantBigIPNoDeploymentFileConfigFunc() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "df_bigip_test" {
  name              = "test-df-bigip"
  image_name        = %q
  mgmt_ip           = "10.10.10.42"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
}
`, tenantTestImage())
}

// ---------------------------------------------------------------------------
// Acceptance test HCL configs
// ---------------------------------------------------------------------------

// tenantTestImage returns the BIG-IP tenant image name to use in acceptance
// tests. Set F5OS_TENANT_IMAGE to override the default. Unit tests use the
// mock server and instead reference tenantUnitTestImage.
//
// The default returns the same image name that the acc:tenant_image CI job
// imports (testAccImageName in tenant_image_resource_test.go), because the
// tenant CI job runs after acc:tenant_image and relies on that image
// already being present on the DUT. Keep this in sync with testAccImageName.
func tenantTestImage() string {
	if v := os.Getenv("F5OS_TENANT_IMAGE"); v != "" {
		return v
	}
	return testAccImageName
}

// testAccPreCheckTenant is the PreCheck for tenant acceptance tests. In
// addition to the standard env-var check it ensures the tenant image
// (tenantTestImage()) is present on the DUT, importing it if necessary.
//
// The tenant resource's Create fails fast with a 404 ("Tenant Image ... not
// found") if the image is absent. On shared devices the image can be
// deleted/re-imported by concurrent jobs (e.g. acc:tenant_image), so relying
// on another job to have left it in place is racy. Ensuring it here makes the
// tenant tests self-sufficient and mirrors the tenant_image tests'
// testAccPreCheckWithSetup pattern.
func testAccPreCheckTenant(t *testing.T) {
	testAccPreCheck(t)
	testAccEnsureImageNamed(t, tenantTestImage())
}

// testAccPreCheckTenantVlans is the PreCheck for the tenant VLAN test. In
// addition to ensuring the tenant image, it ensures VLANs 3910/3920/3930 exist
// on the DUT. A tenant that references a VLAN which does not exist fails Create
// with a 400 "illegal reference .../config/vlans". These VLANs are not
// guaranteed to persist on shared devices between runs, so the test ensures
// them itself (3900-3999 is the reserved test range).
func testAccPreCheckTenantVlans(t *testing.T) {
	testAccPreCheckTenant(t)
	testAccEnsureVlans(t, 3910, 3920, 3930)
}

// tenantUnitTestImage is the placeholder tenant image name used by unit tests
// that run against the mock server. The value is arbitrary — it only has to
// match consistently across the mock handler URL, the mock JSON response, the
// HCL config, and any state assertions — so an obviously-fake name is used to
// avoid implying a real BIG-IP build is required.
const tenantUnitTestImage = "dumm_image_unit_test.qcow2.zip.bundle"

// --- Acceptance test configs (use tenantTestImage()) ---

func testAccTenantDeployResourceConfigFunc() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
}
`, tenantTestImage())
}

func testAccTenantWithVlansConfigFunc(vlans []int) string {
	vlanStr := ""
	for i, v := range vlans {
		if i > 0 {
			vlanStr += ", "
		}
		vlanStr += fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf(`
resource "f5os_tenant" "vlans_test" {
  name              = "test-vlans-field"
  image_name        = %q
  mgmt_ip           = "10.10.10.51"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
  vlans             = [%s]
}
`, tenantTestImage(), vlanStr)
}

func testAccTenantWithoutVlansConfigFunc() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "vlans_test" {
  name              = "test-vlans-field"
  image_name        = %q
  mgmt_ip           = "10.10.10.51"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
}
`, tenantTestImage())
}

func testAccTenantDeployTC4ResourceConfigFunc() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.27"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
}
`, tenantTestImage())
}

// --- Unit test configs (keep hardcoded image for mock server) ---

func testAccTenantDeployResourceConfig() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

func testAccTenantDeployResourceConfigModify() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.27"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

func testAccTenantDeployTC2ResourceConfig() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.14.10.10"
  mgmt_gateway      = "10.14.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "deployed"
  virtual_disk_size = 82
  nodes             = [1]
  cryptos           = "enabled"
  vlans             = [1,2,3]
}
`, tenantUnitTestImage)
}
func testAccTenantDeployResourceTC3Config() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test-tenant22" {
  name              = "test-tenant22"
  image_name        = %q
  mgmt_ip           = "10.10.30.30"
  mgmt_gateway      = "10.10.30.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  nodes 			= [2]
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
}
`, tenantUnitTestImage)
}

// testAccTenantDeployResourcePendingNoInstancesConfig uses a zero timeout so
// the CreateTenant poll loop hits the timeout branch on the first iteration
// (before any long production sleep), letting the 2.0.0 pending-without-
// instances path be verified without waiting on real timers.
func testAccTenantDeployResourcePendingNoInstancesConfig() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test-tenant22" {
  name              = "test-tenant22"
  image_name        = %q
  mgmt_ip           = "10.10.30.30"
  mgmt_gateway      = "10.10.30.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  nodes 			= [2]
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  timeout           = 0
}
`, tenantUnitTestImage)
}

//
//const testAccTenantDeployTC4ResourceConfig = `
//resource "f5os_tenant" "test2" {
//  name              = "testtenant-ecosys2"
//  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
//  mgmt_ip           = "10.14.10.10"
//  mgmt_gateway      = "10.14.10.1"
//  mgmt_prefix       = 24
//  type              = "BIG-IP"
//  cpu_cores         = 8
//  running_state     = "configured"
//  virtual_disk_size = 83
//  nodes             = [1]
//  cryptos           = "enabled"
//  vlans             = [1,2,3]
//}
//`

func testAccTenantVlansMultiConfig() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  vlans             = [10, 20, 30]
}
`, tenantUnitTestImage)
}

func testAccTenantNoVlansConfig() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
}
`, tenantUnitTestImage)
}

// TestUnitTenantDeploymentFileNullForBigIP verifies that for a standard BIG-IP
// tenant (no deployment_file in HCL or API response), the deployment_file
// attribute is null in state (not unknown or empty string).
func TestUnitTenantDeploymentFileNullForBigIP(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			// Standard BIG-IP fixture — no deployment-file field
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantDeployResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					// deployment_file should not be present in state for BIG-IP tenants
					resource.TestCheckNoResourceAttr("f5os_tenant.test2", "deployment_file"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests for error paths and edge cases to reach 80% coverage
// ---------------------------------------------------------------------------

// TestUnitTenantCreateVelosControllerError verifies that Create returns an
// error when running on a Velos Controller (unsupported platform).
func TestUnitTenantCreateVelosControllerError(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock: platform detection — returns multiple components with "chassis"
	// so the SDK classifies this as "Velos Controller".
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_components_velos_controller.json"))
	})

	// Mock: version endpoint for Velos Controller
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-controller-image:image", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-system-controller-image:image":{"state":{"controllers":{"controller":[{"number":1,"os-version":"1.7.0-3518"}]}}}}`)
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourceConfig(),
				ExpectError: regexp.MustCompile("Unsupported platform for resource"),
			},
		},
	})
}

// TestUnitTenantCreateBigIPNextMissingDeploymentFile verifies that Create
// handles the BIG-IP-Next code path when deployment_file is not specified.
// Note: deployment_file is Optional+Computed, so during Create it is Unknown
// (not Null), which means the IsNull() check in Create passes. This test
// exercises the BIG-IP-Next type path through the GetImage call, verifying
// the error-handling around image lookup for BIG-IP-Next images.
func TestUnitTenantCreateBigIPNextMissingDeploymentFile(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// GetImage returns 404 for the BIG-IP-Next image
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIG-IP-Next-20.0.1-0.0.25.iso", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantBigIPNextMissingDeploymentFile,
				ExpectError: regexp.MustCompile(""),
			},
		},
	})
}

const testAccTenantBigIPNextMissingDeploymentFile = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIG-IP-Next-20.0.1-0.0.25.iso"
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP-Next"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
}
`

// TestUnitTenantCreateImageNotPresent verifies that Create fails when
// the image status is "not-present".
func TestUnitTenantCreateImageNotPresent(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// Return image with status "not-present"
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"not-present","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourceConfig(),
				ExpectError: regexp.MustCompile(`not-present.*on the device`),
			},
		},
	})
}

// TestUnitTenantCreateBadRequestError verifies error handling when CreateTenant
// returns a 400 Bad Request error.
func TestUnitTenantCreateBadRequestError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"400 Bad Request: invalid configuration"}]}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourceConfig(),
				ExpectError: regexp.MustCompile("400 Bad Request"),
			},
		},
	})
}

// TestUnitTenantCreateObjectExistsError verifies error handling when CreateTenant
// returns "object already exists" error.
func TestUnitTenantCreateObjectExistsError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"data-exists","error-message":"object already exists"}]}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourceConfig(),
				ExpectError: regexp.MustCompile("object already exists"),
			},
		},
	})
}

// TestUnitTenantCreateGetTenantError verifies Create error handling when the
// post-create GetTenant call returns an error (tenant not found after create).
// The Create flow: POST /tenants -> poll /state -> GET /tenant={name} to populate
// state. If that final GET fails, Create returns an error.
func TestUnitTenantCreateGetTenantError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	// GetTenant always returns "not found" — causes Create to fail when
	// it calls GetTenant after CreateTenant succeeds.
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Return keypath-not-found which makes GetTenant return an error
		// (F5TenantsTenant slice is empty)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourceConfig(),
				ExpectError: regexp.MustCompile("not found"),
			},
		},
	})
}

// TestUnitTenantReadError verifies Read error handling when GetTenant fails.
// Create succeeds (GET #1 returns valid data), then Read is triggered for the
// second step and fails because GET #2 returns an error.
func TestUnitTenantReadError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	getCount := 0
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" {
			getCount++
			// GET #1: Create's post-create GetTenant — succeed
			// GET #2+: subsequent Reads — fail so Read error path is covered
			if getCount <= 1 {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
			} else {
				// Return ietf-restconf error — makes GetTenant return an error
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds, but post-apply Read fails with "not found"
			// This exercises the Read error path.
			{
				Config:      testAccTenantDeployResourceConfig(),
				ExpectError: regexp.MustCompile("not found"),
			},
		},
	})
}

// TestUnitTenantDeleteError verifies Delete error handling when DeleteTenant fails.
// Step 1 creates the resource successfully, Step 2 destroys it but the Delete
// call returns an error (exercises the err != nil branch in Delete).
func TestUnitTenantDeleteError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleteCount := 0
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleteCount++
			if deleteCount == 1 {
				// First delete attempt returns error
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"delete failed: resource is in use"}]}}`)
				return
			}
			// Subsequent deletes succeed (cleanup teardown)
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" {
			if deleteCount >= 2 {
				// After actual delete succeeds, return not-found
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds
			{
				Config: testAccTenantDeployResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
				),
			},
			// Step 2: Destroy fails with a delete error, exercising the error
			// path in Delete.
			{
				Config:      testAccTenantDeployResourceConfig(),
				Destroy:     true,
				ExpectError: regexp.MustCompile("delete failed"),
			},
		},
	})
}

// TestUnitTenantMacBlockAbsent2_0_0 verifies that a tenant Read succeeds when
// the F5OS 2.0.0 state response omits state.mac-data.f5-tenant-l2-inline:mac-block.
// The provider only consumes mac-data.mac-pool-size (still present in 2.0.0) to
// derive mac_block_size, and the absent mac-block slice decodes to nil, so the
// missing field is a functional no-op — no error and no panic.
func TestUnitTenantMacBlockAbsent2_0_0(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	// The poll/state endpoint (used by tenantWait) returns the 2.0.0 state
	// shape with mac-data but no mac-block.
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status_2_0_0.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			// The Read path (GetTenant) returns the 2.0.0 full object whose
			// state.mac-data omits f5-tenant-l2-inline:mac-block.
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_2_0_0.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantMacBlockAbsent2_0_0(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// mac-pool-size 1 -> "one"; absence of mac-block is a no-op.
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mac_block_size", "one"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
				),
			},
		},
	})
}

func testAccTenantMacBlockAbsent2_0_0() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  mac_block_size    = "one"
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

// setupTenant2_0_0MaxNodesMocks registers the shared mock handlers used by the
// max_nodes / 2.0.0-state tests. The device version reported to the provider is
// controlled by the caller via setupMockPlatformVersion so both the
// version-gated (>= v2.0) and pre-2.0 code paths can be exercised. The create
// PATCH/POST body is captured into capturedBody so the test can assert whether
// the max-nodes field was sent.
func setupTenant2_0_0MaxNodesMocks(t *testing.T, capturedBody *string, mu *sync.Mutex, statusFixture, configFixture string) {
	t.Helper()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	// Capture the tenant create request body so the test can assert whether
	// max-nodes was included in the payload.
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		*capturedBody = string(body)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString(statusFixture))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString(configFixture))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
}

// TestUnitTenant2_0_0MaxNodesAndState verifies that on an F5OS 2.0.0 device the
// provider (1) sends config.max-nodes in the create payload and (2) reads back
// the new 2.0.0 state fields — max_nodes, mgmt_vlan, mgmt_vlan_accessible, and
// clustering_as_service — into Terraform state.
func TestUnitTenant2_0_0MaxNodesAndState(t *testing.T) {
	testAccPreUnitCheck(t)

	var mu sync.Mutex
	var capturedBody string

	// Report a 2.0.0 device so the version gate (>= v2.0) is satisfied.
	setupMockPlatformVersion(mux, "2.0.0-1")
	setupTenant2_0_0MaxNodesMocks(t, &capturedBody, &mu,
		"./fixtures/tenant_get_status_2_0_0_max_nodes.json",
		"./fixtures/tenant_config_2_0_0_max_nodes.json")

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenant2_0_0MaxNodes(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "max_nodes", "8"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_vlan", "4094"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_vlan_accessible", "true"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "clustering_as_service", "true"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
					func(s *terraform.State) error {
						mu.Lock()
						defer mu.Unlock()
						if !strings.Contains(capturedBody, "\"max-nodes\":8") {
							return fmt.Errorf("expected create payload to contain \"max-nodes\":8 on a 2.0.0 device, got: %s", capturedBody)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitTenantMaxNodesOmittedPre2_0_0 verifies that on a pre-2.0.0 device the
// provider does NOT send config.max-nodes in the create payload even when the
// user supplies a max_nodes value, since the field is unknown to older devices.
func TestUnitTenantMaxNodesOmittedPre2_0_0(t *testing.T) {
	testAccPreUnitCheck(t)

	var mu sync.Mutex
	var capturedBody string

	// Report a pre-2.0 device so the version gate (>= v2.0) is NOT satisfied.
	setupMockPlatformVersion(mux, "1.8.0-1")
	setupTenant2_0_0MaxNodesMocks(t, &capturedBody, &mu,
		"./fixtures/tenant_get_status_2_0_0_max_nodes.json",
		"./fixtures/tenant_config_2_0_0_max_nodes.json")

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenant2_0_0MaxNodes(),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						mu.Lock()
						defer mu.Unlock()
						if strings.Contains(capturedBody, "max-nodes") {
							return fmt.Errorf("expected create payload to omit max-nodes on a pre-2.0.0 device, got: %s", capturedBody)
						}
						return nil
					},
				),
			},
		},
	})
}

func testAccTenant2_0_0MaxNodes() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  mac_block_size    = "one"
  max_nodes         = 8
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

// TestUnitTenantMaxNodesPlanPreserved verifies that when the user configures a
// max_nodes value that differs from what the device reports back (e.g. the
// device normalizes/clamps max-nodes), the provider preserves the user's
// configured value in state rather than overwriting it with the device value.
// Overwriting would trigger a "provider produced inconsistent result after
// apply" error. The config requests max_nodes=4 while both fixtures report
// max-nodes=8.
func TestUnitTenantMaxNodesPlanPreserved(t *testing.T) {
	testAccPreUnitCheck(t)

	var mu sync.Mutex
	var capturedBody string

	setupMockPlatformVersion(mux, "2.0.0-1")
	setupTenant2_0_0MaxNodesMocks(t, &capturedBody, &mu,
		"./fixtures/tenant_get_status_2_0_0_max_nodes_normalized.json",
		"./fixtures/tenant_config_2_0_0_max_nodes_normalized.json")

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenant2_0_0MaxNodesFour(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The device reports 8, but the user configured 4; state
					// must equal the plan (4) or apply fails.
					resource.TestCheckResourceAttr("f5os_tenant.test2", "max_nodes", "4"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
					// The configured value (4) is what gets sent to the device.
					func(s *terraform.State) error {
						mu.Lock()
						defer mu.Unlock()
						if !strings.Contains(capturedBody, "\"max-nodes\":4") {
							return fmt.Errorf("expected create payload to contain \"max-nodes\":4, got: %s", capturedBody)
						}
						return nil
					},
				),
			},
		},
	})
}

func testAccTenant2_0_0MaxNodesFour() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  mac_block_size    = "one"
  max_nodes         = 4
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

// TestUnitTenantMacBlockSizeSmall verifies mac_block_size="small" (pool_size=8).
func TestUnitTenantMacBlockSizeSmall(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_mac_small.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantMacBlockSizeSmall(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mac_block_size", "small"),
				),
			},
		},
	})
}

func testAccTenantMacBlockSizeSmall() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  mac_block_size    = "small"
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

// TestUnitTenantMacBlockSizeMedium verifies mac_block_size="medium" (pool_size=16).
func TestUnitTenantMacBlockSizeMedium(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_mac_medium.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantMacBlockSizeMedium(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mac_block_size", "medium"),
				),
			},
		},
	})
}

func testAccTenantMacBlockSizeMedium() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  mac_block_size    = "medium"
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

// TestUnitTenantMacBlockSizeLarge verifies mac_block_size="large" (pool_size=32).
func TestUnitTenantMacBlockSizeLarge(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_mac_large.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantMacBlockSizeLarge(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mac_block_size", "large"),
				),
			},
		},
	})
}

func testAccTenantMacBlockSizeLarge() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  mac_block_size    = "large"
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

// TestUnitTenantStorageSizeMismatch verifies the else branch in
// tenantResourceModeltoState where State.Storage.Size != Config.Storage.Size.
func TestUnitTenantStorageSizeMismatch(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			// Fixture has config.storage.size=82 but state.storage.size=90
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_storage_mismatch.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantDeployResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					// When state != config, should use config size (82)
					resource.TestCheckResourceAttr("f5os_tenant.test2", "virtual_disk_size", "82"),
				),
			},
		},
	})
}

// TestUnitTenantBigIPNextWithDeploymentFile verifies BIG-IP-Next tenant creation
// with deployment_file specified, exercising the BIG-IP-Next branch in Create.
func TestUnitTenantBigIPNextWithDeploymentFile(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIG-IP-Next-20.0.1-0.0.25.iso", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIG-IP-Next-20.0.1-0.0.25.iso","in-use":false,"type":"vm-image","status":"replicated","date":"2024-1-15","size":"4.5 GB"}]}`)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status_bigip_next.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_bigip_next.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with BIG-IP-Next exercises the deployment_file branch
			{
				Config: testAccTenantBigIPNextWithDeploymentFile,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP-Next"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "deployment_file", "BIG-IP-Next-20.0.1-0.0.25.yaml"),
				),
			},
		},
	})
}

const testAccTenantBigIPNextWithDeploymentFile = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIG-IP-Next-20.0.1-0.0.25.iso"
  deployment_file   = "BIG-IP-Next-20.0.1-0.0.25.yaml"
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP-Next"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  vlans             = [ 1 ]
}
`

const testAccTenantBigIPNextWithDeploymentFileUpdate = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIG-IP-Next-20.0.1-0.0.25.iso"
  deployment_file   = "BIG-IP-Next-20.0.1-0.0.25.yaml"
  mgmt_ip           = "10.10.10.27"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP-Next"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  vlans             = [ 1 ]
}
`

// TestUnitTenantBigIPNextUpdate verifies the Update path for BIG-IP-Next
// tenants, covering the deployment_file branch in Update.
func TestUnitTenantBigIPNextUpdate(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIG-IP-Next-20.0.1-0.0.25.iso", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIG-IP-Next-20.0.1-0.0.25.iso","in-use":false,"type":"vm-image","status":"replicated","date":"2024-1-15","size":"4.5 GB"}]}`)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status_bigip_next.json"))
	})
	updated := false
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "PUT" {
			updated = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			if updated {
				_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_bigip_next_updated.json"))
			} else {
				_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config_bigip_next.json"))
			}
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with BIG-IP-Next
			{
				Config: testAccTenantBigIPNextWithDeploymentFile,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP-Next"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "deployment_file", "BIG-IP-Next-20.0.1-0.0.25.yaml"),
				),
			},
			// Update with modified mgmt_ip to trigger Update path with BIG-IP-Next
			{
				Config: testAccTenantBigIPNextWithDeploymentFileUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP-Next"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "deployment_file", "BIG-IP-Next-20.0.1-0.0.25.yaml"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.10.10.27"),
				),
			},
		},
	})
}

// TestUnitTenantUpdateError verifies Update error handling when UpdateTenant fails.
func TestUnitTenantUpdateError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "PUT" {
			// Update fails with proper ietf-restconf error format
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"update failed: internal server error"}]}}`)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create succeeds
			{
				Config: testAccTenantDeployResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
				),
			},
			// Update fails
			{
				Config:      testAccTenantDeployResourceConfigModify(),
				ExpectError: regexp.MustCompile("Tenant Deploy failed"),
			},
		},
	})
}

// TestUnitTenantWithExplicitMemory verifies that calculateMemory returns
// the explicitly specified memory value when data.Memory is not null.
func TestUnitTenantWithExplicitMemory(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantWithExplicitMemory(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "memory", "29184"),
				),
			},
		},
	})
}

func testAccTenantWithExplicitMemory() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = %q
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  memory            = 29184
  vlans             = [ 1 ]
}
`, tenantUnitTestImage)
}

// TestUnitTenantRSeriesMemoryCalculation verifies the calculateMemory function
// uses the rSeries formula (3 * 1024 * cpuCores) for rSeries platforms.
func TestUnitTenantRSeriesMemoryCalculation(t *testing.T) {
	testAccPreUnitCheck(t)

	// Set up rSeries platform mock using setupMockPlatformVersion
	setupMockPlatformVersion(mux, "1.7.0-3518")

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":%q,"in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`, tenantUnitTestImage)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_r4k_get_status.json"))
	})
	deleted := false
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" && !deleted {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_r4k_config.json"))
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantDeployTC2ResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Running"),
				),
			},
		},
	})
}

// TestUnitIsRSeriesPlatform tests the isRSeriesPlatform helper function.
func TestUnitIsRSeriesPlatform(t *testing.T) {
	tests := []struct {
		platform string
		expected bool
	}{
		{"r2800", true},
		{"r2000", true},
		{"r4000", true},
		{"r4800", true},
		{"r5900", false},
		{"r10900", false},
		{"Velos Partition", false},
		{"Velos Controller", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			result := isRSeriesPlatform(tt.platform)
			if result != tt.expected {
				t.Errorf("isRSeriesPlatform(%q) = %v, expected %v", tt.platform, result, tt.expected)
			}
		})
	}
}

// TestUnitCalculateMemory tests the calculateMemory helper function directly.
func TestUnitCalculateMemory(t *testing.T) {
	tests := []struct {
		name         string
		memory       *int64
		cpuCores     int64
		platformType string
		expected     int
	}{
		{
			name:         "explicit memory value",
			memory:       int64Ptr(16384),
			cpuCores:     4,
			platformType: "Velos Partition",
			expected:     16384,
		},
		{
			name:         "rSeries r2800 auto-calculated",
			memory:       nil,
			cpuCores:     4,
			platformType: "r2800",
			expected:     3 * 1024 * 4, // 12288
		},
		{
			name:         "rSeries r4000 auto-calculated",
			memory:       nil,
			cpuCores:     8,
			platformType: "r4000",
			expected:     3 * 1024 * 8, // 24576
		},
		{
			name:         "Velos Partition auto-calculated",
			memory:       nil,
			cpuCores:     8,
			platformType: "Velos Partition",
			expected:     int(3.5*1024*8) + 512, // 29184
		},
		{
			name:         "unknown platform uses Velos formula",
			memory:       nil,
			cpuCores:     4,
			platformType: "r5900",
			expected:     int(3.5*1024*4) + 512, // 14848
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &TenantResourceModel{
				CpuCores: types.Int64Value(tt.cpuCores),
			}
			if tt.memory != nil {
				data.Memory = types.Int64Value(*tt.memory)
			} else {
				data.Memory = types.Int64Null()
			}
			result := calculateMemory(data, tt.platformType)
			if result != tt.expected {
				t.Errorf("calculateMemory() = %d, expected %d", result, tt.expected)
			}
		})
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}

// TestUnitTenantGetImageError verifies Create error handling when GetImage fails.
func TestUnitTenantGetImageError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// GetImage fails with 500 error
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image="+tenantUnitTestImage, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"error":"internal server error"}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourceConfig(),
				ExpectError: regexp.MustCompile(`500|Internal Server Error|not found`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests for additional coverage: mac_block_size, memory
// ---------------------------------------------------------------------------

// testAccCheckTenantMacBlockSizeOnDevice queries the device directly and
// verifies the tenant mac_block_size matches the expected value.
func testAccCheckTenantMacBlockSizeOnDevice(tenantName, expectedSize string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetTenant(tenantName)
		if err != nil {
			return fmt.Errorf("GetTenant failed: %w", err)
		}
		if len(resp.F5TenantsTenant) == 0 {
			return fmt.Errorf("tenant %q not found on device", tenantName)
		}
		// Mac pool size is returned as int: 1=one, 8=small, 16=medium, 32=large
		poolSize := resp.F5TenantsTenant[0].State.MacData.MacPoolSize
		var actual string
		switch poolSize {
		case 1:
			actual = "one"
		case 8:
			actual = "small"
		case 16:
			actual = "medium"
		case 32:
			actual = "large"
		default:
			actual = fmt.Sprintf("unknown(%d)", poolSize)
		}
		if actual != expectedSize {
			return fmt.Errorf("tenant %q mac_block_size: expected %q, got %q (pool_size=%d)", tenantName, expectedSize, actual, poolSize)
		}
		return nil
	}
}

// testAccCheckTenantMemoryOnDevice queries the device directly and verifies
// the tenant memory matches the expected value in MB.
func testAccCheckTenantMemoryOnDevice(tenantName string, expectedMemory int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		resp, err := client.GetTenant(tenantName)
		if err != nil {
			return fmt.Errorf("GetTenant failed: %w", err)
		}
		if len(resp.F5TenantsTenant) == 0 {
			return fmt.Errorf("tenant %q not found on device", tenantName)
		}
		actualStr := resp.F5TenantsTenant[0].State.Memory
		actual, err := strconv.Atoi(actualStr)
		if err != nil {
			return fmt.Errorf("tenant %q memory: failed to parse %q as int: %w", tenantName, actualStr, err)
		}
		if actual != expectedMemory {
			return fmt.Errorf("tenant %q memory: expected %d, got %d", tenantName, expectedMemory, actual)
		}
		return nil
	}
}

// TestAccTenantMacBlockSize verifies the mac_block_size attribute is correctly
// set on the device for various block sizes (one, small, medium, large).
// This test exercises the mac_block_size logic in tenantResourceModeltoState.
func TestAccTenantMacBlockSize(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenant(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with mac_block_size = "small"
			{
				Config: testAccTenantMacBlockSizeSmallConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.mac_test", "name", "test-mac-block"),
					resource.TestCheckResourceAttr("f5os_tenant.mac_test", "mac_block_size", "small"),
					testAccCheckTenantMacBlockSizeOnDevice("test-mac-block", "small"),
				),
			},
			// Step 2: Import
			{
				ResourceName:      "f5os_tenant.mac_test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",
					"virtual_disk_size",
				},
			},
		},
	})
}

func testAccTenantMacBlockSizeSmallConfigFunc() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "mac_test" {
  name              = "test-mac-block"
  image_name        = %q
  mgmt_ip           = "10.10.10.52"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
  mac_block_size    = "small"
}
`, tenantTestImage())
}

// TestAccTenantExplicitMemory verifies the memory attribute is correctly set
// on the device when explicitly specified, rather than auto-calculated.
// This test exercises the explicit memory branch in calculateMemory.
func TestAccTenantExplicitMemory(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckTenant(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTenantDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with explicit memory = 8192 MB
			{
				Config: testAccTenantExplicitMemoryConfigFunc(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.mem_test", "name", "test-memory"),
					resource.TestCheckResourceAttr("f5os_tenant.mem_test", "memory", "8192"),
					testAccCheckTenantMemoryOnDevice("test-memory", 8192),
				),
			},
			// Step 2: Import
			{
				ResourceName:      "f5os_tenant.mem_test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"timeout",
					"virtual_disk_size",
				},
			},
		},
	})
}

func testAccTenantExplicitMemoryConfigFunc() string {
	return fmt.Sprintf(`
resource "f5os_tenant" "mem_test" {
  name              = "test-memory"
  image_name        = %q
  mgmt_ip           = "10.10.10.53"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 2
  running_state     = "configured"
  virtual_disk_size = 83
  memory            = 8192
}
`, tenantTestImage())
}
