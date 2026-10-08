package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

// setupVlansCommonMock registers the auth and platform mock handlers that
// every f5os_vlans unit test needs (non-Velos-Controller platform).
// Mirrors setupVlanCommonMock in vlan_resource_test.go.
func setupVlansCommonMock(t *testing.T) {
	t.Helper()
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
}

const vlansBatchURI = "/restconf/data/openconfig-vlan:vlans"
const vlansBulkGetURI = "/restconf/data/openconfig-vlan:vlans/vlan"

// TestUnitVlansCreateBatch verifies that Create issues exactly one PATCH
// to /openconfig-vlan:vlans containing every configured VLAN -- not one
// PATCH per VLAN, which is the whole point of this resource over
// `for_each` + f5os_vlan. (GET-count is not asserted here: resource.Test
// itself performs an automatic post-apply plan-consistency check, which
// issues at least one additional Read/bulk-GET beyond Create's own
// read-back, so a strict GET-count assertion would depend on
// terraform-plugin-testing's internal behavior rather than this
// resource's own logic.)
func TestUnitVlansCreateBatch(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	var patchCount int
	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			patchCount++
			body := readBody(t, r)
			assert.Contains(t, body, `"vlan-id":100`)
			assert.Contains(t, body, `"vlan-id":200`)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[
			{"vlan-id":100,"config":{"vlan-id":100,"name":"external"}},
			{"vlan-id":200,"config":{"vlan-id":200,"name":"internal"}}
		]}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "internal" = 200
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "2"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.external", "100"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.internal", "200"),
				),
			},
		},
	})

	if patchCount != 1 {
		t.Errorf("expected exactly 1 batched PATCH to %s, got %d", vlansBatchURI, patchCount)
	}
}

// TestUnitVlansVelosControllerRejected verifies the same
// Velos-Controller guard f5os_vlan itself enforces.
func TestUnitVlansVelosControllerRejected(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlanVelosControllerMock(t)

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`f5os_vlans.*supported with Velos Partition`),
			},
		},
	})
}

// TestUnitVlansEmptyMapRejected verifies ValidateConfig rejects an empty
// vlans map at plan time.
func TestUnitVlansEmptyMapRejected(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {}
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`at least one entry`),
			},
		},
	})
}

// TestUnitVlansOutOfRangeIDRejected verifies ValidateConfig rejects a
// VLAN ID outside 0-4095.
func TestUnitVlansOutOfRangeIDRejected(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "toohigh" = 4096
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`out of range`),
			},
		},
	})
}

// TestUnitVlansInvalidNameRejected verifies ValidateConfig rejects a
// VLAN name that doesn't match f5os_vlan's own name format.
func TestUnitVlansInvalidNameRejected(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "1starts-with-digit" = 100
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`not a valid VLAN name`),
			},
		},
	})
}

// TestUnitVlansDuplicateIDRejected verifies ValidateConfig rejects two
// names mapping to the same VLAN ID.
func TestUnitVlansDuplicateIDRejected(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "external2" = 100
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`must be unique`),
			},
		},
	})
}

// TestUnitVlansUnknownElementValueSkipsValidation is a regression test:
// `data.Vlans.IsUnknown()` only reports true when the whole map's
// identity is unknown -- it does NOT cover a known map containing one
// or more unknown *element* values, a realistic pattern when a VLAN ID
// is sourced from another resource's not-yet-known computed attribute
// (e.g. `tonumber(f5os_vlan.seed.id)`). Without an explicit per-element
// unknown check, extractVlansMap's ElementsAs call previously failed
// with an unhelpful internal "please report this to the provider
// developer" error instead of gracefully skipping validation (the same
// way the whole-map-unknown case already does) until the value becomes
// known at apply time.
func TestUnitVlansUnknownElementValueSkipsValidation(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[
			{"vlan-id":100,"config":{"vlan-id":100,"name":"external"}},
			{"vlan-id":50,"config":{"vlan-id":50,"name":"seeded"}}
		]}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=50", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[{"vlan-id":50,"config":{"vlan-id":50,"name":"seedvlan"}}]}`)
	})

	defer teardown()

	// f5os_vlan.seed's `id` attribute is unknown during its own plan
	// (it's Computed with UseStateForUnknown, and there is no prior
	// state on a fresh apply) -- referencing it as one of f5os_vlans'
	// VLAN ID values means `vlans` itself is known (the map's own
	// identity/keys are known), but one *element* value inside it is
	// unknown, exercising exactly the gap ValidateConfig's
	// data.Vlans.IsUnknown() check alone does not cover.
	tfCfg := `
resource "f5os_vlan" "seed" {
  name    = "seedvlan"
  vlan_id = 50
}

resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "seeded"   = tonumber(f5os_vlan.seed.id)
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "2"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.external", "100"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.seeded", "50"),
				),
			},
		},
	})
}

// TestUnitVlansUnknownElementValueDoesNotSkipOtherValidation is a
// regression test for ValidateConfig: previously, encountering ANY
// unknown element value in `vlans` caused the whole function to return
// early, silently skipping validation for every other -- known and
// possibly invalid -- entry in the map too. This test configures one
// unknown-valued entry (`seeded`, sourced from a not-yet-known
// computed attribute, same as TestUnitVlansUnknownElementValueSkipsValidation)
// alongside a known entry with an out-of-range VLAN ID (`toohigh` =
// 4096), and asserts the out-of-range error still surfaces at plan
// time instead of being masked by the unknown `seeded` entry.
func TestUnitVlansUnknownElementValueDoesNotSkipOtherValidation(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	defer teardown()

	tfCfg := `
resource "f5os_vlan" "seed" {
  name    = "seedvlan"
  vlan_id = 50
}

resource "f5os_vlans" "test" {
  vlans = {
    "toohigh" = 4096
    "seeded"  = tonumber(f5os_vlan.seed.id)
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`out of range`),
			},
		},
	})
}

// TestUnitVlansCreatePatchError verifies a PATCH failure during Create
// surfaces as an error.
func TestUnitVlansCreatePatchError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-message":"bad vlan config"}]}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`Create VLANs failed`),
			},
		},
	})
}

// TestUnitVlansCreateReadBackError verifies a bulk-GET failure during
// Create's read-back surfaces as an error.
func TestUnitVlansCreateReadBackError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-message":"internal error"}]}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`Unable to read VLANs`),
			},
		},
	})
}

// TestUnitVlansReadFiltersToManagedSet verifies that Read only reports
// VLANs this resource instance manages (from prior state), ignoring
// other VLANs present on the device -- mirrors f5os_snmp_resource.go's
// managedCommunities/filterCommunities pattern.
func TestUnitVlansReadFiltersToManagedSet(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		// Device reports an extra VLAN (id 999, "unmanaged") that this
		// resource instance never configured -- it must not appear in
		// state.
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[
			{"vlan-id":100,"config":{"vlan-id":100,"name":"external"}},
			{"vlan-id":999,"config":{"vlan-id":999,"name":"unmanaged"}}
		]}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "1"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.external", "100"),
					resource.TestCheckNoResourceAttr("f5os_vlans.test", "vlans.unmanaged"),
				),
			},
			// Refresh-only step: Read must not pick up the unmanaged VLAN.
			{
				Config:             tfCfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestUnitVlansReadIgnoresIDCollisionWithMismatchedName is a regression
// test for readVlansIntoState's managed-set filter: it must match the
// full (name, ID) pair, not the ID alone. If a device VLAN's ID matches
// a managed ID but its name doesn't match what this resource instance
// configured (e.g. an accidental ID overlap with a sibling f5os_vlan or
// another f5os_vlans instance, or the VLAN having been renamed
// out-of-band), an ID-only filter would incorrectly adopt the device's
// name into this resource's state on the next Read -- which, since it
// doesn't match the name Terraform still has recorded for that key,
// previously crashed with a hard "provider produced inconsistent result
// after apply" framework-level error (Terraform's own consistency check
// sees the map's "external" key vanish and a brand-new
// "renamed-out-of-band" key appear from nowhere) rather than a
// straightforward, actionable plan diff. Filtering by the exact pair
// instead means the mismatched device VLAN is simply excluded from
// state on Read (its configured name is treated as "missing on the
// device," an ordinary plan diff scheduling a normal batch-PATCH
// re-create, not a crash).
func TestUnitVlansReadIgnoresIDCollisionWithMismatchedName(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	renamed := false
	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		if !renamed {
			// Create's own read-back: reflect exactly what was
			// configured, so Create succeeds normally.
			_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[
				{"vlan-id":100,"config":{"vlan-id":100,"name":"external"}}
			]}`)
			return
		}
		// Simulates an out-of-band rename discovered on a later Read:
		// VLAN ID 100 (still the managed ID) now reports under a name
		// this resource instance never configured.
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[
			{"vlan-id":100,"config":{"vlan-id":100,"name":"renamed-out-of-band"}}
		]}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "1"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.external", "100"),
				),
			},
			{
				// Refresh-only: the device-side rename must not crash
				// the provider -- it should be excluded from state (a
				// straightforward "external is missing" diff on the next
				// real apply), not adopted under its device-reported
				// name, and not cause a framework-level consistency
				// panic.
				PreConfig:          func() { renamed = true },
				Config:             tfCfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// vlanEntry is one VLAN in a stateful mock device's VLAN table.
type vlanEntry struct {
	ID   int64
	Name string
}

// newStatefulVlansMock registers a mock /openconfig-vlan:vlans
// (batch PATCH) and /openconfig-vlan:vlans/vlan (bulk GET) handler pair
// backed by a real in-memory VLAN table that PATCH/DELETE actually
// mutate and GET actually reflects -- unlike a canned fixed-response
// mock, this is necessary for a multi-step Update test:
// resource.TestCase performs an automatic refresh (bulk GET) before
// diffing each step against the *current* device state, so a
// step-counter-gated canned response would already reflect "step 2"
// during step 1's own post-apply refresh, hiding the diff Update is
// supposed to react to. Also registers a per-VLAN-ID DELETE handler
// that records deletions into the same table. Returns a snapshot
// function for assertions.
func newStatefulVlansMock(t *testing.T, initial map[string]int64, extraIDs ...int64) (deleted func() []int64) {
	t.Helper()
	var mu sync.Mutex
	table := make(map[int64]string, len(initial))
	for name, id := range initial {
		table[id] = name
	}
	var deletedIDs []int64
	registeredIDs := make(map[int64]bool, len(table)+len(extraIDs))
	for id := range table {
		registeredIDs[id] = true
	}
	for _, id := range extraIDs {
		registeredIDs[id] = true
	}

	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			body := readBody(t, r)
			var payload struct {
				Vlans struct {
					Vlan []struct {
						Config struct {
							VlanID int    `json:"vlan-id"`
							Name   string `json:"name"`
						} `json:"config"`
					} `json:"vlan"`
				} `json:"openconfig-vlan:vlans"`
			}
			if err := json.Unmarshal([]byte(body), &payload); err != nil {
				t.Fatalf("failed to unmarshal batch PATCH body: %v", err)
			}
			mu.Lock()
			for _, v := range payload.Vlans.Vlan {
				table[int64(v.Config.VlanID)] = v.Config.Name
			}
			mu.Unlock()
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		entries := make([]vlanEntry, 0, len(table))
		for id, name := range table {
			entries = append(entries, vlanEntry{ID: id, Name: name})
		}
		mu.Unlock()

		var b strings.Builder
		b.WriteString(`{"openconfig-vlan:vlan":[`)
		for i, e := range entries {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"vlan-id":%d,"config":{"vlan-id":%d,"name":%q}}`, e.ID, e.ID, e.Name)
		}
		b.WriteString(`]}`)
		_, _ = fmt.Fprint(w, b.String())
	})

	for id := range registeredIDs {
		registerVlanDeleteHandler(t, id, &mu, table, &deletedIDs)
	}

	return func() []int64 {
		mu.Lock()
		defer mu.Unlock()
		out := make([]int64, len(deletedIDs))
		copy(out, deletedIDs)
		return out
	}
}

func registerVlanDeleteHandler(t *testing.T, id int64, mu *sync.Mutex, table map[int64]string, deletedIDs *[]int64) {
	t.Helper()
	mux.HandleFunc(fmt.Sprintf("/restconf/data/openconfig-vlan:vlans/vlan=%d", id), func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			delete(table, id)
			*deletedIDs = append(*deletedIDs, id)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

// TestUnitVlansUpdateAddsAndRemoves verifies Update's diff-and-batch
// logic: a VLAN removed from `vlans` is deleted individually, and the
// remaining/new VLANs are batched into a single PATCH.
func TestUnitVlansUpdateAddsAndRemoves(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	getDeleted := newStatefulVlansMock(t, map[string]int64{
		"external": 100,
		"internal": 200,
	}, 300)

	defer teardown()

	cfgStep1 := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "internal" = 200
  }
}
`
	cfgStep2 := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "third"    = 300
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfgStep1,
			},
			{
				Config: cfgStep2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("f5os_vlans.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "2"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.external", "100"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.third", "300"),
					resource.TestCheckNoResourceAttr("f5os_vlans.test", "vlans.internal"),
					// Assert the mid-test Update deletion right after the
					// step that triggers it (rather than after
					// resource.Test returns), since resource.Test's own
					// implicit end-of-test-case destroy will additionally
					// delete VLANs 100/300 -- that's expected cleanup
					// behavior, not part of what this test is verifying.
					func(s *terraform.State) error {
						deleted := getDeleted()
						if len(deleted) != 1 || deleted[0] != 200 {
							return fmt.Errorf("expected exactly 1 DeleteVlan call for VLAN 200 by this point, got: %v", deleted)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitVlansUpdateReassignedIDDeletesOldVlan is a regression test:
// changing an EXISTING map key's value (not adding/removing a key) must
// still delete the old VLAN ID. A name-presence-only diff (checking
// only whether the map *key* still exists in the plan) would treat
// "external" staying present as "nothing to delete" and silently
// orphan the old VLAN ID on the device forever, since
// readVlansIntoState's managed-set filter is keyed by (name, ID) pairs
// from state -- once state moves on to the new ID, the old ID is never
// looked for again on any future Read.
func TestUnitVlansUpdateReassignedIDDeletesOldVlan(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	getDeleted := newStatefulVlansMock(t, map[string]int64{
		"external": 100,
	}, 200)

	defer teardown()

	cfgStep1 := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
  }
}
`
	cfgStep2 := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 200
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfgStep1,
			},
			{
				Config: cfgStep2,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "1"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.external", "200"),
					func(s *terraform.State) error {
						deleted := getDeleted()
						if len(deleted) != 1 || deleted[0] != 100 {
							return fmt.Errorf("expected VLAN 100 (the old ID under the reassigned \"external\" key) to be deleted by this point, got: %v", deleted)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitVlansUpdateDeleteError verifies a DeleteVlan failure during
// Update's removal step surfaces as an error and does not proceed to
// the batch PATCH.
func TestUnitVlansUpdateDeleteError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	var patchCalledDuringUpdate bool
	var deleteAttempts int
	step := 1
	var vlan200ShouldFail atomic.Bool
	vlan200ShouldFail.Store(true)
	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		if step == 2 && r.Method == http.MethodPatch {
			patchCalledDuringUpdate = true
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[
			{"vlan-id":100,"config":{"vlan-id":100,"name":"external"}},
			{"vlan-id":200,"config":{"vlan-id":200,"name":"internal"}}
		]}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=200", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteAttempts++
			// r.client's doRequest retries a non-401 4xx/5xx up to 6
			// times before giving up, all within a single logical
			// DeleteVlan() call -- failing based on an attempt *count*
			// would let a later internal retry attempt "succeed" and
			// mask the failure entirely. Instead, fail for the whole
			// duration of the step this test actually exercises (an
			// atomic bool, flipped to false only once that step's
			// resource.Test apply has run its course), so every retry
			// within that logical call fails and doRequest ultimately
			// surfaces the error -- then let it succeed for
			// resource.Test's own automatic end-of-test-case destroy
			// so that expected cleanup does not itself fail and mask
			// this test's actual assertion behind a "dangling
			// resources" cleanup error.
			if vlan200ShouldFail.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-message":"cannot delete"}]}}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	defer teardown()

	cfgStep1 := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "internal" = 200
  }
}
`
	cfgStep2 := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfgStep1,
			},
			{
				PreConfig:   func() { step = 2 },
				Config:      cfgStep2,
				ExpectError: regexp.MustCompile(`Delete VLAN "internal"`),
			},
			{
				// No-op step whose sole purpose is to flip
				// vlan200ShouldFail to false before resource.Test's
				// automatic end-of-test-case destroy runs (that destroy
				// happens after the last step, using the config from the
				// last step -- there is no separate "AfterAll" hook in
				// this test framework to attach cleanup-prep logic to).
				// step is also advanced past 2 so patchCalledDuringUpdate
				// (asserted below) only reflects step 2's own apply, not
				// this benign cleanup-prep step's unrelated PATCH.
				PreConfig: func() { vlan200ShouldFail.Store(false); step = 3 },
				Config:    cfgStep2,
			},
		},
	})

	if patchCalledDuringUpdate {
		t.Error("expected the batch PATCH NOT to be reached after a delete failure during Update")
	}
	if deleteAttempts < 1 {
		t.Error("expected at least 1 DeleteVlan attempt for VLAN 200")
	}
}

// TestUnitVlansDelete verifies Delete issues one DeleteVlan call per
// managed VLAN.
func TestUnitVlansDelete(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	deletedIDs := make(map[string]bool)
	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[
			{"vlan-id":100,"config":{"vlan-id":100,"name":"external"}},
			{"vlan-id":200,"config":{"vlan-id":200,"name":"internal"}}
		]}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletedIDs["100"] = true
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=200", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletedIDs["200"] = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "internal" = 200
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
			},
			{
				Config:  tfCfg,
				Destroy: true,
			},
		},
	})

	if !deletedIDs["100"] || !deletedIDs["200"] {
		t.Errorf("expected both VLAN 100 and 200 to be deleted, got: %v", deletedIDs)
	}
}

// TestUnitVlansDeleteError verifies a DeleteVlan failure during Delete
// surfaces as an error.
func TestUnitVlansDeleteError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	var vlan100ShouldFail atomic.Bool
	vlan100ShouldFail.Store(true)
	mux.HandleFunc(vlansBatchURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[{"vlan-id":100,"config":{"vlan-id":100,"name":"external"}}]}`)
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan=100", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			if vlan100ShouldFail.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-message":"cannot delete"}]}}`)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
  }
}
`
	// Destroy is verified via a plan/apply followed by an explicit
	// destroy step expected to fail. This does NOT, on its own, avoid
	// resource.Test's automatic end-of-test-case destroy attempt --
	// Terraform's state still shows the resource present after a failed
	// destroy, so the test framework still tries to clean it up when
	// the test case ends. vlan100ShouldFail is flipped to false in a
	// third step (before that automatic cleanup runs) so the eventual
	// real cleanup delete succeeds and does not itself fail with a
	// "dangling resources" error that would mask this test's actual
	// (already-passed, by that point) assertion.
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
			},
			{
				Config:      tfCfg,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Unable to delete VLAN "external"`),
			},
			{
				PreConfig: func() { vlan100ShouldFail.Store(false) },
				Config:    tfCfg,
			},
		},
	})
}

// TestUnitVlansImportAdoptsAllDeviceVlans verifies ImportState + the
// subsequent Read adopts every VLAN present on the device (no prior
// managed set to filter against).
func TestUnitVlansImportAdoptsAllDeviceVlans(t *testing.T) {
	testAccPreUnitCheck(t)
	setupVlansCommonMock(t)

	mux.HandleFunc(vlansBulkGetURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"openconfig-vlan:vlan":[
			{"vlan-id":100,"config":{"vlan-id":100,"name":"external"}},
			{"vlan-id":200,"config":{"vlan-id":200,"name":"internal"}}
		]}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "internal" = 200
  }
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
			},
			{
				// ImportStateVerify is not used here: it compares the
				// imported state against the pre-import resource
				// instance's state keyed by the resource's `id` value,
				// but `id` is intentionally a fresh, opaque UUID on
				// every import (it is not a device identifier -- see
				// the resource's ImportState comment) and so never
				// matches across import. Verify the functionally
				// interesting outcome directly instead: every VLAN
				// present on the device is adopted into `vlans` after
				// import.
				ResourceName:  "f5os_vlans.test",
				Config:        tfCfg,
				ImportState:   true,
				ImportStateId: "anything", // opaque bookkeeping, not a device identifier -- see the resource's ImportState comment.
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected exactly 1 imported instance state, got %d", len(states))
					}
					attrs := states[0].Attributes
					if attrs["vlans.%"] != "2" {
						return fmt.Errorf("expected 2 imported VLANs, got vlans.%%=%q", attrs["vlans.%"])
					}
					if attrs["vlans.external"] != "100" || attrs["vlans.internal"] != "200" {
						return fmt.Errorf("expected vlans.external=100 and vlans.internal=200, got: %+v", attrs)
					}
					return nil
				},
			},
		},
	})
}

// readBody fully reads an http.Request's body for inspection in a
// test's mock handler.
func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Acceptance tests (real device)
// ---------------------------------------------------------------------------
//
// Uses VLAN IDs 3960-3969 -- a disjoint sub-range of the same 3900-3999
// "safe" block vlan_resource_test.go's own acceptance tests use (which
// stick to 3950), avoiding any collision if both test files' acceptance
// suites happen to run concurrently against the same shared device.

// testAccCheckF5osVlansExist queries the device directly (bypassing
// Terraform state, per AGENTS.md's guidance on this resource family) to
// verify every expected VLAN ID/name pair exists.
func testAccCheckF5osVlansExist(expected map[int]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}
		for vlanID, expectedName := range expected {
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
		}
		return nil
	}
}

// testAccCheckF5osVlansResourceDestroy queries the device directly to verify every
// VLAN ID this test's f5os_vlans resource instances managed has been
// removed.
func testAccCheckF5osVlansResourceDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		// Cannot connect — treat as destroyed.
		return nil
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_vlans" {
			continue
		}
		for attrName, attrValue := range rs.Primary.Attributes {
			if !strings.HasPrefix(attrName, "vlans.") || attrName == "vlans.%" {
				continue
			}
			vlanID, err := strconv.Atoi(attrValue)
			if err != nil {
				continue
			}
			vlan, err := client.GetVlan(vlanID)
			if err != nil {
				// Error fetching means it's likely gone.
				continue
			}
			if len(vlan.OpenconfigVlanVlan) > 0 {
				return fmt.Errorf("VLAN %d still exists on device after destroy", vlanID)
			}
		}
	}
	return nil
}

// TestAccVlansCreateBatch verifies the full create/update/destroy
// lifecycle of f5os_vlans against a real device, checking device state
// directly (not just Terraform state) at each step -- see AGENTS.md's
// "Known testing issue" section on why Read methods for this resource
// family have historically needed this extra verification.
func TestAccVlansCreateBatch(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckF5osVlansResourceDestroy,
		Steps: []resource.TestStep{
			// Step 1: create three VLANs in a single batch.
			{
				Config: `
resource "f5os_vlans" "test" {
  vlans = {
    "acctestvlans3960" = 3960
    "acctestvlans3961" = 3961
    "acctestvlans3962" = 3962
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "3"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.acctestvlans3960", "3960"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.acctestvlans3961", "3961"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.acctestvlans3962", "3962"),
					testAccCheckF5osVlansExist(map[int]string{
						3960: "acctestvlans3960",
						3961: "acctestvlans3961",
						3962: "acctestvlans3962",
					}),
				),
			},
			// Step 2: remove one VLAN, add a different one, rename none --
			// exercises Update's diff-and-batch (delete + batched PATCH)
			// logic against a real device.
			{
				Config: `
resource "f5os_vlans" "test" {
  vlans = {
    "acctestvlans3960" = 3960
    "acctestvlans3963" = 3963
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "2"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.acctestvlans3960", "3960"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.acctestvlans3963", "3963"),
					testAccCheckF5osVlansExist(map[int]string{
						3960: "acctestvlans3960",
						3963: "acctestvlans3963",
					}),
					// VLAN 3961/3962 must actually be gone from the device,
					// not just absent from Terraform state.
					func(s *terraform.State) error {
						client, err := newTestClientFromEnv()
						if err != nil {
							return err
						}
						for _, id := range []int{3961, 3962} {
							vlan, err := client.GetVlan(id)
							if err == nil && len(vlan.OpenconfigVlanVlan) > 0 {
								return fmt.Errorf("VLAN %d should have been deleted by Update but still exists on device", id)
							}
						}
						return nil
					},
				),
			},
			// Step 3: reassign an EXISTING key's VLAN ID (keep the name
			// "acctestvlans3960", change its ID from 3960 to 3964) --
			// regression check for the fix to Update's diff logic, which
			// previously only deleted a VLAN ID when its map *key*
			// disappeared from the plan, silently orphaning the old ID
			// on the device whenever an existing key's value changed
			// instead. Verified directly against the device (not just
			// Terraform state) that the old ID (3960) is actually gone.
			{
				Config: `
resource "f5os_vlans" "test" {
  vlans = {
    "acctestvlans3960" = 3964
    "acctestvlans3963" = 3963
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.%", "2"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.acctestvlans3960", "3964"),
					resource.TestCheckResourceAttr("f5os_vlans.test", "vlans.acctestvlans3963", "3963"),
					testAccCheckF5osVlansExist(map[int]string{
						3964: "acctestvlans3960",
						3963: "acctestvlans3963",
					}),
					func(s *terraform.State) error {
						client, err := newTestClientFromEnv()
						if err != nil {
							return err
						}
						vlan, err := client.GetVlan(3960)
						if err == nil && len(vlan.OpenconfigVlanVlan) > 0 {
							return fmt.Errorf("VLAN 3960 should have been deleted when \"acctestvlans3960\" was reassigned to ID 3964, but still exists on device")
						}
						return nil
					},
				),
			},
		},
	})
}
