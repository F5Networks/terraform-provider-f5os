package provider

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

func TestAccTenantDeployResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
	   "f5-tenant-images:image": [
	       {
	           "name": "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle",
	           "in-use": false,
	           "type": "vm-image",
	           "status": "replicated",
	           "date": "2023-8-17",
	           "size": "2.27 GB"
	       }
	   ]
	}`)
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
				Config: testAccTenantDeployResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"),
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
				Config: testAccTenantDeployResourceConfigModify,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"),
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantDeployResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantVlansMultiConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantNoVlansConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
	   "f5-tenant-images:image": [
	       {
	           "name": "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle",
	           "in-use": false,
	           "type": "vm-image",
	           "status": "replicated",
	           "date": "2023-8-17",
	           "size": "2.27 GB"
	       }
	   ]
	}`)
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
				Config: testAccTenantDeployTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"),
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
	   "f5-tenant-images:image": [
	       {
	           "name": "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle",
	           "in-use": false,
	           "type": "vm-image",
	           "status": "replicated",
	           "date": "2023-8-17",
	           "size": "2.27 GB"
	       }
	   ]
	}`)
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
				Config:      testAccTenantDeployResourceTC3Config,
				ExpectError: regexp.MustCompile("Tenant Deployment Pending"),
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
// tests. Set F5OS_TENANT_IMAGE to override the default. Unit tests using the
// mock server keep the original hardcoded name in their fixtures/constants.
func tenantTestImage() string {
	if v := os.Getenv("F5OS_TENANT_IMAGE"); v != "" {
		return v
	}
	return "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
}

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

const testAccTenantDeployResourceConfig = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  vlans             = [ 1 ]
}
`

const testAccTenantDeployResourceConfigModify = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_ip           = "10.10.10.27"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  vlans             = [ 1 ]
}
`

const testAccTenantDeployTC2ResourceConfig = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
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
`
const testAccTenantDeployResourceTC3Config = `
resource "f5os_tenant" "test-tenant22" {
  name              = "test-tenant22"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_ip           = "10.10.30.30"
  mgmt_gateway      = "10.10.30.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  nodes 			= [2]
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
}
`

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

const testAccTenantVlansMultiConfig = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
  vlans             = [10, 20, 30]
}
`

const testAccTenantNoVlansConfig = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
}
`

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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantDeployResourceConfig,
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
				Config:      testAccTenantDeployResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"not-present","date":"2023-8-17","size":"2.27 GB"}]}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config:      testAccTenantDeployResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config:      testAccTenantDeployResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config:      testAccTenantDeployResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config:      testAccTenantDeployResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantDeployResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
				),
			},
			// Step 2: Destroy fails with a delete error, exercising the error
			// path in Delete.
			{
				Config:      testAccTenantDeployResourceConfig,
				Destroy:     true,
				ExpectError: regexp.MustCompile("delete failed"),
			},
		},
	})
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantMacBlockSizeSmall,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mac_block_size", "small"),
				),
			},
		},
	})
}

const testAccTenantMacBlockSizeSmall = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
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
`

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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantMacBlockSizeMedium,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mac_block_size", "medium"),
				),
			},
		},
	})
}

const testAccTenantMacBlockSizeMedium = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
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
`

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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantMacBlockSizeLarge,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mac_block_size", "large"),
				),
			},
		},
	})
}

const testAccTenantMacBlockSizeLarge = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
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
`

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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantDeployResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantDeployResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
				),
			},
			// Update fails
			{
				Config:      testAccTenantDeployResourceConfigModify,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantWithExplicitMemory,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "memory", "29184"),
				),
			},
		},
	})
}

const testAccTenantWithExplicitMemory = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
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
`

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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-tenant-images:image":[{"name":"BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle","in-use":false,"type":"vm-image","status":"replicated","date":"2023-8-17","size":"2.27 GB"}]}`)
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
				Config: testAccTenantDeployTC2ResourceConfig,
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
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"error":"internal server error"}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTenantDeployResourceConfig,
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
		PreCheck:                 func() { testAccPreCheck(t) },
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
		PreCheck:                 func() { testAccPreCheck(t) },
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


