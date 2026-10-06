/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# There is no i-Series source field for any of these variables -- see
# https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/apply-license-from-iseries
# "i-Series license keys are
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
  description = "The base registration key obtained from F5 for the target r-Series appliance or Velos chassis partition, applied via f5os_license.registration_key. This is a NEW key issued for this F5OS device -- an i-Series/TMOS license key is never valid here. See https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/apply-license-from-iseries."
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
