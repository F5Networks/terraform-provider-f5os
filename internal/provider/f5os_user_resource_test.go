package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccUserResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccUserResourceConfig("testuser", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "testuser"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					resource.TestCheckResourceAttr("f5os_user.test", "id", "testuser"),
				),
			},
			// ImportState testing - passwords cannot be imported
			{
				ResourceName:            "f5os_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
			// Update and Read testing
			{
				Config: testAccUserResourceConfig("testuser", "resource-admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "testuser"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "resource-admin"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccUserResourceConfig(username, role string) string {
	return fmt.Sprintf(`
resource "f5os_user" "test" {
  username = %[1]q
  password = "MyStr0ng!P@ss123"
  role     = %[2]q
}
`, username, role)
}

// Test idempotency - ensure applying same config twice doesn't show changes
func TestAccUserResourceIdempotency(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Create user
			{
				Config: testAccUserResourceConfig("idempotency_test", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "idempotency_test"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
				),
			},
			// Apply same config again - should show no changes
			{
				Config: testAccUserResourceConfig("idempotency_test", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "idempotency_test"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
				),
				PlanOnly: true, // This will fail if there are any planned changes
			},
		},
	})
}

func TestAccUserResourceWithExpiryStatus(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing with expiry status
			{
				Config: testAccUserResourceConfigWithExpiry("testuser2", "operator", "enabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "testuser2"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					resource.TestCheckResourceAttr("f5os_user.test", "expiry_status", "enabled"),
				),
			},
		},
	})
}

func testAccUserResourceConfigWithExpiry(username, role, expiryStatus string) string {
	return fmt.Sprintf(`
resource "f5os_user" "test" {
  username      = %[1]q
  password      = "MyGood!P@ssword99"
  role          = %[2]q
  expiry_status = %[3]q
}
`, username, role, expiryStatus)
}

func TestAccUserResourceWithSecondaryRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing with secondary role
			{
				Config: testAccUserResourceConfigWithSecondaryRole("testuser3", "admin", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "testuser3"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "admin"),
					resource.TestCheckResourceAttr("f5os_user.test", "secondary_role", "operator"),
				),
			},
		},
	})
}

func testAccUserResourceConfigWithSecondaryRole(username, role, secondaryRole string) string {
	return fmt.Sprintf(`
resource "f5os_user" "test" {
  username       = %[1]q
  password       = "MyBig!P@ssword88"
  role           = %[2]q
  secondary_role = %[3]q
}
`, username, role, secondaryRole)
}

// Test validation error for invalid expiry status
func TestAccUserResourceInvalidExpiryStatus(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_user" "test" {
  username      = "testuser5"
  password      = "MyValidP@ss333"
  role          = "operator"
  expiry_status = "invalid_status"
}
`,
				ExpectError: regexp.MustCompile("The value 'invalid_status' is not valid"),
			},
		},
	})
}

// Test validation error for invalid date format
func TestAccUserResourceInvalidDateFormat(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_user" "test" {
  username      = "testuser6"
  password      = "MySecretP@ss444"
  role          = "operator"
  expiry_status = "2024-13-32"
}
`,
				ExpectError: regexp.MustCompile("The date '2024-13-32' is not a valid calendar date"),
			},
		},
	})
}

// Test with valid date in expiry status
func TestAccUserResourceWithValidDate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfigWithExpiry("testuser7", "operator", "2027-12-31"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "testuser7"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					resource.TestCheckResourceAttr("f5os_user.test", "expiry_status", "2027-12-31"),
				),
			},
		},
	})
}

// Test with locked status
func TestAccUserResourceWithLockedStatus(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfigWithExpiry("testuser8", "operator", "locked"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "testuser8"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					resource.TestCheckResourceAttr("f5os_user.test", "expiry_status", "locked"),
				),
			},
		},
	})
}

// Test password validation - basic validation only (F5OS handles policy)
func TestAccUserResourceBasicPasswordValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_user" "test" {
  username = "testuser9"
  password = ""
  role     = "operator"
}
`,
				ExpectError: regexp.MustCompile("Password Cannot Be Empty"),
			},
		},
	})
}

// Test F5OS password policy rejection (this would fail in real F5OS)
func TestAccUserResourceF5OSPasswordPolicy(t *testing.T) {
	// Note: This test demonstrates how F5OS policy errors would be handled
	// In practice, this might pass if the F5OS device accepts "badpwd"
	// The actual test behavior depends on the real F5OS device configuration
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "f5os_user" "test" {
  username = "testuser10"
  password = "badpwd"
  role     = "operator"
}
`,
				// This test expects F5OS to reject the password due to policy requirements.
				// F5OS surfaces the rejection as "User Create Error" at the create level,
				// but include "Error Setting User Password" as a fallback for older API versions.
				ExpectError: regexp.MustCompile("User Create Error|Error Setting User Password"),
			},
		},
	})
}

// Test basic user creation without import (simpler test for debugging)
func TestAccUserResourceBasicOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing only
			{
				Config: testAccUserResourceConfig("uniqueuser2025", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "uniqueuser2025"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					resource.TestCheckResourceAttr("f5os_user.test", "id", "uniqueuser2025"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

// Test user creation with secondary role (no import)
func TestAccUserResourceSecondaryRoleOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing with secondary role
			{
				Config: testAccUserResourceConfigWithSecondaryRole("secuser2025", "admin", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "secuser2025"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "admin"),
					resource.TestCheckResourceAttr("f5os_user.test", "secondary_role", "operator"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test helpers for direct device verification
// ---------------------------------------------------------------------------



// testAccCheckUserRolesOnDevice queries the device's roles endpoint directly
// and verifies that the given user has exactly the expected roles — no more,
// no less. This bypasses the resource's Read method.
func testAccCheckUserRolesOnDevice(username string, expectedRoles []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		respData, err := client.GetRequest("/openconfig-system:system/aaa/authentication/f5-system-aaa:roles")
		if err != nil {
			return fmt.Errorf("failed to get roles from device: %w", err)
		}

		var rolesResponse struct {
			Roles struct {
				Role []struct {
					RoleName string `json:"rolename"`
					Config   struct {
						Users []string `json:"users,omitempty"`
					} `json:"config"`
				} `json:"role"`
			} `json:"f5-system-aaa:roles"`
		}
		if err := json.Unmarshal(respData, &rolesResponse); err != nil {
			return fmt.Errorf("failed to parse roles response: %w", err)
		}

		// Find all roles the user belongs to
		actualRoles := make(map[string]bool)
		for _, role := range rolesResponse.Roles.Role {
			for _, u := range role.Config.Users {
				if u == username {
					actualRoles[role.RoleName] = true
				}
			}
		}

		// Build expected set
		expectedSet := make(map[string]bool)
		for _, r := range expectedRoles {
			expectedSet[r] = true
		}

		// Check for missing expected roles
		for r := range expectedSet {
			if !actualRoles[r] {
				return fmt.Errorf("expected user %q to be in role %q on device, but they are not; actual roles: %v", username, r, actualRoles)
			}
		}
		// Check for unexpected extra roles
		for r := range actualRoles {
			if !expectedSet[r] {
				return fmt.Errorf("user %q has unexpected stale role %q on device; expected only: %v", username, r, expectedRoles)
			}
		}
		return nil
	}
}

// testAccCheckUserDestroy verifies all f5os_user resources in the state have
// been deleted from the device.
func testAccCheckUserDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		return nil // cannot connect — treat as destroyed
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_user" {
			continue
		}
		username := rs.Primary.Attributes["username"]
		uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:users/user=%s", username)
		respData, err := client.GetRequest(uri)
		if err != nil {
			continue // error means user is gone — that's good
		}
		// The SDK returns (body, nil) even for 404. Check the body for
		// "not found" / "invalid-value" to distinguish 404 from 200.
		body := string(respData)
		if strings.Contains(body, "not found") || strings.Contains(body, "invalid-value") || body == "" {
			continue // 404 response — user is gone
		}
		return fmt.Errorf("user %q still exists on device after destroy", username)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Acceptance tests for stale role cleanup during Update
// ---------------------------------------------------------------------------

// TestAccUserRoleChangeRemovesStaleRole verifies on a real device that when a
// user's primary role changes (e.g. operator -> resource-admin), the old role
// assignment is removed. Without the fix, the user would accumulate in both
// roles on the device.
func TestAccUserRoleChangeRemovesStaleRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create user with role=operator
			{
				Config: testAccUserResourceConfig("acc_stale_role", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "acc_stale_role"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					testAccCheckUserRolesOnDevice("acc_stale_role", []string{"operator"}),
				),
			},
			// Step 2: Change role to resource-admin — operator must be removed on the device
			{
				Config: testAccUserResourceConfig("acc_stale_role", "resource-admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "resource-admin"),
					testAccCheckUserRolesOnDevice("acc_stale_role", []string{"resource-admin"}),
				),
			},
			// Delete is automatic via CheckDestroy
		},
	})
}

// TestAccUserRoleChangeTwice verifies on a real device that changing the
// primary role a second time also cleans up the intermediate role. This
// exercises the stale-role cleanup over multiple update cycles.
func TestAccUserRoleChangeTwice(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with role=operator
			{
				Config: testAccUserResourceConfig("acc_twice", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					testAccCheckUserRolesOnDevice("acc_twice", []string{"operator"}),
				),
			},
			// Step 2: Change to resource-admin — operator removed
			{
				Config: testAccUserResourceConfig("acc_twice", "resource-admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "resource-admin"),
					testAccCheckUserRolesOnDevice("acc_twice", []string{"resource-admin"}),
				),
			},
			// Step 3: Change back to operator — resource-admin removed
			{
				Config: testAccUserResourceConfig("acc_twice", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					testAccCheckUserRolesOnDevice("acc_twice", []string{"operator"}),
				),
			},
			// Delete is automatic via CheckDestroy
		},
	})
}

// TestAccUserResourceWithAuthorizedKeys verifies on a real device that a user
// can be created with an SSH authorized key and that the key is stored on the
// device. The verification uses a direct API query to confirm device state,
// bypassing the resource's Read method (which preserves state rather than
// re-reading authorized_keys from the API).
func TestAccUserResourceWithAuthorizedKeys(t *testing.T) {
	// A real ed25519 public key verified to be accepted by F5OS 1.8.3.
	const testSSHKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl terraform-f5os-test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfigWithAuthorizedKeys("sshkeyuser2025", "operator", testSSHKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "sshkeyuser2025"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					resource.TestCheckResourceAttr("f5os_user.test", "authorized_keys.0", testSSHKey),
					testAccCheckUserAuthorizedKeyOnDevice("sshkeyuser2025", testSSHKey),
				),
			},
		},
	})
}

// testAccCheckUserAuthorizedKeyOnDevice queries the device's user config
// endpoint directly and verifies that the given SSH key appears in the
// authorized-keys field. The F5OS API returns authorized-keys as a single
// string inside the config object, not as an array.
func testAccCheckUserAuthorizedKeyOnDevice(username, expectedKey string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("cannot create client: %w", err)
		}

		uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:users/user=%s", username)
		respData, err := client.GetRequest(uri)
		if err != nil {
			return fmt.Errorf("API request failed for user %q: %w", username, err)
		}

		// The API returns authorized-keys inside the config sub-object as a string.
		var response struct {
			Users []struct {
				Config struct {
					AuthorizedKeys string `json:"authorized-keys"`
				} `json:"config"`
			} `json:"f5-system-aaa:user"`
		}
		if err := json.Unmarshal(respData, &response); err != nil {
			return fmt.Errorf("failed to parse user response for %q: %w", username, err)
		}
		if len(response.Users) == 0 {
			return fmt.Errorf("user %q not found on device", username)
		}
		deviceKey := response.Users[0].Config.AuthorizedKeys
		if !strings.Contains(deviceKey, expectedKey) {
			return fmt.Errorf("authorized key not found on device for user %q; device has: %q", username, deviceKey)
		}
		return nil
	}
}

// testAccUserResourceConfigWithAuthorizedKeys returns an HCL config that
// creates a user with a single SSH authorized key.
func testAccUserResourceConfigWithAuthorizedKeys(username, role, sshKey string) string {
	return fmt.Sprintf(`
resource "f5os_user" "test" {
  username        = %q
  password        = "MyStr0ng!P@ss123"
  role            = %q
  authorized_keys = [%q]
}
`, username, role, sshKey)
}

// ---------------------------------------------------------------------------
// Shared mock-server state for f5os_user unit tests
// ---------------------------------------------------------------------------

// userMockState holds mutable mock-server state shared across handlers.
type userMockState struct {
	mu sync.Mutex
	// roleMembers tracks which users are assigned to which roles.
	// key = rolename, value = set of usernames
	roleMembers map[string]map[string]bool
	// users tracks created users. key = username
	users map[string]bool
	// userConfigRole stores the role from the user config payload (create/update).
	// This is the "primary role" that the GET user handler returns.
	userConfigRole map[string]string
	// roleRemovals records every (username, role) removal for assertions.
	roleRemovals []roleRemovalRecord
	// failGetRolesCount when > 0, the GET /f5-system-aaa:roles endpoint returns
	// HTTP 500 and decrements the counter. This lets tests fail a specific number
	// of getUserRoles calls while subsequent calls succeed (e.g., for Read).
	failGetRolesCount int
}

type roleRemovalRecord struct {
	Username string
	Role     string
}

// setupUserMock registers all the standard mock handlers needed for
// f5os_user unit tests and returns a pointer to the shared state.
// The caller is responsible for calling teardown().
//
// initialRoles seeds the role membership table so that getUserRoles
// returns the expected roles for a user.  Format: map[rolename][]username
func setupUserMock(t *testing.T, initialRoles map[string][]string) *userMockState {
	t.Helper()
	testAccPreUnitCheck(t)

	st := &userMockState{
		roleMembers:    make(map[string]map[string]bool),
		users:          make(map[string]bool),
		userConfigRole: make(map[string]string),
	}

	// Seed initial role memberships
	for role, users := range initialRoles {
		st.roleMembers[role] = make(map[string]bool)
		for _, u := range users {
			st.roleMembers[role][u] = true
		}
	}

	// --- Provider bootstrap handlers ---

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
		_, _ = fmt.Fprintf(w, "%s", "")
	})

	// --- Users endpoints: single handler dispatches on path + method ---
	// http.ServeMux uses longest prefix matching, so we register the
	// shortest common prefix and inspect the full path inside.

	usersBase := "/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:users"

	mux.HandleFunc(usersBase+"/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// set-password / change-password — POST to .../f5-system-aaa:user=<name>/...
		if r.Method == "POST" && (strings.Contains(path, "set-password") || strings.Contains(path, "change-password")) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "%s", "")
			return
		}

		// Per-user: .../user=<username>
		userPrefix := usersBase + "/user="
		if len(path) > len(userPrefix) && path[:len(userPrefix)] == userPrefix {
			username := path[len(userPrefix):]

			st.mu.Lock()
			defer st.mu.Unlock()

			switch r.Method {
			case "GET":
				if !st.users[username] {
					w.WriteHeader(http.StatusNotFound)
					_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"invalid-value","error-message":"uri keypath not found"}]}}`)
					return
				}
				// Use the stored config role for deterministic results
				primaryRole := st.userConfigRole[username]
				resp := fmt.Sprintf(`{"f5-system-aaa:user":[{"username":"%s","config":{"username":"%s","role":"%s"}}]}`, username, username, primaryRole)
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", resp)

			case "PATCH":
				if !st.users[username] {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				// Parse the payload to capture the updated role
				var patchPayload struct {
					User struct {
						Config struct {
							Role string `json:"role"`
						} `json:"config"`
					} `json:"f5-system-aaa:user"`
				}
				body, _ := io.ReadAll(r.Body)
				if json.Unmarshal(body, &patchPayload) == nil && patchPayload.User.Config.Role != "" {
					st.userConfigRole[username] = patchPayload.User.Config.Role
				}
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", "")

			case "DELETE":
				delete(st.users, username)
				for role := range st.roleMembers {
					delete(st.roleMembers[role], username)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", "")

			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	// Collection endpoint: POST to create user (exact path, no trailing sub-path)
	mux.HandleFunc(usersBase, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var payload struct {
				User struct {
					Username string `json:"username"`
					Config   struct {
						Username string `json:"username"`
						Role     string `json:"role"`
					} `json:"config"`
				} `json:"f5-system-aaa:user"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			st.mu.Lock()
			st.users[payload.User.Username] = true
			if payload.User.Config.Role != "" {
				st.userConfigRole[payload.User.Username] = payload.User.Config.Role
			}
			st.mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, "%s", "")
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	// --- Roles endpoints: single handler with subtree match ---
	// Register with trailing "/" so http.ServeMux matches all sub-paths.
	rolesBase := "/restconf/data/openconfig-system:system/aaa/authentication/f5-system-aaa:roles"

	mux.HandleFunc(rolesBase+"/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Per-role user assignment: PUT (assign) / DELETE (remove)
		// Path: .../f5-system-aaa:role=<role>/f5-system-aaa:config/f5-system-aaa:users=<username>
		rolePrefix := rolesBase + "/f5-system-aaa:role="
		if strings.HasPrefix(path, rolePrefix) {
			rest := path[len(rolePrefix):]
			slashIdx := strings.Index(rest, "/")
			if slashIdx < 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			roleName := rest[:slashIdx]
			usersTag := "f5-system-aaa:users="
			userIdx := strings.Index(rest, usersTag)
			if userIdx < 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			username := rest[userIdx+len(usersTag):]

			st.mu.Lock()
			defer st.mu.Unlock()

			switch r.Method {
			case "PUT":
				if st.roleMembers[roleName] == nil {
					st.roleMembers[roleName] = make(map[string]bool)
				}
				st.roleMembers[roleName][username] = true
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", "")

			case "DELETE":
				if members, ok := st.roleMembers[roleName]; ok {
					delete(members, username)
				}
				st.roleRemovals = append(st.roleRemovals, roleRemovalRecord{Username: username, Role: roleName})
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "%s", "")

			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	// GET all roles — exact path match (no trailing /)
	mux.HandleFunc(rolesBase, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		st.mu.Lock()
		defer st.mu.Unlock()

		if st.failGetRolesCount > 0 {
			st.failGetRolesCount--
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"simulated roles endpoint failure"}]}}`)
			return
		}

		type roleConfig struct {
			Users []string `json:"users,omitempty"`
		}
		type roleEntry struct {
			RoleName string     `json:"rolename"`
			Config   roleConfig `json:"config"`
		}
		type rolesWrapper struct {
			Roles struct {
				Role []roleEntry `json:"role"`
			} `json:"f5-system-aaa:roles"`
		}

		var resp rolesWrapper
		allRoles := map[string]bool{"admin": true, "operator": true, "resource-admin": true, "user": true}
		for role := range st.roleMembers {
			allRoles[role] = true
		}
		sortedRoles := make([]string, 0, len(allRoles))
		for role := range allRoles {
			sortedRoles = append(sortedRoles, role)
		}
		sort.Strings(sortedRoles)
		for _, role := range sortedRoles {
			entry := roleEntry{RoleName: role}
			if members, ok := st.roleMembers[role]; ok {
				sortedMembers := make([]string, 0, len(members))
				for u := range members {
					sortedMembers = append(sortedMembers, u)
				}
				sort.Strings(sortedMembers)
				entry.Config.Users = sortedMembers
			}
			resp.Roles.Role = append(resp.Roles.Role, entry)
		}

		body, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	return st
}

// ---------------------------------------------------------------------------
// Unit tests for stale role cleanup during Update
// ---------------------------------------------------------------------------

// TestUnitUserRoleChangeRemovesStaleRole verifies that when a user's primary
// role changes (e.g. operator -> admin), the old role assignment is removed
// before the new one is assigned. Without the fix, the user would accumulate
// in both roles.
func TestUnitUserRoleChangeRemovesStaleRole(t *testing.T) {
	st := setupUserMock(t, map[string][]string{
		"operator": {"unittest_rolechange"},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create user with role=operator
			{
				Config: testAccUserResourceConfig("unittest_rolechange", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "unittest_rolechange"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
				),
			},
			// Step 2: Change role to admin — operator should be removed
			{
				Config: testAccUserResourceConfig("unittest_rolechange", "admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "admin"),
					func(s *terraform.State) error {
						st.mu.Lock()
						defer st.mu.Unlock()

						// Verify operator was removed
						operatorRemoved := false
						for _, removal := range st.roleRemovals {
							if removal.Username == "unittest_rolechange" && removal.Role == "operator" {
								operatorRemoved = true
								break
							}
						}
						if !operatorRemoved {
							return fmt.Errorf("expected stale role 'operator' to be removed for user 'unittest_rolechange', but no removal was recorded; removals: %+v", st.roleRemovals)
						}

						// Verify user is now in admin role only
						if !st.roleMembers["admin"]["unittest_rolechange"] {
							return fmt.Errorf("expected user 'unittest_rolechange' to be in 'admin' role")
						}
						if st.roleMembers["operator"]["unittest_rolechange"] {
							return fmt.Errorf("expected user 'unittest_rolechange' to NOT be in 'operator' role after role change")
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitUserSecondaryRoleRemovalCleansUp verifies that when a user's
// secondary_role is removed from the config, the old secondary role
// assignment is cleaned up on the device.
func TestUnitUserSecondaryRoleRemovalCleansUp(t *testing.T) {
	st := setupUserMock(t, map[string][]string{
		"admin":    {"unittest_secrole"},
		"operator": {"unittest_secrole"},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with primary=admin, secondary=operator
			{
				Config: testAccUserResourceConfigWithSecondaryRole("unittest_secrole", "admin", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "admin"),
					resource.TestCheckResourceAttr("f5os_user.test", "secondary_role", "operator"),
				),
			},
			// Step 2: Remove secondary_role — operator should be removed
			{
				Config: testAccUserResourceConfig("unittest_secrole", "admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "admin"),
					func(s *terraform.State) error {
						st.mu.Lock()
						defer st.mu.Unlock()

						// Verify operator was removed
						operatorRemoved := false
						for _, removal := range st.roleRemovals {
							if removal.Username == "unittest_secrole" && removal.Role == "operator" {
								operatorRemoved = true
								break
							}
						}
						if !operatorRemoved {
							return fmt.Errorf("expected secondary role 'operator' to be removed for user 'unittest_secrole', but no removal was recorded; removals: %+v", st.roleRemovals)
						}

						// Verify user is in admin only
						if !st.roleMembers["admin"]["unittest_secrole"] {
							return fmt.Errorf("expected user to remain in 'admin' role")
						}
						if st.roleMembers["operator"]["unittest_secrole"] {
							return fmt.Errorf("expected user to NOT be in 'operator' role after secondary role removal")
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitUserSameRolesNoRemoval verifies that when a user's roles don't
// change during an update (e.g. only password changes), no role removals
// are triggered.
func TestUnitUserSameRolesNoRemoval(t *testing.T) {
	st := setupUserMock(t, map[string][]string{
		"operator": {"unittest_norole"},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with role=operator
			{
				Config: testAccUserResourceConfig("unittest_norole", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
				),
			},
			// Step 2: "Update" with same role — no removals should occur
			{
				Config: testUnitUserResourceConfigDifferentPassword("unittest_norole", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					func(s *terraform.State) error {
						st.mu.Lock()
						defer st.mu.Unlock()

						// Filter removals for this user
						for _, removal := range st.roleRemovals {
							if removal.Username == "unittest_norole" {
								return fmt.Errorf("expected no role removals for user 'unittest_norole' when roles unchanged, but got removal of role %q", removal.Role)
							}
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitUserGetRolesFailureDuringUpdate verifies that when getUserRoles()
// fails during Update (e.g. the roles endpoint returns HTTP 500), the Update
// still succeeds — the failure is logged as a warning, the new role is
// assigned, but stale roles cannot be removed because the current role set
// is unknown. This tests the graceful degradation path at
// f5os_user_resource.go:789-795.
func TestUnitUserGetRolesFailureDuringUpdate(t *testing.T) {
	st := setupUserMock(t, map[string][]string{
		"operator": {"unittest_getfail"},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create user with role=operator (roles endpoint works)
			{
				Config: testAccUserResourceConfig("unittest_getfail", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "unittest_getfail"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
				),
			},
			// Step 2: Change role to admin while roles endpoint fails.
			// getUserRoles returns an error, so stale role cleanup is skipped.
			// But the update itself must still succeed — the new role is
			// assigned and state is saved.
			{
				PreConfig: func() {
					st.mu.Lock()
					// The SDK retries each request 3 times with 10s delay.
					// Before the Update apply, the framework calls Read which
					// also invokes getUserRoles (consuming retries from the
					// counter). We set the counter high enough to fail ALL
					// getUserRoles calls during this step — both the Read
					// refresh and the Update's own call. The post-apply Read
					// may also fail, producing a non-empty plan, which we
					// tolerate with ExpectNonEmptyPlan.
					st.failGetRolesCount = 9
					st.mu.Unlock()
				},
				Config:             testAccUserResourceConfig("unittest_getfail", "admin"),
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Update must succeed — Terraform state reflects new role
					resource.TestCheckResourceAttr("f5os_user.test", "role", "admin"),
					func(s *terraform.State) error {
						st.mu.Lock()
						defer st.mu.Unlock()

						// No role removals should have occurred because
						// getUserRoles failed — we couldn't determine the
						// current roles to compare against.
						for _, removal := range st.roleRemovals {
							if removal.Username == "unittest_getfail" {
								return fmt.Errorf("expected no role removals when getUserRoles fails, but got removal of role %q", removal.Role)
							}
						}

						// The user should still be in the operator role
						// (stale, not cleaned up) AND in admin (newly assigned).
						if !st.roleMembers["admin"]["unittest_getfail"] {
							return fmt.Errorf("expected user to be assigned to 'admin' role")
						}
						if !st.roleMembers["operator"]["unittest_getfail"] {
							return fmt.Errorf("expected user to still be in stale 'operator' role (removal was skipped due to getUserRoles failure)")
						}
						return nil
					},
				),
			},
		},
	})
}

// testUnitUserResourceConfigDifferentPassword generates a config with a
// different password to force a Terraform update without changing roles.
func testUnitUserResourceConfigDifferentPassword(username, role string) string {
	return fmt.Sprintf(`
resource "f5os_user" "test" {
  username = %[1]q
  password = "DifferentP@ss456"
  role     = %[2]q
}
`, username, role)
}

// ---------------------------------------------------------------------------
// Additional acceptance tests for coverage improvements
// ---------------------------------------------------------------------------

// TestAccUserResourcePasswordUpdate verifies that changing a user's password
// via the f5os_user resource triggers the Update → setUserPassword path.
func TestAccUserResourcePasswordUpdate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroy,
		Steps: []resource.TestStep{
			// Create user with initial password
			{
				Config: testAccUserResourceConfigPassword("pwchangeuser", "operator", "Init!alP@ss123"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "pwchangeuser"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
				),
			},
			// Update password (triggers Update → setUserPassword path)
			{
				Config: testAccUserResourceConfigPassword("pwchangeuser", "operator", "Updat3d!P@ss456XyzAbc"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "pwchangeuser"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
				),
			},
		},
	})
}

func testAccUserResourceConfigPassword(username, role, password string) string {
	return fmt.Sprintf(`
resource "f5os_user" "test" {
  username = %[1]q
  password = %[3]q
  role     = %[2]q
}
`, username, role, password)
}

// ---------------------------------------------------------------------------
// Unit tests for validator error paths
// ---------------------------------------------------------------------------

// TestUnitUserInvalidPassword verifies the basicPasswordValidator rejects
// empty passwords, exercising the error branch.
func TestUnitUserInvalidPassword(t *testing.T) {
	setupUserMock(t, map[string][]string{})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserResourceConfigPassword("emptypw", "operator", ""),
				ExpectError: regexp.MustCompile("Password Cannot Be Empty"),
			},
		},
	})
}

// TestUnitUserInvalidExpiryFormat verifies the expiry validator rejects
// values that are neither predefined strings nor valid YYYY-MM-DD dates.
func TestUnitUserInvalidExpiryFormat(t *testing.T) {
	setupUserMock(t, map[string][]string{})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserResourceConfigWithExpiry("badexpiry", "operator", "12-31-2026"),
				ExpectError: regexp.MustCompile("Invalid Expiry Status Value"),
			},
		},
	})
}

// TestUnitUserMarkdownDescriptionCoverage verifies that MarkdownDescription
// returns a non-empty string for each validator type.
func TestUnitUserMarkdownDescriptionCoverage(t *testing.T) {
	expiryVal := expiryStatusValidator{}
	pwVal := basicPasswordValidator{}

	if desc := expiryVal.MarkdownDescription(context.TODO()); desc == "" {
		t.Error("expiryStatusValidator.MarkdownDescription returned empty string")
	}
	if desc := pwVal.MarkdownDescription(context.TODO()); desc == "" {
		t.Error("basicPasswordValidator.MarkdownDescription returned empty string")
	}
}

// ---------------------------------------------------------------------------
// Unit tests for Read and Delete error paths to improve coverage
// ---------------------------------------------------------------------------

// TestUnitUserReadWithEmptyRole verifies the Read path when the API returns
// an empty role for a user that has a role in state — exercises the
// "preserve role from state" warning branch in Read.
func TestUnitUserReadWithEmptyRole(t *testing.T) {
	st := setupUserMock(t, map[string][]string{})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: create user normally so state has role="operator".
			{
				Config: testAccUserResourceConfig("emptyrolemock", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "emptyrolemock"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
				),
			},
			// Step 2: before the next plan, clear the config role from the mock
			// so that GET returns role="" — this triggers the "preserve from state"
			// warning branch in Read.  The plan should be empty (role preserved).
			{
				PreConfig: func() {
					st.mu.Lock()
					delete(st.userConfigRole, "emptyrolemock")
					st.mu.Unlock()
				},
				Config: testAccUserResourceConfig("emptyrolemock", "operator"),
				Check:  resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
			},
		},
	})
}

// TestUnitUserDeleteWithGetUserRolesFailure verifies the Delete path when
// getUserRoles fails — exercises the warning path that continues with deletion.
func TestUnitUserDeleteWithGetUserRolesFailure(t *testing.T) {
	st := setupUserMock(t, map[string][]string{
		"operator": {"deleterolesfailmock"},
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create user
			{
				Config: testAccUserResourceConfig("deleterolesfailmock", "operator"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "deleterolesfailmock"),
				),
			},
			// Destroy with getUserRoles failure
			{
				PreConfig: func() {
					st.mu.Lock()
					// Set counter high to fail all getUserRoles calls during destroy
					st.failGetRolesCount = 10
					st.mu.Unlock()
				},
				Config:  testAccUserResourceConfig("deleterolesfailmock", "operator"),
				Destroy: true,
				// Exercises Delete warning path at lines 329-333
			},
		},
	})
}
