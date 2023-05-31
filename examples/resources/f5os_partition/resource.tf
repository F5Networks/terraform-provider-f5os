provider "f5os" {
  username = "<chassis_controller_username>"
  password = "<chassis_controller_password>"
  host     = "<chassis_controller_ip>"
}
# Manages F5OS partition
resource "f5os_partition" "velos-part" {
  name              = "TerraformPartition"
  os_version        = "1.3.1-5968"
  ipv4_mgmt_address = "10.144.140.125/24"
  ipv4_mgmt_gateway = "10.144.140.253"
  ipv6_mgmt_address = "2001::1/64"
  ipv6_mgmt_gateway = "2001::"
  slots             = [1, 2]
}