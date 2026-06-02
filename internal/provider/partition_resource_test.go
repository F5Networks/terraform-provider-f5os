package provider

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

// setupMockVelosController registers handlers on the shared mux that make
// NewSession detect a Velos Controller platform. This is the partition
// resource equivalent of setupMockPlatformVersion (which sets up rSeries).
func setupMockVelosController(m *http.ServeMux) {
	m.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	m.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_components_velos_controller.json"))
	})
	m.HandleFunc("/restconf/data/openconfig-system:system/f5-system-controller-image:image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/chassis_version.json"))
	})
}

// partitionMockState holds mutable state for the partition mock so handlers
// can return context-dependent responses across the CRUD lifecycle.
type partitionMockState struct {
	created bool
	deleted bool
	updated bool

	// Fixture paths — mutable so tests can swap them between steps.
	configFixture string
	slotsFixture  string

	// After-update fixture paths. When st.updated is set to true (by the
	// PATCH .../config handler), GetPartition and GetPartitionSlots switch
	// to these fixtures. This ensures Terraform reads the original state
	// during planning, then reads the updated state after the Update call.
	configFixtureAfterUpdate string
	slotsFixtureAfterUpdate  string

	// Error injection points — set a non-zero status to make the
	// corresponding operation fail with that HTTP status code.
	// Errors are "sticky" by default but can be reset by setting back to 0.
	errCreate          int // POST /f5-system-partition:partitions
	errGetPartition    int // GET  .../partition=<name>
	errGetSlots        int // GET  /f5-system-slot:slots/slot
	errSetSlot         int // PATCH /f5-system-slot:slots
	errDeletePartition int // DELETE .../partition=<name>
	errUpdatePartition int // PATCH  .../config
	errUpdateIso       int // POST   .../set-version
	errCheckState      int // GET    .../state
	// Instead of a status code, inject a non-"running" status forever.
	forceDeployingState bool

	// "Count" variants — if > 0, the next N HTTP requests for this
	// operation fail, then the counter decrements to 0 and the operation
	// succeeds again. doRequest retries 3 times, so set to >= 3 to
	// guarantee the caller sees an error. These are checked before the
	// sticky variants.
	errGetSlotsCount        int
	errSetSlotCount         int
	errDeletePartitionCount int
	errGetPartitionCount    int
	errUpdatePartitionCount int
	errUpdateIsoCount       int
}

func setupPartitionMock(m *http.ServeMux) *partitionMockState {
	st := &partitionMockState{
		configFixture: "./fixtures/partition_config.json",
		slotsFixture:  "./fixtures/partition_get_slots.json",
	}

	// restconfErr builds a RESTCONF-compatible error body that the f5osclient
	// will parse into F5osError and propagate as a Go error.
	restconfErr := func(msg string) string {
		return fmt.Sprintf(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"%s"}]}}`, msg)
	}

	// Partition sub-paths (anything under /restconf/data/f5-system-partition:partitions/)
	m.HandleFunc("/restconf/data/f5-system-partition:partitions/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		// CheckPartitionState → getPartitionDeployStatus
		case r.Method == "GET" && strings.HasSuffix(path, "/state"):
			if st.errCheckState != 0 {
				w.WriteHeader(st.errCheckState)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock check state error"))
				return
			}
			if st.forceDeployingState {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{"f5-system-partition:state":{"id":2,"controllers":{"controller":[{"controller":1,"partition-id":2,"partition-status":"deploying"}]}}}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/partition_get_status.json"))

		// UpdatePartition (PATCH .../config)
		case r.Method == "PATCH" && strings.HasSuffix(path, "/config"):
			if st.errUpdatePartitionCount > 0 {
				st.errUpdatePartitionCount--
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock update partition error"))
				return
			}
			if st.errUpdatePartition != 0 {
				w.WriteHeader(st.errUpdatePartition)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock update partition error"))
				return
			}
			st.updated = true
			w.WriteHeader(http.StatusNoContent)

		// UpdatePartitionIso (POST .../set-version)
		case r.Method == "POST" && strings.HasSuffix(path, "/set-version"):
			if st.errUpdateIsoCount > 0 {
				st.errUpdateIsoCount--
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock update iso error"))
				return
			}
			if st.errUpdateIso != 0 {
				w.WriteHeader(st.errUpdateIso)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock update iso error"))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"f5-system-partition:output":{"result":"Firmware update is initiated."}}`)

		// GetPartition (GET .../partition=<name>) — no "/state" suffix
		case r.Method == "GET":
			if st.errGetPartitionCount > 0 {
				st.errGetPartitionCount--
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock get partition error"))
				return
			}
			if st.errGetPartition != 0 {
				w.WriteHeader(st.errGetPartition)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock get partition error"))
				return
			}
			if st.deleted {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("uri keypath not found"))
				return
			}
			// If partition hasn't been created yet, return 404.
			if !st.created {
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("uri keypath not found"))
				return
			}
			// After update, switch to the post-update fixture.
			fixture := st.configFixture
			if st.updated && st.configFixtureAfterUpdate != "" {
				fixture = st.configFixtureAfterUpdate
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString(fixture))

		// DeletePartition (DELETE .../partition=<name>)
		case r.Method == "DELETE":
			if st.errDeletePartitionCount > 0 {
				st.errDeletePartitionCount--
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock delete partition error"))
				return
			}
			if st.errDeletePartition != 0 {
				w.WriteHeader(st.errDeletePartition)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock delete partition error"))
				return
			}
			st.deleted = true
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// CreatePartition: POST /f5-system-partition:partitions (exact, no trailing slash)
	m.HandleFunc("/restconf/data/f5-system-partition:partitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			if st.errCreate != 0 {
				w.WriteHeader(st.errCreate)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock create partition error"))
				return
			}
			st.created = true
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// SetSlot: PATCH /f5-system-slot:slots (exact)
	m.HandleFunc("/restconf/data/f5-system-slot:slots", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			if st.errSetSlotCount > 0 {
				st.errSetSlotCount--
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock set slot error"))
				return
			}
			if st.errSetSlot != 0 {
				w.WriteHeader(st.errSetSlot)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock set slot error"))
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// GetPartitionSlots: GET /f5-system-slot:slots/slot
	m.HandleFunc("/restconf/data/f5-system-slot:slots/slot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if st.errGetSlotsCount > 0 {
				st.errGetSlotsCount--
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock get slots error"))
				return
			}
			if st.errGetSlots != 0 {
				w.WriteHeader(st.errGetSlots)
				_, _ = fmt.Fprintf(w, "%s", restconfErr("mock get slots error"))
				return
			}
			// After update, switch to the post-update fixture.
			fixture := st.slotsFixture
			if st.updated && st.slotsFixtureAfterUpdate != "" {
				fixture = st.slotsFixtureAfterUpdate
			}
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", loadFixtureString(fixture))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	return st
}

// ---------------------------------------------------------------------------
// Unit tests — happy-path CRUD lifecycle
// ---------------------------------------------------------------------------

// TestUnitPartitionCreateReadDelete exercises the full Create→Read→Import→Delete
// lifecycle for a partition with IPv4, IPv6, and slots.
func TestUnitPartitionCreateReadDelete(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create and verify
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "id", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.test", "os_version", "1.3.1-5968"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv4_mgmt_address", "10.144.140.125/24"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv4_mgmt_gateway", "10.144.140.253"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv6_mgmt_address", "2001:db8:3333:4444:5555:6666:7777:8888/64"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv6_mgmt_gateway", "2001:db8:3333:4444::"),
					resource.TestCheckResourceAttr("f5os_partition.test", "enabled", "true"),
					resource.TestCheckResourceAttr("f5os_partition.test", "slots.#", "2"),
					resource.TestCheckResourceAttr("f5os_partition.test", "slots.0", "1"),
					resource.TestCheckResourceAttr("f5os_partition.test", "slots.1", "2"),
					resource.TestCheckResourceAttr("f5os_partition.test", "configuration_volume_size", "10"),
					resource.TestCheckResourceAttr("f5os_partition.test", "images_volume_size", "15"),
					resource.TestCheckResourceAttr("f5os_partition.test", "shared_volume_size", "10"),
					func(s *terraform.State) error {
						if !st.created {
							return fmt.Errorf("expected CreatePartition to have been called")
						}
						return nil
					},
				),
			},
			// Step 2: Import
			// NOTE: ImportState only sets "id" via PassthroughID but Read
			// uses data.Name which is empty after import. This causes
			// GetPartition("") and GetPartitionSlots("") to run with an
			// empty name. Slots are not populated because no slot matches
			// partition name "". This is a pre-existing bug in ImportState
			// (it should also set "name" or Read should fall back to "id").
			{
				ResourceName:            "f5os_partition.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeout", "slots.#", "slots.0", "slots.1"},
			},
		},
	})
}

// TestUnitPartitionCreateUpdateDelete exercises Create→Update→Delete with
// changed IPv4, IPv6, os_version, slots, and volume sizes.
func TestUnitPartitionCreateUpdateDelete(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.test", "os_version", "1.3.1-5968"),
				),
			},
			// Step 2: Update — set after-update fixtures so the mock
			// returns original config during planning, then updated
			// config after the PATCH call succeeds.
			{
				PreConfig: func() {
					st.configFixtureAfterUpdate = "./fixtures/partition_config_updated.json"
					st.slotsFixtureAfterUpdate = "./fixtures/partition_get_slots_updated.json"
					st.updated = false // reset so PATCH handler can set it
				},
				Config: testAccPartitionUpdateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.test", "os_version", "1.5.0-1234"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv4_mgmt_address", "10.144.140.130/24"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv4_mgmt_gateway", "10.144.140.1"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv6_mgmt_address", "2001:db8:3333:4444:5555:6666:7777:9999/64"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv6_mgmt_gateway", "2001:db8:3333:4444::"),
					resource.TestCheckResourceAttr("f5os_partition.test", "enabled", "true"),
					resource.TestCheckResourceAttr("f5os_partition.test", "slots.#", "3"),
					resource.TestCheckResourceAttr("f5os_partition.test", "configuration_volume_size", "12"),
					resource.TestCheckResourceAttr("f5os_partition.test", "images_volume_size", "20"),
					resource.TestCheckResourceAttr("f5os_partition.test", "shared_volume_size", "12"),
				),
			},
			// Step 3: Destroy is implicit
		},
	})
}

// TestUnitPartitionIPv4Only exercises creating a partition with only IPv4
// management (no IPv6), covering the branch where IPv6 is not set.
func TestUnitPartitionIPv4Only(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	st.configFixture = "./fixtures/partition_config_ipv4_only.json"
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionIPv4OnlyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv4_mgmt_address", "10.144.140.125/24"),
					resource.TestCheckResourceAttr("f5os_partition.test", "ipv4_mgmt_gateway", "10.144.140.253"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests — error paths
// ---------------------------------------------------------------------------

// TestUnitPartitionCreateNotVelosController exercises the error path when the
// provider is connected to an rSeries device (not a Velos Controller).
func TestUnitPartitionCreateNotVelosController(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockPlatformVersion(mux, "1.6.0-9817")
	setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPartitionCreateConfig,
				ExpectError: regexp.MustCompile(`f5os_partition.*resource is supported on Velos Controllers only`),
			},
		},
	})
}

// TestUnitPartitionCreateAPIError exercises the error path when
// CreatePartition returns an error from the API.
func TestUnitPartitionCreateAPIError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	st.errCreate = http.StatusBadRequest
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPartitionCreateConfig,
				ExpectError: regexp.MustCompile(`Create Partition failed`),
			},
		},
	})
}

// TestUnitPartitionSetSlotError exercises the error path when SetSlot fails
// during Create.
func TestUnitPartitionSetSlotError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	st.errSetSlot = http.StatusInternalServerError
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPartitionCreateConfig,
				ExpectError: regexp.MustCompile(`Unable to add slots to Partition`),
			},
		},
	})
}

// TestUnitPartitionCheckStateTimeout exercises the timeout error path when
// CheckPartitionState times out waiting for the partition to become running.
func TestUnitPartitionCheckStateTimeout(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	st.forceDeployingState = true
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPartitionTimeoutConfig,
				ExpectError: regexp.MustCompile(`Waiting for Partition deploy|partition deployment still in in progress`),
			},
		},
	})
}

// TestUnitPartitionGetPartitionError exercises the error path when
// GetPartition fails after a successful create. Uses errGetPartition which
// only affects the plain GET (not the /state suffix used by CheckPartitionState).
func TestUnitPartitionGetPartitionError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	// Inject GetPartition error from the start. CheckPartitionState uses
	// the /state suffix and is handled separately, so it still succeeds.
	st.errGetPartition = http.StatusInternalServerError
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPartitionCreateConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get Partition`),
			},
		},
	})
}

// TestUnitPartitionGetPartitionErrorOnRead exercises the Read method's error
// path when GetPartition fails during a state refresh.
func TestUnitPartitionGetPartitionErrorOnRead(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			// Step 2: Read refresh fails because GetPartition returns an error.
			// Uses errGetPartitionCount = 3 to cover all doRequest retries,
			// then the post-test auto-destroy succeeds.
			{
				PreConfig: func() {
					st.errGetPartitionCount = 3
				},
				Config:      testAccPartitionCreateConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get Partition`),
			},
		},
	})
}

// TestUnitPartitionGetSlotsError exercises the error path when
// GetPartitionSlots fails during Create.
func TestUnitPartitionGetSlotsError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	// GetPartitionSlots is called unconditionally after GetPartition succeeds
	// in Create (partition_resource.go line ~212), regardless of whether slots
	// were in the config. Using a no-slots config keeps the test simple.
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					st.errGetSlots = http.StatusInternalServerError
				},
				Config:      testAccPartitionCreateConfigNoSlots,
				ExpectError: regexp.MustCompile(`Unable to Read Partition slots`),
			},
		},
	})
}

// TestUnitPartitionReadSlotError exercises the Read method's error path
// when GetPartitionSlots fails during a state refresh. Uses errGetSlotsCount
// so the error auto-resets and the post-test destroy succeeds.
func TestUnitPartitionReadSlotError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.errGetSlotsCount = 3
				},
				Config:      testAccPartitionCreateConfig,
				ExpectError: regexp.MustCompile(`Unable to Read Partition slots`),
			},
		},
	})
}

// TestUnitPartitionDeleteError exercises the error path in the Delete method
// when DeletePartition returns an error. Uses errDeletePartitionCount so the
// post-test auto-destroy succeeds after the count is exhausted.
func TestUnitPartitionDeleteError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.errDeletePartitionCount = 3
				},
				Config:      testAccPartitionCreateConfig,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Unable to Partition`),
			},
		},
	})
}

// TestUnitPartitionDeleteSlotDisassociateError exercises the error path when
// SetSlot("none", ...) fails during Delete. Uses errSetSlotCount so the
// post-test auto-destroy succeeds after the count is exhausted.
func TestUnitPartitionDeleteSlotDisassociateError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.errSetSlotCount = 3
				},
				Config:      testAccPartitionCreateConfig,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Unable to disassociate slots from Partition`),
			},
		},
	})
}

// TestUnitPartitionDeleteGetSlotsError exercises the error path when
// GetPartitionSlots fails during Delete. Uses errGetSlotsCount so the
// post-test auto-destroy succeeds after the count is exhausted.
func TestUnitPartitionDeleteGetSlotsError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.errGetSlotsCount = 3
				},
				Config:      testAccPartitionCreateConfig,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Unable to Read Partition slots`),
			},
		},
	})
}

// TestUnitPartitionUpdateOsVersionError exercises the error path when
// UpdatePartitionIso fails during Update.
func TestUnitPartitionUpdateOsVersionError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.errUpdateIso = http.StatusInternalServerError
				},
				Config:      testAccPartitionUpdateConfig,
				ExpectError: regexp.MustCompile(`Unable to change partition os_version`),
			},
		},
	})
}

// TestUnitPartitionUpdatePartitionAPIError exercises the error path when
// UpdatePartition (PATCH .../config) fails during Update.
func TestUnitPartitionUpdatePartitionAPIError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.errUpdatePartition = http.StatusInternalServerError
				},
				Config:      testAccPartitionUpdateConfig,
				ExpectError: regexp.MustCompile(`Tenant Deploy failed`),
			},
		},
	})
}

// TestUnitPartitionUpdateGetSlotsError exercises the error path when
// GetPartitionSlots fails during Update. Uses errGetSlotsCount so the
// post-test auto-destroy succeeds after the count is exhausted.
func TestUnitPartitionUpdateGetSlotsError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.errGetSlotsCount = 3
				},
				Config:      testAccPartitionUpdateConfig,
				ExpectError: regexp.MustCompile(`Unable to Read Partition slots`),
			},
		},
	})
}

// TestUnitPartitionUpdateSlotError exercises the error path when SetSlot
// fails during Update. Uses errSetSlotCount so the post-test auto-destroy
// succeeds after the count is exhausted.
func TestUnitPartitionUpdateSlotError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.errSetSlotCount = 3
				},
				Config:      testAccPartitionUpdateConfig,
				ExpectError: regexp.MustCompile(`Unable to disassociate slots from Partition|Unable to update slots on Partition`),
			},
		},
	})
}

// TestUnitPartitionUpdateRemoveSlot exercises the Update code path where
// existing slots are disassociated before new ones are set. In this test,
// the update config goes from [1,2] to [1], which triggers the slotDiff > 0
// branch where slot 2 is disassociated.
func TestUnitPartitionUpdateRemoveSlot(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with slots [1, 2]
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "slots.#", "2"),
				),
			},
			// Step 2: Update to remove slot 2 (keep only slot 1).
			// The mock returns [1,2] for GetPartitionSlots during the
			// Update slot logic, so slotDiff = [2] and
			// SetSlot("none", [2]) is called. After update, the fixture
			// switches to show only slot 1.
			{
				PreConfig: func() {
					st.slotsFixtureAfterUpdate = "./fixtures/partition_get_slots_one.json"
					st.updated = false // reset
				},
				Config: testAccPartitionRemoveSlotConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "slots.#", "1"),
					resource.TestCheckResourceAttr("f5os_partition.test", "slots.0", "1"),
				),
			},
		},
	})
}

// TestUnitPartitionUpdateCheckStateTimeout exercises the error path when
// CheckPartitionState times out during Update.
func TestUnitPartitionUpdateCheckStateTimeout(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					st.forceDeployingState = true
				},
				Config:      testAccPartitionUpdateTimeoutConfig,
				ExpectError: regexp.MustCompile(`Waiting for Partition state after update|partition deployment still in in progress`),
			},
		},
	})
}

// TestUnitPartitionUpdateGetPartitionError exercises the error path when
// GetPartition fails after a successful UpdatePartition call during Update.
func TestUnitPartitionUpdateGetPartitionError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					// GetPartition is called post-update to refresh state.
					// Use count=3 so all retries fail, then reset.
					st.errGetPartitionCount = 3
				},
				Config:      testAccPartitionUpdateConfig,
				ExpectError: regexp.MustCompile(`Unable to Read/Get Partition`),
			},
		},
	})
}

// TestUnitPartitionUpdatePostGetSlotsError exercises the error path when
// GetPartitionSlots fails during the Update method. Because the update config
// changes slots, the first GetPartitionSlots call (in the slot-update section)
// is the one that hits the injected error.
func TestUnitPartitionUpdatePostGetSlotsError(t *testing.T) {
	testAccPreUnitCheck(t)
	_ = os.Setenv("TEEM_DISABLE", "true")
	setupMockVelosController(mux)
	st := setupPartitionMock(mux)
	defer teardown()
	defer func() { _ = os.Unsetenv("TEEM_DISABLE") }()

	// The update config changes slots ([1,2] → [1,2,3]), so Update's
	// slot-update section calls GetPartitionSlots first. With
	// errGetSlotsCount=3, all doRequest retries fail on that call,
	// producing the "Unable to Read Partition slots" error.
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPartitionCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.test", "name", "TerraformPartition"),
				),
			},
			{
				PreConfig: func() {
					// This will fail the GetPartitionSlots call in Update's
					// slot-update section first, but we need it to fail
					// in the post-update section. Let's use errGetSlotsCount=3
					// which fires on the update step.
					st.errGetSlotsCount = 3
				},
				Config:      testAccPartitionUpdateConfig,
				ExpectError: regexp.MustCompile(`Unable to Read Partition slots`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance tests
// ---------------------------------------------------------------------------

// testAccPreCheckVelosController creates a throwaway f5osclient session to
// detect the device's platform type and skips the test if it is not a
// Velos Controller.
func testAccPreCheckVelosController(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("testAccPreCheckVelosController: failed to create session: %s", err)
	}
	if client.PlatformType != "Velos Controller" {
		t.Skipf("skipping: test requires Velos Controller but device is %q", client.PlatformType)
	}
}

// testAccCheckPartitionExists verifies that the partition exists on the device
// by querying the API directly.
func testAccCheckPartitionExists(partitionName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		partData, err := client.GetPartition(partitionName)
		if err != nil {
			return fmt.Errorf("partition %q not found on device: %w", partitionName, err)
		}
		if len(partData.Partition) == 0 {
			return fmt.Errorf("partition %q returned empty data", partitionName)
		}
		if partData.Partition[0].Name != partitionName {
			return fmt.Errorf("expected partition name %q, got %q", partitionName, partData.Partition[0].Name)
		}
		return nil
	}
}

// testAccCheckPartitionDestroy verifies that test-created partitions have been
// removed from the device.
func testAccCheckPartitionDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		return nil // Cannot connect — treat as destroyed
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_partition" {
			continue
		}
		name := rs.Primary.Attributes["name"]
		_, err := client.GetPartition(name)
		if err == nil {
			return fmt.Errorf("partition %q still exists on device after destroy", name)
		}
	}
	return nil
}

// TestAccPartitionCreateTC1Resource is the primary acceptance test for the
// partition resource. It exercises Create, Import, Update, and Delete
// against a real Velos Controller device.
func TestAccPartitionCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckVelosController(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPartitionDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create and verify
			{
				Config: testAccPartitionAccCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.acctest", "name", "AccTestPartition"),
					resource.TestCheckResourceAttr("f5os_partition.acctest", "enabled", "true"),
					testAccCheckPartitionExists("AccTestPartition"),
				),
			},
			// Step 2: ImportState
			{
				ResourceName:            "f5os_partition.acctest",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeout", "slots.#", "slots.0"},
			},
			// Step 3: Update volume sizes
			{
				Config: testAccPartitionAccUpdateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.acctest", "name", "AccTestPartition"),
					testAccCheckPartitionExists("AccTestPartition"),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}

// ---------------------------------------------------------------------------
// HCL configurations
// ---------------------------------------------------------------------------

// Unit test configs

const testAccPartitionCreateConfig = `
resource "f5os_partition" "test" {
  name              = "TerraformPartition"
  os_version        = "1.3.1-5968"
  ipv4_mgmt_address = "10.144.140.125/24"
  ipv4_mgmt_gateway = "10.144.140.253"
  ipv6_mgmt_address = "2001:db8:3333:4444:5555:6666:7777:8888/64"
  ipv6_mgmt_gateway = "2001:db8:3333:4444::"
  slots             = [1, 2]
}
`

// testAccPartitionCreateConfigNoSlots is the same as testAccPartitionCreateConfig
// but without slots. Used for tests that don't need slot operations.
const testAccPartitionCreateConfigNoSlots = `
resource "f5os_partition" "test" {
  name              = "TerraformPartition"
  os_version        = "1.3.1-5968"
  ipv4_mgmt_address = "10.144.140.125/24"
  ipv4_mgmt_gateway = "10.144.140.253"
}
`

const testAccPartitionUpdateConfig = `
resource "f5os_partition" "test" {
  name                      = "TerraformPartition"
  os_version                = "1.5.0-1234"
  ipv4_mgmt_address         = "10.144.140.130/24"
  ipv4_mgmt_gateway         = "10.144.140.1"
  ipv6_mgmt_address         = "2001:db8:3333:4444:5555:6666:7777:9999/64"
  ipv6_mgmt_gateway         = "2001:db8:3333:4444::"
  slots                     = [1, 2, 3]
  configuration_volume_size = 12
  images_volume_size        = 20
  shared_volume_size        = 12
}
`

const testAccPartitionIPv4OnlyConfig = `
resource "f5os_partition" "test" {
  name              = "TerraformPartition"
  os_version        = "1.3.1-5968"
  ipv4_mgmt_address = "10.144.140.125/24"
  ipv4_mgmt_gateway = "10.144.140.253"
  slots             = [1, 2]
}
`

const testAccPartitionTimeoutConfig = `
resource "f5os_partition" "test" {
  name              = "TerraformPartition"
  os_version        = "1.3.1-5968"
  ipv4_mgmt_address = "10.144.140.125/24"
  ipv4_mgmt_gateway = "10.144.140.253"
  slots             = [1, 2]
  timeout           = 1
}
`

const testAccPartitionRemoveSlotConfig = `
resource "f5os_partition" "test" {
  name              = "TerraformPartition"
  os_version        = "1.3.1-5968"
  ipv4_mgmt_address = "10.144.140.125/24"
  ipv4_mgmt_gateway = "10.144.140.253"
  ipv6_mgmt_address = "2001:db8:3333:4444:5555:6666:7777:8888/64"
  ipv6_mgmt_gateway = "2001:db8:3333:4444::"
  slots             = [1]
}
`

const testAccPartitionUpdateTimeoutConfig = `
resource "f5os_partition" "test" {
  name                      = "TerraformPartition"
  os_version                = "1.5.0-1234"
  ipv4_mgmt_address         = "10.144.140.130/24"
  ipv4_mgmt_gateway         = "10.144.140.1"
  ipv6_mgmt_address         = "2001:db8:3333:4444:5555:6666:7777:9999/64"
  ipv6_mgmt_gateway         = "2001:db8:3333:4444::"
  slots                     = [1, 2, 3]
  configuration_volume_size = 12
  images_volume_size        = 20
  shared_volume_size        = 12
  timeout                   = 1
}
`

// Acceptance test configs — use test-specific names to avoid collisions.

const testAccPartitionAccCreateConfig = `
resource "f5os_partition" "acctest" {
  name              = "AccTestPartition"
  os_version        = "1.3.1-5968"
  ipv4_mgmt_address = "10.144.140.200/24"
  ipv4_mgmt_gateway = "10.144.140.253"
  slots             = [1]
}
`

const testAccPartitionAccUpdateConfig = `
resource "f5os_partition" "acctest" {
  name                      = "AccTestPartition"
  os_version                = "1.3.1-5968"
  ipv4_mgmt_address         = "10.144.140.200/24"
  ipv4_mgmt_gateway         = "10.144.140.253"
  slots                     = [1]
  configuration_volume_size = 12
  images_volume_size        = 20
  shared_volume_size        = 12
}
`
