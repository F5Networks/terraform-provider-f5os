package provider

import (
	"fmt"
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
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing for standard user
			{
				Config: testAccUserPasswordChangeResourceConfig("testuser", "old_password_123", "new_password_456"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "user_name", "testuser"),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "old_password", "old_password_123"),
					resource.TestCheckResourceAttr("f5os_user_password_change.test", "new_password", "new_password_456"),
					resource.TestCheckResourceAttrSet("f5os_user_password_change.test", "id"),
				),
			},
		},
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

func testAccUserPasswordChangeResourceConfig(userName, oldPassword, newPassword string) string {
	return fmt.Sprintf(`
resource "f5os_user_password_change" "test" {
  user_name    = %[1]q
  old_password = %[2]q
  new_password = %[3]q
}
`, userName, oldPassword, newPassword)
}
