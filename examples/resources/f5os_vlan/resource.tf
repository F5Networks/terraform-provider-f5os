# Manages Vlans on F5OS platforms
resource "f5os_vlan" "vlan-id" {
  vlan_id = 4
  name    = "vlan4"
}