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
