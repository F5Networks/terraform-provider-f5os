/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

output "license_id" {
  description = "Terraform-synthetic ID of the configured f5os_license resource. A non-null value here means Create completed without the device rejecting the registration key -- it does NOT by itself prove the license is active; for a device-side check, see https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/apply-license-from-iseries."
  value       = f5os_license.from_iseries.id
}
