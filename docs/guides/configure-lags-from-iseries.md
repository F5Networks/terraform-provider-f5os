---
page_title: "Configuring F5OS LAGs from discovered iSeries configuration"
description: |-
  How to use examples/migration/vlans-from-iseries/lags.tf and scripts/lags-from-iseries.sh to create f5os_lag resources on an rSeries F5OS target matching the trunk (LAG) configuration discovered on a source BIG-IP iSeries device, including LACP mode and member interfaces.
---

# Configuring F5OS LAGs from discovered iSeries configuration

`examples/migration/vlans-from-iseries/lags.tf` is Phase 5 of an
iSeries -> rSeries (F5OS) migration workflow: it configures one
`f5os_lag` resource per trunk discovered on a source BIG-IP iSeries
device, preserving the trunk name, LACP type/mode/interval, member
interfaces (mapped to F5OS names), and native/trunk VLAN assignment.
`scripts/lags-from-iseries.sh` converts the trunk/VLAN membership
portion of the `extracted-sys-settings.json` produced by
[`terraform-provider-bigip`'s `scripts/extract-sys-settings.sh`](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings)
(that provider's Phase 1) into the `lags` map this configuration
expects.

For the full cross-repo phase sequence, see the [overall iSeries to
rSeries migration flow](iseries-to-rseries-migration-flow.html).

This phase lives in the **same** `examples/migration/vlans-from-iseries`
directory, and therefore the same Terraform state, as [Phase 3, VLAN
creation](create-vlans-from-iseries.html) and [Phase 4, interface
configuration](configure-interfaces-from-iseries.html) -- not a separate
directory -- specifically so `f5os_lag.from_iseries` can hold a real
Terraform reference to `f5os_vlan.from_iseries`, making VLAN creation a
graph dependency Terraform itself enforces rather than just an
operational instruction to run one step before the other.

## Trunk membership is inverted between TMOS and F5OS

TMOS and F5OS represent link aggregation with **inverted ownership**
(see [`terraform-provider-bigip`'s interface/trunk naming
guide](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping#trunk-lag-mapping-is-not-11)
for the full comparison): a TMOS trunk object owns its member list
directly (`bigip_net_trunks` reports `interfaces = ["1.1", "1.2"]` on
the trunk itself); the F5OS device API instead sets membership on each
*physical interface* (`aggregate-id <lag-name>`), not on the LAG object.
This provider's `f5os_lag` resource re-exposes membership as a
`members` argument on the LAG resource itself (mirroring TMOS's
ownership direction, not the raw device API's), so `lags.tf` below
still looks like a single per-trunk resource with a member list, the
same shape `scripts/lags-from-iseries.sh` produces.

## Field mapping: TMOS trunk to F5OS LAG

| TMOS (`bigip_net_trunks` field) | F5OS (`f5os_lag` argument) | Notes |
|---|---|---|
| `name` | `lags` map key (LAG name) | Preserved verbatim -- F5OS LAG names are free-form identifiers, not numeric IDs, so unlike physical interfaces there is no name-mapping step for the LAG name itself (member interface names still need mapping -- see below). |
| `lacp` (`"enabled"`/`"disabled"`) | `lag_type` (`"LACP"`/`"STATIC"`) | A TMOS trunk with `lacp = "disabled"` (a static/non-LACP trunk) maps to F5OS's `STATIC` `lag_type`. |
| `lacp_mode` (`"active"`/`"passive"`) | `mode` (`"ACTIVE"`/`"PASSIVE"`) | LACP-only; `null` for `STATIC` LAGs (matches `f5os_lag`'s own validation, which rejects a non-null `mode` when `lag_type` is `STATIC`). |
| `lacp_timeout` (`"long"`/`"short"`) | `interval` (`"SLOW"`/`"FAST"`) | LACP-only, same nullability as `mode`. TMOS's "long" timeout (30s periodic) corresponds to F5OS's `SLOW` interval; "short" (1s) to `FAST`. |
| `interfaces` (TMOS member names, e.g. `["1.1", "1.2"]`) | `members` (F5OS-mapped names, e.g. `["1.0", "2.0"]`) | Same blade-drop mapping as physical interfaces (see [Phase 4's interface name mapping](configure-interfaces-from-iseries.html#interface-name-mapping-i-series-11-to-rseries-10)) -- `scripts/lags-from-iseries.sh` applies it to each member name independently. |
| `distribution_hash` | *(not migrated)* | `f5os_lag` hardcodes `src-dst-ipport` internally (see `internal/provider/lag_resource.go`) and does not expose a configurable distribution-hash argument; this is not something `scripts/lags-from-iseries.sh` can preserve. |
| `bandwidth`, `working_member_count`, `stp`, `type` | *(not migrated)* | Runtime state/counters on F5OS (`state` block, `oper-status`), not something configured to match a TMOS value. |
| *(none -- VLANs are separate TMOS objects tagged to the trunk)* | `native_vlan` / `trunk_vlans` | Same derivation as `f5os_interface` in Phase 4: `bigip_net_vlans`' per-VLAN `interfaces[]` membership list is inverted into a per-trunk `native_vlan`/`trunk_vlans` pair (a trunk name appears in that list exactly like a physical interface name would). |

## Example Usage

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# Configures one F5OS LAG (Link Aggregation Group) per trunk discovered on
# the source iSeries device, preserving the trunk name, LACP mode
# (active/passive) and LACP/static type, member interfaces (mapped to
# rSeries names), and native/trunk VLAN assignment.
#
# This is Phase 5 of the iSeries -> rSeries (F5OS) migration workflow
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
```

The `lags` variable definition (in `variables.tf`, alongside the `vlans`
and `interfaces` variables from Phases 3 and 4):

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# vlans is keyed by VLAN name (preserved from the source iSeries device)
# and maps to the numeric VLAN ID/tag (also preserved from the source
# device). Populate this from the iSeries `data.bigip_net_vlans` output
# discovered by scripts/extract-sys-settings.sh in the sibling
# terraform-provider-bigip repo -- see scripts/vlans-from-iseries.sh in
# this repo, which converts that provider's `extracted-sys-settings.json`
# into a ready-to-use vlans.auto.tfvars.json for this configuration.
#
# Example:
#   vlans = {
#     "extracted-external" = 100
#     "extracted-internal" = 101
#   }
# ---------------------------------------------------------------------------
variable "vlans" {
  description = "Map of VLAN name (from the source iSeries device) to VLAN ID/tag (also preserved from the source device) to create on the F5OS (rSeries/Velos partition) target."
  type        = map(number)

  validation {
    condition     = alltrue([for id in values(var.vlans) : id >= 0 && id <= 4095])
    error_message = "Every VLAN ID in var.vlans must be between 0 and 4095 (the F5OS f5os_vlan valid range)."
  }

  # Mirrors f5os_vlan's `name` schema constraint (see
  # internal/provider/vlan_resource.go): must start with a letter,
  # remaining characters alphanumeric/period/comma/hyphen/underscore
  # only, max 58 characters. Catching this here means a malformed name
  # in var.vlans (e.g. from a hand-edited tfvars file, or a name
  # scripts/vlans-from-iseries.sh already warned about but couldn't
  # itself reject) fails at `terraform plan` with a clear message
  # instead of a runtime error from the F5OS device during apply.
  validation {
    condition     = alltrue([for name in keys(var.vlans) : can(regex("^[A-Za-z][A-Za-z0-9.,_-]{0,57}$", name))])
    error_message = "Every VLAN name in var.vlans must start with a letter, contain only alphanumeric characters, periods, commas, hyphens, or underscores, and not exceed 58 characters."
  }

  # interfaces.tf builds a tag -> name reverse lookup
  # (local.vlan_name_by_tag) to resolve native_vlan/trunk_vlans back to
  # the f5os_vlan resource that owns each tag. That reverse lookup is a
  # Terraform `for` expression over map keys and hard-fails at plan
  # time with "Duplicate object key" if two VLAN names share the same
  # tag -- which can legitimately happen on a real TMOS source, where
  # VLAN IDs are scoped per route domain/partition and can repeat
  # across them. Reject that case here, at the source of the ambiguity,
  # with an error that actually explains what's wrong, instead of
  # letting it surface downstream in interfaces.tf as a confusing
  # "Duplicate object key" error with no indication of which VLANs or
  # why.
  validation {
    condition     = length(values(var.vlans)) == length(distinct(values(var.vlans)))
    error_message = "Every value in var.vlans must be unique -- two VLAN names cannot share the same VLAN ID/tag. This configuration has no partition/route-domain concept to disambiguate them (unlike the source iSeries device, where VLAN IDs can repeat across route domains): rename or merge the colliding VLANs before applying, since interfaces.tf's tag-to-name lookup cannot distinguish them either."
  }
}

# ---------------------------------------------------------------------------
# interfaces is keyed by the already-mapped F5OS (rSeries) interface name
# (e.g. "1.0"), not the source TMOS name (e.g. "1.1") -- the name mapping
# itself happens in scripts/interfaces-from-iseries.sh, which strips the
# TMOS blade prefix (rSeries has no blade concept: "<blade>.<port>" ->
# "<port>.0"). Each entry's native_vlan/trunk_vlans values are VLAN
# ID/tags, not names -- they are looked up against the VLANs created by
# f5os_vlan.from_iseries in interfaces.tf via local.vlan_name_by_tag, so
# that a real Terraform reference (and therefore ordering dependency)
# exists between the two resources rather than just matching numeric
# literals that happen to be the same.
#
# native_vlan is nullable: an interface with no untagged VLAN membership
# on the source device has no native VLAN to configure.
#
# Populate this from iSeries `bigip_net_interfaces`/`bigip_net_vlans`
# output via scripts/interfaces-from-iseries.sh in this repo, which
# converts terraform-provider-bigip's `extracted-sys-settings.json` into
# a ready-to-use interfaces.auto.tfvars.json for this configuration.
#
# Example:
#   interfaces = {
#     "1.0" = { native_vlan = 100, trunk_vlans = [], enabled = true }
#     "2.0" = { native_vlan = 101, trunk_vlans = [], enabled = true }
#   }
# ---------------------------------------------------------------------------
variable "interfaces" {
  description = "Map of F5OS (rSeries) interface name to its native/trunk VLAN assignment and enabled state, mapped from the source iSeries device's interface/VLAN layout."
  type = map(object({
    native_vlan = number
    trunk_vlans = list(number)
    enabled     = bool
  }))
  default = {}

  validation {
    # Terraform's `||` does not short-circuit -- both operands are
    # always evaluated -- so `iface.native_vlan == null ||
    # (iface.native_vlan >= 0 && ...)` would still evaluate the range
    # comparison against a null native_vlan and fail with "argument
    # must not be null" on the very interfaces this variable documents
    # as legitimately having none. The ternary below only evaluates the
    # range check when native_vlan is non-null.
    condition = alltrue([
      for iface in values(var.interfaces) : (
        iface.native_vlan == null ? true : (iface.native_vlan >= 0 && iface.native_vlan <= 4095)
      ) && alltrue([for id in iface.trunk_vlans : id >= 0 && id <= 4095])
    ])
    error_message = "Every native_vlan/trunk_vlans VLAN ID in var.interfaces must be between 0 and 4095 (the F5OS f5os_vlan valid range), or native_vlan must be null."
  }

  validation {
    # Same non-short-circuiting-|| pitfall as above applies to
    # contains(): only call it when native_vlan is non-null.
    condition = alltrue([
      for iface in values(var.interfaces) : (
        iface.native_vlan == null ? true : contains(values(var.vlans), iface.native_vlan)
      ) && alltrue([for id in iface.trunk_vlans : contains(values(var.vlans), id)])
    ])
    error_message = "Every native_vlan/trunk_vlans VLAN ID in var.interfaces must also be present in var.vlans, so the referenced VLAN is actually created by f5os_vlan.from_iseries."
  }

  # Mirrors f5os_interface's `name` documentation (see
  # internal/provider/interface_resource.go): rSeries names are
  # `<port>.0`, VELOS partition names are `<blade>/<port>.0`. Catching an
  # unmapped or otherwise invalid name here means a malformed key in
  # var.interfaces (e.g. from a hand-edited tfvars file, or a TMOS name
  # scripts/interfaces-from-iseries.sh couldn't map and passed through
  # unchanged -- see that script's "Interface name mapping" header
  # comment) fails at `terraform plan` with a clear message instead of a
  # runtime error from the F5OS device during apply.
  validation {
    condition     = alltrue([for name in keys(var.interfaces) : can(regex("^(?:[0-9]+\\.[0-9]+|[0-9]+/[0-9]+\\.[0-9]+)$", name))])
    error_message = "Every interface name in var.interfaces must be a valid F5OS interface name: '<port>.0' for rSeries (e.g. \"1.0\") or '<blade>/<port>.0' for VELOS partitions (e.g. \"1/1.0\"). Unmapped or invalid TMOS interface names must be renamed manually before applying."
  }
}

# ---------------------------------------------------------------------------
# lags is keyed by LAG name (preserved verbatim from the source iSeries
# trunk name -- F5OS LAG names are free-form identifiers, not numeric
# IDs, so there is no name-mapping step here the way there is for
# var.interfaces). Each entry mirrors f5os_lag's own attributes directly:
# lag_type ("LACP"/"STATIC", from the source trunk's `lacp`
# enabled/disabled state), mode/interval (only meaningful for LACP,
# from the source trunk's `lacp_mode`/`lacp_timeout`), members (already
# mapped to F5OS interface names -- the name mapping itself happens in
# scripts/lags-from-iseries.sh, same blade-drop rule as
# scripts/interfaces-from-iseries.sh), and native_vlan/trunk_vlans (VLAN
# ID/tags, not names -- looked up against the VLANs created by
# f5os_vlan.from_iseries in lags.tf via local.vlan_name_by_tag, the same
# lookup interfaces.tf uses).
#
# native_vlan is nullable: a trunk with no untagged VLAN membership on
# the source device has no native VLAN to configure.
#
# Populate this from iSeries `bigip_net_trunks`/`bigip_net_vlans`
# output via scripts/lags-from-iseries.sh in this repo, which converts
# terraform-provider-bigip's `extracted-sys-settings.json` into a
# ready-to-use lags.auto.tfvars.json for this configuration.
#
# Example:
#   lags = {
#     "lag1" = {
#       lag_type    = "LACP"
#       mode        = "ACTIVE"
#       interval    = "SLOW"
#       members     = ["1.0", "2.0"]
#       native_vlan = null
#       trunk_vlans = [200]
#     }
#   }
# ---------------------------------------------------------------------------
variable "lags" {
  description = "Map of LAG name (from the source iSeries trunk name) to its type/LACP settings, member interfaces (already mapped to F5OS names), and native/trunk VLAN assignment, mapped from the source iSeries device's trunk/VLAN layout."
  type = map(object({
    lag_type    = string
    mode        = string
    interval    = string
    members     = list(string)
    native_vlan = number
    trunk_vlans = list(number)
  }))
  default = {}

  # Mirrors f5os_lag's own lag_type validator (see
  # internal/provider/lag_resource.go): only "LACP" or "STATIC".
  validation {
    condition     = alltrue([for lag in values(var.lags) : contains(["LACP", "STATIC"], lag.lag_type)])
    error_message = "Every lag_type in var.lags must be \"LACP\" or \"STATIC\" (the F5OS f5os_lag valid lag_type values)."
  }

  # Mirrors f5os_lag's own ValidateConfig (see
  # internal/provider/lag_resource.go): mode/interval are LACP-only --
  # f5os_lag itself rejects a non-null mode/interval when lag_type is
  # STATIC. Catching this here means a script or hand-edited tfvars bug
  # fails at `terraform plan` instead of `terraform apply`.
  validation {
    condition = alltrue([
      for lag in values(var.lags) : lag.lag_type == "STATIC" ? (lag.mode == null && lag.interval == null) : true
    ])
    error_message = "mode/interval must both be null in var.lags when lag_type is \"STATIC\" -- LACP mode/interval are only applicable to LACP LAGs (matches f5os_lag's own validation)."
  }

  # Mirrors f5os_lag's own mode/interval validators (see
  # internal/provider/lag_resource.go): only meaningful (and only
  # validated here) for LACP LAGs, since the block above already
  # requires both to be null for STATIC LAGs. mode/interval are
  # Optional on f5os_lag itself (the device applies its own default
  # when omitted), so null must remain a valid value here too --
  # operators accepting the device default for an LACP LAG must not be
  # forced to guess and hard-code one of the enumerated values just to
  # satisfy this validation.
  # Same non-short-circuiting-|| pitfall as var.interfaces' native_vlan
  # validations above applies to contains(): `lag.mode == null ||
  # contains(..., lag.mode)` would still evaluate contains() against a
  # null lag.mode and fail with "argument must not be null" on the
  # very LAGs this validation exists to allow (an LACP LAG accepting
  # the device default). The ternary below only calls contains() when
  # mode is non-null.
  validation {
    condition = alltrue([
      for lag in values(var.lags) : lag.lag_type == "LACP" ? (lag.mode == null ? true : contains(["ACTIVE", "PASSIVE"], lag.mode)) : true
    ])
    error_message = "Every mode in var.lags must be \"ACTIVE\", \"PASSIVE\", or null (to accept the F5OS device default) when lag_type is \"LACP\" (the F5OS f5os_lag valid mode values)."
  }

  validation {
    condition = alltrue([
      for lag in values(var.lags) : lag.lag_type == "LACP" ? (lag.interval == null ? true : contains(["SLOW", "FAST"], lag.interval)) : true
    ])
    error_message = "Every interval in var.lags must be \"SLOW\", \"FAST\", or null (to accept the F5OS device default) when lag_type is \"LACP\" (the F5OS f5os_lag valid interval values)."
  }

  # Mirrors var.interfaces' name-format validation above, applied to
  # each LAG's member interfaces instead of a map key: catches an
  # unmapped or otherwise invalid F5OS interface name that
  # scripts/lags-from-iseries.sh couldn't map and passed through
  # unchanged.
  validation {
    condition = alltrue([
      for lag in values(var.lags) : alltrue([
        for m in lag.members : can(regex("^(?:[0-9]+\\.[0-9]+|[0-9]+/[0-9]+\\.[0-9]+)$", m))
      ])
    ])
    error_message = "Every member interface name in var.lags must be a valid F5OS interface name: '<port>.0' for rSeries (e.g. \"1.0\") or '<blade>/<port>.0' for VELOS partitions (e.g. \"1/1.0\"). Unmapped or invalid TMOS interface names must be renamed manually before applying."
  }

  # Mirrors var.interfaces' VLAN-range validation above.
  validation {
    condition = alltrue([
      for lag in values(var.lags) : (
        lag.native_vlan == null ? true : (lag.native_vlan >= 0 && lag.native_vlan <= 4095)
        ) && alltrue([for id in lag.trunk_vlans : id >= 0 && id <= 4095]
      )
    ])
    error_message = "Every native_vlan/trunk_vlans VLAN ID in var.lags must be between 0 and 4095 (the F5OS f5os_vlan valid range), or native_vlan must be null."
  }

  # Mirrors var.interfaces' VLAN-membership validation above.
  validation {
    condition = alltrue([
      for lag in values(var.lags) : (
        lag.native_vlan == null ? true : contains(values(var.vlans), lag.native_vlan)
        ) && alltrue([for id in lag.trunk_vlans : contains(values(var.vlans), id)]
      )
    ])
    error_message = "Every native_vlan/trunk_vlans VLAN ID in var.lags must also be present in var.vlans, so the referenced VLAN is actually created by f5os_vlan.from_iseries."
  }
}

# ---------------------------------------------------------------------------
# tenants is keyed by tenant name (a fresh identifier the operator
# assigns for the migrated tenant -- there is no source iSeries object
# this name is preserved from, unlike vlans/interfaces/lags, since a
# BIG-IP iSeries appliance has no equivalent of an F5OS tenant to name
# it after). Each entry mirrors f5os_tenant's own attributes directly:
# image_name (a tenant image already imported on the target device --
# see f5os_tenant_image and the "Upload BIG-IP tenant image to rSeries"
# story; f5os_tenant's own Create logic errors out if the named image's
# status is "not-present" on the device), cpu_cores/memory (sized per
<<<<<<< HEAD
# workload -- see
# https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/deploy-tenants-from-iseries
# for sizing
# guidance, since there is no i-Series field this maps from directly),
=======
# workload -- see docs/guides/deploy-tenants-from-iseries.md for sizing
# guidance, since there is no iSeries field this maps from directly),
>>>>>>> 2f900d0 (docs: update iSeries migration guides)
# vlans (VLAN ID/tags, not names -- looked up against the VLANs created
# by f5os_vlan.from_iseries in tenant.tf via local.vlan_name_by_tag, the
# same lookup interfaces.tf/lags.tf use), and mgmt_ip/mgmt_gateway/
# mgmt_prefix (the tenant's management-plane addressing).
#
# memory/max_nodes/mac_block_size/timeout/deployment_file are nullable:
# f5os_tenant itself treats a null memory as "auto-calculate from
# cpu_cores" (see calculateMemory in internal/provider/tenant_resource.go),
# a null max_nodes/mac_block_size as "use the device default" (max_nodes
# is additionally ignored entirely on F5OS versions before 2.0.0), a
# null timeout as "use the resource's own 360s default", and
# deployment_file is only meaningful (and only validated below) when
# type is "BIG-IP-Next".
#
# Populate this by hand -- unlike var.vlans/var.interfaces/var.lags,
# there is no Phase 1 JSON field to convert (see
<<<<<<< HEAD
# https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/deploy-tenants-from-iseries
# for why tenant sizing
# cannot be automatically extracted from the source i-Series device the
=======
# docs/guides/deploy-tenants-from-iseries.md for why tenant sizing
# cannot be automatically extracted from the source iSeries device the
>>>>>>> 2f900d0 (docs: update iSeries migration guides)
# way VLANs/interfaces/trunks are): cpu_cores/memory/virtual_disk_size
# need sizing-guidance input, and mgmt_ip/mgmt_gateway/mgmt_prefix need
# operator-assigned management-network addressing for the new tenant.
# vlans, however, are just the VLAN ID/tags already present in
# var.vlans -- copy the relevant values in from the same
# vlans.auto.tfvars.json Phase 3 already generated.
#
# Example:
#   tenants = {
#     "tenant1" = {
#       image_name        = "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle"
#       type              = "BIG-IP"
#       deployment_file   = null
#       cpu_cores         = 8
#       memory            = null
#       virtual_disk_size = 82
#       nodes             = [1]
#       max_nodes         = null
#       mac_block_size    = null
#       cryptos           = "enabled"
#       running_state     = "deployed"
#       timeout           = 600
#       mgmt_ip           = "10.100.100.26"
#       mgmt_gateway      = "10.100.100.1"
#       mgmt_prefix       = 24
#       vlans             = [100, 200]
#     }
#   }
# ---------------------------------------------------------------------------
variable "tenants" {
  description = "Map of tenant name (operator-assigned; no source iSeries equivalent) to its sizing (cpu_cores/memory/virtual_disk_size/nodes), image, management addressing (mgmt_ip/mgmt_gateway/mgmt_prefix), and migrated VLAN ID/tags, deployed to the F5OS (rSeries/Velos partition) target."
  type = map(object({
    image_name        = string
    type              = string
    deployment_file   = string
    cpu_cores         = number
    memory            = number
    virtual_disk_size = number
    nodes             = list(number)
    max_nodes         = number
    mac_block_size    = string
    cryptos           = string
    running_state     = string
    timeout           = number
    mgmt_ip           = string
    mgmt_gateway      = string
    mgmt_prefix       = number
    vlans             = list(number)
  }))
  default = {}

  # Mirrors f5os_tenant's own name schema documentation (see
  # internal/provider/tenant_resource.go): first character must be a
  # letter, only lowercase alphanumeric characters and hyphens allowed,
  # max 50 characters. Catching this here means a malformed name in
  # var.tenants (e.g. from a hand-edited tfvars file) fails at
  # `terraform plan` with a clear message instead of a runtime error
  # from the F5OS device during apply.
  validation {
    condition     = alltrue([for name in keys(var.tenants) : can(regex("^[a-z][a-z0-9-]{0,49}$", name))])
    error_message = "Every tenant name in var.tenants must start with a lowercase letter, contain only lowercase alphanumeric characters or hyphens, and not exceed 50 characters (the F5OS f5os_tenant valid name format)."
  }

  # Mirrors f5os_tenant's own type validator (see
  # internal/provider/tenant_resource.go): type is Optional+Computed
  # with a "BIG-IP" default, so null must remain valid here too --
  # same non-short-circuiting-|| pitfall as var.lags' mode/interval
  # validations above applies to contains(), so the ternary below only
  # calls contains() when type is non-null.
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.type == null ? true : contains(["BIG-IP", "BIG-IP-Next"], t.type)])
    error_message = "Every type in var.tenants must be \"BIG-IP\", \"BIG-IP-Next\", or null (to use the F5OS device default, \"BIG-IP\")."
  }

  # Mirrors f5os_tenant's own Create-time validation (see
  # internal/provider/tenant_resource.go): deployment_file is required
  # only when type is "BIG-IP-Next", and ignored otherwise -- tenant.tf
  # only passes it through in that case, matching this validation.
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.type == "BIG-IP-Next" ? t.deployment_file != null : true])
    error_message = "deployment_file must be set in var.tenants when type is \"BIG-IP-Next\" (matches f5os_tenant's own Create-time requirement)."
  }

  # Mirrors f5os_tenant's own running_state validator (see
  # internal/provider/tenant_resource.go). "deployed" is required to
  # satisfy this story's acceptance criteria (the tenant must actually
  # reach running state), but "configured" remains valid here too since
  # some migrations stage a tenant without starting it immediately.
  # running_state is Optional+Computed with a "configured" default, so
  # null must remain valid here too -- same non-short-circuiting-||
  # pitfall as above applies to contains().
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.running_state == null ? true : contains(["configured", "deployed"], t.running_state)])
    error_message = "Every running_state in var.tenants must be \"configured\", \"deployed\", or null (to use the F5OS device default, \"configured\")."
  }

  # Mirrors f5os_tenant's own cryptos validator (see
  # internal/provider/tenant_resource.go): cryptos is Optional+Computed
  # with an "enabled" default, so null must remain valid here too --
  # same non-short-circuiting-|| pitfall as above applies to contains().
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.cryptos == null ? true : contains(["enabled", "disabled"], t.cryptos)])
    error_message = "Every cryptos in var.tenants must be \"enabled\", \"disabled\", or null (to use the F5OS device default, \"enabled\")."
  }

  # Mirrors f5os_tenant's own mac_block_size validator (see
  # internal/provider/tenant_resource.go). Nullable: f5os_tenant treats
  # a null mac_block_size as "use the device default" -- same
  # non-short-circuiting-|| pitfall as var.lags' mode/interval
  # validations above applies to contains(), so the ternary below only
  # calls contains() when mac_block_size is non-null.
  validation {
    condition = alltrue([
      for t in values(var.tenants) : t.mac_block_size == null ? true : contains(["one", "small", "medium", "large"], t.mac_block_size)
    ])
    error_message = "Every mac_block_size in var.tenants must be \"one\", \"small\", \"medium\", \"large\", or null (to use the F5OS device default)."
  }

  # cpu_cores/virtual_disk_size are Required (non-nullable, unlike
  # memory/max_nodes/timeout below) on f5os_tenant itself, and must be
  # positive to mean anything -- catch a zero/negative value (e.g. an
  # unpopulated sizing field left at its Go zero value by a hand-edited
  # tfvars file) here instead of a confusing device-side rejection.
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.cpu_cores > 0 && t.virtual_disk_size > 0])
    error_message = "Every cpu_cores and virtual_disk_size in var.tenants must be a positive number (both are Required, non-nullable attributes on f5os_tenant)."
  }

  # Mirrors f5os_tenant's own memory MarkdownDescription/calculateMemory
  # behavior (see internal/provider/tenant_resource.go): null means "let
  # the provider auto-calculate from cpu_cores", so only validate when
  # non-null.
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.memory == null ? true : t.memory > 0])
    error_message = "Every memory in var.tenants must be a positive number of MB, or null (to let f5os_tenant auto-calculate it from cpu_cores)."
  }

  # Mirrors f5os_tenant's own max_nodes validator (see
  # internal/provider/tenant_resource.go): int64validator.AtLeast(1),
  # nullable (ignored entirely on F5OS versions before 2.0.0).
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.max_nodes == null ? true : t.max_nodes >= 1])
    error_message = "Every max_nodes in var.tenants must be at least 1, or null (to use the F5OS device default / omit on pre-2.0.0 devices)."
  }

  # Mirrors f5os_tenant's own timeout default (see
  # internal/provider/tenant_resource.go: 360s). Nullable here (tenant.tf
  # passes null through so the resource applies its own default);
  # non-null values must be positive. This story's acceptance criteria
  # ("tenant reaches running state within timeout") is satisfied by
  # whatever positive value the operator sets (or the resource's 360s
  # default if left null) -- this validation only guards against a
  # nonsensical zero/negative override, not a specific minimum, since
  # unlike f5os_tenant_image's large binary transfers a tenant's
  # deploy-to-running-state time is far more workload/config dependent.
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.timeout == null ? true : t.timeout > 0])
    error_message = "Every timeout in var.tenants must be a positive number of seconds, or null (to use f5os_tenant's own 360s default)."
  }

  # Mirrors var.interfaces'/var.lags' VLAN-range validation above.
  validation {
    condition     = alltrue([for t in values(var.tenants) : alltrue([for id in t.vlans : id >= 0 && id <= 4095])])
    error_message = "Every VLAN ID in var.tenants' vlans must be between 0 and 4095 (the F5OS f5os_vlan valid range)."
  }

  # Mirrors var.interfaces'/var.lags' VLAN-membership validation above.
  validation {
    condition     = alltrue([for t in values(var.tenants) : alltrue([for id in t.vlans : contains(values(var.vlans), id)])])
    error_message = "Every VLAN ID in var.tenants' vlans must also be present in var.vlans, so the referenced VLAN is actually created by f5os_vlan.from_iseries."
  }

  # mgmt_prefix is a CIDR prefix length; f5os_tenant itself does not
  # range-validate it (the device does, at apply time), but 0-32 is the
  # only meaningful range for an IPv4 prefix length, which is what
  # mgmt_ip/mgmt_gateway are documented and exemplified as throughout
  # this provider (see examples/resources/f5os_tenant/resource.tf).
  # Catching an out-of-range value here fails at `terraform plan`
  # instead of a runtime error from the F5OS device during apply.
  validation {
    condition     = alltrue([for t in values(var.tenants) : t.mgmt_prefix >= 0 && t.mgmt_prefix <= 32])
    error_message = "Every mgmt_prefix in var.tenants must be between 0 and 32 (a valid IPv4 CIDR prefix length)."
  }
}
```

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

output "created_vlans" {
  description = "Map of VLAN name to the F5OS-assigned resource id (the numeric VLAN ID as a string) for every VLAN created from the iSeries source."
  value       = { for name, vlan in f5os_vlan.from_iseries : name => vlan.id }
}

output "configured_interfaces" {
  description = "Map of F5OS (rSeries) interface name to its configured native_vlan/trunk_vlans/enabled state, for every interface configured from the iSeries source."
  value = {
    for name, iface in f5os_interface.from_iseries : name => {
      native_vlan = iface.native_vlan
      trunk_vlans = iface.trunk_vlans
      enabled     = iface.enabled
    }
  }
}

output "configured_lags" {
  description = "Map of LAG name to its configured lag_type/mode/interval/members/native_vlan/trunk_vlans, for every LAG configured from the iSeries source."
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

output "deployed_tenants" {
  description = "Map of tenant name to its running_state/status/sizing/mgmt_ip/vlans, for every tenant deployed from the iSeries source."
  value = {
    for name, tenant in f5os_tenant.from_iseries : name => {
      running_state     = tenant.running_state
      status            = tenant.status
      cpu_cores         = tenant.cpu_cores
      memory            = tenant.memory
      virtual_disk_size = tenant.virtual_disk_size
      mgmt_ip           = tenant.mgmt_ip
      mgmt_gateway      = tenant.mgmt_gateway
      mgmt_prefix       = tenant.mgmt_prefix
      vlans             = tenant.vlans
    }
  }
}
```

## Populating `var.lags` from a Phase 1 extraction

Run `terraform-provider-bigip`'s `scripts/extract-sys-settings.sh`
against the source iSeries device first (see that provider's
[extraction
guide](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings)),
then convert its output with this repo's `scripts/lags-from-iseries.sh`,
alongside `scripts/vlans-from-iseries.sh` and
`scripts/interfaces-from-iseries.sh` from Phases 3 and 4:

```sh
./scripts/vlans-from-iseries.sh extracted-sys-settings.json \
  examples/migration/vlans-from-iseries/vlans.auto.tfvars.json
./scripts/interfaces-from-iseries.sh extracted-sys-settings.json \
  examples/migration/vlans-from-iseries/interfaces.auto.tfvars.json
./scripts/lags-from-iseries.sh extracted-sys-settings.json \
  examples/migration/vlans-from-iseries/lags.auto.tfvars.json
```

Terraform automatically loads any `*.auto.tfvars.json` file found in the
working directory, so no explicit `-var-file` flag is needed as long as
all three generated files are placed alongside `main.tf` as shown above.
See `terraform.tfvars.json.example` in the same directory for the
expected shape of all three variables together:

```json
{
  "vlans": {
    "extracted-external": 100,
    "extracted-internal": 101,
    "extracted-trunked": 200
  },
  "interfaces": {
    "1.0": {
      "native_vlan": 100,
      "trunk_vlans": [],
      "enabled": true
    },
    "2.0": {
      "native_vlan": 101,
      "trunk_vlans": [],
      "enabled": true
    }
  },
  "lags": {
    "lag1": {
      "lag_type": "LACP",
      "mode": "ACTIVE",
      "interval": "SLOW",
      "members": ["3.0", "4.0"],
      "native_vlan": null,
      "trunk_vlans": [200]
    }
  },
  "tenants": {
    "tenant1": {
      "image_name": "BIGIP-17.1.0-0.0.16.ALL-F5OS.qcow2.zip.bundle",
      "type": "BIG-IP",
      "deployment_file": null,
      "cpu_cores": 8,
      "memory": null,
      "virtual_disk_size": 82,
      "nodes": [1],
      "max_nodes": null,
      "mac_block_size": null,
      "cryptos": "enabled",
      "running_state": "deployed",
      "timeout": 600,
      "mgmt_ip": "10.100.100.26",
      "mgmt_gateway": "10.100.100.1",
      "mgmt_prefix": 24,
      "vlans": [100, 200]
    }
  }
}
```

Note `native_vlan`/`trunk_vlans` in this file are VLAN ID/tags, matched
against `var.vlans`' values (not names) -- `lags.tf` resolves each one
back to the corresponding `f5os_vlan.from_iseries` resource instance
internally, exactly like `interfaces.tf` does in Phase 4. `var.lags`
validates this the same way `var.interfaces` does: one block rejects any
`native_vlan`/`trunk_vlans` ID that doesn't also appear in `var.vlans`;
another rejects any `members` entry that isn't a validly-shaped F5OS
interface name.

`var.lags` additionally validates the LACP-specific fields, mirroring
`f5os_lag`'s own schema validators and `ValidateConfig` logic (see
`internal/provider/lag_resource.go`): `lag_type` must be `"LACP"` or
`"STATIC"`; `mode`/`interval` must both be `null` when `lag_type` is
`"STATIC"`; and when `lag_type` is `"LACP"`, `mode` must be `"ACTIVE"` or
`"PASSIVE"` and `interval` must be `"SLOW"` or `"FAST"`. Each of these
fails at `terraform plan` time with a clear message instead of a
runtime error from the F5OS device during apply.

### Unmapped member interface names

Identical to Phase 4's interface name mapping: a trunk member name that
doesn't match the expected TMOS `<blade>.<port>` pattern is passed
through into the output `members` list unchanged (so it isn't silently
dropped) and reported to stderr for manual review -- it is not a valid
rSeries name and must be renamed by hand before applying (`var.lags`'
member-name validation will otherwise reject it at `terraform plan`
time anyway).

### Member name collisions within a LAG

Dropping the blade number (see [Phase 4's interface name
mapping](configure-interfaces-from-iseries.html#interface-name-mapping-i-series-11-to-rseries-10))
can make two *different* TMOS member interfaces on the same trunk
collapse to the same rSeries name on a multi-blade source. Unlike Phase
4's `interfaces` map (where a collision drops one physical interface's
configuration entirely), here the colliding names are de-duplicated
into a single entry in that LAG's `members` list, since `f5os_lag`'s
`members` is a set of interface names, not a map keyed by name -- there
is nothing to silently overwrite. The script still reports every such
collision to stderr, since it signals the blade-drop mapping's
single-blade assumption doesn't hold for the source device.

### Ambiguous native VLANs

A trunk with more than one *untagged* VLAN membership on the source
device is an invalid TMOS configuration this script can't silently
resolve into a single `native_vlan` -- those are left with
`native_vlan: null` in the output and reported to stderr for manual
review, identical to Phase 4's [Ambiguous native
VLANs](configure-interfaces-from-iseries.html#ambiguous-native-vlans)
handling for physical interfaces.

## Applying

```sh
cd examples/migration/vlans-from-iseries
terraform init
terraform apply
```

Update the `provider "f5os"` block's `host`/`username`/`password` (or use
the corresponding `F5OS_HOST`/`F5OS_USERNAME`/`F5OS_PASSWORD` environment
variables) to point at the target rSeries appliance before applying.
`f5os_lag` (like `f5os_vlan`/`f5os_interface`) rejects the "Velos
Controller" platform type -- this configuration targets an rSeries
appliance or a Velos chassis partition, not a Velos controller.

Every member interface listed in a LAG's `members` must exist and have
no VLAN configuration of its own before it can join the LAG (see
`f5os_lag`'s `members` documentation) -- if Phase 4's `interfaces.tf`
also configures one of the same physical interfaces directly (with a
`native_vlan`/`trunk_vlans` of its own), remove that interface from
`var.interfaces` before adding it to a LAG's `members` here, or the
apply will conflict.

## Related migration guides

- [Inventorying TMOS version and hardware](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/inventory-tmos-version) (Phase 0, `terraform-provider-bigip`)
- [Extracting iSeries system settings](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings) (Phase 1, `terraform-provider-bigip`)
- [Interface and trunk naming: TMOS (iSeries) vs F5OS (rSeries/VELOS)](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping) (`terraform-provider-bigip`)
- [Generating and downloading a UCS backup](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/generate-ucs-backup) (Phase 2, `terraform-provider-bigip`)
- [Creating VLANs on F5OS from discovered iSeries configuration](create-vlans-from-iseries.html) (Phase 3, this repo) -- prerequisite for this guide.
- [Configuring F5OS interfaces from discovered iSeries configuration](configure-interfaces-from-iseries.html) (Phase 4, this repo) -- prerequisite for this guide; not required to be applied first (LAG members do not need a `f5os_interface` resource of their own), but conflicts if the same physical interface is configured in both places (see "Applying" above).
- [Applying a license to F5OS as part of an iSeries migration](apply-license-from-iseries.html) (this repo) -- independent of this phase; can be applied before, after, or in parallel.
