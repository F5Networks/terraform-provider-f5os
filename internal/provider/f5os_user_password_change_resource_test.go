package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserPasswordChangeResource(t *testing.T) {
	// Skip this test unless specifically requested as it requires knowing actual passwords
	if testing.Short() {
		t.Skip("Skipping full password change test in short mode - requires actual device passwords")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Test with wrong password to verify error handling
			{
				Config:      testAccUserPasswordChangeResourceConfig("admin", "wrong_password_123", "new_password_456"),
				ExpectError: regexp.MustCompile("(?i)(incorrect|invalid|authentication)"), // Case-insensitive pattern
			},
		},
	})
}

func TestAccUserPasswordChangeResource_StandardUser(t *testing.T) {
	const (
		username    = "pwchgtest2025"
		oldPassword = "OldPass!2024Abcd"
		newPassword = "Xyz789!ZWqrtMn#P"
	)
	// Create a user on the device with a known password so the provider's
	// change-password call has a real target. The user is removed via t.Cleanup.
	testAccCreateEphemeralUser(t, username, oldPassword)

	// Authenticate as the ephemeral user so that the change-password endpoint
	// succeeds — it requires the session user to be the same as the target user.
	// host and port are omitted from the provider block so they fall back to the
	// F5OS_HOST / F5OS_PORT environment variables.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserPasswordChangeResourceConfigAsUser(username, oldPassword, newPassword),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "user_name", username),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "old_password", oldPassword),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "new_password", newPassword),
					resource.TestCheckResourceAttrSet("f5os_user_password_change.test", "id"),
				),
			},
		},
	})
}

// testAccCreateEphemeralUser creates a temporary user on the device with the
// given password (via the admin set-password endpoint), then registers
// t.Cleanup to remove it. Calls t.Skipf if the device cannot be reached or
// either API call fails, so the test is skipped rather than failing due to
// setup errors.
func testAccCreateEphemeralUser(t *testing.T, username, password string) {
	t.Helper()
	client, err := newTestClientFromEnv()
	if err != nil {
		t.Skipf("Cannot connect to device: %v", err)
		return
	}

	// Remove any stale instance (roles first, then the user itself).
	for _, role := range []string{"operator", "resource-admin", "admin"} {
		_ = client.DeleteRequest(fmt.Sprintf(
			"/openconfig-system:system/aaa/authentication/f5-system-aaa:roles/f5-system-aaa:role=%s/f5-system-aaa:config/f5-system-aaa:users=%s",
			role, username,
		))
	}
	_ = client.DeleteRequest(fmt.Sprintf(
		"/openconfig-system:system/aaa/authentication/f5-system-aaa:users/user=%s", username,
	))

	// Create the user (F5OS rejects password in the create payload).
	createBody, _ := json.Marshal(map[string]interface{}{
		"f5-system-aaa:user": map[string]interface{}{
			"username": username,
			"config":   map[string]interface{}{"username": username, "role": "resource-admin"},
		},
	})
	if _, err := client.PostRequest(
		"/openconfig-system:system/aaa/authentication/f5-system-aaa:users",
		createBody,
	); err != nil {
		t.Skipf("Cannot create ephemeral user %q on device: %v", username, err)
		return
	}

	// Set the known initial password via the admin set-password endpoint.
	setPassBody, _ := json.Marshal(map[string]string{"f5-system-aaa:password": password})
	setPassURI := fmt.Sprintf(
		"/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=%s/f5-system-aaa:config/f5-system-aaa:set-password",
		username,
	)
	if _, err := client.PostRequest(setPassURI, setPassBody); err != nil {
		_ = client.DeleteRequest(fmt.Sprintf(
			"/openconfig-system:system/aaa/authentication/f5-system-aaa:users/user=%s", username,
		))
		t.Skipf("Cannot set password for ephemeral user %q: %v", username, err)
		return
	}

	t.Cleanup(func() {
		for _, role := range []string{"operator", "resource-admin", "admin"} {
			// Best-effort: user may not be in every role.
			_ = client.DeleteRequest(fmt.Sprintf(
				"/openconfig-system:system/aaa/authentication/f5-system-aaa:roles/f5-system-aaa:role=%s/f5-system-aaa:config/f5-system-aaa:users=%s",
				role, username,
			))
		}
		if delErr := client.DeleteRequest(fmt.Sprintf(
			"/openconfig-system:system/aaa/authentication/f5-system-aaa:users/user=%s", username,
		)); delErr != nil {
			t.Errorf("testAccCreateEphemeralUser cleanup: failed to delete user %s: %v", username, delErr)
		}
	})
}

func TestAccUserPasswordChangeResource_Root(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Test root password change - may fail with access denied depending on user permissions
			{
				Config:      testAccUserPasswordChangeResourceConfig("root", "root_old_password", "root_new_password_123"),
				ExpectError: regexp.MustCompile("(?i)(access denied|incorrect|permission|unauthorized)"), // Case-insensitive
			},
		},
	})
}

func TestAccUserPasswordChangeResource_SamePassword(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Test validation error when old and new passwords are the same
			{
				Config:      testAccUserPasswordChangeResourceConfig("admin", "same_password", "same_password"),
				ExpectError: regexp.MustCompile("Old and new password cannot be the same"),
			},
		},
	})
}

// testAccUserPasswordChangeResourceConfigAsUser generates an HCL config that
// overrides the provider's username/password to authenticate as the given user.
// host and port are omitted so they fall back to F5OS_HOST / F5OS_PORT env vars.
// This lets the change-password endpoint succeed when the session user is the
// same as the target user.
func testAccUserPasswordChangeResourceConfigAsUser(username, oldPassword, newPassword string) string {
	return fmt.Sprintf(`
provider "f5os" {
  username = %[1]q
  password = %[2]q
}

resource "f5os_user_password_change" "test" {
  user_name    = %[1]q
  old_password = %[2]q
  new_password = %[3]q
}
`, username, oldPassword, newPassword)
}

func testAccUserPasswordChangeResourceConfig(userName, oldPassword, newPassword string) string {
	return fmt.Sprintf(`
resource "f5os_user_password_change" "test" {
  user_name    = %[1]q
  old_password = %[2]q
  new_password = %[3]q
}
`, userName, oldPassword, newPassword)
}

// ---------------------------------------------------------------------------
// Unit Tests
// ---------------------------------------------------------------------------

// TestUnitUserPasswordChangeCreate tests the Create operation with mock server
func TestUnitUserPasswordChangeCreate(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock the password change endpoint
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserPasswordChangeResourceConfig("testuser", "OldPass123!", "NewPass456!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "user_name", "testuser"),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "old_password", "OldPass123!"),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "new_password", "NewPass456!"),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "id", "testuser"),
				),
			},
		},
	})
}

// TestUnitUserPasswordChangeRead tests that Read preserves state
func TestUnitUserPasswordChangeRead(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock the password change endpoint (needed for Create)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserPasswordChangeResourceConfig("testuser", "OldPass123!", "NewPass456!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "id", "testuser"),
				),
			},
			// Refresh should preserve state (Read operation)
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "user_name", "testuser"),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "id", "testuser"),
				),
			},
		},
	})
}

// TestUnitUserPasswordChangeUpdate tests the Update operation
func TestUnitUserPasswordChangeUpdate(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock the password change endpoint
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserPasswordChangeResourceConfig("testuser", "OldPass123!", "NewPass456!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "new_password", "NewPass456!"),
				),
			},
			// Update to a different new password
			{
				Config: testAccUserPasswordChangeResourceConfig("testuser", "NewPass456!", "AnotherPass789!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "old_password", "NewPass456!"),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "new_password", "AnotherPass789!"),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "id", "testuser"),
				),
			},
		},
	})
}

// TestUnitUserPasswordChangeUpdateSamePassword tests validation in Update
func TestUnitUserPasswordChangeUpdateSamePassword(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock the password change endpoint for initial create
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserPasswordChangeResourceConfig("testuser", "OldPass123!", "NewPass456!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "id", "testuser"),
				),
			},
			// Try to update with same old and new password - should fail
			{
				Config:      testAccUserPasswordChangeResourceConfig("testuser", "SamePass!", "SamePass!"),
				ExpectError: regexp.MustCompile("Old and new password cannot be the same"),
			},
		},
	})
}

// TestUnitUserPasswordChangeDelete tests that Delete removes resource from state
func TestUnitUserPasswordChangeDelete(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock the password change endpoint
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserPasswordChangeResourceConfig("testuser", "OldPass123!", "NewPass456!"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "id", "testuser"),
				),
			},
			// Step 2: Remove config to trigger Delete
			{
				Config: `# Empty config to trigger resource deletion`,
			},
		},
	})
}

// TestUnitUserPasswordChangePolicyViolation tests password policy error handling
func TestUnitUserPasswordChangePolicyViolation(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock the password change endpoint to return policy violation error
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"password does not meet minimum length requirements"}]}}`))
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPasswordChangeResourceConfig("testuser", "OldPass123!", "weak"),
				ExpectError: regexp.MustCompile("(?i)(password does not meet|policy requirements)"),
			},
		},
	})
}

// TestUnitUserPasswordChangeIncorrectOldPassword tests incorrect old password error
func TestUnitUserPasswordChangeIncorrectOldPassword(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock the password change endpoint to return incorrect password error
	// Using 400 instead of 401 to avoid triggering SDK retry logic
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"Old password is incorrect"}]}}`))
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPasswordChangeResourceConfig("testuser", "WrongOldPass!", "NewPass456!"),
				ExpectError: regexp.MustCompile("(?i)(old password is incorrect|incorrect)"),
			},
		},
	})
}

// TestUnitUserPasswordChangeGenericAPIError tests generic API error handling
func TestUnitUserPasswordChangeGenericAPIError(t *testing.T) {
	testAccPreUnitCheck(t)

	// Mock the password change endpoint to return generic error
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=testuser/f5-system-aaa:config/f5-system-aaa:change-password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"Internal server error"}]}}`))
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserPasswordChangeResourceConfig("testuser", "OldPass123!", "NewPass456!"),
				ExpectError: regexp.MustCompile("(?i)(API request failed|User password change failed)"),
			},
		},
	})
}
