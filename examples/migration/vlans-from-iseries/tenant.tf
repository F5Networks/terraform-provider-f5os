/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# Deploys one F5OS tenant per BIG-IP instance discovered on the source
# i-Series device, sized appropriately for its workload (CPU cores,
# memory, and virtual disk size), with the VLANs migrated in Phase 3
# attached and the management IP/gateway/prefix configured so the tenant
# is reachable once running.
#
# This is Phase 6 of the i-Series -> r-Series (F5OS) migration workflow
# (VLAN creation, Phase 3/main.tf; interface configuration, Phase
# 4/interfaces.tf; and LAG configuration, Phase 5/lags.tf, are this
# phase's prerequisites -- all in this same directory): unlike Phases
# 3-5, there is no direct Phase 1 JSON source for a tenant's
# cpu_cores/memory/virtual_disk_size -- TMOS's per-i-Series-appliance
# resourcing does not map onto per-tenant sizing on F5OS the way
# VLANs/interfaces/trunks do structurally. See
# https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/deploy-tenants-from-iseries
# for full sizing guidance
# and the complete workflow.
#
# Depends on VLAN creation completing first: vlans below is resolved
# from VLAN ID/tag to the actual f5os_vlan resource via
# local.vlan_name_by_tag (defined in main.tf, alongside
# f5os_vlan.from_iseries itself, so this file has no dependency on
# interfaces.tf or lags.tf or vice versa), so Terraform infers a real
# dependency on f5os_vlan.from_iseries from the resource reference
# alone; the explicit depends_on below is additional, deliberate
# documentation of that same intent.
#
# Deliberately does NOT depend_on f5os_interface.from_iseries /
# f5os_lag.from_iseries: a tenant's `vlans` argument only requires the
# VLAN ID to exist in the chassis partition/platform (which f5os_vlan
# guarantees), not that any particular physical interface or LAG
# already carries that VLAN -- the L2 wiring (Phases 4/5) and the
# tenant's VLAN membership (this phase) are independent concerns to
# F5OS itself. Operationally, though, the tenant's data-plane VLANs
# should already be trunked onto the interfaces/LAGs the tenant will
# actually use before (or by the time) it reaches running_state =
# "deployed", or its data-plane traffic has nowhere to go once running
# -- this is a deployment-sequencing concern for the operator, not a
# hard Terraform dependency, since enforcing one here would break the
# "each phase's file can be deleted independently" property the rest of
# this directory preserves (e.g. an operator who only needs the
# management-plane parts of a migration, with data-plane VLAN trunking
# handled out of band).
# ---------------------------------------------------------------------------

resource "f5os_tenant" "from_iseries" {
  for_each = var.tenants

  depends_on = [f5os_vlan.from_iseries]

  name              = each.key
  image_name        = each.value.image_name
  type              = each.value.type
  deployment_file   = each.value.type == "BIG-IP-Next" ? each.value.deployment_file : null
  cpu_cores         = each.value.cpu_cores
  memory            = each.value.memory
  virtual_disk_size = each.value.virtual_disk_size
  nodes             = each.value.nodes
  max_nodes         = each.value.max_nodes
  mac_block_size    = each.value.mac_block_size
  cryptos           = each.value.cryptos
  running_state     = each.value.running_state
  timeout           = each.value.timeout

  mgmt_ip      = each.value.mgmt_ip
  mgmt_gateway = each.value.mgmt_gateway
  mgmt_prefix  = each.value.mgmt_prefix

  # each.value.vlans is a list of VLAN ID/tags (matching var.vlans'
  # values), not names; local.vlan_name_by_tag[...].vlan_id round-trips
  # through the actual created f5os_vlan resource's attribute rather
  # than reusing the tag literal directly, so this expression -- not
  # just depends_on -- is what Terraform's graph actually walks to
  # order these resources.
  vlans = [
    for tag in each.value.vlans :
    f5os_vlan.from_iseries[local.vlan_name_by_tag[tag]].vlan_id
  ]
}
