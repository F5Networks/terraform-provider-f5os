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
