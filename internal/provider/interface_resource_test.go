package provider

import (
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

var count = 0

func TestAccInterfaceCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.1", "11"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.2", "12"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_interface.test_interface",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccInterfaceCreateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.1", "11"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.2", "12"),
				),
			},
			{
				Config: testAccInterfaceCreateTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "11"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.1", "12"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "trunk_vlans.2", "13"),
				),
			},
		},
	})
}

func TestAccInterfaceCreateUnitTC3Resource(t *testing.T) {
	// Define our mocked connection object
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
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "/restconf/data/openconfig-interfaces:interfaces" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", "")
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "/restconf/data/openconfig-interfaces:interfaces/interface=1.0" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/interface_get_r5k_status.json"))
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccInterfaceCreateunitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_interface.test_interface",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccInterfaceCreateUnitTC4Resource(t *testing.T) {
	// Define our mocked connection object
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
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "/restconf/data/openconfig-interfaces:interfaces" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", "")
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("\n\n\n ############### count interfaces:%+v ##############\n\n\n", count)
		if r.Method == "GET" && (count == 0 || count == 1 || count == 2) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/interface_get_r5k_status.json"))
		}
		if r.Method == "GET" && (count == 3 || count == 4 || count == 5) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/interface_get_r5k_modified_status.json"))
		}
		count++
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccInterfaceCreateunitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			{
				Config: testAccInterfaceCreateunitmodifyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "12"),
				),
			},
		},
	})
}

const testAccInterfaceCreateunitResourceConfig = `
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  native_vlan = 13
  trunk_vlans = [10,11,12]
}`

const testAccInterfaceCreateunitmodifyResourceConfig = `
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  native_vlan = 12
  trunk_vlans = [10,11,13]
}`

const testAccInterfaceCreateResourceConfig = `
resource "f5os_vlan" "vlan10" {
 vlan_id = 10
 name = "vlan10"
}
resource "f5os_vlan" "vlan11" {
 vlan_id = 11
 name = "vlan11"
}
resource "f5os_vlan" "vlan12" {
 vlan_id = 12
 name = "vlan12"
}
resource "f5os_vlan" "vlan13" {
 vlan_id = 13
 name = "vlan13"
}
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  native_vlan = f5os_vlan.vlan13.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan10.vlan_id,
    f5os_vlan.vlan11.vlan_id,
    f5os_vlan.vlan12.vlan_id
  ]
}
`

const testAccInterfaceCreateTC2ResourceConfig = `
resource "f5os_vlan" "vlan10" {
 vlan_id = 10
 name = "vlan10"
}
resource "f5os_vlan" "vlan11" {
 vlan_id = 11
 name = "vlan11"
}
resource "f5os_vlan" "vlan12" {
 vlan_id = 12
 name = "vlan12"
}
resource "f5os_vlan" "vlan13" {
 vlan_id = 13
 name = "vlan13"
}
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  native_vlan = f5os_vlan.vlan11.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan10.vlan_id,
    f5os_vlan.vlan12.vlan_id,
	f5os_vlan.vlan13.vlan_id
  ]
}
`
