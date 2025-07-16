package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
  password = "MyStrongP@ss123"
  role     = %[2]q
}
`, username, role)
}

// Test idempotency - ensure applying same config twice doesn't show changes
func TestAccUserResourceIdempotency(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
  password      = "MyGoodP@ssword99"
  role          = %[2]q
  expiry_status = %[3]q
}
`, username, role, expiryStatus)
}

func TestAccUserResourceWithSecondaryRole(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
  password       = "MyBigP@ssword88"
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
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfigWithExpiry("testuser7", "operator", "2025-12-31"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user.test", "username", "testuser7"),
					resource.TestCheckResourceAttr("f5os_user.test", "role", "operator"),
					resource.TestCheckResourceAttr("f5os_user.test", "expiry_status", "2025-12-31"),
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
				// This test expects F5OS to reject the password due to policy requirements
				ExpectError: regexp.MustCompile("Error Setting User Password"),
			},
		},
	})
}

// Test basic user creation without import (simpler test for debugging)
func TestAccUserResourceBasicOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
