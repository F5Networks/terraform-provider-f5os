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
