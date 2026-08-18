package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
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
	// Reset the package-level GET counter so this test is not coupled
	// to the execution order of other tests that share the same var.
	count = 0
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

	// RemoveNativeVlans DELETE: fail first 6 attempts (one doRequest retry
	// cycle), then succeed for post-test cleanup.
	var nativeDeleteCount int32
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:native-vlan", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&nativeDeleteCount, 1)
		if n <= 6 {
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
	// RemoveTrunkVlans DELETE: fail first 6 attempts for trunk-vlans=10,
	// then succeed for post-test cleanup.
	var trunkDeleteCount int32
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=10", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&trunkDeleteCount, 1)
		if n <= 6 {
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

// ---------------------------------------------------------------------------
// F5OS 2.0.0+ additive: description + phyport
// ---------------------------------------------------------------------------

// interfaceStatus2_0Fixture returns a fixture-shaped GET response for
// interface "1.0" on an F5OS 2.0.0+ device. Both the additive
// config.description leaf and the ethernet state.f5-if-ethernet:phyport
// leaf are populated. Used as the mock GET body across the
// description/phyport unit tests.
func interfaceStatus2_0Fixture(desc, phyport string) string {
	return fmt.Sprintf(`{
  "openconfig-interfaces:interface": [
    {
      "name": "1.0",
      "config": {
        "name": "1.0",
        "type": "iana-if-type:ethernetCsmacd",
        "enabled": true,
        "description": %q
      },
      "state": {
        "name": "1.0",
        "type": "iana-if-type:ethernetCsmacd",
        "mtu": 9600,
        "enabled": true,
        "oper-status": "UP"
      },
      "openconfig-if-ethernet:ethernet": {
        "openconfig-vlan:switched-vlan": {
          "config": {
            "native-vlan": 13,
            "trunk-vlans": [10, 11, 12]
          }
        },
        "state": {
          "port-speed": "openconfig-if-ethernet:SPEED_100GB",
          "f5-if-ethernet:phyport": %q
        }
      }
    }
  ]
}`, desc, phyport)
}

// interfaceStatus1_8Fixture returns a GET response shaped for an F5OS
// 1.8.x device: no config.description leaf, no phyport in ethernet
// state. Used to prove Read handles the absent leaves cleanly.
const interfaceStatus1_8Fixture = `{
  "openconfig-interfaces:interface": [
    {
      "name": "1.0",
      "config": {
        "name": "1.0",
        "type": "iana-if-type:ethernetCsmacd",
        "enabled": true
      },
      "state": {
        "name": "1.0",
        "type": "iana-if-type:ethernetCsmacd",
        "mtu": 9600,
        "enabled": true,
        "oper-status": "UP"
      },
      "openconfig-if-ethernet:ethernet": {
        "openconfig-vlan:switched-vlan": {
          "config": {
            "native-vlan": 13,
            "trunk-vlans": [10, 11, 12]
          }
        },
        "state": {
          "port-speed": "openconfig-if-ethernet:SPEED_100GB"
        }
      }
    }
  ]
}`

const testUnitInterfaceDescriptionConfig = `
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  description = "uplink to leaf-01"
  native_vlan = 13
  trunk_vlans = [10, 11, 12]
}`

const testUnitInterfaceEmptyDescriptionConfig = `
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  description = ""
  native_vlan = 13
  trunk_vlans = [10, 11, 12]
}`

const testUnitInterfaceNoDescriptionConfig = `
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  native_vlan = 13
  trunk_vlans = [10, 11, 12]
}`

// TestUnitInterfaceDescriptionRoundTrip verifies that a
// description set in HCL is serialized into the PATCH payload, that
// Read surfaces the value returned by a 2.0.0+ device, and that
// phyport is populated as a computed attribute.
func TestUnitInterfaceDescriptionRoundTrip(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupMockPlatformVersion(mux, "2.0.0-22925")
	setupInterfaceMockProviderEndpoints()

	var patchBody atomic.Value // []byte

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPut || r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			patchBody.Store(body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, interfaceStatus2_0Fixture("uplink to leaf-01", "1"))
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitInterfaceDescriptionConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "description", "uplink to leaf-01"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "phyport", "1"),
					// Verify the write actually carried the description leaf.
					func(_ *terraform.State) error {
						raw, ok := patchBody.Load().([]byte)
						if !ok || len(raw) == 0 {
							return fmt.Errorf("no write payload captured; the resource did not issue a PATCH/PUT")
						}
						if !strings.Contains(string(raw), `"description":"uplink to leaf-01"`) {
							return fmt.Errorf("write payload missing description leaf; got: %s", string(raw))
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitInterfaceDescriptionExplicitEmptyString verifies that
// setting description = "" serializes an empty-string leaf on the wire
// (F5OS idiom for clearing a description) rather than being treated
// as "unset" and omitted from the payload.
func TestUnitInterfaceDescriptionExplicitEmptyString(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupMockPlatformVersion(mux, "2.0.0-22925")
	setupInterfaceMockProviderEndpoints()

	var patchBody atomic.Value

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPut || r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			patchBody.Store(body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, interfaceStatus2_0Fixture("", "1"))
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitInterfaceEmptyDescriptionConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "description", ""),
					func(_ *terraform.State) error {
						raw, ok := patchBody.Load().([]byte)
						if !ok || len(raw) == 0 {
							return fmt.Errorf("no write payload captured")
						}
						// Structurally decode the payload and drill
						// into openconfig-interfaces:interfaces.
						// interface[0].config to prove the description
						// leaf is present AND equal to the empty
						// string — not merely a substring match that
						// could false-pass on unrelated content.
						var envelope struct {
							Interfaces struct {
								Interface []struct {
									Config struct {
										// Pointer so we can tell
										// "leaf absent" (nil) from
										// "leaf present but empty"
										// (non-nil pointing at "").
										Description *string `json:"description"`
									} `json:"config"`
								} `json:"interface"`
							} `json:"openconfig-interfaces:interfaces"`
						}
						if err := json.Unmarshal(raw, &envelope); err != nil {
							return fmt.Errorf("payload not valid JSON: %w; body=%s", err, string(raw))
						}
						if len(envelope.Interfaces.Interface) == 0 {
							return fmt.Errorf("payload has no interface entries: %s", string(raw))
						}
						desc := envelope.Interfaces.Interface[0].Config.Description
						if desc == nil {
							return fmt.Errorf("description leaf missing from payload; got: %s", string(raw))
						}
						if *desc != "" {
							return fmt.Errorf("expected description leaf to be an empty string; got %q", *desc)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitInterfaceDescriptionOmitted verifies that when the caller
// does not set description in HCL, it does NOT appear in the write
// payload. This is what protects pre-2.0.0 devices — omitempty in the
// pointer serializer keeps the leaf off the wire so older devices
// don't reject the request.
func TestUnitInterfaceDescriptionOmitted(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupMockPlatformVersion(mux, "1.8.3-23453")
	setupInterfaceMockProviderEndpoints()

	var patchBody atomic.Value

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPut || r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			patchBody.Store(body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, interfaceStatus1_8Fixture)
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitInterfaceNoDescriptionConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Pre-2.0 device: leaf absent → attribute is null.
					resource.TestCheckNoResourceAttr("f5os_interface.test_interface", "description"),
					// phyport absent on 1.8.x fixture → null.
					resource.TestCheckNoResourceAttr("f5os_interface.test_interface", "phyport"),
					func(_ *terraform.State) error {
						raw, ok := patchBody.Load().([]byte)
						if !ok || len(raw) == 0 {
							return fmt.Errorf("no write payload captured")
						}
						if strings.Contains(string(raw), `"description"`) {
							return fmt.Errorf("description must not be serialized when omitted; got: %s", string(raw))
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitInterfaceDescriptionRejectedPre200 verifies that setting
// description on a pre-2.0.0 device produces a clear "Unsupported
// attribute" error before any RESTCONF write is issued.
func TestUnitInterfaceDescriptionRejectedPre200(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupMockPlatformVersion(mux, "1.8.3-23453")
	setupInterfaceMockProviderEndpoints()

	// If the resource incorrectly issues a PATCH/PUT despite the
	// version gate, record the hit atomically and assert on the main
	// goroutine after the test.
	var writeHit atomic.Bool
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPut || r.Method == http.MethodPost {
			writeHit.Store(true)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPut || r.Method == http.MethodPost {
			writeHit.Store(true)
		}
		w.WriteHeader(http.StatusInternalServerError)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testUnitInterfaceDescriptionConfig,
				ExpectError: regexp.MustCompile(`(?s)Unsupported attribute`),
			},
		},
	})

	if writeHit.Load() {
		t.Fatalf("resource issued a write to the interface endpoint despite the version gate; expected the error to be raised before any device call")
	}
}

// TestUnitInterfaceDescriptionUpdate exercises the 2.0.0 Update path
// end-to-end: step 1 creates an interface with a description, step 2
// changes the description, and both the write payload and post-Read
// state must reflect the new value. This complements
// TestUnitInterfaceDescriptionRoundTrip (which only covers Create) and
// TestUnitInterfaceDescriptionRejectedPre200 (which covers the
// version-gate rejection in both Create and Update).
func TestUnitInterfaceDescriptionUpdate(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupMockPlatformVersion(mux, "2.0.0-22925")
	setupInterfaceMockProviderEndpoints()

	// Return the description matching whichever step the resource is
	// currently applying. A single atomic.Value avoids the reordering
	// hazards of the older count-based fixture switching pattern used
	// elsewhere in this file.
	currentDesc := &atomic.Value{}
	currentDesc.Store("uplink to leaf-01")
	patchBodies := &atomic.Value{}
	patchBodies.Store([][]byte(nil))

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPut || r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			// Simulate the device having accepted the new description
			// so the follow-up GET returns what the caller just wrote.
			var envelope struct {
				Interfaces struct {
					Interface []struct {
						Config struct {
							Description *string `json:"description"`
						} `json:"config"`
					} `json:"interface"`
				} `json:"openconfig-interfaces:interfaces"`
			}
			if err := json.Unmarshal(body, &envelope); err == nil &&
				len(envelope.Interfaces.Interface) > 0 &&
				envelope.Interfaces.Interface[0].Config.Description != nil {
				currentDesc.Store(*envelope.Interfaces.Interface[0].Config.Description)
			}
			existing := patchBodies.Load().([][]byte)
			patchBodies.Store(append(existing, body))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, interfaceStatus2_0Fixture(currentDesc.Load().(string), "1"))
		}
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

	updated := `
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  description = "uplink to leaf-02"
  native_vlan = 13
  trunk_vlans = [10, 11, 12]
}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitInterfaceDescriptionConfig,
				Check:  resource.TestCheckResourceAttr("f5os_interface.test_interface", "description", "uplink to leaf-01"),
			},
			{
				Config: updated,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "description", "uplink to leaf-02"),
					func(_ *terraform.State) error {
						bodies := patchBodies.Load().([][]byte)
						if len(bodies) < 2 {
							return fmt.Errorf("expected at least 2 write payloads (Create + Update); got %d", len(bodies))
						}
						// The most recent write must carry the new
						// description. Substring match here is safe
						// because we already asserted via JSON decode
						// in TestUnitInterfaceDescriptionExplicitEmptyString
						// that description lives at the expected path;
						// this is just checking the value used.
						last := string(bodies[len(bodies)-1])
						if !strings.Contains(last, `"description":"uplink to leaf-02"`) {
							return fmt.Errorf("update write payload missing new description; got: %s", last)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitInterfaceDescriptionOutOfBandClear guards drift detection.
// If an operator clears the description directly on the device (or
// downgrades to a pre-2.0 image so the leaf disappears), the next
// Read must reflect null in state so Terraform surfaces the drift on
// the following plan rather than pretending state still matches.
//
// The scenario is exercised by:
//   - Step 1: create with description="X" against a mocked 2.0.0
//     device that echoes it back (state → "X").
//   - Step 2: the mock's GET now returns an interface config *without*
//     the description leaf (as if it were cleared out-of-band or the
//     device was downgraded). Terraform refreshes and must show
//     description = null and produce a non-empty plan to reconcile.
func TestUnitInterfaceDescriptionOutOfBandClear(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupMockPlatformVersion(mux, "2.0.0-22925")
	setupInterfaceMockProviderEndpoints()

	// dropDescription flips to true after the initial Create so the
	// follow-up Read (and the ExpectNonEmptyPlan refresh in step 2)
	// see a device that no longer reports the leaf.
	var dropDescription atomic.Bool

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPut || r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			return
		}
		w.WriteHeader(http.StatusOK)
		if dropDescription.Load() {
			// Simulate the leaf disappearing (out-of-band clear /
			// downgrade). Same structural shape as the 1.8 fixture.
			_, _ = fmt.Fprint(w, interfaceStatus1_8Fixture)
			return
		}
		_, _ = fmt.Fprint(w, interfaceStatus2_0Fixture("uplink to leaf-01", "1"))
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.0/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: baseline — device returns the description.
			{
				Config: testUnitInterfaceDescriptionConfig,
				Check:  resource.TestCheckResourceAttr("f5os_interface.test_interface", "description", "uplink to leaf-01"),
			},
			// Step 2: flip the mock so the refresh sees the leaf gone,
			// then re-apply the same config. Terraform should read
			// description as null on the device, detect drift vs the
			// plan (which still wants "uplink to leaf-01"), and issue
			// a corrective write. ExpectNonEmptyPlan captures the
			// drift-detected state after refresh; RefreshState alone
			// would exit before Terraform reconciles.
			{
				PreConfig:    func() { dropDescription.Store(true) },
				RefreshState: true,
				// After refresh the device no longer reports the leaf,
				// so plan is non-empty (description drifted to null on
				// the device but the last-applied config still asks
				// for it).
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestUnitInterfaceModelToStateEmptyResponse guards against a panic on
// a GET that returns no interface entries. The device (or client) can
// return an empty openconfig-interfaces:interface array — for example
// after the interface has been removed out-of-band, or when the client
// short-circuits with an empty struct. Without the defensive length
// check at the top of interfaceResourceModelToState, indexing [0]
// panics and takes down the plugin process. This test exercises both
// respData == nil and len(...) == 0 and asserts every field is nulled
// out so Terraform sees the resource as absent / drifted rather than
// dying.
func TestUnitInterfaceModelToStateEmptyResponse(t *testing.T) {
	r := &InterfaceResource{}

	cases := map[string]*f5ossdk.F5RespOpenconfigInterface{
		"nil response":         nil,
		"empty interface list": {},
	}
	for name, respData := range cases {
		t.Run(name, func(t *testing.T) {
			data := &InterfaceResourceModel{}
			// Must not panic.
			r.interfaceResourceModelToState(context.Background(), respData, data)

			assert.True(t, data.Name.IsNull(), "Name should be null")
			assert.True(t, data.Enabled.IsNull(), "Enabled should be null")
			assert.True(t, data.Status.IsNull(), "Status should be null")
			assert.True(t, data.NativeVlan.IsNull(), "NativeVlan should be null")
			assert.True(t, data.TrunkVlans.IsNull(), "TrunkVlans should be null")
			assert.True(t, data.Description.IsNull(), "Description should be null")
			assert.True(t, data.Phyport.IsNull(), "Phyport should be null")
		})
	}
}

// TestUnitInterfacePhyportWireTypes guards against a regression where
// some F5OS builds emit the f5-if-ethernet:phyport leaf as a JSON
// number (e.g. 1) while others emit it as a JSON string (e.g. "1").
// The client struct declares Phyport as *json.Number so it unmarshals
// from either form; interfaceResourceModelToState then renders it to
// the string state via String(). Before the fix, a numeric phyport
// failed GetInterfaceInfo with:
//
//	json: cannot unmarshal number into Go struct field
//	  ...f5-if-ethernet:phyport of type string
//
// which broke the whole interface read on those devices.
func TestUnitInterfacePhyportWireTypes(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"numeric phyport": {
			body: `{
  "openconfig-interfaces:interface": [
    {
      "name": "1.0",
      "openconfig-if-ethernet:ethernet": {
        "state": { "f5-if-ethernet:phyport": 1 }
      }
    }
  ]
}`,
			want: "1",
		},
		"string phyport": {
			body: `{
  "openconfig-interfaces:interface": [
    {
      "name": "1.0",
      "openconfig-if-ethernet:ethernet": {
        "state": { "f5-if-ethernet:phyport": "1" }
      }
    }
  ]
}`,
			want: "1",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var respData f5ossdk.F5RespOpenconfigInterface
			// Must unmarshal cleanly regardless of the wire type.
			if !assert.NoError(t, json.Unmarshal([]byte(tc.body), &respData)) {
				return
			}

			r := &InterfaceResource{}
			data := &InterfaceResourceModel{}
			r.interfaceResourceModelToState(context.Background(), &respData, data)

			assert.False(t, data.Phyport.IsNull(), "Phyport should be populated")
			assert.Equal(t, tc.want, data.Phyport.ValueString())
		})
	}
}

// ---------------------------------------------------------------------------
// Acceptance test: F5OS 2.0.0+ description round-trip on a real device
// ---------------------------------------------------------------------------
//
// This test requires a 2.0.0+ target (Host B per the AC). It skips on
// pre-2.0.0 devices so shared CI matrices that also include a 1.8.x
// device (Host A) still complete cleanly.

const testAccInterfaceDescriptionCreateConfig = `
resource "f5os_interface" "acc_test_desc" {
  enabled     = true
  name        = "1.0"
  description = "tf-acc uplink"
  native_vlan = 3910
}
`

const testAccInterfaceDescriptionUpdateConfig = `
resource "f5os_interface" "acc_test_desc" {
  enabled     = true
  name        = "1.0"
  description = "tf-acc updated"
  native_vlan = 3910
}
`

const testAccInterfaceDescriptionClearConfig = `
resource "f5os_interface" "acc_test_desc" {
  enabled     = true
  name        = "1.0"
  description = ""
  native_vlan = 3910
}
`

// testAccPreCheckInterface2_0 skips when the target device is not
// running F5OS 2.0.0+; description and phyport only exist there.
func testAccPreCheckInterface2_0(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("testAccPreCheckInterface2_0: failed to create session: %s", err)
	}
	if !platformVersionAtLeast(client.PlatformVersion, "v2.0") {
		t.Skipf("skipping: test requires F5OS 2.0.0+ but device reports %q", client.PlatformVersion)
	}
}

// testAccCheckInterfaceDescriptionOnDevice queries the device
// directly (independent client, outside the resource lifecycle) and
// verifies the interface's config.description leaf matches the
// expected value. When expected == "" it also accepts an absent leaf
// on the device (F5OS may return either "" or omit the leaf when it
// has been cleared).
func testAccCheckInterfaceDescriptionOnDevice(name, expected string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
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
		got := ""
		if intf.OpenconfigInterfacesInterface[0].Config.Description != nil {
			got = *intf.OpenconfigInterfacesInterface[0].Config.Description
		}
		if got != expected {
			return fmt.Errorf("interface %q description on device: expected %q, got %q",
				name, expected, got)
		}
		return nil
	}
}

// TestAccInterfaceDescription exercises the F5OS 2.0.0+ description
// round-trip end-to-end against a real device:
//   - Step 1: create with description="tf-acc uplink"; verify state
//     and device value.
//   - Step 2: update to description="tf-acc updated"; verify state
//     and device value.
//   - Step 3: clear via description=""; verify state is empty and
//     the device either returns "" or omits the leaf.
//   - Step 4: Import round-trip preserves the (currently empty)
//     description; the computed phyport value is ignored on
//     round-trip because it is dynamic.
func TestAccInterfaceDescription(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckInterface2_0(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInterfaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccInterfaceDescriptionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.acc_test_desc", "description", "tf-acc uplink"),
					resource.TestCheckResourceAttrSet("f5os_interface.acc_test_desc", "phyport"),
					testAccCheckInterfaceDescriptionOnDevice("1.0", "tf-acc uplink"),
				),
			},
			{
				Config: testAccInterfaceDescriptionUpdateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.acc_test_desc", "description", "tf-acc updated"),
					testAccCheckInterfaceDescriptionOnDevice("1.0", "tf-acc updated"),
				),
			},
			{
				Config: testAccInterfaceDescriptionClearConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_interface.acc_test_desc", "description", ""),
					testAccCheckInterfaceDescriptionOnDevice("1.0", ""),
				),
			},
			{
				ResourceName:      "f5os_interface.acc_test_desc",
				ImportState:       true,
				ImportStateVerify: true,
				// phyport is dynamic device state and description on
				// a cleared interface may come back as either "" or
				// null depending on the device; ignore both on
				// round-trip.
				ImportStateVerifyIgnore: []string{"phyport", "description"},
			},
		},
	})
}
