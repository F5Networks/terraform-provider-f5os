/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# Configures one F5OS LAG (Link Aggregation Group) per trunk discovered on
# the source i-Series device, preserving the trunk name, LACP mode
# (active/passive) and LACP/static type, member interfaces (mapped to
# rSeries names), and native/trunk VLAN assignment.
#
# This is Phase 5 of the i-Series -> r-Series (F5OS) migration workflow
# (VLAN creation, Phase 3/main.tf, and interface configuration, Phase
# 4/interfaces.tf, are this phase's prerequisites -- both in this same
# directory): it consumes the trunk portion of the JSON produced by
# terraform-provider-bigip's `scripts/extract-sys-settings.sh` (Phase 1),
# converted into the `lags` map below via `scripts/lags-from-iseries.sh`
# in this repo. See
# https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/configure-lags-from-iseries
# for the
# full workflow, including how TMOS trunk membership (`interfaces =
# ["1.1", "1.2"]`, owned by the trunk) inverts to F5OS LAG membership
# (`members`, still expressed on the LAG resource itself by this
# provider, unlike the raw device API where membership is set per
# physical-interface `aggregate-id`).
#
# Depends on VLAN creation completing first: native_vlan/trunk_vlans
# below are resolved from VLAN ID/tag to the actual f5os_vlan resource
# via local.vlan_name_by_tag (defined in main.tf, alongside
# f5os_vlan.from_iseries itself, so this file has no dependency on
# interfaces.tf or vice versa -- an operator migrating only trunks can
# delete interfaces.tf entirely without this file breaking), so
# Terraform infers a real dependency on f5os_vlan.from_iseries from the
# resource reference alone; the explicit depends_on below is additional,
# deliberate documentation of that same intent.
# ---------------------------------------------------------------------------

resource "f5os_lag" "from_iseries" {
  for_each = var.lags

  depends_on = [f5os_vlan.from_iseries]

  name     = each.key
  lag_type = each.value.lag_type
  mode     = each.value.lag_type == "LACP" ? each.value.mode : null
  interval = each.value.lag_type == "LACP" ? each.value.interval : null
  members  = each.value.members

  # each.value.native_vlan is nullable (a trunk with no untagged VLAN
  # membership on the source device has no native VLAN to configure);
  # local.vlan_name_by_tag[...].vlan_id round-trips through the actual
  # created resource's attribute rather than reusing the tag literal
  # directly, so this expression -- not just depends_on -- is what
  # Terraform's graph actually walks to order these resources.
  native_vlan = (
    each.value.native_vlan == null
    ? null
    : f5os_vlan.from_iseries[local.vlan_name_by_tag[each.value.native_vlan]].vlan_id
  )

  trunk_vlans = [
    for tag in each.value.trunk_vlans :
    f5os_vlan.from_iseries[local.vlan_name_by_tag[tag]].vlan_id
  ]
}
