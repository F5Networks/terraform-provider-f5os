package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/mock"
)

type MockedF5osConn struct {
	mock.Mock
}

func (m *MockedF5osConn) UpdateInterface(id string, payload interface{}) ([]byte, error) {
	args := m.Called(id)
	return (args.Get(0)).([]byte), args.Error(1)
}

func (m *MockedF5osConn) NewSession(payload interface{}) (interface{}, error) {
	args := m.Called(payload)
	return (args.Get(0)).([]byte), args.Error(1)
}

func TestAccInterfaceCreateTC3Resource(t *testing.T) {
	// Define our mocked connection object
	testAccPreUnitCheck(t)
	f5osConn := new(MockedF5osConn)
	f5osConn.On("NewSession", 1).Return([]byte(`{
  "ietf-restconf:errors": {
    "error": [
      {
        "error-type": "application",
        "error-tag": "invalid-value",
        "error-message": "uri keypath not found"
      }
    ]
  }
}`), nil)
	// Setup expectation
	f5osConn.On("UpdateInterface", 1).Return(float64(1000), nil)
	f5osConn.On("UpdateInterface", 2).Return(float64(2000), nil)
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_interface.test_interface",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
func TestAccInterfaceCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_interface.test_interface",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccInterfaceCreateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccInterfaceCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "13"),
				),
			},
			{
				Config: testAccInterfaceCreateTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_vlan.vlan10", "vlan_id", "10"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan11", "vlan_id", "11"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan12", "vlan_id", "12"),
					resource.TestCheckResourceAttr("f5os_vlan.vlan13", "vlan_id", "13"),
					resource.TestCheckResourceAttr("f5os_interface.test_interface", "native_vlan", "11"),
				),
			},
		},
	})
}

const testAccInterfaceCreateResourceConfig = `
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
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  native_vlan = f5os_vlan.vlan13.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan10.vlan_id,
    f5os_vlan.vlan11.vlan_id,
    f5os_vlan.vlan12.vlan_id
  ]
}
`

const testAccInterfaceCreateTC2ResourceConfig = `
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
resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  native_vlan = f5os_vlan.vlan11.vlan_id
  trunk_vlans = [
    f5os_vlan.vlan10.vlan_id,
    f5os_vlan.vlan12.vlan_id,
	f5os_vlan.vlan13.vlan_id
  ]
}
`
