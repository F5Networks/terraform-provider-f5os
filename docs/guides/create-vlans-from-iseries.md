---
page_title: "Creating VLANs on F5OS from discovered i-Series configuration"
description: |-
  How to use examples/migration/vlans-from-iseries and scripts/vlans-from-iseries.sh to create the f5os_vlan resources needed on an rSeries/Velos partition F5OS target from VLANs discovered on a source BIG-IP i-Series device.
---

# Creating VLANs on F5OS from discovered i-Series configuration

`examples/migration/vlans-from-iseries` is Phase 3 of an i-Series ->
r-Series (F5OS) migration workflow: it creates one `f5os_vlan` resource
per VLAN discovered on a source BIG-IP i-Series device, preserving both
the VLAN ID/tag and the name from that device, via a single `for_each`
over a `vlans` variable map. `scripts/vlans-from-iseries.sh` converts the
VLAN portion of the `extracted-sys-settings.json` produced by
[`terraform-provider-bigip`'s `scripts/extract-sys-settings.sh`](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings)
(that provider's Phase 1) into the `vlans` map this configuration expects.

## Why `for_each` over a variable map instead of one resource block per VLAN

The number and identity of VLANs on a source i-Series device is not known
ahead of time -- it varies per device and is only known after running the
Phase 1 extraction. A `for_each` over a `map(number)` variable (VLAN name
-> VLAN ID) lets the same configuration create any number of VLANs
without hand-writing a resource block per VLAN, and keys each resource
instance by VLAN name so `terraform plan` diffs stay stable and readable
even as VLANs are added or removed on the source device between runs.

## Example Usage

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# Creates every VLAN discovered on a source BIG-IP i-Series device on the
# F5OS (rSeries appliance or Velos chassis partition) layer, preserving
# both the VLAN ID/tag and the name from the source device.
#
# This is Phase 3 of an i-Series -> r-Series (F5OS) migration workflow: it
# consumes the VLAN portion of the JSON produced by
# terraform-provider-bigip's `scripts/extract-sys-settings.sh` (Phase 1),
# converted into the `vlans` map below via `scripts/vlans-from-iseries.sh`
# in this repo. See docs/guides/create-vlans-from-iseries.md for the full
# workflow.
#
# terraform.tfvars.json.example shows the expected shape for `vlans`;
# `terraform apply -var-file=vlans.auto.tfvars.json` (or any *.auto.tfvars
# file) supplies the real, discovered values.
# ---------------------------------------------------------------------------

terraform {
  required_providers {
    f5os = {
      source = "f5networks/f5os"
    }
  }
}

provider "f5os" {
  host     = "https://192.0.2.1"
  username = "admin"
  password = "secret"
}

resource "f5os_vlan" "from_iseries" {
  for_each = var.vlans

  name    = each.key
  vlan_id = each.value
}
```

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
```

## Populating `var.vlans` from a Phase 1 extraction

Run `terraform-provider-bigip`'s `scripts/extract-sys-settings.sh` against
the source i-Series device first (see that provider's [extraction
guide](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings)),
then convert its output with this repo's `scripts/vlans-from-iseries.sh`:

```sh
./scripts/vlans-from-iseries.sh extracted-sys-settings.json \
  examples/migration/vlans-from-iseries/vlans.auto.tfvars.json
```

Terraform automatically loads any `*.auto.tfvars.json` file found in the
working directory, so no explicit `-var-file` flag is needed as long as
the generated file is placed alongside `main.tf` as shown above. See
`terraform.tfvars.json.example` in the same directory for the expected
shape:

```json
{
  "vlans": {
    "extracted-external": 100,
    "extracted-internal": 101
  }
}
```

Both the VLAN name and VLAN ID/tag are preserved verbatim from the source
device -- this script performs no renaming or renumbering.

### VLAN name compatibility

`f5os_vlan`'s `name` attribute has no partition concept, must start
with a letter, allows only alphanumeric characters plus `.`, `,`, `-`,
and `_`, and has a 58-character limit (see
`internal/provider/vlan_resource.go`'s schema description), while TMOS
VLAN names are commonly full paths (e.g. `/Common/vlan100`).
`vlans-from-iseries.sh` strips a leading partition path down to the
final path segment (`/Common/vlan100` -> `vlan100`) before emitting the
`vlans` map. Any name that still doesn't start with a letter, still
contains a character outside that allowed set (for example a space or
`!`), or still exceeds 58 characters after stripping, is emitted as-is
with a warning printed to stderr so it can be reviewed and fixed
manually before applying -- `var.vlans`' validation (see below) also
catches this at `terraform plan` time if left unfixed.

Stripping the partition prefix can also collapse two distinct source
VLANs into the same name -- for example, `/Common/vlan100` and
`/Partition2/vlan100` both become `vlan100`. Since `vlans` is a flat map
keyed by name, only one of the colliding VLANs survives in the output
(the last one seen wins); the others are silently dropped from
migration unless caught. The script detects this and prints a warning
naming every colliding source VLAN and which one was kept, so a VLAN
missing from the F5OS target isn't a surprise -- rename the losing
VLAN(s) in the output `vlans` map manually if all of them need to be
migrated.

### Duplicate VLAN tags

TMOS scopes VLAN IDs per route domain/partition, so two VLANs with
different names can legitimately share the same numeric tag on the
source i-Series device -- but F5OS has no equivalent scoping and
requires VLAN IDs to be unique. `vlans-from-iseries.sh` detects this
and prints a warning naming every VLAN sharing a duplicate tag before
applying; rename or merge the colliding VLANs manually if this occurs.

## Applying

```sh
cd examples/migration/vlans-from-iseries
terraform init
terraform apply
```

Update the `provider "f5os"` block's `host`/`username`/`password` (or use
the corresponding `F5OS_HOST`/`F5OS_USERNAME`/`F5OS_PASSWORD` environment
variables) to point at the target rSeries appliance or Velos chassis
partition before applying.

## Related migration guides

- [Inventorying TMOS version and hardware](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/inventory-tmos-version) (Phase 0, `terraform-provider-bigip`)
- [Extracting i-Series system settings](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings) (Phase 1, `terraform-provider-bigip`)
- [Interface and trunk naming: TMOS (i-Series) vs F5OS (r-Series/VELOS)](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping) (`terraform-provider-bigip`) -- for mapping `f5os_interface`/`f5os_lag` `native_vlan`/`trunk_vlans` once the VLANs created here exist on the F5OS target.
- [Generating and downloading a UCS backup](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/generate-ucs-backup) (Phase 2, `terraform-provider-bigip`)
