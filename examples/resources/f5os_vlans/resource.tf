# Creates/updates every VLAN in `vlans` in a single RESTCONF PATCH call,
# significantly faster than `for_each` + f5os_vlan for large VLAN sets
# (e.g. an i-Series-to-F5OS migration with hundreds of VLANs). See the
# f5os_vlans resource documentation for the trade-off versus `for_each`
# + f5os_vlan (per-VLAN Terraform resource addressing).
resource "f5os_vlans" "test" {
  vlans = {
    "external" = 100
    "internal" = 101
    "storage"  = 102
  }
}
