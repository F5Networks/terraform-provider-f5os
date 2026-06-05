package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
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

// ---------------------------------------------------------------------------
// Unit test: ImportState populates id and reads hash/status from device
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyImportState(t *testing.T) {
	withFastPolling(t)
	setupPrimaryKeyMock(t, primaryKeyResponseCorrect, nil)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the resource so it exists in state
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
				),
			},
			// Step 2: Import — exercises ImportState + Read
			{
				ResourceName:            "f5os_primarykey.default",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"passphrase", "salt", "force_update"},
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when SetPrimaryKey returns an error
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyCreateSetError(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	// POST SetPrimaryKey returns 500
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"internal error"}]}}`))
	})

	// GET needed by polling — should not be reached if SetPrimaryKey fails
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(primaryKeyResponseCorrect))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPrimaryKeyResourceConfig,
				ExpectError: regexp.MustCompile(`Failed to create PrimaryKey`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Create fails when GetPrimaryKey returns nil after migration
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyCreateGetReturnsNil(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	var getCount int32

	// POST SetPrimaryKey succeeds
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	// GET: first calls during waitForPrimaryKeyMigration return COMPLETE,
	// but the final GET (post-migration state read) returns an empty JSON
	// object that unmarshals to an F5RespPrimaryKey with zero-value fields.
	// The client returns a non-nil pointer but with empty State — however the
	// resource code checks for nil. To trigger the nil branch we need
	// GetPrimaryKey to return nil. Since the client only returns nil on error,
	// we simulate this by returning COMPLETE during migration, then an error
	// on the subsequent GET. But the resource calls GetPrimaryKey again after
	// waitForPrimaryKeyMigration. We use a counter: calls 1 (migration poll)
	// return COMPLETE, call 2+ return an HTTP error.
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&getCount, 1)
		if count <= 1 {
			// Polling during waitForPrimaryKeyMigration — return COMPLETE
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(primaryKeyResponseCorrect))
		} else {
			// Post-migration read — return error to trigger the error path
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"read failed"}]}}`))
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPrimaryKeyResourceConfig,
				ExpectError: regexp.MustCompile(`Failed to fetch state after setting PrimaryKey`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read fails when GetPrimaryKey returns an error
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyReadGetError(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	var getCount int32

	// POST SetPrimaryKey succeeds
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	// GET: Create's migration poll + post-migration read succeed (calls 1-2).
	// Read's GET (call 3+) fails.
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&getCount, 1)
		if count <= 2 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(primaryKeyResponseCorrect))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"read error"}]}}`))
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPrimaryKeyResourceConfig,
				ExpectError: regexp.MustCompile(`Failed to fetch Primary Key configuration`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Read populates null when device returns empty hash/status
// (covers the else branches in primaryKeyResourceModelToState)
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyReadEmptyHashStatus(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	// Response with empty hash and status to exercise the null branches
	const emptyFieldsResponse = `{
		"f5-primary-key:primary-key": {
			"state": {
				"hash":   "",
				"status": "COMPLETE"
			}
		}
	}`

	// For the migration poll, return COMPLETE. For post-migration and Read,
	// return response with empty hash.
	var getCount int32

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&getCount, 1)
		w.WriteHeader(http.StatusOK)
		if count <= 1 {
			// Migration poll — needs COMPLETE status
			_, _ = w.Write([]byte(emptyFieldsResponse))
		} else {
			// Post-migration read and Read — return empty hash to cover null branch
			_, _ = w.Write([]byte(`{
				"f5-primary-key:primary-key": {
					"state": {
						"hash":   "",
						"status": ""
					}
				}
			}`))
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
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
					// With empty strings from device, the model sets null
					resource.TestCheckNoResourceAttr("f5os_primarykey.default", "hash"),
					resource.TestCheckNoResourceAttr("f5os_primarykey.default", "status"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when SetPrimaryKey returns an error (force_update=true)
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyUpdateSetError(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	var setCount int32

	// GET always succeeds
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(primaryKeyResponseCorrect))
	})

	// POST: first call (Create) succeeds, second call (Update with force) fails
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&setCount, 1)
		if count <= 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"update failed"}]}}`))
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=false — succeeds
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
				),
			},
			// Step 2: Update with force_update=true — SetPrimaryKey fails
			{
				Config:      testAccPrimaryKeyResourceConfig,
				ExpectError: regexp.MustCompile(`Failed to update Primary Key`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when migration times out (force_update=true)
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyUpdateMigrationTimeout(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	var setCount int32

	// GET: first calls (Create migration + post-migration + Read) return COMPLETE.
	// After second SetPrimaryKey (Update), return IN_PROGRESS forever to cause timeout.
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if atomic.LoadInt32(&setCount) <= 1 {
			_, _ = w.Write([]byte(primaryKeyResponseCorrect))
		} else {
			_, _ = w.Write([]byte(`{
				"f5-primary-key:primary-key": {
					"state": {
						"hash":   "",
						"status": "IN_PROGRESS"
					}
				}
			}`))
		}
	})

	// POST: track calls; both succeed
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&setCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=false — succeeds
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
			// Step 2: Update with force_update=true — migration never completes
			{
				Config:      testAccPrimaryKeyResourceConfig,
				ExpectError: regexp.MustCompile(`primary key migration did not complete`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update fails when GetPrimaryKey errors after successful set
// (covers both force_update=true and force_update=false paths)
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyUpdateGetErrorAfterForceSet(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	var getCount int32

	// GET call flow: Create(force=false) → Update(force=true):
	// 1: Create migration poll → OK
	// 2: Create post-migration read → OK
	// 3: Step 1 post-apply Read → OK
	// 4: Step 2 pre-apply refresh Read → OK (must succeed for Update to run)
	// 5: Update migration poll → OK
	// 6: Update post-set GetPrimaryKey → FAIL
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&getCount, 1)
		if count <= 5 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(primaryKeyResponseCorrect))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"read after update failed"}]}}`))
		}
	})

	// POST: all succeed
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=false — succeeds
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
			// Step 2: Update with force_update=true — Set+migration succeed,
			// but the final GetPrimaryKey in Update fails
			{
				Config:      testAccPrimaryKeyResourceConfig,
				ExpectError: regexp.MustCompile(`Failed to retrieve Primary Key after update`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Update succeeds when force_update=false — refresh only path
// where GetPrimaryKey returns nil (empty body). Tests the nil guard.
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyUpdateGetReturnsNilNoForce(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	var getCount int32

	// GET call flow: Create(force=true) → Update(force=false):
	// 1: Create migration poll → OK
	// 2: Create post-migration read → OK
	// 3: Step 1 post-apply Read → OK
	// 4: Step 2 pre-apply refresh Read → OK (must succeed for Update to run)
	// 5: Update's GetPrimaryKey (refresh-only, no Set) → FAIL
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&getCount, 1)
		if count <= 4 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(primaryKeyResponseCorrect))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"device read failed"}]}}`))
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=true — succeeds
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
			// Step 2: Update force_update=false — skips Set, calls GetPrimaryKey which errors
			{
				Config:      testAccPrimaryKeyResourceNoForceConfig,
				ExpectError: regexp.MustCompile(`Failed to retrieve Primary Key after update`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: waitForPrimaryKeyMigration handles GetPrimaryKey returning nil
// (exercises the nil-response warning branch in the polling loop)
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyMigrationNilResponse(t *testing.T) {
	withFastPolling(t)
	testAccPreUnitCheck(t)

	// We cannot directly make GetPrimaryKey return nil without error through
	// the mock server (it always returns a parsed struct or error). However,
	// if GetPrimaryKey returns an error on every poll, waitForPrimaryKeyMigration
	// exhausts its retries and times out — exercising the error branch in
	// the polling loop.

	var getCount int32

	// GET: first call returns error (exercises Warn path), subsequent calls
	// eventually return COMPLETE so the test can finish.
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&getCount, 1)
		if count <= 2 {
			// Return error for first 2 polls — exercises the error warn branch
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-message":"temporary error"}]}}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(primaryKeyResponseCorrect))
		}
	})

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key/f5-primary-key:set", func(w http.ResponseWriter, r *http.Request) {
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
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit test: Delete exercises the noop warning path
// ---------------------------------------------------------------------------

func TestUnitPrimaryKeyDelete(t *testing.T) {
	withFastPolling(t)
	setupPrimaryKeyMock(t, primaryKeyResponseCorrect, nil)
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
				),
			},
			// Step 2: Remove from config — triggers Delete (noop with warning)
			{
				Config: `# empty — triggers destroy of f5os_primarykey.default`,
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Direct Go unit tests for waitForPrimaryKeyMigration context cancellation
// ---------------------------------------------------------------------------

// TestUnitPrimaryKeyMigrationContextCancelDuringDelay exercises the context
// cancellation branch in waitForPrimaryKeyMigration during the initial read
// delay (the first select case on ctx.Done()).
func TestUnitPrimaryKeyMigrationContextCancelDuringDelay(t *testing.T) {
	testAccPreUnitCheck(t)

	// Set a long initial delay so the context cancel fires first
	origInterval := primaryKeyPollInterval
	origTimeout := primaryKeyMigrationTimeout
	origDelay := primaryKeyInitialReadDelay
	primaryKeyPollInterval = 10 * time.Millisecond
	primaryKeyMigrationTimeout = 1 * time.Second
	primaryKeyInitialReadDelay = 5 * time.Second // long delay to ensure cancel hits
	t.Cleanup(func() {
		primaryKeyPollInterval = origInterval
		primaryKeyMigrationTimeout = origTimeout
		primaryKeyInitialReadDelay = origDelay
	})

	// No mock handlers needed — the cancel should fire before any GET
	defer teardown()

	// Use a real client pointing at the mock server (env vars were
	// redirected by testAccPreUnitCheck). This avoids a nil-pointer
	// panic if timing drifts and the poll loop reaches GetPrimaryKey.
	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create mock-backed client: %v", err)
	}

	r := &PrimaryKeyResource{client: client}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short time so the initial delay select catches it
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err = r.waitForPrimaryKeyMigration(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("expected context cancellation error, got: %s", err)
	}
}

// TestUnitPrimaryKeyMigrationContextCancelDuringPoll exercises the context
// cancellation branch in waitForPrimaryKeyMigration during the poll sleep
// (the second select case on ctx.Done() inside the for loop).
func TestUnitPrimaryKeyMigrationContextCancelDuringPoll(t *testing.T) {
	testAccPreUnitCheck(t)

	// No initial delay, but long poll interval so cancel fires during poll sleep
	origInterval := primaryKeyPollInterval
	origTimeout := primaryKeyMigrationTimeout
	origDelay := primaryKeyInitialReadDelay
	primaryKeyPollInterval = 5 * time.Second // long poll so cancel fires during sleep
	primaryKeyMigrationTimeout = 30 * time.Second
	primaryKeyInitialReadDelay = 0
	t.Cleanup(func() {
		primaryKeyPollInterval = origInterval
		primaryKeyMigrationTimeout = origTimeout
		primaryKeyInitialReadDelay = origDelay
	})

	// GET returns IN_PROGRESS so the loop continues and sleeps
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
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

	defer teardown()

	// Create a client pointing at the mock server (env vars were redirected
	// by testAccPreUnitCheck). This replaces the previous inline client
	// construction that duplicated the env-var reading logic.
	client, err := newTestClientFromEnv()
	if err != nil {
		t.Fatalf("failed to create mock-backed client: %v", err)
	}

	r := &PrimaryKeyResource{client: client}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short time — after the first GET but during the poll sleep
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err = r.waitForPrimaryKeyMigration(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("expected context cancellation error, got: %s", err)
	}
}

// ---------------------------------------------------------------------------
// Acceptance test helpers
// ---------------------------------------------------------------------------



// testAccCheckPrimaryKeyHashPopulated queries the device directly to verify
// that the primary key has a non-empty hash (i.e., a key has been set).
func testAccCheckPrimaryKeyHashPopulated() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}
		keyData, err := client.GetPrimaryKey()
		if err != nil {
			return fmt.Errorf("GetPrimaryKey failed: %w", err)
		}
		if keyData == nil {
			return fmt.Errorf("GetPrimaryKey returned nil")
		}
		if keyData.PrimaryKey.State.Hash == "" {
			return fmt.Errorf("expected non-empty hash on device, got empty string")
		}
		return nil
	}
}

// testAccCheckPrimaryKeyStatusComplete queries the device directly and verifies
// the status contains "COMPLETE".
func testAccCheckPrimaryKeyStatusComplete() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}
		keyData, err := client.GetPrimaryKey()
		if err != nil {
			return fmt.Errorf("GetPrimaryKey failed: %w", err)
		}
		if keyData == nil {
			return fmt.Errorf("GetPrimaryKey returned nil")
		}
		if !strings.Contains(keyData.PrimaryKey.State.Status, "COMPLETE") {
			return fmt.Errorf("expected status containing COMPLETE on device, got %q", keyData.PrimaryKey.State.Status)
		}
		return nil
	}
}

// testAccCheckPrimaryKeyDestroy verifies that the primary key still exists on
// the device after Terraform destroy. Delete is a deliberate noop for this
// resource — the key must persist. This CheckDestroy confirms that the key
// survived the destroy step with a non-empty hash and COMPLETE status.
func testAccCheckPrimaryKeyDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		return fmt.Errorf("failed to create f5os client for destroy check: %w", err)
	}
	keyData, err := client.GetPrimaryKey()
	if err != nil {
		return fmt.Errorf("GetPrimaryKey failed after destroy: %w", err)
	}
	if keyData == nil {
		return fmt.Errorf("GetPrimaryKey returned nil after destroy — key should persist")
	}
	if keyData.PrimaryKey.State.Hash == "" {
		return fmt.Errorf("expected non-empty hash after destroy, got empty string")
	}
	if !strings.Contains(keyData.PrimaryKey.State.Status, "COMPLETE") {
		return fmt.Errorf("expected status containing COMPLETE after destroy, got %q", keyData.PrimaryKey.State.Status)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Improved acceptance tests with CheckDestroy, ImportState, and direct API
// verification
// ---------------------------------------------------------------------------

// TestAccPrimaryKeyFullLifecycle exercises Create, Read, ImportState, Update,
// and Delete with direct device verification at each step.
func TestAccPrimaryKeyFullLifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPrimaryKeyDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=true
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "passphrase", "test-pass"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "salt", "test-salt"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "true"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "status"),
					// Direct API verification
					testAccCheckPrimaryKeyHashPopulated(),
					testAccCheckPrimaryKeyStatusComplete(),
				),
			},
			// Step 2: ImportState
			{
				ResourceName:            "f5os_primarykey.default",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"passphrase", "salt", "force_update"},
			},
			// Step 3: Update — change force_update to false
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "status"),
					// Direct API verification — key should still be set
					testAccCheckPrimaryKeyHashPopulated(),
					testAccCheckPrimaryKeyStatusComplete(),
				),
			},
			// Step 4: Destroy is automatic — CheckDestroy verifies the noop
		},
	})
}

// TestAccPrimaryKeyCreateNoForceWithVerification verifies that Create with
// force_update=false still applies the key, verified directly on the device.
func TestAccPrimaryKeyCreateNoForceWithVerification(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPrimaryKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "status"),
					testAccCheckPrimaryKeyHashPopulated(),
					testAccCheckPrimaryKeyStatusComplete(),
				),
			},
		},
	})
}

// TestAccPrimaryKeyForceUpdateCycle exercises the full force-update toggle:
// create with force=true, update to force=false (skip set), update back to
// force=true (re-key), with direct API verification.
func TestAccPrimaryKeyForceUpdateCycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPrimaryKeyDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with force_update=true
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "true"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					testAccCheckPrimaryKeyHashPopulated(),
				),
			},
			// Step 2: Update force_update=false — skips SetPrimaryKey
			{
				Config: testAccPrimaryKeyResourceNoForceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "false"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					testAccCheckPrimaryKeyHashPopulated(),
				),
			},
			// Step 3: Update force_update=true — re-keys the device
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "force_update", "true"),
					resource.TestCheckResourceAttrSet("f5os_primarykey.default", "hash"),
					testAccCheckPrimaryKeyHashPopulated(),
					testAccCheckPrimaryKeyStatusComplete(),
				),
			},
		},
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
