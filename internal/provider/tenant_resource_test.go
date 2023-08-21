package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestAccTenantDeployResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
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
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.0", "1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.#", "1"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_tenant.test2",
				ImportState:       true,
				ImportStateVerify: false,
			},
		},
	})
}

func TestAccTenantDeployResourceTC5(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantDeployTC5,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.velos_bigip_next_tenant_tc5", "id", "testtenant-ecosys03"),
					resource.TestCheckResourceAttr("f5os_tenant.velos_bigip_next_tenant_tc5", "name", "testtenant-ecosys03"),
					resource.TestCheckResourceAttr("f5os_tenant.velos_bigip_next_tenant_tc5", "image_name", "BIG-IP-Next-20.0.1-2.123.17"),
					resource.TestCheckResourceAttr("f5os_tenant.velos_bigip_next_tenant_tc5", "mgmt_ip", "100.10.100.110"),
					resource.TestCheckResourceAttr("f5os_tenant.velos_bigip_next_tenant_tc5", "mgmt_gateway", "100.10.100.1"),
					resource.TestCheckResourceAttr("f5os_tenant.velos_bigip_next_tenant_tc5", "type", "BIG-IP-Next"),
					resource.TestCheckResourceAttr("f5os_tenant.velos_bigip_next_tenant_tc5", "status", "Configured"),
				),
			},
		},
	})
}

func TestAccTenantDeployResourceTC4(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
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
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.0", "1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.#", "1"),
				),
			},
			{
				Config: testAccTenantDeployTC4ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.10.10.27"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_gateway", "10.10.10.1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.0", "1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "vlans.#", "1"),
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
		w.WriteHeader(http.StatusOK)
		if r.Method == "GET" && (count == 0 || count == 1 || count == 2) {
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
		}
		if r.Method == "GET" && (count == 3 || count == 4 || count == 5) {
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_update_config.json"))
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
	mux.HandleFunc("/restconf/data/f5-tenants:tenants", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_r4k_get_status.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_r4k_config.json"))
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
				ExpectError: regexp.MustCompile("Tenant Deploy failed, got error"),
			},
		},
	})
}

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

const testAccTenantDeployTC4ResourceConfig = `
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
//  virtual_disk_size = 82
//  nodes             = [1]
//  cryptos           = "enabled"
//  vlans             = [1,2,3]
//}
//`

const testAccTenantDeployTC5 = `
resource "f5os_tenant" "velos_bigip_next_tenant_tc5" {
	cpu_cores       = 4
	cryptos         = "enabled"
	deployment_file = "BIG-IP-Next-20.0.1-2.123.17.yaml"
	image_name = "BIG-IP-Next-20.0.1-2.123.17"
	mgmt_gateway           = "100.10.100.1"
	mgmt_ip                = "100.10.100.110"
	mgmt_prefix            = 24
	dag_ipv6_prefix_length = 100
	mac_block_size         = "medium"
	name                   = "testtenant-ecosys03"
	nodes                  = [2]
	running_state          = "configured"
	timeout           = 600
	type              = "BIG-IP-Next"
	virtual_disk_size = 30
  }
`
