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

# F5OS 2.0.0+: an optional description leaf and a computed phyport
# state leaf are surfaced. `description` is only accepted on 2.0.0+;
# supplying it on older devices returns a clear "Unsupported
# attribute" error before any device write.
resource "f5os_interface" "test_interface_with_description" {
  name        = "1.1"
  enabled     = true
  description = "uplink to leaf-01"
  native_vlan = 5
}
