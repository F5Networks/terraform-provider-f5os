package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTenantImageDataSourceTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantImageDatasourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_tenant_image.test", "id", "BIG-IP-Next-20.0.1-2.123.17"),
				),
			},
			{
				Config:      testAccTenantImageDatasourceFailConfig,
				ExpectError: regexp.MustCompile("TF-001:Unable to Get Image Details"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_tenant_image.test", "id", "BIG-IP-Next-20.0.1-2.123.18"),
				),
			},
		},
	})
}

const testAccTenantImageDatasourceConfig = `
data "f5os_tenant_image" "test" {
  image_name             = "BIG-IP-Next-20.0.1-2.123.17"
}
`
const testAccTenantImageDatasourceFailConfig = `
data "f5os_tenant_image" "test" {
  image_name             = "BIG-IP-Next-20.0.1-2.123.18"
}
`
