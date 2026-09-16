/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# dns_servers/dns_search_domains are populated from the source i-Series
# device's `bigip_sys_dns` (name_servers/search), passed straight through
# to f5os_dns.
#
# Example:
#   dns_servers        = ["172.27.1.1"]
#   dns_search_domains  = ["openstack.internal"]
# ---------------------------------------------------------------------------
variable "dns_servers" {
  description = "List of DNS server IP addresses discovered on the source i-Series device (bigip_sys_dns.name_servers), applied verbatim to f5os_dns.dns_servers."
  type        = list(string)
  default     = []
}

variable "dns_search_domains" {
  description = "List of DNS search domains discovered on the source i-Series device (bigip_sys_dns.search), applied verbatim to f5os_dns.dns_domains."
  type        = list(string)
  default     = []
}

# ---------------------------------------------------------------------------
# ntp_servers is a flat list of NTP server addresses/hostnames from the
# source i-Series device's `bigip_sys_ntp.servers`. Each entry becomes a
# separate f5os_ntp_server resource (that resource manages one server per
# instance, unlike f5os_dns/f5os_snmp/f5os_auth which are singletons), with
# iburst enabled per the acceptance criteria for faster resynchronization
# after the r-Series migration reboot/cutover.
#
# TMOS's bigip_sys_ntp has no per-server key_id/prefer equivalent in the
# extracted data (those are separate `net ntp keys`/`sys ntp` fields this
# extraction doesn't capture) -- if the source device uses NTP
# authentication, configure key_id/prefer manually after applying this
# configuration; see f5os_ntp_server's key_id/prefer arguments.
#
# Example:
#   ntp_servers = ["ntp1.example.com", "ntp2.example.com"]
# ---------------------------------------------------------------------------
variable "ntp_servers" {
  description = "List of NTP server addresses/hostnames discovered on the source i-Series device (bigip_sys_ntp.servers), each created as a separate f5os_ntp_server resource with iburst enabled."
  type        = list(string)
  default     = []
}

# ---------------------------------------------------------------------------
# snmp_sys_contact/snmp_sys_location mirror bigip_sys_snmp's
# sys_contact/sys_location straight into f5os_snmp's snmp_mib block.
#
# Note: bigip_sys_snmp's `allowedaddresses` (SNMP client access control
# list) has no equivalent concept in f5os_snmp -- F5OS does not expose a
# per-address SNMP access list through this resource. If the source
# device restricts SNMP access by source address, that restriction is
# NOT carried over by this configuration and must be enforced by another
# mechanism (e.g. a management-network ACL) on the F5OS target.
#
# Example:
#   snmp_sys_contact  = "Customer Name <admin@customer.com>"
#   snmp_sys_location = "Network Closet 1"
# ---------------------------------------------------------------------------
variable "snmp_sys_contact" {
  description = "SNMP system contact discovered on the source i-Series device (bigip_sys_snmp.sys_contact), applied to f5os_snmp.snmp_mib.syscontact."
  type        = string
  default     = null
}

variable "snmp_sys_location" {
  description = "SNMP system location discovered on the source i-Series device (bigip_sys_snmp.sys_location), applied to f5os_snmp.snmp_mib.syslocation."
  type        = string
  default     = null
}

# ---------------------------------------------------------------------------
# users is keyed by username (preserved from the source i-Series device)
# and maps to the F5OS role to assign. scripts/system-settings-from-iseries.sh
# derives f5os_role from each bigip_auth_user's partition_access[0].role,
# approximating TMOS's role model onto F5OS's (see the script and guide
# for the mapping table and its limitations) -- review/adjust the mapping
# before applying, since the two role models are not equivalent and this
# is a best-effort translation, not a lossless one.
#
# Passwords are never present in the source extraction (BIG-IP never
# returns them on read -- see bigip_auth_user's `password` field, always
# null in extracted-sys-settings.json) and so cannot be migrated
# automatically; set a temporary password for each user here (or via a
# *.auto.tfvars.json this variable is merged from) and require the user to
# change it on first login, or use f5os_user_password_change afterward.
#
# Example:
#   users = {
#     "netops-admin" = { f5os_role = "admin", password = "ChangeMe123!" }
#     "netops-ro"    = { f5os_role = "operator", password = "ChangeMe456!" }
#   }
# ---------------------------------------------------------------------------
variable "users" {
  description = "Map of username (from the source i-Series device) to F5OS role and initial password, to create on the F5OS (rSeries/Velos partition) target via f5os_user."
  type = map(object({
    f5os_role = string
    password  = string
  }))
  default = {}

  # Not marked sensitive: f5os_user.from_iseries.password (below) is
  # keyed via for_each = var.users, and Terraform forbids sensitive
  # values as for_each arguments (the value could otherwise be exposed
  # as a resource instance key). Each password is still individually
  # masked in plan/apply output regardless, because f5os_user's own
  # `password` schema attribute is marked Sensitive.

  validation {
    condition     = alltrue([for u in values(var.users) : contains(["admin", "operator", "resource-admin"], u.f5os_role)])
    error_message = "Every user's f5os_role in var.users must be one of \"admin\", \"operator\", or \"resource-admin\" (the F5OS f5os_user valid primary roles)."
  }
}

# ---------------------------------------------------------------------------
# auth_order is the local/remote authentication method precedence to
# configure via f5os_auth. Defaults to `[\"local\"]` since the sample
# i-Series extraction has no LDAP/RADIUS/TACACS+ configured
# (bigip_auth_ldap/bigip_auth_radius/bigip_auth_tacacs entries are only
# present in extracted-sys-settings.json if configured on the source
# device) -- this configuration does not attempt to migrate remote AAA
# server definitions themselves, since f5os_auth's `ldap` block only
# configures LDAP *object-class* search behavior, not server
# connection details, and F5OS's remote-auth server configuration is
# out of scope for this phase. If the source device uses LDAP/RADIUS/
# TACACS+, configure the equivalent F5OS remote-auth resources
# separately and adjust auth_order to match.
# ---------------------------------------------------------------------------
variable "auth_order" {
  description = "Ordered list of authentication methods for f5os_auth.auth_order. Defaults to [\"local\"] -- see the note above if the source i-Series device uses LDAP/RADIUS/TACACS+."
  type        = list(string)
  default     = ["local"]

  # Mirrors f5os_auth's own listAuthOrderValidator (see
  # internal/provider/f5os_auth_resource.go): non-empty, every entry one
  # of "local"/"radius"/"tacacs"/"ldap", no duplicates. Catching this
  # here means a typo in a hand-edited var.auth_order fails at
  # `terraform plan` with a clear message instead of only being caught
  # by the provider's validator during `terraform apply`.
  validation {
    condition     = length(var.auth_order) > 0 && alltrue([for m in var.auth_order : contains(["local", "radius", "tacacs", "ldap"], m)])
    error_message = "var.auth_order must be non-empty and every entry must be one of \"local\", \"radius\", \"tacacs\", or \"ldap\" (the F5OS f5os_auth valid authentication methods)."
  }

  validation {
    condition     = length(var.auth_order) == length(distinct(var.auth_order))
    error_message = "var.auth_order must not contain duplicate authentication methods."
  }
}
