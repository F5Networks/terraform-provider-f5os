package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLagInterfaceCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccLagInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "11"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.2", "12"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "1.0"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.1", "2.0"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_lag.test_lag",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccLagInterfaceCreateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccLagInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "11"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.2", "12"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "1.0"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.1", "2.0"),
				),
			},
			{
				Config: testAccLagInterfaceCreateTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "native_vlan", "11"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.0", "10"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.1", "12"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "trunk_vlans.2", "13"),
					resource.TestCheckResourceAttr("f5os_lag.test_lag", "members.0", "2.0"),
				),
			},
		},
	})
}

const testAccLagInterfaceCreateResourceConfig = `
resource "f5os_vlan" "vlan10" {
 vlan_id = 10
 name = "vlan10"
}
resource "f5os_vlan" "vlan11" {
 vlan_id = 11
 name = "vlan11"
}
resource "f5os_vlan" "vlan12" {
 vlan_id = 12
 name = "vlan12"
}
resource "f5os_vlan" "vlan13" {
 vlan_id = 13
 name = "vlan13"
}
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  native_vlan = f5os_vlan.vlan13.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan10.vlan_id,
    f5os_vlan.vlan11.vlan_id,
    f5os_vlan.vlan12.vlan_id
  ]
  members = ["1.0", "2.0"]
}
`
const testAccLagInterfaceCreateTC2ResourceConfig = `
resource "f5os_vlan" "vlan10" {
 vlan_id = 10
 name = "vlan10"
}
resource "f5os_vlan" "vlan11" {
 vlan_id = 11
 name = "vlan11"
}
resource "f5os_vlan" "vlan12" {
 vlan_id = 12
 name = "vlan12"
}
resource "f5os_vlan" "vlan13" {
 vlan_id = 13
 name = "vlan13"
}
resource "f5os_lag" "test_lag" {
  name        = "tf-lag"
  native_vlan = f5os_vlan.vlan11.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan10.vlan_id,
    f5os_vlan.vlan12.vlan_id,
	f5os_vlan.vlan13.vlan_id
  ]
  members = ["2.0"]
}
`
