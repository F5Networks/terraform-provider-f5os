package provider

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestAccVlanCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan2"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_vlan.vlan-id",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccVlanCreateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan2"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			{
				Config: testAccVlanCreateResourceTC2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan3"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
		},
	})
}

func TestAccVlanCreateUnitTC1Resource(t *testing.T) {
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
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
    "openconfig-vlan:vlan": [
        {
            "vlan-id": 400,
            "config": {
                "vlan-id": 400,
                "name": "mytestvlan2"
            }
        }
    ]
}`)
	})
	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan2"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_vlan.vlan-id",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccVlanCreateUnitTC2Resource(t *testing.T) {
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
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && (count == 0 || count == 1 || count == 2) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"openconfig-vlan:vlan": [{
            "vlan-id": 400,
            "config": {
                "vlan-id": 400,
                "name": "mytestvlan2"
            }}]}`)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"openconfig-vlan:vlan": [{
            "vlan-id": 400,
            "config": {
                "vlan-id": 400,
                "name": "mytestvlan3"
            }}]}`)
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
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan2"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			{
				Config: testAccVlanCreateResourceTC2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan3"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
		},
	})
}

const testAccVlanCreateResourceConfig = `
resource "f5os_vlan" "vlan-id" {
 vlan_id = 400
 name = "mytestvlan2"
}
`

const testAccVlanCreateResourceTC2Config = `
resource "f5os_vlan" "vlan-id" {
 vlan_id = 400
 name = "mytestvlan3"
}
`
