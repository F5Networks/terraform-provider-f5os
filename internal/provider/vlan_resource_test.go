package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVlanCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan2"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_vlan.vlan-id",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccVlanCreateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccVlanCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan2"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
			{
				Config: testAccVlanCreateResourceTC2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "id", "400"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "name", "mytestvlan3"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan-id", "vlan_id", "400"),
				),
			},
		},
	})
}

const testAccVlanCreateResourceConfig = `
resource "f5os_vlan" "vlan-id" {
 vlan_id = 400
 name = "mytestvlan2"
}
`

const testAccVlanCreateResourceTC2Config = `
resource "f5os_vlan" "vlan-id" {
 vlan_id = 400
 name = "mytestvlan3"
}
`
