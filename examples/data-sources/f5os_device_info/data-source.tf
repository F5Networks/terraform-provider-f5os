data "f5os_device_info" "device_info" {
  gather_info_of = ["all", "!partition_images", "!controller_images"]
}

#The following example with fetch information for interfaces and vlans
data "f5os_device_info" "device_info" {
  gather_info_of = ["interfaces", "vlans"]
}