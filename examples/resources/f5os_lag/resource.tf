resource "f5os_lag" "test_lag" {
  name        = "test_lag"
  members     = ["1.0"]
  native_vlan = 5
  trunk_vlans = [
    1,
    2,
    3
  ]
}