package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// ---------------------------------------------------------------------------
// Acceptance tests (real device)
// ---------------------------------------------------------------------------

// TestAccPartitionChangePasswordRejectsRSeries verifies that the resource
// correctly rejects rSeries platforms with a clear error message. The
// f5os_partition_change_password resource is VELOS-partition-only.
func TestAccPartitionChangePasswordRejectsRSeries(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckPlatformRSeries(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPartitionChangePasswordConfig("testuser", "old_pass_123", "new_pass_456"),
				ExpectError: regexp.MustCompile(`(?s)supported with Velos Partition`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests (mock server)
// ---------------------------------------------------------------------------

// setupPartitionChangePasswordMock registers the common provider-level mock
// endpoints (aaa + Velos Partition platform detection) that every
// partition_change_password unit test targeting a Velos Partition needs.
func setupPartitionChangePasswordMock() {
	// Platform detection — returns a single component so the SDK
	// classifies this as "Velos Partition" (len == 1 branch).
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"openconfig-platform:component": [
				{
					"name": "blade-1",
					"state": {
						"name": "blade-1"
					}
				}
			]
		}`))
	})

	// aaa endpoint (provider Configure reads this)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
}

// TestUnitPartitionChangePasswordVelosPartitionSuccess simulates a VELOS
// partition environment and verifies the full Create code path succeeds.
func TestUnitPartitionChangePasswordVelosPartitionSuccess(t *testing.T) {
	testAccPreUnitCheck(t)
	setupPartitionChangePasswordMock()

	// Mock: change-password endpoint
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if _, ok := body["f5-system-aaa:old-password"]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "missing old-password"}`))
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
			// Step 1: Create — exercises the Create code path.
			{
				Config: testAccPartitionChangePasswordConfig("testuser", "OldP@ssw0rd!", "NewP@ssw0rd!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition_change_password.test", "user_name", "testuser"),
					resource.TestCheckResourceAttr("f5os_partition_change_password.test", "id", "testuser"),
				),
			},
			// Step 2: Update — changing passwords triggers the Update
			// code path (user_name has RequiresReplace, but passwords
			// do not, so Terraform calls Update, not destroy+create).
			{
				Config: testAccPartitionChangePasswordConfig("testuser", "NewP@ssw0rd!", "EvenN3wer!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition_change_password.test", "user_name", "testuser"),
					resource.TestCheckResourceAttr("f5os_partition_change_password.test", "id", "testuser"),
				),
			},
		},
	})
}

// TestUnitPartitionChangePasswordPlatformRejection verifies the resource
// rejects non-VELOS platforms using a mock that returns an rSeries response.
func TestUnitPartitionChangePasswordPlatformRejection(t *testing.T) {
	testAccPreUnitCheck(t)

	// Reuse shared helper to register rSeries platform + version mock handlers.
	setupMockPlatformVersion(mux, "1.5.4-37447")

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPartitionChangePasswordConfig("testuser", "OldP@ssw0rd!", "NewP@ssw0rd!"),
				ExpectError: regexp.MustCompile(`(?s)supported with Velos Partition`),
			},
		},
	})
}

// TestUnitPartitionChangePasswordAPIError verifies that API errors from the
// change-password endpoint are surfaced to the user.
func TestUnitPartitionChangePasswordAPIError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupPartitionChangePasswordMock()

	// Mock: change-password returns an error
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"ietf-restconf:errors": {
				"error": [{
					"error-type": "application",
					"error-tag": "invalid-value",
					"error-message": "Old password is incorrect"
				}]
			}
		}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccPartitionChangePasswordConfig("testuser", "WrongP@ss!", "NewP@ssw0rd!"),
				ExpectError: regexp.MustCompile(`Partition Password change failed`),
			},
		},
	})
}

// TestUnitPartitionChangePasswordUpdateAPIError verifies that when the
// PartitionPasswordChange API call fails during Update (password change),
// the error is surfaced via diagnostics. Step 1 creates successfully,
// Step 2 changes passwords triggering Update which hits a 400 error.
func TestUnitPartitionChangePasswordUpdateAPIError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupPartitionChangePasswordMock()

	// Mock: change-password succeeds on first call (Create), fails on second (Update)
	var callCount int
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 1 {
			// Create succeeds
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		// Update fails
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"ietf-restconf:errors": {
				"error": [{
					"error-type": "application",
					"error-tag": "invalid-value",
					"error-message": "Old password is incorrect"
				}]
			}
		}`))
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds
			{
				Config: testAccPartitionChangePasswordConfig("testuser", "OldP@ssw0rd!", "NewP@ssw0rd!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition_change_password.test", "id", "testuser"),
				),
			},
			// Step 2: Change passwords triggers Update, which fails
			{
				Config:      testAccPartitionChangePasswordConfig("testuser", "NewP@ssw0rd!", "EvenN3wer!"),
				ExpectError: regexp.MustCompile(`Partition Password change failed`),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Config helper
// ---------------------------------------------------------------------------

func testAccPartitionChangePasswordConfig(userName, oldPassword, newPassword string) string {
	return fmt.Sprintf(`
resource "f5os_partition_change_password" "test" {
  user_name    = %[1]q
  old_password = %[2]q
  new_password = %[3]q
}
`, userName, oldPassword, newPassword)
}
