/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

output "dns_id" {
  description = "Terraform-synthetic ID of the configured f5os_dns resource, or null if var.dns_servers was empty (no DNS resource created -- see main.tf's count on f5os_dns.from_iseries)."
  value       = length(f5os_dns.from_iseries) > 0 ? f5os_dns.from_iseries[0].id : null
}

output "ntp_servers" {
  description = "Map of NTP server address to the F5OS-assigned resource id for every NTP server created from the i-Series source."
  value       = { for addr, ntp in f5os_ntp_server.from_iseries : addr => ntp.id }
}

output "snmp_id" {
  description = "Terraform-synthetic ID of the configured f5os_snmp resource."
  value       = f5os_snmp.from_iseries.id
}

output "auth_id" {
  description = "Terraform-synthetic ID of the configured f5os_auth resource."
  value       = f5os_auth.from_iseries.id
}

output "created_users" {
  description = "Map of username to the F5OS-assigned resource id for every platform user created from the i-Series source."
  value       = { for name, user in f5os_user.from_iseries : name => user.id }
}
