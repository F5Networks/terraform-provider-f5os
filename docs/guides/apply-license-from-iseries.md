---
page_title: "Applying a license to F5OS as part of an i-Series migration"
description: |-
  How to use examples/migration/license-from-iseries to activate the F5OS platform license on an rSeries appliance or Velos chassis partition using a new registration key obtained from F5, and how to verify activation succeeded.
---

# Applying a license to F5OS as part of an i-Series migration

`examples/migration/license-from-iseries` is a licensing phase of an
i-Series -> r-Series (F5OS) migration workflow, independent of (and can
be applied before, after, or in parallel with) the VLAN creation
workflow in [Creating VLANs on F5OS from discovered i-Series
configuration](create-vlans-from-iseries.html) and the [system-settings
workflow](configure-system-settings-from-iseries.html): it activates the
target rSeries appliance's (or Velos chassis partition's) platform
license via [`f5os_license`](../resources/license.html), using a
registration key obtained from F5 specifically for that device.

For the full cross-repo phase sequence, see the [overall i-Series to
r-Series migration flow](iseries-to-rseries-migration-flow.html).

## i-Series license keys are not valid on r-Series

Every other phase of this migration workflow converts data discovered
on the source i-Series device into F5OS configuration
(`scripts/vlans-from-iseries.sh`, `scripts/interfaces-from-iseries.sh`,
`scripts/lags-from-iseries.sh`, `scripts/system-settings-from-iseries.sh`).
Licensing is different: an i-Series device's TMOS registration key is
tied to that specific BIG-IP platform/serial number and its licensed
module set, and **cannot be reused, converted, or migrated onto an
r-Series appliance or Velos chassis partition** -- F5OS uses an entirely
separate licensing SKU and activation record from TMOS, keyed to the
r-Series/Velos hardware's own serial number/base registration key, not
the i-Series device being replaced.

There is no `scripts/license-from-iseries.sh` conversion script in this
repo, and there will not be one: this is a hard prerequisite, not a
migration step with source data to transform.

**Before applying this configuration, obtain a new base registration
key (and, if applicable, add-on keys) from F5 for the target r-Series
appliance or Velos chassis partition.** Contact F5 support or your F5
account team with the device's serial number if you do not already
have one. Applying `f5os_license` with an i-Series/TMOS key, a
placeholder value, or a key issued for a different device fails at
`terraform apply` time -- the device validates the key against F5's
license activation service during the underlying EULA/install calls
(see `f5os_license`'s own documentation and
`internal/provider/license_resource.go`), not merely a local format
check, so a wrong or reused key is rejected with an explicit device-side
error rather than silently accepted.

## Example Usage

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# Applies the F5OS platform license to an rSeries appliance (or Velos
# chassis partition) via f5os_license, using a registration key obtained
# from F5 specifically for this device.
#
# This is a licensing phase of an i-Series -> r-Series (F5OS) migration
# workflow, independent of (and can be applied in parallel with, or
# before) the VLAN creation workflow in
# examples/migration/vlans-from-iseries and the system-settings workflow
# in examples/migration/system-settings-from-iseries: unlike those
# phases, there is no source data to extract or convert from the
# i-Series device -- the registration key is a brand-new credential
# issued by F5 for this specific r-Series/Velos target, obtained
# out-of-band before running `terraform apply` here. See
# docs/guides/apply-license-from-iseries.md for the full workflow,
# including why i-Series/TMOS license keys cannot be reused and how to
# verify activation succeeded.
#
# terraform.tfvars.json.example shows the expected shape for every
# variable. Unlike the other migration phases in this repo, there is no
# conversion script that produces this file automatically -- populate it
# by hand with the registration key(s) obtained from F5.
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

resource "f5os_license" "from_iseries" {
  registration_key = var.registration_key
  addon_keys       = length(var.addon_keys) > 0 ? var.addon_keys : null
  license_server   = var.license_server
}
```

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# There is no i-Series source field for any of these variables -- see
# docs/guides/apply-license-from-iseries.md's "i-Series license keys are
# not valid on r-Series" section. registration_key/addon_keys are new
# registration keys obtained from F5 specifically for the target
# r-Series appliance or Velos chassis; the i-Series device's own TMOS
# license key(s) are never valid input here and are not read from the
# Phase 1 extraction at all.
#
# Example:
#   registration_key = "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZ"
#   addon_keys        = ["NNNWWWW-9PPPPKK"]
# ---------------------------------------------------------------------------
variable "registration_key" {
  description = "The base registration key obtained from F5 for the target r-Series appliance or Velos chassis partition, applied via f5os_license.registration_key. This is a NEW key issued for this F5OS device -- an i-Series/TMOS license key is never valid here (see docs/guides/apply-license-from-iseries.md)."
  type        = string
  sensitive   = true

  validation {
    condition     = length(trimspace(var.registration_key)) > 0
    error_message = "var.registration_key must not be empty -- obtain a base registration key from F5 for the target r-Series/Velos device before applying this configuration."
  }
}

variable "addon_keys" {
  description = "Optional list of additional (add-on) registration keys from F5 to apply alongside var.registration_key, via f5os_license.addon_keys. Leave empty if the target device only has a base license."
  type        = list(string)
  default     = []
  sensitive   = true
}

variable "license_server" {
  description = "Optional license server URL to pass to f5os_license.license_server. Leave null to use the device's default (F5's public licensing service)."
  type        = string
  default     = null
}
```

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

output "license_id" {
  description = "Terraform-synthetic ID of the configured f5os_license resource. A non-null value here means Create completed without the device rejecting the registration key -- it does NOT by itself prove the license is active; see docs/guides/apply-license-from-iseries.md's \"Verifying license activation\" section for a device-side check."
  value       = f5os_license.from_iseries.id
}
```

## Populating the variables

Unlike the other phases in this migration workflow, there is no
extraction script that populates these variables from the source
i-Series device -- `var.registration_key`/`var.addon_keys` come directly
from the new key(s) issued by F5 for this r-Series/Velos target (see
above). Copy `terraform.tfvars.json.example` in the same directory to a
`*.auto.tfvars.json` file (Terraform loads any such file automatically)
and fill in the real key(s):

```json
{
  "registration_key": "W9XXX-8YYYZ-8KKK7-7PPP2-ZZZZZZ",
  "addon_keys": [],
  "license_server": null
}
```

`var.registration_key`/`var.addon_keys` are marked `sensitive` in
`variables.tf`, matching `f5os_license.registration_key`/`.addon_keys`'
own `Sensitive` schema attributes -- Terraform masks them from `plan`/
`apply` output and from this configuration's state file display, but
they are still stored in plaintext in the Terraform state file itself
(same caveat as any other `Sensitive` attribute); protect
`terraform.tfstate` accordingly.

## Applying

```sh
cd examples/migration/license-from-iseries
terraform init
terraform apply
```

Update the `provider "f5os"` block's `host`/`username`/`password` (or
use the corresponding `F5OS_HOST`/`F5OS_USERNAME`/`F5OS_PASSWORD`
environment variables) to point at the target rSeries appliance or
Velos chassis partition before applying.

~> **NOTE:** `f5os_license`'s `terraform destroy` is a no-op -- it does
not attempt to deactivate or revoke the license on the device (there is
no supported "unlicense" operation). Destroying this resource only
removes it from Terraform state; the device remains licensed. See
`f5os_license`'s own documentation for details.

## Verifying license activation

A successful `terraform apply` means the device accepted the
registration key(s) at the time `f5os_license.from_iseries` was
created -- it does not, by itself, prove licensing remains active
afterward (for example if the key were later revoked). Confirm
activation directly against the device's `f5-system-licensing:licensing`
RESTCONF endpoint:

```sh
curl -sku "$F5OS_USERNAME:$F5OS_PASSWORD" \
  "https://$F5OS_HOST:8888/restconf/data/openconfig-system:system/f5-system-licensing:licensing" \
  | jq '.["f5-system-licensing:licensing"].state.license'
```

A licensed device reports `"Licensed"` for
`.state.license`; an unlicensed or expired device reports a different
value (for example `"Not Licensed"`) -- if you see anything other than
`"Licensed"` after `terraform apply` completes successfully, re-check
the registration key with F5 rather than assuming the Terraform run
alone guarantees activation. The same endpoint also reports
`.state.registration-key.base`, which should match
`var.registration_key`, and `.state.raw-license`, the full raw license
text F5's activation service returned.

This is the same verification approach
`TestAccLicenseResource` (`internal/provider/license_resource_test.go`)
uses internally -- it queries this endpoint via a side-channel
`f5osclient` session rather than trusting Terraform state, specifically
because `f5os_license`'s `Read` only refreshes `registration_key` from
the device (not `addon_keys`/`license_server`), and because asserting a
sensitive value via `TestCheckResourceAttr` would echo it into test
output on any mismatch.

## Related migration guides

- [Inventorying TMOS version and hardware](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/inventory-tmos-version) (Phase 0, `terraform-provider-bigip`)
- [Extracting i-Series system settings](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings) (Phase 1, `terraform-provider-bigip`)
- [Creating VLANs on F5OS from discovered i-Series configuration](create-vlans-from-iseries.html) (Phase 3, this repo) -- independent of this phase; can be applied before, after, or in parallel.
- [Configuring system settings on F5OS from discovered i-Series configuration](configure-system-settings-from-iseries.html) (this repo) -- also independent of this phase.
- [Generating and downloading a UCS backup](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/generate-ucs-backup) (Phase 2, `terraform-provider-bigip`)
