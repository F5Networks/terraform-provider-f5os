package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

func withFastPolling(t *testing.T) {
	origInterval := primaryKeyPollInterval
	origTimeout := primaryKeyMigrationTimeout
	origDelay := primaryKeyInitialReadDelay

	primaryKeyPollInterval = 10 * time.Millisecond
	primaryKeyMigrationTimeout = 100 * time.Millisecond
	primaryKeyInitialReadDelay = 0

	t.Cleanup(func() {
		primaryKeyPollInterval = origInterval
		primaryKeyMigrationTimeout = origTimeout
		primaryKeyInitialReadDelay = origDelay
	})
}

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

// TestAccPrimaryKeyCreateAlwaysSetsOnDevice verifies on a real device that
// Create with force_update=false still calls SetPrimaryKey and populates
// hash and status. Create must always apply the configured passphrase/salt
// because it only runs on new or recreated resources.
func TestAccPrimaryKeyCreateAlwaysSetsOnDevice(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceNoForceConfig, // force_update=false
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					// Create always calls SetPrimaryKey — hash and status must be populated
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
	withFastPolling(t)
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
	withFastPolling(t)
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

// TestUnitPrimaryKeyOldTagsProduceEmpty documents the compounded effect of the
// broken pre-fix response format (namespace-prefixed nested keys) combined with
// the async migration wait. With the old format, json.Unmarshal silently ignores
// "f5-primary-key:state" / "f5-primary-key:hash" / "f5-primary-key:status",
// leaving status as "" forever. waitForPrimaryKeyMigration never sees "COMPLETE"
// and times out — Create fails rather than silently writing empty state.
func TestUnitPrimaryKeyOldTagsProduceEmpty(t *testing.T) {
	withFastPolling(t)
	setupPrimaryKeyMock(t, primaryKeyResponseOldFormat, nil)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig,
				// With the old response format status is never "COMPLETE", so
				// waitForPrimaryKeyMigration times out and Create returns an error.
				ExpectError: regexp.MustCompile(`primary key migration did not complete`),
			},
		},
	})
}

// TestUnitPrimaryKeyForceUpdateChange verifies that Update calls SetPrimaryKey
// when force_update changes from false to true. This is the explicit re-key
// signal: the user is asking to rotate the primary key.
// Steps: Create (force_update=false, SetPrimaryKey called once) →
//
//	Update (force_update=true, SetPrimaryKey called a second time).
func TestUnitPrimaryKeyForceUpdateChange(t *testing.T) {
	withFastPolling(t)
	var setCount int32
	setupPrimaryKeyMock(t, primaryKeyResponseCorrect, &setCount)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=false — SetPrimaryKey called once (Create always sets)
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
			// Step 2: Update force_update=true — Update must call SetPrimaryKey again
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "true"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
					func(s *terraform.State) error {
						// Create (step 1) + Update with force_update=true (step 2) = 2 calls
						if atomic.LoadInt32(&setCount) != 2 {
							return fmt.Errorf("expected SetPrimaryKey called twice (Create + force Update), got %d", setCount)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitPrimaryKeyCreateAlwaysSets verifies that Create always calls
// SetPrimaryKey regardless of force_update. Before the fix, Create with
// force_update=false would skip SetPrimaryKey when a key already existed,
// silently failing to apply new passphrase/salt on key rotation (after a
// RequiresReplace destroy+recreate cycle).
func TestUnitPrimaryKeyCreateAlwaysSets(t *testing.T) {
	withFastPolling(t)
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
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
					func(s *terraform.State) error {
						// Create must always call SetPrimaryKey, even when force_update=false.
						if atomic.LoadInt32(&setCount) != 1 {
							return fmt.Errorf("expected exactly 1 SetPrimaryKey call from Create, got %d", setCount)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitPrimaryKeyUpdateSkipsSetWhenNoForce verifies that Update skips
// SetPrimaryKey when force_update=false, and only refreshes state from the
// device. This is the correct home for the force_update guard.
func TestUnitPrimaryKeyUpdateSkipsSetWhenNoForce(t *testing.T) {
	withFastPolling(t)
	var setCount int32
	setupPrimaryKeyMock(t, primaryKeyResponseCorrect, &setCount)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=true — SetPrimaryKey called once
			{
				Config: testAccPrimaryKeyResourceConfig, // force_update=true
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "true"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
			// Step 2: Update force_update=false — Update must NOT call SetPrimaryKey
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
					func(s *terraform.State) error {
						// Create (step 1) called SetPrimaryKey once.
						// Update (step 2, force_update=false) must NOT add another call.
						if atomic.LoadInt32(&setCount) != 1 {
							return fmt.Errorf("expected SetPrimaryKey called exactly once (Create only), got %d", setCount)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitPrimaryKeyAsyncMigration verifies that Create waits for the async primary key
// migration to complete before returning. The device returns IN_PROGRESS immediately,
// but the migration takes several seconds to complete. The test simulates this by having
// the mock server return IN_PROGRESS for first few calls, then COMPLETE.
func TestUnitPrimaryKeyAsyncMigration(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	var callCount int32

	// GET primary-key state - return IN_PROGRESS on first 2 calls, then COMPLETE
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("unexpected HTTP method on primary-key endpoint: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		count := atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
		if count <= 2 {
			// First two GETs (polls during wait) return IN_PROGRESS
			_, _ = w.Write([]byte(`{
				"f5-primary-key:primary-key": {
					"state": {
						"hash":   "",
						"status": "IN_PROGRESS"
					}
				}
			}`))
		} else {
			// Subsequent GETs return COMPLETE with hash
			_, _ = w.Write([]byte(`{
				"f5-primary-key:primary-key": {
					"state": {
						"hash":   "migrated-hash-value",
						"status": "COMPLETE"
					}
				}
			}`))
		}
	})

	// POST SetPrimaryKey
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected HTTP method on set endpoint: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
					// After migration completes, state should have the final hash
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "migrated-hash-value"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
					// Verify polling happened: callCount should be > 2 (SetPrimaryKey returns immediately,
					// then polling calls GetPrimaryKey multiple times)
					func(s *terraform.State) error {
						if atomic.LoadInt32(&callCount) <= 2 {
							return fmt.Errorf("expected multiple polling calls, got only %d", callCount)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitPrimaryKeyMigrationTimeout verifies that if the migration never completes,
// Create fails with a timeout error. This test sets up the mock to always return
// IN_PROGRESS to simulate a stuck migration.
func TestUnitPrimaryKeyMigrationTimeout(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	// GET primary-key state - always return IN_PROGRESS (migration never completes)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"f5-primary-key:primary-key": {
				"state": {
					"hash":   "",
					"status": "IN_PROGRESS"
				}
			}
		}`))
	})

	// POST SetPrimaryKey
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPrimaryKeyResourceConfig,
				ExpectError: regexp.MustCompile(`primary key migration did not complete`),
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
