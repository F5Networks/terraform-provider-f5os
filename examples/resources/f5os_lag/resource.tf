# LACP LAG (default when lag_type is omitted)
resource "f5os_lag" "lacp_lag" {
  name        = "lacp_lag"
  lag_type    = "LACP"
  members     = ["1.0", "2.0"]
  native_vlan = 5
  trunk_vlans = [1, 2, 3]
  mode        = "ACTIVE"
  interval    = "SLOW"
}

# Static LAG (no LACP negotiation)
resource "f5os_lag" "static_lag" {
  name        = "static_lag"
  lag_type    = "STATIC"
  members     = ["1.0", "2.0"]
  native_vlan = 5
  trunk_vlans = [1, 2, 3]
}
