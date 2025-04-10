package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLicenseResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZZ"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_license.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The license registration key is sensitive and won't be returned in read operations
				ImportStateVerifyIgnore: []string{"registration_key", "addon_keys"},
			},
			// Update and Read testing
			{
				Config: testAccLicenseResourceConfig("W9XXX-8YYYZ-8KKK7-7PPP2-WWWWWWW"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_license.test", "registration_key", "W9XXX-8YYYZ-8KKK7-7PPP2-WWWWWWW"),
				),
			},
		},
	})
}

func testAccLicenseResourceConfig(regKey string) string {
	return fmt.Sprintf(`
resource "f5os_license" "test" {
  registration_key = %[1]q
}
`, regKey)
}
