package provider

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDeviceInfo(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: deviceInfoConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("data.f5os_device_info.device_info", "interfaces.#", assertLengthGreaterThankZero),
					resource.TestCheckResourceAttrWith("data.f5os_device_info.device_info", "vlans.#", assertLengthGreaterThankZero),
				),
			},
		},
	})
}

func assertLengthGreaterThankZero(value string) error {
	length, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("failed to parse vlans length: %v", err)
	}
	if length <= 0 {
		return fmt.Errorf("expected vlans length to be greater than 0, got %d", length)
	}
	return nil
}

const deviceInfoConfig = `
data "f5os_device_info" "device_info" {
  gather_info_of = ["all", "!partition_images", "!controller_images"]
}
`
