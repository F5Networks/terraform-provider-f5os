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
