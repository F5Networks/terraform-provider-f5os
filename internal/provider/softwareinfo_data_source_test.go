package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccSystemsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccSystemsDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_softwareinfo.test", "id", "b4b8f21c-beed-4730-a3ca-db73db6ce92a"),
				),
				ExpectError: regexp.MustCompile("Attribute 'id' expected"),
			},
		},
	})
}

const testAccSystemsDataSourceConfig = `
data "f5os_softwareinfo" "test" {}
`
