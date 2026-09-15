/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

output "created_vlans" {
  description = "Map of VLAN name to the F5OS-assigned resource id (the numeric VLAN ID as a string) for every VLAN created from the i-Series source."
  value       = { for name, vlan in f5os_vlan.from_iseries : name => vlan.id }
}

output "configured_interfaces" {
  description = "Map of F5OS (rSeries) interface name to its configured native_vlan/trunk_vlans/enabled state, for every interface configured from the i-Series source."
  value = {
    for name, iface in f5os_interface.from_iseries : name => {
      native_vlan = iface.native_vlan
      trunk_vlans = iface.trunk_vlans
      enabled     = iface.enabled
    }
  }
}
