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
# workflow. interfaces.tf in this same directory (Phase 4) configures each
# F5OS interface's native/trunk VLAN assignment against the VLANs created
# here, and depends on this file's f5os_vlan resources completing first.
#
# terraform.tfvars.json.example shows the expected shape for `vlans` and
# `interfaces`; any *.auto.tfvars(.json) file placed in this directory
# (e.g. the output of scripts/vlans-from-iseries.sh /
# scripts/interfaces-from-iseries.sh) is loaded automatically by
# `terraform apply` with no extra flags needed.
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

locals {
  # Reverses var.vlans (name -> tag) into (tag -> name) so a VLAN
  # ID/tag referenced by var.interfaces/var.lags can be resolved back
  # to the f5os_vlan.from_iseries resource instance that owns it --
  # this is what makes the dependency on VLAN creation a real
  # Terraform reference rather than two configurations that merely
  # happen to agree on the same numeric literals.
  #
  # Declared here (alongside f5os_vlan.from_iseries itself) rather than
  # in interfaces.tf or lags.tf, since both Phase 4 (interfaces.tf) and
  # Phase 5 (lags.tf) reference it independently of each other -- an
  # operator migrating only trunks (or only interfaces) can delete the
  # other phase's file entirely without this local becoming undeclared.
  vlan_name_by_tag = { for name, tag in var.vlans : tag => name }
}
