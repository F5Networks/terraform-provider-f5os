package provider

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func TestUnitInterfaceCreateTC3Resource(t *testing.T) {
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

func TestUnitInterfaceCreateTC4Resource(t *testing.T) {
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

// ---------------------------------------------------------------------------
// Unit test mock helpers
// ---------------------------------------------------------------------------

// setupInterfaceMockProviderEndpoints registers the standard provider-level
// mock handlers (auth, platform, vlans) for interface unit tests.
func setupInterfaceMockProviderEndpoints() {
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
		w.WriteHeader(http.StatusOK)
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create rejects Velos Controller platform
// ---------------------------------------------------------------------------

func TestUnitInterfaceCreateVelosControllerError(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	// Return Velos Controller platform detection response
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_components_velos_controller.json"))
	})
	// Version endpoint for Velos Controller
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-controller-image:image", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-system-controller-image:image":{"state":{"controllers":{"controller":[{"number":1,"os-version":"1.7.0-3518"}]}}}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccInterfaceCreateunitResourceConfig,
				ExpectError: regexp.MustCompile(`Client Error|supported with Velos Partition`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when UpdateInterface returns an error
// ---------------------------------------------------------------------------

func TestUnitInterfaceCreateUpdateError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupInterfaceMockProviderEndpoints()

	// getSwitchedVlans succeeds but PatchRequest fails
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		// PATCH for UpdateInterface returns error
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"interface update failed"}]}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccInterfaceCreateunitResourceConfig,
				ExpectError: regexp.MustCompile(`F5OS Client Error|interface update failed|Updating Interface failed`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when GetInterface returns an error after successful
// UpdateInterface (covers the post-create GetInterface error path)
// ---------------------------------------------------------------------------

func TestUnitInterfaceCreateGetInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupInterfaceMockProviderEndpoints()

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// PATCH interfaces succeeds (UpdateInterface)
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// GET interface=1.0 fails (GetInterface)
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"get interface failed"}]}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccInterfaceCreateunitResourceConfig,
				ExpectError: regexp.MustCompile(`F5OS Client Error|Unable to Read/Get Interface`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read fails when GetInterface returns an error during Read refresh
// ---------------------------------------------------------------------------

func TestUnitInterfaceReadGetInterfaceError(t *testing.T) {
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
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// GET interface=1.0: Create's GetInterface succeeds, post-apply Read fails.
	// Create calls GetInterface once; the Read refresh calls it again.
	var getCount int32
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&getCount, 1)
		if n <= 1 {
			// Create's GetInterface call succeeds
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/interface_get_r5k_status.json"))
		} else {
			// Read refresh GetInterface call fails
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"get interface failed"}]}}`)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccInterfaceCreateunitResourceConfig,
				ExpectError: regexp.MustCompile(`F5OS Client Error|Unable to Read/Get Interface`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when UpdateInterface returns an error
// ---------------------------------------------------------------------------

func TestUnitInterfaceUpdateError(t *testing.T) {
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
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Track PATCH calls: first PATCH (Create) succeeds, second (Update) fails
	var patchCount int32
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&patchCount, 1)
		if n <= 1 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"interface update failed"}]}}`)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/interface_get_r5k_status.json"))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds
			{
				Config: testAccInterfaceCreateunitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			// Step 2: Update fails
			{
				Config:      testAccInterfaceCreateunitmodifyResourceConfig,
				ExpectError: regexp.MustCompile(`F5OS Client Error|Update.*failed`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when GetInterface returns an error after
// UpdateInterface succeeds
// ---------------------------------------------------------------------------

func TestUnitInterfaceUpdateGetInterfaceError(t *testing.T) {
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
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// GET interface=1.0: first two succeed (Create + Create's GetInterface + Read),
	// subsequent ones fail (Update's GetInterface)
	var getCount int32
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&getCount, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/interface_get_r5k_status.json"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"get interface failed during update"}]}}`)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds
			{
				Config: testAccInterfaceCreateunitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			// Step 2: Update — UpdateInterface succeeds but post-update
			// GetInterface fails
			{
				Config:      testAccInterfaceCreateunitmodifyResourceConfig,
				ExpectError: regexp.MustCompile(`F5OS Client Error|Unable to Read/Get Interface`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Delete fails when RemoveNativeVlans returns an error
// ---------------------------------------------------------------------------

func TestUnitInterfaceDeleteRemoveNativeVlansError(t *testing.T) {
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
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/interface_get_r5k_status.json"))
	})

	// RemoveNativeVlans DELETE: fail first 3 attempts (one doRequest retry
	// cycle), then succeed for post-test cleanup.
	var nativeDeleteCount int32
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:native-vlan", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&nativeDeleteCount, 1)
		if n <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"remove native vlan failed"}]}}`)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	// Trunk VLAN cleanup handlers
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=10", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=11", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=12", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds
			{
				Config: testAccInterfaceCreateunitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			// Step 2: Remove from config triggers destroy; DELETE native-vlan fails
			{
				Config:      `# empty config triggers destroy`,
				ExpectError: regexp.MustCompile(`Client Error|Removing Native vlan failed`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Delete fails when RemoveTrunkVlans returns an error
// ---------------------------------------------------------------------------

func TestUnitInterfaceDeleteRemoveTrunkVlansError(t *testing.T) {
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
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/interface_get_r5k_status.json"))
	})

	// RemoveNativeVlans succeeds (DELETE native-vlan returns 204)
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:native-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	// RemoveTrunkVlans DELETE: fail first 3 attempts for trunk-vlans=10,
	// then succeed for post-test cleanup.
	var trunkDeleteCount int32
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=10", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&trunkDeleteCount, 1)
		if n <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"remove trunk vlan failed"}]}}`)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	// Other trunk VLANs succeed for cleanup
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=11", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=12", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds
			{
				Config: testAccInterfaceCreateunitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			// Step 2: Remove from config triggers destroy; RemoveTrunkVlans fails
			{
				Config:      `# empty config triggers destroy`,
				ExpectError: regexp.MustCompile(`Client Error|Removing Trunk vlan ID failed`),
			},
		},
	})
}

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

// ---------------------------------------------------------------------------
// Acceptance test helpers: direct device API verification
// ---------------------------------------------------------------------------

// testAccCheckInterfaceOnDevice queries the device directly and verifies
// the interface has the expected native_vlan and trunk_vlans configuration.
func testAccCheckInterfaceOnDevice(name string, expectNativeVlan int, expectTrunkVlans []int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create F5OS client: %w", err)
		}
		intf, err := client.GetInterface(name)
		if err != nil {
			return fmt.Errorf("failed to read interface %s from device: %w", name, err)
		}
		if len(intf.OpenconfigInterfacesInterface) == 0 {
			return fmt.Errorf("interface %q not found on device", name)
		}
		data := intf.OpenconfigInterfacesInterface[0]
		gotNative := data.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.NativeVlan
		if gotNative != expectNativeVlan {
			return fmt.Errorf("interface %q native_vlan: expected %d, got %d", name, expectNativeVlan, gotNative)
		}
		gotTrunks := data.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.TrunkVlans
		if len(gotTrunks) != len(expectTrunkVlans) {
			return fmt.Errorf("interface %q trunk_vlans: expected %v, got %v", name, expectTrunkVlans, gotTrunks)
		}
		trunkMap := make(map[int]bool)
		for _, v := range gotTrunks {
			trunkMap[v] = true
		}
		for _, v := range expectTrunkVlans {
			if !trunkMap[v] {
				return fmt.Errorf("interface %q trunk_vlans: expected %d in %v", name, v, gotTrunks)
			}
		}
		return nil
	}
}

// testAccCheckInterfaceDestroy verifies the interface's vlans were cleaned up.
func testAccCheckInterfaceDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		return fmt.Errorf("failed to create F5OS client for destroy check: %w", err)
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_interface" {
			continue
		}
		name := rs.Primary.Attributes["name"]
		if name == "" {
			continue
		}
		intf, err := client.GetInterface(name)
		if err != nil {
			return fmt.Errorf("error reading interface %q during destroy check: %w", name, err)
		}
		// A 404 returns no error but an empty response — treat as destroyed.
		if len(intf.OpenconfigInterfacesInterface) == 0 {
			continue
		}
		data := intf.OpenconfigInterfacesInterface[0]
		nv := data.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.NativeVlan
		tv := data.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.TrunkVlans
		if nv != 0 || len(tv) > 0 {
			return fmt.Errorf("interface %q still has vlan config after destroy: native_vlan=%d, trunk_vlans=%v", name, nv, tv)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Enhanced acceptance test with CheckDestroy and direct API verification
// ---------------------------------------------------------------------------

// Acceptance-test configs using VLAN IDs in the 3900-3999 range per safety rules
const testAccInterfaceEnhancedCreateConfig = `
resource "f5os_vlan" "vlan3910" {
 vlan_id = 3910
 name = "test-vlan-3910"
}
resource "f5os_vlan" "vlan3911" {
 vlan_id = 3911
 name = "test-vlan-3911"
}
resource "f5os_vlan" "vlan3912" {
 vlan_id = 3912
 name = "test-vlan-3912"
}
resource "f5os_interface" "acc_test" {
  enabled     = true
  name        = "1.0"
  native_vlan = f5os_vlan.vlan3910.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan3911.vlan_id,
    f5os_vlan.vlan3912.vlan_id
  ]
}
`

const testAccInterfaceEnhancedUpdateConfig = `
resource "f5os_vlan" "vlan3910" {
 vlan_id = 3910
 name = "test-vlan-3910"
}
resource "f5os_vlan" "vlan3911" {
 vlan_id = 3911
 name = "test-vlan-3911"
}
resource "f5os_vlan" "vlan3912" {
 vlan_id = 3912
 name = "test-vlan-3912"
}
resource "f5os_interface" "acc_test" {
  enabled     = true
  name        = "1.0"
  native_vlan = f5os_vlan.vlan3911.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan3910.vlan_id,
    f5os_vlan.vlan3912.vlan_id
  ]
}
`

func TestAccInterfaceEnhanced(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInterfaceDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with direct device verification
			{
				Config: testAccInterfaceEnhancedCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.acc_test", "native_vlan", "3910"),
					resource.TestCheckResourceAttr("f5os_interface.acc_test", "enabled", "true"),
					testAccCheckInterfaceOnDevice("1.0", 3910, []int{3911, 3912}),
				),
			},
			// Step 2: Import state
			{
				ResourceName:      "f5os_interface.acc_test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update with direct device verification
			{
				Config: testAccInterfaceEnhancedUpdateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.acc_test", "native_vlan", "3911"),
					testAccCheckInterfaceOnDevice("1.0", 3911, []int{3910, 3912}),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}
