package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// HTTPGetter is an interface for an http client
type HTTPGetter interface {
	Get(url string) (*http.Response, error)
	//Do(req *http.Request) (*http.Response, error)
}

// MockHTTPGetter is a mock implementation of an HTTPGetter
type MockHTTPGetter struct {
	mock.Mock
}

// Get is a mocked implementation of an HTTP Get request
func (m *MockHTTPGetter) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}
func TestAccTenantDeployResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		//IsUnitTest:               true,
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
				ImportStateVerify: true,
			},
		},
	})
}

func TestUnitTenantDeployResource(t *testing.T) {
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
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_get_status.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenants:tenants/tenant=testtenant-ecosys2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_config.json"))
	})
	// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		//PreCheck:                 func() { testAccPreUnitCheck(t) },
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
		},
	})
}

func TestUnitTenantDeployResourceTC2(t *testing.T) {
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
				Config:      testAccTenantDeployResourceTC2Config,
				ExpectError: regexp.MustCompile("Tenant Deploy failed, got error"),
			},
		},
	})
}

const testAccTenantDeployResourceConfig = `
resource "f5os_tenant" "" {
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

const testAccTenantDeployResourceTC2Config = `
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
