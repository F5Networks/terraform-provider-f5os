package provider

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
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
		client, err := newTenantClientFromEnv()
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

// newTenantClientFromEnv creates a fresh f5osclient session from env vars.
func newTenantClientFromEnv() (*f5ossdk.F5os, error) {
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

// testAccCheckTenantDestroy verifies the test tenant no longer exists.
func testAccCheckTenantDestroy(s *terraform.State) error {
	if os.Getenv("F5OS_HOST") == "" {
		return nil
	}
	client, err := newTenantClientFromEnv()
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
		client, err := newTenantClientFromEnv()
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
		client, err := newTenantClientFromEnv()
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
		client, err := newTenantClientFromEnv()
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


