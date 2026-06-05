package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
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

func TestUnitLagInterfaceCreateTC3Resource(t *testing.T) {
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
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
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

func TestUnitLagInterfaceCreateTC4Resource(t *testing.T) {
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
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
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
  lag_type    = "LACP"
  native_vlan = 29
  trunk_vlans = [27, 28]
  members = ["1.1", "1.2"]
}`

const testAccInterfaceCreateUnitModifyResourceConfig = `
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  lag_type    = "LACP"
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

// ---------------------------------------------------------------------------
// Shared mock registration helpers for LAG unit tests
// ---------------------------------------------------------------------------

// lagMockProviderEndpoints registers the standard provider-init endpoints
// (auth, platform state, version) needed by every LAG unit test.
func lagMockProviderEndpoints(t *testing.T) {
	t.Helper()
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
}

// lagMockWriteEndpoints registers the PATCH handler for the root data
// endpoint used by CreateLagInterface (and addLagMembers, addLagModeInterval).
func lagMockWriteEndpoints() {
	mux.HandleFunc("/restconf/data/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
}

// ---------------------------------------------------------------------------
// Unit tests: Create error paths
// ---------------------------------------------------------------------------

// TestUnitLagCreateApiError verifies that Create surfaces an error when
// the CreateLagInterface API call fails (root PATCH returns 500).
func TestUnitLagCreateApiError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)

	// Make the root PATCH fail with 400 (which the SDK will convert to an error)
	mux.HandleFunc("/restconf/data/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"create failed"}]}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLagInterfaceCreateUnitResourceConfig,
				ExpectError: regexp.MustCompile(`Creating LAG interface failed`),
			},
		},
	})
}

// TestUnitLagCreateGetLagInterfaceError verifies that Create surfaces
// an error when the post-create GetLagInterface call fails.
func TestUnitLagCreateGetLagInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	// GetLagInterface returns 400 (triggers SDK error after retries)
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"get lag failed"}]}}`)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLagInterfaceCreateUnitResourceConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get LAG Interface`),
			},
		},
	})
}

// TestUnitLagCreateGetLacpInterfaceError verifies that Create surfaces
// an error when the post-create GetLacpInterface call fails.
func TestUnitLagCreateGetLacpInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	// GetLagInterface succeeds
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
	})
	// GetLacpInterface returns 500
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"get lacp failed"}]}}`)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLagInterfaceCreateUnitResourceConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get LACP Interface`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests: Read error paths
// ---------------------------------------------------------------------------

// TestUnitLagReadGetLagInterfaceError verifies that Read surfaces an error
// when GetLagInterface fails on the post-apply refresh.
func TestUnitLagReadGetLagInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	lagGetCount := 0
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			lagGetCount++
			// GET#1: Create's post-create read (succeed)
			// GETs 2-4: framework's post-apply refresh Read + SDK retries (fail — error under test)
			// GET#5+: destroy reads (succeed to allow cleanup)
			if lagGetCount >= 2 && lagGetCount <= 4 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"read failed"}]}}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
			return
		}
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
	})
	mux.HandleFunc("/restconf/data/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLagInterfaceCreateUnitResourceConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get LAG interface`),
			},
		},
	})
}

// TestUnitLagReadGetLacpInterfaceError verifies that Read surfaces an error
// when GetLacpInterface fails on the post-apply refresh.
func TestUnitLagReadGetLacpInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	lacpGetCount := 0
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			lacpGetCount++
			// First GET succeeds (Create's post-create read); second GET fails (framework's post-apply refresh Read)
			if lacpGetCount <= 1 {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
			} else {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"lacp read failed"}]}}`)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLagInterfaceCreateUnitResourceConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get LACP Interface`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests: Update error paths
// ---------------------------------------------------------------------------

// TestUnitLagUpdateGetLagMembersError verifies that Update surfaces an error
// when the GetLagInterface call (to read current members) fails.
func TestUnitLagUpdateGetLagMembersError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	lagGetCount := 0
	failLagGet := false
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" {
			lagGetCount++
			if failLagGet {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"member read failed"}]}}`)
				// After the SDK exhausts its 3 retries, stop failing so cleanup works
				if lagGetCount >= 7 {
					failLagGet = false
				}
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
			// After step 1 completes (Create + 2 Reads = 3 GETs), start failing
			if lagGetCount >= 3 {
				failLagGet = true
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
	})
	mux.HandleFunc("/restconf/data/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
				),
			},
			{
				Config:      testAccInterfaceCreateUnitModifyResourceConfig,
				ExpectError: regexp.MustCompile(`Unable to Read LAG interface members`),
			},
		},
	})
}

// TestUnitLagUpdateRemoveMembersError verifies that Update surfaces an error
// when RemoveLagMembers fails.
func TestUnitLagUpdateRemoveMembersError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
	})
	// RemoveLagMembers calls DELETE on each member — fail first 3 attempts (SDK retry cycle),
	// then succeed on subsequent attempts (cleanup destroy).
	// The SDK builds: /openconfig-interfaces:interfaces/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id
	memberRemoveAttempt := 0
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			memberRemoveAttempt++
			if memberRemoveAttempt <= 3 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"remove member failed"}]}}`)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
				),
			},
			{
				Config:      testAccInterfaceCreateUnitModifyResourceConfig,
				ExpectError: regexp.MustCompile(`Unable to remove members from LAG interface`),
			},
		},
	})
}

// TestUnitLagUpdateLagInterfaceError verifies that Update surfaces an error
// when the UpdateLagInterface API call fails.
func TestUnitLagUpdateLagInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)

	mux.HandleFunc("/restconf/data/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
	})
	// RemoveLagMembers succeeds
	mux.HandleFunc("/restconf/data/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// UpdateLagInterface PATCHes /openconfig-interfaces:interfaces — always fail
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"update lag failed"}]}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	// Switched-vlan GET for UpdateLagInterface
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"openconfig-vlan:switched-vlan":{"config":{"native-vlan":29,"trunk-vlans":[27,28]}}}`)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
				),
			},
			{
				Config:      testAccInterfaceCreateUnitModifyResourceConfig,
				ExpectError: regexp.MustCompile(`Update LAG interface failed`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests: Delete error paths
// ---------------------------------------------------------------------------

// TestUnitLagDeleteGetLagInterfaceError verifies that Delete surfaces an error
// when GetLagInterface fails (to read current members before removal).
// Uses a 2-step approach: step 1 creates, step 2 removes the config (triggers
// Delete). The mock fails the Delete's member-read GET, and ExpectError catches
// it. The mock then recovers for the framework's cleanup destroy.
func TestUnitLagDeleteGetLagInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	lagGetCount := 0
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == "GET" {
			lagGetCount++
			// GETs 1-2: step 1 Create read + post-apply Read (succeed)
			// GET 3: step 2 pre-destroy Read/refresh (succeed)
			// GETs 4-6: Delete's member lookup + SDK retries (fail — error under test)
			// GETs 7+: framework retry destroy (succeed for cleanup)
			if lagGetCount >= 4 && lagGetCount <= 6 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"get lag for delete failed"}]}}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
	})
	mux.HandleFunc("/restconf/data/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
				),
			},
			{
				Config:      testAccLagEmptyConfig,
				ExpectError: regexp.MustCompile(`Unable to Read LAG interface members`),
			},
		},
	})
}

// TestUnitLagDeleteRemoveMembersError verifies that Delete surfaces an error
// when RemoveLagMembers fails. Step 1 creates, step 2 removes config
// (triggers Delete). The mock fails the member DELETE, and ExpectError catches it.
func TestUnitLagDeleteRemoveMembersError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
	})
	// RemoveLagMembers — fail first 3 attempts (SDK retry cycle), then recover.
	// The SDK builds: /openconfig-interfaces:interfaces/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id
	memberDeleteAttempt := 0
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			memberDeleteAttempt++
			if memberDeleteAttempt <= 3 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"remove member 1.1 failed"}]}}`)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
				),
			},
			{
				Config:      testAccLagEmptyConfig,
				ExpectError: regexp.MustCompile(`Removing LAG interface member failed`),
			},
		},
	})
}

// TestUnitLagDeleteRemoveLacpInterfaceError verifies that Delete surfaces
// an error when RemoveLacpInterface fails. Step 1 creates, step 2 removes
// config (triggers Delete). The mock fails the LACP DELETE, and ExpectError
// catches it.
func TestUnitLagDeleteRemoveLacpInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
	})
	lacpDeleteAttempt := 0
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			lacpDeleteAttempt++
			// Fail first 3 deletes (SDK retry cycle — error under test),
			// succeed on subsequent attempts (framework cleanup)
			if lacpDeleteAttempt <= 3 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"delete lacp failed"}]}}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
	})
	mux.HandleFunc("/restconf/data/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
				),
			},
			{
				Config:      testAccLagEmptyConfig,
				ExpectError: regexp.MustCompile(`Unable to delete LACP interface`),
			},
		},
	})
}

// TestUnitLagDeleteRemoveLagInterfaceError verifies that Delete surfaces
// an error when RemoveLagInterface fails. Step 1 creates, step 2 removes
// config (triggers Delete). The mock fails the LAG DELETE, and ExpectError
// catches it.
func TestUnitLagDeleteRemoveLagInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	lagDeleteAttempt := 0
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			lagDeleteAttempt++
			// Fail first 3 deletes (SDK retry cycle — error under test),
			// succeed on subsequent attempts (framework cleanup)
			if lagDeleteAttempt <= 3 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-message":"delete lag failed"}]}}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_config.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lacp_config.json"))
	})
	mux.HandleFunc("/restconf/data/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLagInterfaceCreateUnitResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
				),
			},
			{
				Config:      testAccLagEmptyConfig,
				ExpectError: regexp.MustCompile(`Unable to delete LAG interface`),
			},
		},
	})
}

// testAccLagEmptyConfig is an empty HCL config used to trigger resource
// deletion in step 2 of Delete error tests.
const testAccLagEmptyConfig = `
# empty — triggers deletion of all resources from step 1
`

// ---------------------------------------------------------------------------
// Unit tests: Static LAG lifecycle
// ---------------------------------------------------------------------------

const testUnitLagStaticCreateConfig = `
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  lag_type    = "STATIC"
  native_vlan = 29
  trunk_vlans = [27, 28]
  members     = ["1.1", "1.2"]
}
`

const testUnitLagStaticUpdateConfig = `
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  lag_type    = "STATIC"
  native_vlan = 28
  trunk_vlans = [27, 29]
  members     = ["1.2"]
}
`

// TestUnitLagStaticCreateReadImport verifies that a static LAG can be
// created, read, and imported without any LACP interaction. The mock
// server does NOT register a handler for the LACP endpoint — if Create
// or Read attempts to query it, the test will fail with a 404.
func TestUnitLagStaticCreateReadImport(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	// GetLagInterface returns the static fixture (lag-type: STATIC)
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_static_config.json"))
	})

	// NO handler for /restconf/data/openconfig-lacp:lacp/interfaces/interface=tf-lag
	// If the code tries to query LACP, it will hit the catch-all or 404.

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create and verify static LAG attributes
			{
				Config: testUnitLagStaticCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "name", "tf-lag"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "lag_type", "STATIC"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.#", "2"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.#", "2"),
					// mode and interval should not be in state for static LAGs
					resource.TestCheckNoResourceAttr("f5os_lag.test_lag", "mode"),
					resource.TestCheckNoResourceAttr("f5os_lag.test_lag", "interval"),
				),
			},
			// Step 2: ImportState — Read must detect STATIC from device response
			{
				ResourceName:      "f5os_lag.test_lag",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestUnitLagStaticUpdate verifies that updating a static LAG (change VLANs
// and remove a member) works without any LACP interaction.
func TestUnitLagStaticUpdate(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)
	lagMockWriteEndpoints()

	var getCount int
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			getCount++
			w.WriteHeader(http.StatusOK)
			// First 4 GETs (Create + Read cycles): return original config
			// Subsequent GETs (Update + Read cycles): return modified config
			if getCount <= 4 {
				_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_static_config.json"))
			} else {
				_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_lag_static_config_modified.json"))
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Mock endpoints for member removal and VLAN cleanup during Update.
	// Paths must use the full prefix that the SDK builds: /openconfig-interfaces:interfaces/interface=...
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:native-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=27", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=28", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.1/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=1.2/openconfig-if-ethernet:ethernet/config/openconfig-if-aggregate:aggregate-id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface=tf-lag/openconfig-if-aggregate:aggregation/openconfig-vlan:switched-vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// NO handler for LACP endpoint

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create static LAG
			{
				Config: testUnitLagStaticCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "lag_type", "STATIC"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "29"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.#", "2"),
				),
			},
			// Step 2: Update — change native VLAN, remove member
			{
				Config: testUnitLagStaticUpdateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "lag_type", "STATIC"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "28"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.#", "1"),
				),
			},
		},
	})
}

// TestUnitLagStaticModeIntervalRejected verifies that the ValidateConfig
// method rejects mode and interval when lag_type is STATIC.
func TestUnitLagStaticModeIntervalRejected(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_lag" "test_lag" {
  name     = "tf-lag"
  lag_type = "STATIC"
  mode     = "ACTIVE"
  members  = ["1.1"]
}
`,
				ExpectError: regexp.MustCompile(`mode cannot be set when lag_type is STATIC`),
			},
		},
	})
}

// TestUnitLagStaticIntervalRejected verifies that interval alone is also
// rejected when lag_type is STATIC.
func TestUnitLagStaticIntervalRejected(t *testing.T) {
	testAccPreUnitCheck(t)
	lagMockProviderEndpoints(t)

	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_lag" "test_lag" {
  name     = "tf-lag"
  lag_type = "STATIC"
  interval = "FAST"
  members  = ["1.1"]
}
`,
				ExpectError: regexp.MustCompile(`interval cannot be set when lag_type is STATIC`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test helpers — direct device API verification
// ---------------------------------------------------------------------------



// testAccCheckLagOnDevice queries the device directly and verifies that the
// LAG interface exists with the expected native_vlan, trunk_vlans, members,
// mode, and interval.
func testAccCheckLagOnDevice(lagName string, expectedNativeVlan int, expectedTrunkVlans []int, expectedMembers []string, expectedMode, expectedInterval string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		intfData, err := client.GetLagInterface(lagName)
		if err != nil {
			return fmt.Errorf("failed to get LAG interface: %w", err)
		}
		if len(intfData.OpenconfigInterfacesInterface) == 0 {
			return fmt.Errorf("LAG interface %q not found on device", lagName)
		}

		intf := intfData.OpenconfigInterfacesInterface[0]
		gotNativeVlan := intf.OpenconfigIfAggregateAggregation.OpenconfigVlanSwitchedVlan.Config.NativeVlan
		if gotNativeVlan != expectedNativeVlan {
			return fmt.Errorf("native_vlan: expected %d, got %d", expectedNativeVlan, gotNativeVlan)
		}

		gotTrunkVlans := intf.OpenconfigIfAggregateAggregation.OpenconfigVlanSwitchedVlan.Config.TrunkVlans
		if len(gotTrunkVlans) != len(expectedTrunkVlans) {
			return fmt.Errorf("trunk_vlans: expected %v, got %v", expectedTrunkVlans, gotTrunkVlans)
		}
		for i, v := range expectedTrunkVlans {
			if gotTrunkVlans[i] != v {
				return fmt.Errorf("trunk_vlans[%d]: expected %d, got %d", i, v, gotTrunkVlans[i])
			}
		}

		var gotMembers []string
		for _, m := range intf.OpenconfigIfAggregateAggregation.State.Members.Member {
			gotMembers = append(gotMembers, m.Name)
		}
		if len(gotMembers) != len(expectedMembers) {
			return fmt.Errorf("members: expected %v, got %v", expectedMembers, gotMembers)
		}
		for _, expected := range expectedMembers {
			found := false
			for _, got := range gotMembers {
				if got == expected {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("member %q not found on device; got %v", expected, gotMembers)
			}
		}

		lacpData, err := client.GetLacpInterface(lagName)
		if err != nil {
			return fmt.Errorf("failed to get LACP interface: %w", err)
		}
		if len(lacpData.OpenConfigLacpInterface) == 0 {
			return fmt.Errorf("LACP interface %q not found on device", lagName)
		}
		lacpCfg := lacpData.OpenConfigLacpInterface[0].Config
		if lacpCfg.Mode != expectedMode {
			return fmt.Errorf("mode: expected %q, got %q", expectedMode, lacpCfg.Mode)
		}
		if lacpCfg.Interval != expectedInterval {
			return fmt.Errorf("interval: expected %q, got %q", expectedInterval, lacpCfg.Interval)
		}
		return nil
	}
}

// testAccCheckLagDestroy verifies that the LAG interface has been removed
// from the device after the test completes.
func testAccCheckLagDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		return fmt.Errorf("CheckDestroy: failed to create client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_lag" {
			continue
		}
		lagName := rs.Primary.Attributes["name"]
		data, err := client.GetLagInterface(lagName)
		if err != nil {
			continue // Error reading — treat as destroyed
		}
		if data != nil && len(data.OpenconfigInterfacesInterface) > 0 {
			return fmt.Errorf("LAG interface %q still exists on device", lagName)
		}
	}
	return nil
}

// testAccCheckVlansDestroy verifies that VLANs in the 3940-3999 range have
// been removed from the device.
func testAccCheckVlansDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		return fmt.Errorf("CheckDestroy: failed to create client: %w", err)
	}

	data, err := client.GetRequest("/openconfig-vlan:vlans")
	if err != nil {
		return nil
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	vlans, ok := resp["openconfig-vlan:vlans"].(map[string]interface{})
	if !ok {
		return nil
	}
	vlanList, ok := vlans["vlan"].([]interface{})
	if !ok {
		return nil
	}
	for _, v := range vlanList {
		vlanMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		vlanID, ok := vlanMap["vlan-id"].(float64)
		if !ok {
			continue
		}
		if int(vlanID) >= 3940 && int(vlanID) <= 3999 {
			return fmt.Errorf("test VLAN %d still exists on device", int(vlanID))
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Acceptance tests: LAG CRUD with direct device API verification
// ---------------------------------------------------------------------------

// TestAccLagCRUDResource tests the full Create → Import → Update → Destroy
// lifecycle of the f5os_lag resource with direct device API verification.
func TestAccLagCRUDResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if err := testAccCheckLagDestroy(s); err != nil {
				return err
			}
			return testAccCheckVlansDestroy(s)
		},
		Steps: []resource.TestStep{
			// Step 1: Create LAG with VLANs and members
			{
				Config: testAccLagCRUDCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Terraform state checks
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "name", "tf-acc-lag"),
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "native_vlan", "3941"),
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "trunk_vlans.#", "2"),
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "members.#", "2"),
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "mode", "ACTIVE"),
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "interval", "FAST"),
					// Direct device API verification
					testAccCheckLagOnDevice(
						"tf-acc-lag",
						3941,
						[]int{3942, 3943},
						[]string{"1.0", "2.0"},
						"ACTIVE",
						"FAST",
					),
				),
			},
			// Step 2: Import state
			{
				ResourceName:      "f5os_lag.crud_test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update — change native VLAN, trunk VLANs, and remove a member
			{
				Config: testAccLagCRUDUpdateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "name", "tf-acc-lag"),
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "native_vlan", "3942"),
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "trunk_vlans.#", "2"),
					resource.TestCheckResourceAttr("f5os_lag.crud_test", "members.#", "1"),
					// Direct device API verification
					testAccCheckLagOnDevice(
						"tf-acc-lag",
						3942,
						[]int{3941, 3943},
						[]string{"2.0"},
						"ACTIVE",
						"FAST",
					),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}

const testAccLagCRUDCreateConfig = `
resource "f5os_vlan" "vlan3941" {
  vlan_id = 3941
  name    = "tf-acc-lag-v3941"
}
resource "f5os_vlan" "vlan3942" {
  vlan_id = 3942
  name    = "tf-acc-lag-v3942"
}
resource "f5os_vlan" "vlan3943" {
  vlan_id = 3943
  name    = "tf-acc-lag-v3943"
}
resource "f5os_lag" "crud_test" {
  name        = "tf-acc-lag"
  lag_type    = "LACP"
  native_vlan = f5os_vlan.vlan3941.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan3942.vlan_id,
    f5os_vlan.vlan3943.vlan_id,
  ]
  members  = ["1.0", "2.0"]
  mode     = "ACTIVE"
  interval = "FAST"
}
`

const testAccLagCRUDUpdateConfig = `
resource "f5os_vlan" "vlan3941" {
  vlan_id = 3941
  name    = "tf-acc-lag-v3941"
}
resource "f5os_vlan" "vlan3942" {
  vlan_id = 3942
  name    = "tf-acc-lag-v3942"
}
resource "f5os_vlan" "vlan3943" {
  vlan_id = 3943
  name    = "tf-acc-lag-v3943"
}
resource "f5os_lag" "crud_test" {
  name        = "tf-acc-lag"
  lag_type    = "LACP"
  native_vlan = f5os_vlan.vlan3942.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan3941.vlan_id,
    f5os_vlan.vlan3943.vlan_id,
  ]
  members  = ["2.0"]
  mode     = "ACTIVE"
  interval = "FAST"
}
`

// ---------------------------------------------------------------------------
// Acceptance tests: Static LAG support
// ---------------------------------------------------------------------------

// testAccCheckStaticLagOnDevice queries the device directly and verifies that
// a static LAG exists with the expected properties and NO LACP configuration.
func testAccCheckStaticLagOnDevice(lagName string, expectedNativeVlan int, expectedTrunkVlans []int, expectedMembers []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		intfData, err := client.GetLagInterface(lagName)
		if err != nil {
			return fmt.Errorf("failed to get LAG interface: %w", err)
		}
		if len(intfData.OpenconfigInterfacesInterface) == 0 {
			return fmt.Errorf("LAG interface %q not found on device", lagName)
		}

		intf := intfData.OpenconfigInterfacesInterface[0]

		// Verify lag-type is STATIC
		gotLagType := intf.OpenconfigIfAggregateAggregation.Config.LagType
		if gotLagType != "STATIC" {
			return fmt.Errorf("lag-type: expected STATIC, got %q", gotLagType)
		}

		// Verify native VLAN
		if expectedNativeVlan != 0 {
			gotNativeVlan := intf.OpenconfigIfAggregateAggregation.OpenconfigVlanSwitchedVlan.Config.NativeVlan
			if gotNativeVlan != expectedNativeVlan {
				return fmt.Errorf("native_vlan: expected %d, got %d", expectedNativeVlan, gotNativeVlan)
			}
		}

		// Verify trunk VLANs
		if expectedTrunkVlans != nil {
			gotTrunkVlans := intf.OpenconfigIfAggregateAggregation.OpenconfigVlanSwitchedVlan.Config.TrunkVlans
			if len(gotTrunkVlans) != len(expectedTrunkVlans) {
				return fmt.Errorf("trunk_vlans: expected %v, got %v", expectedTrunkVlans, gotTrunkVlans)
			}
			for i, v := range expectedTrunkVlans {
				if gotTrunkVlans[i] != v {
					return fmt.Errorf("trunk_vlans[%d]: expected %d, got %d", i, v, gotTrunkVlans[i])
				}
			}
		}

		// Verify members
		var gotMembers []string
		for _, m := range intf.OpenconfigIfAggregateAggregation.State.Members.Member {
			gotMembers = append(gotMembers, m.Name)
		}
		if len(gotMembers) != len(expectedMembers) {
			return fmt.Errorf("members: expected %v, got %v", expectedMembers, gotMembers)
		}
		for _, expected := range expectedMembers {
			found := false
			for _, got := range gotMembers {
				if got == expected {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("member %q not found on device; got %v", expected, gotMembers)
			}
		}

		// Verify NO LACP configuration exists
		lacpData, _ := client.GetLacpInterface(lagName)
		if lacpData != nil && len(lacpData.OpenConfigLacpInterface) > 0 {
			return fmt.Errorf("static LAG %q should not have LACP interface, but found one", lagName)
		}

		return nil
	}
}

// TestAccLagStaticCreateResource tests creating a static LAG (no LACP).
// This is the core customer-reported gap: static LAGs were not supported.
func TestAccLagStaticCreateResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLagDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create a static LAG with explicit lag_type = "STATIC"
			{
				Config: testAccLagStaticCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.static_test", "name", "tf-acc-static"),
					resource.TestCheckResourceAttr("f5os_lag.static_test", "lag_type", "STATIC"),
					resource.TestCheckResourceAttr("f5os_lag.static_test", "members.#", "2"),
					resource.TestCheckNoResourceAttr("f5os_lag.static_test", "mode"),
					resource.TestCheckNoResourceAttr("f5os_lag.static_test", "interval"),
					testAccCheckStaticLagOnDevice(
						"tf-acc-static",
						3951,
						[]int{3952, 3953},
						[]string{"1.0", "2.0"},
					),
				),
			},
			// Step 2: Import state — Read must detect STATIC from device
			{
				ResourceName:      "f5os_lag.static_test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

const testAccLagStaticCreateConfig = `
resource "f5os_vlan" "svlan3951" {
  vlan_id = 3951
  name    = "tf-acc-slag-v3951"
}
resource "f5os_vlan" "svlan3952" {
  vlan_id = 3952
  name    = "tf-acc-slag-v3952"
}
resource "f5os_vlan" "svlan3953" {
  vlan_id = 3953
  name    = "tf-acc-slag-v3953"
}
resource "f5os_lag" "static_test" {
  name        = "tf-acc-static"
  lag_type    = "STATIC"
  native_vlan = f5os_vlan.svlan3951.vlan_id
  trunk_vlans = [
    f5os_vlan.svlan3952.vlan_id,
    f5os_vlan.svlan3953.vlan_id,
  ]
  members = ["1.0", "2.0"]
}
`

// TestAccLagDefaultIsLACPResource tests that omitting lag_type defaults to
// LACP, preserving backward compatibility with existing configurations.
func TestAccLagDefaultIsLACPResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLagDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLagDefaultIsLACPConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.default_test", "name", "tf-acc-deflag"),
					resource.TestCheckResourceAttr("f5os_lag.default_test", "lag_type", "LACP"),
					resource.TestCheckResourceAttr("f5os_lag.default_test", "members.#", "1"),
				),
			},
		},
	})
}

const testAccLagDefaultIsLACPConfig = `
resource "f5os_lag" "default_test" {
  name    = "tf-acc-deflag"
  members = ["2.0"]
}
`

// TestAccLagStaticUpdateResource tests updating a static LAG — change VLANs
// and members without any LACP interaction.
func TestAccLagStaticUpdateResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLagDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create static LAG with 2 members
			{
				Config: testAccLagStaticUpdateStep1Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.update_test", "lag_type", "STATIC"),
					resource.TestCheckResourceAttr("f5os_lag.update_test", "members.#", "2"),
					resource.TestCheckResourceAttr("f5os_lag.update_test", "native_vlan", "3961"),
					testAccCheckStaticLagOnDevice(
						"tf-acc-supd",
						3961,
						[]int{3962},
						[]string{"1.0", "2.0"},
					),
				),
			},
			// Step 2: Update — change native VLAN, remove a member
			{
				Config: testAccLagStaticUpdateStep2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_lag.update_test", "lag_type", "STATIC"),
					resource.TestCheckResourceAttr("f5os_lag.update_test", "members.#", "1"),
					resource.TestCheckResourceAttr("f5os_lag.update_test", "native_vlan", "3962"),
					testAccCheckStaticLagOnDevice(
						"tf-acc-supd",
						3962,
						[]int{3961},
						[]string{"2.0"},
					),
				),
			},
		},
	})
}

const testAccLagStaticUpdateStep1Config = `
resource "f5os_vlan" "supd3961" {
  vlan_id = 3961
  name    = "tf-acc-supd-v3961"
}
resource "f5os_vlan" "supd3962" {
  vlan_id = 3962
  name    = "tf-acc-supd-v3962"
}
resource "f5os_lag" "update_test" {
  name        = "tf-acc-supd"
  lag_type    = "STATIC"
  native_vlan = f5os_vlan.supd3961.vlan_id
  trunk_vlans = [f5os_vlan.supd3962.vlan_id]
  members     = ["1.0", "2.0"]
}
`

const testAccLagStaticUpdateStep2Config = `
resource "f5os_vlan" "supd3961" {
  vlan_id = 3961
  name    = "tf-acc-supd-v3961"
}
resource "f5os_vlan" "supd3962" {
  vlan_id = 3962
  name    = "tf-acc-supd-v3962"
}
resource "f5os_lag" "update_test" {
  name        = "tf-acc-supd"
  lag_type    = "STATIC"
  native_vlan = f5os_vlan.supd3962.vlan_id
  trunk_vlans = [f5os_vlan.supd3961.vlan_id]
  members     = ["2.0"]
}
`
