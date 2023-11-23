package provider

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLagInterfaceCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccLagInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "11"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.2", "12"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "1.0"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.1", "2.0"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_lag.test_lag",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLagInterfaceCreateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccLagInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "11"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.2", "12"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "1.0"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.1", "2.0"),
				),
			},
			{
				Config: testAccLagInterfaceCreateTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "11"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "12"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.2", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "2.0"),
				),
			},
		},
	})
}

func TestAccLagInterfaceCreateUnitTC3Resource(t *testing.T) {
	// Define our mocked connection object
	testAccPreUnitCheck(t)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/rseries_platform_state_ok.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-image:image/state/install", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/rseries_platform_version.json"))
	})
	mux.HandleFunc("/restconf/data/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
		}
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "27"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "28"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "1.1"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.1", "1.2"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_lag.test_lag",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLagInterfaceCreateUnitTC4Resource(t *testing.T) {
	// Define our mocked connection object
	testAccPreUnitCheck(t)
	count := 0
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/rseries_platform_state_ok.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-image:image/state/install", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/rseries_platform_version.json"))
	})
	mux.HandleFunc("/restconf/data/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:native-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=27", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=28", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("\n\n\n ############### count LAG:%+v ##############\n\n\n", count)
		if r.Method == "GET" && (count == 0 || count == 1 || count == 2 || count == 3) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
		}
		if r.Method == "GET" && (count == 4 || count == 5 || count == 6 || count == 7) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config_modified.json"))
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
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "27"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "28"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "1.1"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.1", "1.2"),
				),
			},
			{
				Config: testAccInterfaceCreateUnitModifyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "28"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "27"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "29"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "1.2"),
				),
			},
		},
	})
}

const testAccLagInterfaceCreateUnitResourceConfig = `
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  native_vlan = 29
  trunk_vlans = [27, 28]
  members = ["1.1", "1.2"]
}`

const testAccInterfaceCreateUnitModifyResourceConfig = `
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  native_vlan = 28
  trunk_vlans = [27, 29]
  members = ["1.2"]
}`

const testAccLagInterfaceCreateResourceConfig = `
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
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  native_vlan = f5os_vlan.vlan13.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan10.vlan_id,
    f5os_vlan.vlan11.vlan_id,
    f5os_vlan.vlan12.vlan_id
  ]
  members = ["1.0", "2.0"]
}
`
const testAccLagInterfaceCreateTC2ResourceConfig = `
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
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  native_vlan = f5os_vlan.vlan11.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan10.vlan_id,
    f5os_vlan.vlan12.vlan_id,
	f5os_vlan.vlan13.vlan_id
  ]
  members = ["2.0"]
}
`
