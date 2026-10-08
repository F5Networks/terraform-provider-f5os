---
page_title: "Configuring system settings on F5OS from discovered i-Series configuration"
description: |-
  How to use examples/migration/system-settings-from-iseries and scripts/system-settings-from-iseries.sh to configure DNS, NTP, SNMP, AAA authentication order, and local platform users on an rSeries/Velos partition F5OS target from settings discovered on a source BIG-IP i-Series device.
---

# Configuring system settings on F5OS from discovered i-Series configuration

`examples/migration/system-settings-from-iseries` is a system-settings
phase of an i-Series -> r-Series (F5OS) migration workflow, independent of
(and can be applied in parallel with) the VLAN creation workflow in
[Creating VLANs on F5OS from discovered i-Series
configuration](create-vlans-from-iseries.html): it configures
`f5os_dns`, `f5os_ntp_server`, `f5os_snmp`, `f5os_auth`, and `f5os_user`
to match DNS, NTP, SNMP, authentication order, and local user settings
discovered on a source BIG-IP i-Series device.
`scripts/system-settings-from-iseries.sh` converts the relevant portion
of the `extracted-sys-settings.json` produced by
[`terraform-provider-bigip`'s `scripts/extract-sys-settings.sh`](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings)
(that provider's Phase 1) into the variables this configuration expects.

For the full cross-repo phase sequence, see the [overall i-Series to
r-Series migration flow](iseries-to-rseries-migration-flow.html).

## What is (and isn't) migrated

| Source (i-Series) | Target (F5OS) | Notes |
|---|---|---|
| `bigip_sys_dns.name_servers` / `.search` | `f5os_dns.dns_servers` / `.dns_domains` | Direct pass-through. |
| `bigip_sys_ntp.servers` | `f5os_ntp_server.server` (one resource per server, `for_each`) | `iburst` is always enabled on the created resources for faster resynchronization after migration. Per-server `key_id`/`prefer` (NTP authentication) are not present in the source extraction and are not set. |
| `bigip_sys_snmp.sys_contact` / `.sys_location` | `f5os_snmp.snmp_mib.syscontact` / `.syslocation` | Direct pass-through. `bigip_sys_snmp.allowedaddresses` (an SNMP client source-address access list) has **no equivalent** in `f5os_snmp` and is **not migrated** -- see [SNMP access-list caveat](#snmp-access-list-caveat) below. |
| `bigip_auth_user.*` | `f5os_user.username` / `.role` | Username preserved verbatim; role is approximated (TMOS's partition-scoped role model has no direct F5OS equivalent) -- see [User role mapping](#user-role-mapping) below. Passwords are **never** present in the source extraction and must be set manually -- see [Passwords are not migrated](#passwords-are-not-migrated) below. The source device's `admin` user is **skipped entirely** -- see [Built-in admin account is not migrated](#built-in-admin-account-is-not-migrated) below. |
| n/a | `f5os_auth.auth_order` | Always set to `["local"]` by the conversion script -- see [Authentication order](#authentication-order) below. |

## Example Usage

```terraform
/*
Copyright 2019 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

# ---------------------------------------------------------------------------
# Configures DNS, NTP, SNMP, AAA authentication, and local platform users
# on the F5OS (rSeries appliance or Velos chassis partition) layer to
# match settings discovered on a source BIG-IP i-Series device.
#
# This is a system-settings phase of an i-Series -> r-Series (F5OS)
# migration workflow, parallel to (and independent of) the VLAN creation
# workflow in examples/migration/vlans-from-iseries: it consumes the
# DNS/NTP/SNMP/user portions of the JSON produced by
# terraform-provider-bigip's `scripts/extract-sys-settings.sh`, converted
# into this configuration's variables via
# `scripts/system-settings-from-iseries.sh` in this repo. See
# https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/configure-system-settings-from-iseries
# for the full
# workflow, including the role-mapping and password-migration caveats.
#
# terraform.tfvars.json.example shows the expected shape for every
# variable; any *.auto.tfvars(.json) file placed in this directory (e.g.
# the output of scripts/system-settings-from-iseries.sh) is loaded
# automatically by `terraform apply` with no extra flags needed.
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

resource "f5os_dns" "from_iseries" {
  # f5os_dns.dns_servers is Required (see
  # internal/provider/dns_resource.go): an empty list fails at apply
  # time with a device error, not a clear Terraform-side message. This
  # configuration is applied whether or not the source i-Series device
  # had a bigip_sys_dns entry (var.dns_servers defaults to []), so guard
  # creation on at least one discovered DNS server instead.
  count = length(var.dns_servers) > 0 ? 1 : 0

  dns_servers = var.dns_servers
  dns_domains = var.dns_search_domains
}

resource "f5os_ntp_server" "from_iseries" {
  for_each = toset(var.ntp_servers)

  server = each.value
  iburst = true
}

resource "f5os_snmp" "from_iseries" {
  snmp_mib = {
    syscontact  = var.snmp_sys_contact
    syslocation = var.snmp_sys_location
  }
}

resource "f5os_auth" "from_iseries" {
  auth_order = var.auth_order
}

resource "f5os_user" "from_iseries" {
  for_each = var.users

  username = each.key
  password = each.value.password
  role     = each.value.f5os_role
}
```

```terraform
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
```

```terraform
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
```

## Populating the variables from a Phase 1 extraction

Run `terraform-provider-bigip`'s `scripts/extract-sys-settings.sh` against
the source i-Series device first (see that provider's [extraction
guide](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings)),
then convert its output with this repo's
`scripts/system-settings-from-iseries.sh`:

```sh
./scripts/system-settings-from-iseries.sh extracted-sys-settings.json \
  examples/migration/system-settings-from-iseries/system-settings.auto.tfvars.json
```

Terraform automatically loads any `*.auto.tfvars.json` file found in the
working directory, so no explicit `-var-file` flag is needed as long as
the generated file is placed alongside `main.tf` as shown above. See
`terraform.tfvars.json.example` in the same directory for the expected
shape:

```json
{
  "dns_servers": ["172.27.1.1"],
  "dns_search_domains": ["openstack.internal"],
  "ntp_servers": [],
  "snmp_sys_contact": "Customer Name <admin@customer.com>",
  "snmp_sys_location": "Network Closet 1",
  "auth_order": ["local"],
  "users": {
    "anotherUser": { "f5os_role": "operator", "password": "REPLACE-ME" },
    "guestUser": { "f5os_role": "operator", "password": "REPLACE-ME" }
  }
}
```

### Passwords are not migrated

`bigip_auth_user.password` is always `null` in `extracted-sys-settings.json`
-- BIG-IP never returns a user's password on read, so it cannot be
extracted regardless of permissions. `system-settings-from-iseries.sh`
therefore writes every user in the output `users` map with a
`REPLACE-ME` placeholder password, and prints a summary count reminding
you to change them. **You must edit every password in the generated
`*.auto.tfvars.json` before running `terraform apply`** -- `f5os_user`
will happily create accounts with the literal password `REPLACE-ME` if
you don't, since it has no way to know that value is a placeholder.

### Built-in admin account is not migrated

Every F5OS system (rSeries appliance or Velos chassis partition) already
has a built-in `admin` account created during initial device
deployment, before Terraform ever runs. `f5os_user` creates users via a
`POST` to the device's user-list endpoint, which fails with an "already
exists" conflict error if attempted against that pre-existing `admin`
account. `system-settings-from-iseries.sh` therefore drops any
source-device user named `admin` from the output `users` map entirely
(with a note printed to stderr) -- it is never a candidate for
`f5os_user`, regardless of its TMOS role, since the account already
exists on every F5OS target out of band.

This is different from -- and takes precedence over -- the role
approximation described below: even a source `admin` user with TMOS
role `admin` is skipped, not merely mapped to F5OS role `admin`, because
`f5os_user` cannot create it either way.

If the source device's `admin` password needs to be replicated to the
F5OS target, use
[`f5os_user_password_change`](user_password_change.html) instead, which
is designed specifically for changing passwords on default accounts
(`admin`, `root`) that already exist on the device:

```terraform
resource "f5os_user_password_change" "admin" {
  user_name    = "admin"
  old_password = "<current F5OS admin password>"
  new_password = "<password to migrate>"
}
```

### User role mapping

TMOS's role model is partition-scoped: `bigip_auth_user.partition_access`
is a list of `{ partition, role }` pairs, so a single user can have
different roles on different partitions. F5OS's `f5os_user.role` is a
single platform-wide role (from `admin`, `operator`, `resource-admin`).
These models are not equivalent, so the conversion script uses a
best-effort approximation, not a lossless translation:

- The user's **first** `partition_access` entry's `role` is used as the
  source of truth (later entries, if any, for other partitions are
  ignored).
- TMOS `admin` -> F5OS `admin` (both are full-administrative roles).
- Every other TMOS role (`guest`, `manager`, `auditor`,
  `application-editor`, etc.) -> F5OS `operator`, since none of them has a
  precise F5OS equivalent and `operator` is the least-privileged
  non-`admin` F5OS role.

The script prints a warning for every user whose role was approximated
(i.e. every non-`admin` TMOS role) naming the user and the source role,
so each mapping can be reviewed. Edit `f5os_role` in the generated
`users` map manually if a different F5OS role (for example
`resource-admin`) is actually required for a given user.

### Authentication order

`system-settings-from-iseries.sh` always sets `auth_order` to `["local"]`
in its output, regardless of whether the source i-Series device has
LDAP, RADIUS, or TACACS+ configured (`bigip_auth_ldap`/`bigip_auth_radius`/
`bigip_auth_tacacs` entries in `extracted-sys-settings.json`, present only
if configured on the source device). This is intentional, not an
oversight: `f5os_auth`'s `auth_order` only controls authentication
*method precedence* -- it does not itself configure a remote-auth
server's connection details (LDAP bind DN, RADIUS shared secret, and so
on), which is out of scope for this phase. If the source device uses
LDAP/RADIUS/TACACS+, configure the equivalent F5OS remote-auth resources
separately first, then set `var.auth_order` to include them (e.g.
`["local", "ldap"]`) before applying.

### SNMP access-list caveat

`bigip_sys_snmp.allowedaddresses` restricts which source addresses may
query SNMP on the i-Series device. `f5os_snmp` has no equivalent
attribute -- F5OS does not expose a per-address SNMP client access list
through this resource. If the source device has any `allowedaddresses`
configured, `system-settings-from-iseries.sh` prints a warning naming
them, but **this restriction is not migrated in any form**. If
equivalent access control is still required on the F5OS target, enforce
it through another mechanism (for example a management-network ACL or
firewall rule) outside of this configuration.

## Applying

```sh
cd examples/migration/system-settings-from-iseries
terraform init
terraform apply
```

Update the `provider "f5os"` block's `host`/`username`/`password` (or use
the corresponding `F5OS_HOST`/`F5OS_USERNAME`/`F5OS_PASSWORD` environment
variables) to point at the target rSeries appliance or Velos chassis
partition before applying.

~> **NOTE:** `f5os_dns` and `f5os_auth`'s `terraform destroy` behavior
intentionally does not fully revert every change (DNS is left configured
on the device since it's critical for connectivity; `f5os_auth` restores
`auth_order` and role GID mappings but does not touch `password_policy`).
See each resource's own documentation for details before running
`terraform destroy` against a shared/production device.

## Related migration guides

- [Inventorying TMOS version and hardware](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/inventory-tmos-version) (Phase 0, `terraform-provider-bigip`)
- [Extracting i-Series system settings](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings) (Phase 1, `terraform-provider-bigip`)
- [Creating VLANs on F5OS from discovered i-Series configuration](create-vlans-from-iseries.html) (this repo) -- the parallel VLAN-creation phase of this migration workflow.
- [Applying a license to F5OS as part of an i-Series migration](apply-license-from-iseries.html) (this repo) -- also independent of this phase.
- [Generating and downloading a UCS backup](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/generate-ucs-backup) (Phase 2, `terraform-provider-bigip`)
