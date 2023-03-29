package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTenantDeployResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		//IsUnitTest:               true,
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantDeployResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant.test2", "id", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "name", "testtenant-ecosys2"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "image_name", "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_ip", "10.10.10.26"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "mgmt_gateway", "10.10.10.1"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "type", "BIG-IP"),
					resource.TestCheckResourceAttr("f5os_tenant.test2", "status", "Configured"),
				),
			},
		},
	})
}

const testAccTenantDeployResourceConfig = `
resource "f5os_tenant" "test2" {
  name              = "testtenant-ecosys2"
  image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
  mgmt_ip           = "10.10.10.26"
  mgmt_gateway      = "10.10.10.1"
  mgmt_prefix       = 24
  type              = "BIG-IP"
  cpu_cores         = 8
  running_state     = "configured"
  virtual_disk_size = 82
}
`
