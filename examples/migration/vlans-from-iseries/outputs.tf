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

output "configured_lags" {
  description = "Map of LAG name to its configured lag_type/mode/interval/members/native_vlan/trunk_vlans, for every LAG configured from the i-Series source."
  value = {
    for name, lag in f5os_lag.from_iseries : name => {
      lag_type    = lag.lag_type
      mode        = lag.mode
      interval    = lag.interval
      members     = lag.members
      native_vlan = lag.native_vlan
      trunk_vlans = lag.trunk_vlans
    }
  }
}
