/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# Configures every F5OS (rSeries) interface discovered on the source
# i-Series device with its native/trunk VLAN assignment and enabled
# state, matching the source device's interface/VLAN layout.
#
# This is Phase 4 of the i-Series -> r-Series (F5OS) migration workflow
# (VLAN creation, Phase 3/main.tf in this same directory, is this
# phase's prerequisite): it consumes the interface portion of the JSON
# produced by terraform-provider-bigip's `scripts/extract-sys-settings.sh`
# (Phase 1), converted into the `interfaces` map below via
# `scripts/interfaces-from-iseries.sh` in this repo. See
# docs/guides/configure-interfaces-from-iseries.md for the full
# workflow, including how TMOS interface names (`1.1`) are mapped to
# rSeries names (`1.0`).
#
# Depends on VLAN creation completing first: native_vlan/trunk_vlans
# below are resolved from VLAN ID/tag to the actual f5os_vlan resource
# via local.vlan_name_by_tag, so Terraform infers a real dependency on
# f5os_vlan.from_iseries from the resource reference alone; the explicit
# depends_on below is additional, deliberate documentation of that same
# intent (and a safety net if a future edit removes the reference
# without noticing the ordering requirement it was providing).
# ---------------------------------------------------------------------------

locals {
  # Reverses var.vlans (name -> tag) into (tag -> name) so a VLAN
  # ID/tag referenced by var.interfaces can be resolved back to the
  # f5os_vlan.from_iseries resource instance that owns it -- this is
  # what makes the dependency on VLAN creation a real Terraform
  # reference rather than two configurations that merely happen to
  # agree on the same numeric literals.
  vlan_name_by_tag = { for name, tag in var.vlans : tag => name }
}

resource "f5os_interface" "from_iseries" {
  for_each = var.interfaces

  depends_on = [f5os_vlan.from_iseries]

  name    = each.key
  enabled = each.value.enabled

  # each.value.native_vlan is nullable (an interface with no untagged
  # VLAN membership on the source device has no native VLAN to
  # configure); local.vlan_name_by_tag[...].vlan_id round-trips through
  # the actual created resource's attribute rather than reusing the
  # tag literal directly, so this expression -- not just depends_on --
  # is what Terraform's graph actually walks to order these resources.
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
