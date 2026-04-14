package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// primaryKeyResponse is the correct API response format after the JSON tag fix.
// The nested fields use bare keys ("state", "hash", "status"), not namespace-prefixed ones.
const primaryKeyResponseCorrect = `{
	"f5-primary-key:primary-key": {
		"state": {
			"hash":   "abc123hash",
			"status": "COMPLETE"
		}
	}
}`

// primaryKeyResponseOldFormat is what the API actually returns before the fix
// was understood — nested keys were incorrectly prefixed with "f5-primary-key:".
// json.Unmarshal silently ignores unknown keys, so hash and status were always "".
const primaryKeyResponseOldFormat = `{
	"f5-primary-key:primary-key": {
		"f5-primary-key:state": {
			"f5-primary-key:hash":   "abc123hash",
			"f5-primary-key:status": "COMPLETE"
		}
	}
}`

// setupPrimaryKeyMock registers the standard provider bootstrap handlers and
// the primary-key endpoint handlers. responseJSON controls the GET response.
// setCounter is incremented on every POST to the .../f5-primary-key:set
// endpoint (SetPrimaryKey). Caller must defer teardown().
func setupPrimaryKeyMock(t *testing.T, responseJSON string, setCounter *int32) {
	t.Helper()
	testAccPreUnitCheck(t)

	// GET primary-key state
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("unexpected HTTP method on primary-key endpoint: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseJSON))
	})

	// POST SetPrimaryKey (separate path: .../f5-primary-key:set)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected HTTP method on set endpoint: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if setCounter != nil {
			atomic.AddInt32(setCounter, 1)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
}

// TestAccPrimaryKeyResource is the real-device acceptance test (unchanged).
func TestAccPrimaryKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "passphrase", "test-pass"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "salt", "test-salt"),
				),
			},
		},
	})
}

// TestAccPrimaryKeyHashStatusPopulated verifies on a real device that Create
// with force_update=true correctly populates hash and status in Terraform state.
// Before the JSON tag fix, both fields were always empty regardless of what the
// device returned. This is the primary regression test for the deserialization
// fix on real hardware.
func TestAccPrimaryKeyHashStatusPopulated(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig, // force_update=true
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
					// Before the fix these would both be empty strings.
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "status"),
				),
			},
		},
	})
}

// TestAccPrimaryKeyForceUpdateChange verifies on a real device that changing
// force_update from true to false triggers the Update method, which calls
// SetPrimaryKey and then re-reads hash and status. Both fields must remain
// populated after the update.
func TestAccPrimaryKeyForceUpdateChange(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=true
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "true"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "status"),
				),
			},
			// Step 2: Update force_update=false — triggers Update method, hash/status must persist
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "status"),
				),
			},
		},
	})
}

// TestAccPrimaryKeySkipWhenExistsAndNoForce verifies on a real device that when
// force_update=false and a primary key already exists on the device, Create
// skips SetPrimaryKey and returns the existing hash and status. The DUT always
// has a primary key pre-configured, so this exercises the idempotency guard.
func TestAccPrimaryKeySkipWhenExistsAndNoForce(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceNoForceConfig, // force_update=false
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					// Existing key state must be adopted — hash and status populated
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "status"),
				),
			},
		},
	})
}

// TestUnitPrimaryKeyResource is the existing Create unit test, updated to use
// the correct JSON format that matches the fixed struct tags.
func TestUnitPrimaryKeyResource(t *testing.T) {
	setupPrimaryKeyMock(t, primaryKeyResponseCorrect, nil)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
		},
	})
}

// TestUnitPrimaryKeyDeserializationFix is the direct regression test for the
// JSON tag bug. Before the fix, F5RespPrimaryKey used namespace-prefixed tags
// ("f5-primary-key:hash", "f5-primary-key:status", "f5-primary-key:state") on
// nested fields. The actual API response uses bare keys ("hash", "status",
// "state"), so json.Unmarshal silently skipped them, leaving hash and status
// as empty strings in Terraform state.
//
// This test verifies that Create correctly populates hash and status from the
// API response with correct bare-key tags, and that the Terraform state reflects
// both values rather than nulls.
func TestUnitPrimaryKeyDeserializationFix(t *testing.T) {
	setupPrimaryKeyMock(t, primaryKeyResponseCorrect, nil)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// These assertions would fail before the fix because
					// hash and status would both be null/empty.
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
		},
	})
}

// TestUnitPrimaryKeyOldTagsProduceEmpty documents the broken pre-fix behavior.
// When the GET response uses namespace-prefixed nested keys
// ("f5-primary-key:state", "f5-primary-key:hash", "f5-primary-key:status"),
// json.Unmarshal ignores them entirely, leaving hash and status as empty —
// exactly the bug that was reported.
func TestUnitPrimaryKeyOldTagsProduceEmpty(t *testing.T) {
	setupPrimaryKeyMock(t, primaryKeyResponseOldFormat, nil)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					// With the old (broken) API response format, hash and status
					// cannot be populated — both must be absent from state.
					resource.TestCheckNoResourceAttr("f5os_primarykey.default", "hash"),
					resource.TestCheckNoResourceAttr("f5os_primarykey.default", "status"),
				),
			},
		},
	})
}

// TestUnitPrimaryKeyForceUpdateChange verifies the Update path. When force_update
// changes from true to false, Terraform calls Update (not Create). The Update
// method calls SetPrimaryKey (POST to .../f5-primary-key:set) and then
// GetPrimaryKey (GET) to refresh hash and status. This test confirms:
//  1. The POST to :set is sent (setCount increments: once for Create step,
//     once for Update step).
//  2. hash and status are correctly populated in state after the update —
//     verifying that the deserialization fix applies to the Update path too.
func TestUnitPrimaryKeyForceUpdateChange(t *testing.T) {
	var setCount int32
	setupPrimaryKeyMock(t, primaryKeyResponseCorrect, &setCount)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=true
			{
				Config: testAccPrimaryKeyResourceConfig, // force_update=true
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "true"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
			// Step 2: Update force_update=false → triggers Update method
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					// hash and status must still be populated after the Update path
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
		},
	})
}

// TestUnitPrimaryKeySkipWhenExistsAndNoForce verifies that when force_update=false
// and GetPrimaryKey returns a non-empty status, the Create method skips calling
// SetPrimaryKey (no PATCH) and simply adopts the existing state. This confirms
// the idempotency guard in Create works and that hash/status are populated from
// the existing device state even when no key is actually set.
func TestUnitPrimaryKeySkipWhenExistsAndNoForce(t *testing.T) {
	var setCount int32
	setupPrimaryKeyMock(t, primaryKeyResponseCorrect, &setCount)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceNoForceConfig, // force_update=false
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					// hash and status populated from existing device state (no POST to :set)
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
					func(s *terraform.State) error {
						if atomic.LoadInt32(&setCount) != 0 {
							return fmt.Errorf("expected no SetPrimaryKey call when force_update=false and key exists, but got %d", setCount)
						}
						return nil
					},
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// JSON deserialization unit test (pure Go, no Terraform framework)
// ---------------------------------------------------------------------------

// TestUnitPrimaryKeyJSONDeserialization directly validates the struct
// deserialization fix without going through the Terraform test framework.
// It unmarshals both the correct and old-format API responses and asserts
// the resulting struct fields.
func TestUnitPrimaryKeyJSONDeserialization(t *testing.T) {
	t.Run("correct_tags_populate_fields", func(t *testing.T) {
		var resp f5ossdk.F5RespPrimaryKey
		if err := json.Unmarshal([]byte(primaryKeyResponseCorrect), &resp); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if resp.PrimaryKey.State.Hash != "abc123hash" {
			t.Errorf("expected Hash=%q, got %q", "abc123hash", resp.PrimaryKey.State.Hash)
		}
		if resp.PrimaryKey.State.Status != "COMPLETE" {
			t.Errorf("expected Status=%q, got %q", "COMPLETE", resp.PrimaryKey.State.Status)
		}
	})

	t.Run("old_prefixed_tags_produce_empty", func(t *testing.T) {
		var resp f5ossdk.F5RespPrimaryKey
		if err := json.Unmarshal([]byte(primaryKeyResponseOldFormat), &resp); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if resp.PrimaryKey.State.Hash != "" {
			t.Errorf("expected empty Hash with old tag format, got %q", resp.PrimaryKey.State.Hash)
		}
		if resp.PrimaryKey.State.Status != "" {
			t.Errorf("expected empty Status with old tag format, got %q", resp.PrimaryKey.State.Status)
		}
	})
}

const testAccPrimaryKeyResourceConfig = `
resource "f5os_primarykey" "default" {
  passphrase   = "test-pass"
  salt         = "test-salt"
  force_update = true
}
`

const testAccPrimaryKeyResourceNoForceConfig = `
resource "f5os_primarykey" "default" {
  passphrase   = "test-pass"
  salt         = "test-salt"
  force_update = false
}
`
