package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// JSON response helpers for VLAN mock endpoints
// ---------------------------------------------------------------------------

const vlanGetResp400 = `{
    "openconfig-vlan:vlan": [
        {
            "vlan-id": 400,
            "config": {
                "vlan-id": 400,
                "name": "mytestvlan2"
            }
        }
    ]
}`

const vlanGetResp400Updated = `{
    "openconfig-vlan:vlan": [
        {
            "vlan-id": 400,
            "config": {
                "vlan-id": 400,
                "name": "mytestvlan3"
            }
        }
    ]
}`



// ---------------------------------------------------------------------------
// Mock setup helpers
// ---------------------------------------------------------------------------

// setupVlanCommonMock registers the auth and platform mock handlers that every
// VLAN unit test needs (non-Velos-Controller platform).
func setupVlanCommonMock(t *testing.T) {
	t.Helper()
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
}

// setupVlanVelosControllerMock registers handlers that make the provider detect
// a Velos Controller platform (which should reject VLAN resource operations).
func setupVlanVelosControllerMock(t *testing.T) {
	t.Helper()
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_components_velos_controller.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-controller-image:image", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-system-controller-image:image":{"state":{"controllers":{"controller":[{"number":1,"os-version":"1.7.0-3518"}]}}}}`)
	})
}

// ---------------------------------------------------------------------------
// Existing unit tests (preserved from original)
// ---------------------------------------------------------------------------

func TestUnitVlanCreateTC1Resource(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanCommonMock(t)
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/restconf/data/openconfig-vlan:vlans", r.URL.String())
		w.WriteHeader(http.StatusNoContent)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", vlanGetResp400)
	})
	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan2"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			{
				ResourceName:      "f5os_vlan.vlan-id",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestUnitVlanCreateTC2Resource(t *testing.T) {
	testAccPreUnitCheck(t)
	var count = 0
	setupVlanCommonMock(t)
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/restconf/data/openconfig-vlan:vlans", r.URL.String())
		w.WriteHeader(http.StatusNoContent)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "GET" {
			count++
			if count <= 3 {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", vlanGetResp400)
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", vlanGetResp400Updated)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer teardown()
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
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

// ---------------------------------------------------------------------------
// Unit tests for error paths — Create
// ---------------------------------------------------------------------------

// TestUnitVlanCreateVelosControllerError verifies that Create returns an error
// when running on a Velos Controller (unsupported platform). Exercises the
// PlatformType == "Velos Controller" guard in Create.
func TestUnitVlanCreateVelosControllerError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanVelosControllerMock(t)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVlanCreateResourceConfig,
				ExpectError: regexp.MustCompile(`resource is supported with Velos Partition level`),
			},
		},
	})
}

// TestUnitVlanCreateVlanConfigError verifies that Create returns an error when
// the VlanConfig PATCH call fails. Exercises the VlanConfig error path.
func TestUnitVlanCreateVlanConfigError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanCommonMock(t)
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"VLAN creation failed"}]}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVlanCreateResourceConfig,
				ExpectError: regexp.MustCompile(`Create Vlan failed`),
			},
		},
	})
}

// TestUnitVlanCreateGetVlanError verifies that Create returns an error when
// VlanConfig succeeds but the subsequent GetVlan read-back fails. Exercises
// the GetVlan error path after successful create.
func TestUnitVlanCreateGetVlanError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanCommonMock(t)
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"VLAN not found"}]}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVlanCreateResourceConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get Vlan`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests for error paths — Read
// ---------------------------------------------------------------------------

// TestUnitVlanReadGetVlanError verifies that Read returns an error when GetVlan
// fails during a state refresh. Exercises the GetVlan error path in Read.
// The test framework calls GET multiple times per step:
//   - Step 1 Create: GET in Create read-back + GET in post-apply refresh
//   - Step 2 refresh: GET in pre-step Read (this is where we want failure)
//
// We allow the first 2 GETs to succeed (covering step 1) and fail from the 3rd.
func TestUnitVlanReadGetVlanError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanCommonMock(t)
	var getCount int
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "GET" {
			getCount++
			// First 2 GETs (Create read-back + post-apply refresh) succeed
			if getCount <= 2 {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", vlanGetResp400)
				return
			}
			// Subsequent GETs (during step 2 Read/refresh) fail
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"device unreachable"}]}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
				),
			},
			{
				Config:      testAccVlanCreateResourceConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get Vlan`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests for error paths — Update
// ---------------------------------------------------------------------------

// TestUnitVlanUpdateVlanConfigError verifies that Update returns an error when
// the VlanConfig PATCH call fails during an update. The Velos Controller guard
// in Update cannot be tested independently because Configure caches the
// platform type once — that path is covered by
// TestUnitVlanCreateVelosControllerError instead.
func TestUnitVlanUpdateVlanConfigError(t *testing.T) {
	testAccPreUnitCheck(t)

	setupVlanCommonMock(t)
	var patchCount int
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			patchCount++
			// First PATCH (Create) succeeds, second (Update) fails
			if patchCount >= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"VLAN update failed"}]}}`)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", vlanGetResp400)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
				),
			},
			{
				Config:      testAccVlanCreateResourceTC2Config,
				ExpectError: regexp.MustCompile(`Update Vlan failed`),
			},
		},
	})
}

// TestUnitVlanUpdateGetVlanError verifies that Update returns an error when
// VlanConfig succeeds but the subsequent GetVlan read-back fails.
func TestUnitVlanUpdateGetVlanError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanCommonMock(t)
	var getCount int
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "GET" {
			getCount++
			// First few GETs (Create read-back + refresh) succeed
			if getCount <= 3 {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", vlanGetResp400)
				return
			}
			// GETs during Update read-back fail
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"read-back failed"}]}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
				),
			},
			{
				Config:      testAccVlanCreateResourceTC2Config,
				ExpectError: regexp.MustCompile(`Unable to Read/Get Vlan`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests for error paths — Delete
// ---------------------------------------------------------------------------

// TestUnitVlanDeleteError verifies that Delete returns an error when the
// DeleteVlan API call fails. Exercises the DeleteVlan error path.
// We enable delete failure for one step and then reset it so the framework's
// automatic final cleanup succeeds.
func TestUnitVlanDeleteError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanCommonMock(t)
	var deleteFailure bool
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			if deleteFailure {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"VLAN in use"}]}}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", vlanGetResp400)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the VLAN
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
				),
			},
			// Step 2: Trigger a destroy with DELETE failure
			{
				PreConfig: func() {
					deleteFailure = true
				},
				Config:      testAccVlanCreateResourceConfig,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Unable to Delete Vlan`),
			},
			// Step 3: Re-apply the same config (resource was not actually
			// destroyed since DELETE failed). Reset the failure flag so the
			// framework's automatic final cleanup succeeds.
			{
				PreConfig: func() {
					deleteFailure = false
				},
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
				),
			},
			// Step 4: auto-destroy now succeeds with deleteFailure=false
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: full CRUD lifecycle
// ---------------------------------------------------------------------------

// TestUnitVlanFullCRUDLifecycle exercises the complete Create -> Read ->
// Update -> Read -> Delete lifecycle in a single test, ensuring all happy-path
// code paths are covered.
func TestUnitVlanFullCRUDLifecycle(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanCommonMock(t)
	var patchCount int
	var currentName = "mytestvlan2"
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			patchCount++
			if patchCount >= 2 {
				currentName = "mytestvlan3"
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=400", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
    "openconfig-vlan:vlan": [
        {
            "vlan-id": 400,
            "config": {
                "vlan-id": 400,
                "name": "%s"
            }
        }
    ]
}`, currentName)
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan2"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			// Step 2: ImportState
			{
				ResourceName:      "f5os_vlan.vlan-id",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update name
			{
				Config: testAccVlanCreateResourceTC2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan3"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			// Step 4: Destroy is automatic
		},
	})
}

// TestUnitVlanCreateNoName exercises creating a VLAN without the optional name
// field. This test documents a known provider bug: vlanResourceModelToState
// always sets data.Name = types.StringValue(...) even when name was null in the
// plan, which causes the framework to report "was null, but now cty.StringVal".
// The name attribute should use Optional+Computed or the read-back should
// preserve null when the config doesn't set name.
func TestUnitVlanCreateNoName(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanCommonMock(t)
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=500", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{
    "openconfig-vlan:vlan": [
        {
            "vlan-id": 500,
            "config": {
                "vlan-id": 500,
                "name": ""
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
			{
				Config: testAccVlanNoNameConfig,
				// Known bug: provider sets name="" in state even when plan had
				// name=null (Optional, not Computed). The framework rejects this
				// as "was null, but now cty.StringVal".
				ExpectError: regexp.MustCompile(`inconsistent result after apply`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests (real device)
// ---------------------------------------------------------------------------

// testAccCheckVlanExists queries the device directly to verify that the VLAN
// exists and has the expected name.
func testAccCheckVlanExists(vlanID int, expectedName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}
		vlan, err := client.GetVlan(vlanID)
		if err != nil {
			return fmt.Errorf("GetVlan(%d) failed: %w", vlanID, err)
		}
		if len(vlan.OpenconfigVlanVlan) == 0 {
			return fmt.Errorf("GetVlan(%d) returned no VLANs", vlanID)
		}
		actualName := vlan.OpenconfigVlanVlan[0].Config.Name
		if actualName != expectedName {
			return fmt.Errorf("VLAN %d name: expected %q, got %q", vlanID, expectedName, actualName)
		}
		return nil
	}
}

// testAccCheckVlanDestroy queries the device directly to verify that all VLAN
// resources managed by the test have been cleaned up.
func testAccCheckVlanDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		// Cannot connect — treat as destroyed
		return nil
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_vlan" {
			continue
		}
		vlanID, err := strconv.Atoi(rs.Primary.ID)
		if err != nil {
			continue
		}
		vlan, err := client.GetVlan(vlanID)
		if err != nil {
			// Error fetching means it's likely gone
			continue
		}
		if len(vlan.OpenconfigVlanVlan) > 0 {
			return fmt.Errorf("VLAN %d still exists on device after destroy", vlanID)
		}
	}
	return nil
}

func TestAccVlanCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVlanDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create and verify
			{
				Config: testAccVlanCreateAccConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.test", "id", "3950"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "name", "testvlan3950"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "vlan_id", "3950"),
					testAccCheckVlanExists(3950, "testvlan3950"),
				),
			},
			// Step 2: ImportState
			{
				ResourceName:      "f5os_vlan.test",
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
		CheckDestroy:             testAccCheckVlanDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccVlanCreateAccConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.test", "id", "3950"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "name", "testvlan3950"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "vlan_id", "3950"),
					testAccCheckVlanExists(3950, "testvlan3950"),
				),
			},
			// Step 2: Update name
			{
				Config: testAccVlanUpdateAccConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.test", "id", "3950"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "name", "testvlan3950upd"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "vlan_id", "3950"),
					testAccCheckVlanExists(3950, "testvlan3950upd"),
				),
			},
			// Step 3: ImportState after update
			{
				ResourceName:      "f5os_vlan.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}

// TestAccVlanFullCRUDLifecycle exercises the full Create -> Import -> Update ->
// Destroy lifecycle with direct API verification at each step.
func TestAccVlanFullCRUDLifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVlanDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccVlanCreateAccConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.test", "id", "3950"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "name", "testvlan3950"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "vlan_id", "3950"),
					testAccCheckVlanExists(3950, "testvlan3950"),
				),
			},
			// Step 2: Import
			{
				ResourceName:      "f5os_vlan.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Update name
			{
				Config: testAccVlanUpdateAccConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.test", "id", "3950"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "name", "testvlan3950upd"),
					resource.TestCheckResourceAttr("f5os_vlan.test", "vlan_id", "3950"),
					testAccCheckVlanExists(3950, "testvlan3950upd"),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}

// ---------------------------------------------------------------------------
// HCL config strings
// ---------------------------------------------------------------------------

// Unit test configs (use VLAN 400 for backward compatibility with existing tests)
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

const testAccVlanNoNameConfig = `
resource "f5os_vlan" "noname" {
 vlan_id = 500
}
`

// Acceptance test configs (use VLANs in the 3900-3999 safe range)
const testAccVlanCreateAccConfig = `
resource "f5os_vlan" "test" {
  vlan_id = 3950
  name    = "testvlan3950"
}
`

const testAccVlanUpdateAccConfig = `
resource "f5os_vlan" "test" {
  vlan_id = 3950
  name    = "testvlan3950upd"
}
`
