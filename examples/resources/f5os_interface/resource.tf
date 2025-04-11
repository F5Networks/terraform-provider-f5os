resource "f5os_interface" "test_interface" {
  enabled     = true
  name        = "1.0"
  native_vlan = 5
  trunk_vlans = [
    1,
    2,
    3
  ]
}