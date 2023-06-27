package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTenantImageCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantImageCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant_image.test", "id", "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_tenant_image.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTenantImageCreateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantImageCreateTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant_image.test", "id", "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_tenant_image.test",
				ImportState:       true,
				ImportStateVerify: false,
			},
		},
	})
}

const testAccTenantImageCreateResourceConfig = `
resource "f5os_tenant_image" "test" {
  image_name  = "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"
  remote_host = "spkapexsrvc01.olympus.f5net.com"
  remote_path = "v17.1.0.1/daily/current/VM"
  local_path  = "images/tenant"
  timeout = 360
}
`

const testAccTenantImageCreateTC2ResourceConfig = `
resource "f5os_tenant_image" "test" {
  image_name  = "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"
  remote_host = "spkapexsrvc01.olympus.f5net.com"
  remote_path = "v17.1.0.1/daily/current/VM"
  local_path  = "images"
  timeout = 360
}
`
