---
page_title: "Configuring F5OS interfaces from discovered i-Series configuration"
description: |-
  How to use examples/migration/vlans-from-iseries/interfaces.tf and scripts/interfaces-from-iseries.sh to assign native/trunk VLANs and enabled state to f5os_interface resources on an rSeries F5OS target, matching the interface/VLAN layout discovered on a source BIG-IP i-Series device.
---

# Configuring F5OS interfaces from discovered i-Series configuration

`examples/migration/vlans-from-iseries/interfaces.tf` is Phase 4 of an
i-Series -> r-Series (F5OS) migration workflow: it configures one
`f5os_interface` resource per physical interface discovered on a source
BIG-IP i-Series device, assigning the same native VLAN (untagged) and
trunk VLANs (tagged) membership and enabled state as the source device,
via a single `for_each` over an `interfaces` variable map.
`scripts/interfaces-from-iseries.sh` converts the interface/VLAN
membership portion of the `extracted-sys-settings.json` produced by
[`terraform-provider-bigip`'s `scripts/extract-sys-settings.sh`](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings)
(that provider's Phase 1) into the `interfaces` map this configuration
expects, including the TMOS -> rSeries interface name mapping described
below.

This phase lives in the **same** `examples/migration/vlans-from-iseries`
directory, and therefore the same Terraform state, as [Phase 3, VLAN
creation](create-vlans-from-iseries.html) -- not a separate directory --
specifically so `f5os_interface.from_iseries` can hold a real Terraform
reference to `f5os_vlan.from_iseries`, making VLAN creation a graph
dependency Terraform itself enforces rather than just an operational
instruction to run one step before the other.

## Interface name mapping: i-Series (`1.1`) to rSeries (`1.0`)

TMOS (i-Series) names physical interfaces `<blade>.<port>` (e.g. `1.1`,
`1.2`); single-appliance i-Series/VE platforms are always blade `1`.
F5OS-A (rSeries) has no blade concept at all: a single rSeries appliance
maps to `<port>.0`, with the blade number **dropped entirely**, not just
reformatted. `scripts/interfaces-from-iseries.sh` performs this mapping
automatically -- `1.1` becomes `1.0`, `1.2` becomes `2.0`, and so on --
see [`terraform-provider-bigip`'s interface/trunk naming
guide](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping)
for the full naming-convention reference, including the VELOS
(`<blade>/<port>.<subport>`) form this script does **not** produce (a
VELOS chassis partition target needs a different mapping, applied
manually).

The dedicated management interface (`mgmt`) is excluded entirely: it has
no rSeries data-plane equivalent, is not part of TMOS's numbered
`<blade>.<port>` scheme, and is never a member of a VLAN's tagged/
untagged interface list on the source device in the first place, so it
never needs a native/trunk VLAN translation.

Any interface name that doesn't match the expected TMOS `<blade>.<port>`
pattern is passed through into the output map unchanged (so it isn't
silently dropped) and reported to stderr for manual review -- it is not
a valid rSeries name and must be renamed by hand before applying.

Dropping the blade number this way only produces a 1:1 mapping when the
source is genuinely single-blade (true for i-Series/VE, per the
`mgmt`-only-management-interface note above). On a multi-blade source,
two TMOS interfaces on different blades but the same port (e.g. `1.1`
and `2.1`) collapse to the same rSeries name (`1.0`); the script detects
this and reports it to stderr rather than silently keeping only one of
the colliding interfaces' `native_vlan`/`trunk_vlans`/`enabled` values --
see "rSeries name collisions" below.

## Trunks (LAGs) are out of scope

F5OS represents link aggregation as a separate `f5os_lag` resource, not
`f5os_interface` -- a TMOS trunk name has no entry in
`bigip_net_interfaces` (it isn't a physical interface) and is therefore
never emitted into this script's `interfaces` output map. Any such
trunk's VLAN membership is still surfaced separately (printed to stderr,
not written to any file) so it isn't silently lost; translate it to
`f5os_lag`'s `native_vlan`/`trunk_vlans` manually -- see
[`terraform-provider-bigip`'s interface/trunk naming
guide](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping#trunk-lag-mapping-is-not-11)
for that mapping.

## Example Usage

```terraform
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
# via local.vlan_name_by_tag (defined in main.tf, alongside
# f5os_vlan.from_iseries itself, so this file has no dependency on
# lags.tf or vice versa), so Terraform infers a real dependency on
# f5os_vlan.from_iseries from the resource reference alone; the explicit
# depends_on below is additional, deliberate documentation of that same
# intent (and a safety net if a future edit removes the reference
# without noticing the ordering requirement it was providing).
# ---------------------------------------------------------------------------

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
```

The `interfaces` variable definition (in `variables.tf`, alongside the
`vlans` variable from Phase 3):

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# vlans is keyed by VLAN name (preserved from the source i-Series device)
# and maps to the numeric VLAN ID/tag (also preserved from the source
# device). Populate this from the i-Series `data.bigip_net_vlans` output
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
  description = "Map of VLAN name (from the source i-Series device) to VLAN ID/tag (also preserved from the source device) to create on the F5OS (rSeries/Velos partition) target."
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
    error_message = "Every value in var.vlans must be unique -- two VLAN names cannot share the same VLAN ID/tag. This configuration has no partition/route-domain concept to disambiguate them (unlike the source i-Series device, where VLAN IDs can repeat across route domains): rename or merge the colliding VLANs before applying, since interfaces.tf's tag-to-name lookup cannot distinguish them either."
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
# Populate this from i-Series `bigip_net_interfaces`/`bigip_net_vlans`
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
  description = "Map of F5OS (rSeries) interface name to its native/trunk VLAN assignment and enabled state, mapped from the source i-Series device's interface/VLAN layout."
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
# lags is keyed by LAG name (preserved verbatim from the source i-Series
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
# Populate this from i-Series `bigip_net_trunks`/`bigip_net_vlans`
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
  description = "Map of LAG name (from the source i-Series trunk name) to its type/LACP settings, member interfaces (already mapped to F5OS names), and native/trunk VLAN assignment, mapped from the source i-Series device's trunk/VLAN layout."
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
```

```terraform
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
```

## Populating `var.interfaces` from a Phase 1 extraction

Run `terraform-provider-bigip`'s `scripts/extract-sys-settings.sh`
against the source i-Series device first (see that provider's
[extraction
guide](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings)),
then convert its output with this repo's
`scripts/interfaces-from-iseries.sh`, alongside
`scripts/vlans-from-iseries.sh` from Phase 3:

```sh
./scripts/vlans-from-iseries.sh extracted-sys-settings.json \
  examples/migration/vlans-from-iseries/vlans.auto.tfvars.json
./scripts/interfaces-from-iseries.sh extracted-sys-settings.json \
  examples/migration/vlans-from-iseries/interfaces.auto.tfvars.json
```

Terraform automatically loads any `*.auto.tfvars.json` file found in the
working directory, so no explicit `-var-file` flag is needed as long as
both generated files are placed alongside `main.tf` as shown above. See
`terraform.tfvars.json.example` in the same directory for the expected
shape of both variables together:

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
  }
}
```

Note `native_vlan`/`trunk_vlans` in this file are VLAN ID/tags, matched
against `var.vlans`' values (not names) -- `interfaces.tf` resolves each
one back to the corresponding `f5os_vlan.from_iseries` resource instance
internally. `var.interfaces` validates this automatically: one
`validation` block rejects any `native_vlan`/`trunk_vlans` ID that
doesn't also appear in `var.vlans`, surfacing a missing-VLAN mistake at
`terraform plan` time rather than as a confusing runtime error from the
F5OS device; another rejects any map key that isn't a validly-shaped
F5OS interface name (`<port>.0` for rSeries, `<blade>/<port>.0` for
VELOS partitions), catching an unmapped or otherwise malformed TMOS name
that `interfaces-from-iseries.sh` passed through unchanged (see
"Interface name mapping" above).

If the source extraction is missing VLAN membership data entirely
(`bigip_net_vlans` was omitted -- see [Registry
fallback](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings#registry-fallback)
in the Phase 1 extraction guide), `interfaces-from-iseries.sh` prints a
warning to stderr and every interface is emitted with `native_vlan:
null` and an empty `trunk_vlans` -- this looks identical to "no VLAN
membership on the source device" in the output file, so check for that
warning before assuming the generated `interfaces` map accurately
reflects the source device.

### Ambiguous native VLANs

A physical interface with more than one *untagged* VLAN membership on
the source device is an invalid TMOS configuration this script can't
silently resolve into a single `native_vlan` -- those are left with
`native_vlan: null` in the output and reported to stderr for manual
review, matching `terraform-provider-bigip`'s
`extract-sys-settings.sh` `interface_vlans` derivation behavior (see
that repo's [extraction guide](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings#interface-vlan-mapping)).

### rSeries name collisions

If two (or more) distinct TMOS interface names map to the same rSeries
name after the blade number is dropped (see "Interface name mapping"
above), only one of them ends up in the output map -- the others'
`native_vlan`/`trunk_vlans`/`enabled` values are dropped entirely. This
is reported to stderr with the offending rSeries name and every
colliding TMOS name so it can be resolved manually (there is no way for
the script to guess which TMOS interface's configuration should "win").
This should not occur against a genuine single-blade i-Series/VE
source; it is a signal the source device or extraction isn't what this
mapping assumes.

### Duplicate VLAN tags in `var.vlans`

`interfaces.tf` builds a VLAN-tag-to-name reverse lookup to resolve
`native_vlan`/`trunk_vlans` back to the `f5os_vlan.from_iseries`
resource that owns each tag (see [Phase 3's VLAN name
compatibility](create-vlans-from-iseries.html#vlan-name-compatibility)).
Because a real TMOS source can have two VLAN names sharing the same
tag (VLAN IDs are scoped per route domain/partition on TMOS, unlike
this Terraform configuration), `var.vlans` includes a validation
rejecting duplicate tags outright, with an error explaining the
conflict -- this configuration has no route-domain/partition to
disambiguate them, so any such collision must be resolved (renamed or
merged) in `var.vlans` before applying, not just in `var.interfaces`.

## Applying

```sh
cd examples/migration/vlans-from-iseries
terraform init
terraform apply
```

Update the `provider "f5os"` block's `host`/`username`/`password` (or use
the corresponding `F5OS_HOST`/`F5OS_USERNAME`/`F5OS_PASSWORD` environment
variables) to point at the target rSeries appliance before applying.
`f5os_interface` (like `f5os_vlan`) rejects the "Velos Controller"
platform type -- this configuration targets an rSeries appliance or a
Velos chassis partition, not a Velos controller.

## Related migration guides

- [Inventorying TMOS version and hardware](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/inventory-tmos-version) (Phase 0, `terraform-provider-bigip`)
- [Extracting i-Series system settings](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings) (Phase 1, `terraform-provider-bigip`)
- [Interface and trunk naming: TMOS (i-Series) vs F5OS (r-Series/VELOS)](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping) (`terraform-provider-bigip`)
- [Generating and downloading a UCS backup](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/generate-ucs-backup) (Phase 2, `terraform-provider-bigip`)
- [Creating VLANs on F5OS from discovered i-Series configuration](create-vlans-from-iseries.html) (Phase 3, this repo) -- prerequisite for this guide.
- [Configuring F5OS LAGs from discovered i-Series configuration](configure-lags-from-iseries.html) (Phase 5, this repo) -- for trunk (LAG) member interfaces, not covered by this guide (a TMOS trunk name has no `bigip_net_interfaces` entry of its own).
- [Applying a license to F5OS as part of an i-Series migration](apply-license-from-iseries.html) (this repo) -- independent of this phase; can be applied before, after, or in parallel.
