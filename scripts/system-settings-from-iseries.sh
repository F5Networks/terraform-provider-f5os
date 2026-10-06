#!/usr/bin/env bash
#
# system-settings-from-iseries.sh -- input prep for the i-Series -> r-Series
# (F5OS) system-settings migration workflow. Converts the DNS/NTP/SNMP/
# auth-user portion of the `extracted-sys-settings.json` produced by
# terraform-provider-bigip's `scripts/extract-sys-settings.sh` (that
# repo's Phase 1) into a `system-settings.auto.tfvars.json` file shaped
# for this repo's examples/migration/system-settings-from-iseries/
# configuration, which creates equivalent f5os_dns/f5os_ntp_server/
# f5os_snmp/f5os_auth/f5os_user resources.
#
# Usage:
#   ./scripts/system-settings-from-iseries.sh [extracted-sys-settings.json] [system-settings.auto.tfvars.json]
#
# Arguments (both optional, positional):
#   extracted-sys-settings.json     Input file produced by
#                                     terraform-provider-bigip's
#                                     extract-sys-settings.sh. Defaults to
#                                     extracted-sys-settings.json in the
#                                     current directory.
#   system-settings.auto.tfvars.json Output file. Defaults to
#                                     system-settings.auto.tfvars.json in
#                                     the current directory. Terraform
#                                     automatically loads any
#                                     *.auto.tfvars(.json) file found in
#                                     the working directory, so no
#                                     explicit -var-file flag is needed if
#                                     this output is placed alongside
#                                     main.tf.
#
# What this script does NOT migrate (see
# https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/configure-system-settings-from-iseries
# for details):
#   - User passwords: bigip_auth_user.password is always null in the
#     extraction (BIG-IP never returns it on read) -- every user in the
#     output `users` map gets a REPLACE-ME placeholder password that MUST
#     be changed before applying.
#   - LDAP/RADIUS/TACACS+ server definitions: only auth_order is derived
#     (always ["local"] by this script), since f5os_auth does not itself
#     configure remote-auth server connection details.
#   - SNMP allowedaddresses (client source-address ACL): f5os_snmp has no
#     equivalent concept; this restriction is silently dropped and a
#     warning is printed if the source device has any configured.
#   - NTP per-server key_id/prefer (authentication): not present in the
#     source extraction; every server is emitted with only iburst=true.

set -euo pipefail

command -v jq >/dev/null || {
  echo "jq not found in PATH" >&2
  exit 1
}

IN_FILE="${1:-extracted-sys-settings.json}"
OUT_FILE="${2:-system-settings.auto.tfvars.json}"

if [ ! -f "${IN_FILE}" ]; then
  echo "ERROR: input file not found: ${IN_FILE}" >&2
  exit 1
fi

# --- DNS ---------------------------------------------------------------
DNS_COUNT="$(jq '[ .[] | select(.type == "bigip_sys_dns") ] | length' "${IN_FILE}")"
if [ "${DNS_COUNT}" -eq 0 ]; then
  echo "WARNING: no bigip_sys_dns entry found in ${IN_FILE} -- dns_servers/dns_search_domains will be empty." >&2
fi

# --- NTP -----------------------------------------------------------------
# f5os_ntp_server manages one server per resource instance (unlike
# f5os_dns/f5os_snmp/f5os_auth, which are singletons), so the output here
# is a flat list consumed via for_each in main.tf, not a nested object.
NTP_COUNT="$(jq '[ .[] | select(.type == "bigip_sys_ntp") | .values.servers[]? ] | length' "${IN_FILE}")"
if [ "${NTP_COUNT}" -eq 0 ]; then
  echo "NOTE: no NTP servers found under a bigip_sys_ntp entry in ${IN_FILE} -- ntp_servers will be empty." >&2
fi

# --- SNMP ------------------------------------------------------------------
# f5os_snmp has no equivalent to bigip_sys_snmp's allowedaddresses
# (an SNMP client source-address access list) -- warn if the source
# device restricts SNMP access by address, since that restriction is not
# carried over by this configuration at all.
jq -r '
  [ .[] | select(.type == "bigip_sys_snmp") | .values.allowedaddresses[]? ]
  | if length > 0 then
      "WARNING: source device restricts SNMP access to: " + join(", ") +
      " via bigip_sys_snmp.allowedaddresses -- f5os_snmp has no equivalent access-list concept; this restriction is NOT migrated and must be enforced separately (e.g. a management-network ACL) if still required."
    else empty end
' "${IN_FILE}" >&2

# --- Users -------------------------------------------------------------
# TMOS's partition-scoped role model (partition_access[].role, one entry
# per partition) has no direct equivalent in F5OS's flat, single-role-per-
# platform-user model (f5os_user.role). This script takes the FIRST
# partition_access entry's role as the source of truth and maps it:
#   admin  -> admin       (full administrative access on both platforms)
#   *      -> operator     (everything else -- guest, manager, auditor,
#                           application-editor, etc. -- is approximated
#                           to F5OS's non-admin "operator" role, since
#                           none of TMOS's other roles have a precise
#                           F5OS platform-role equivalent)
# This is a best-effort, lossy translation -- review the output `users`
# map and adjust f5os_role per user as needed for your actual security
# requirements before applying. A warning is printed for every user whose
# role was approximated (i.e. every non-"admin" TMOS role).
jq -r '
  [ .[] | select(.type == "bigip_auth_user") ]
  | .[]
  | select(.values.name != "admin")
  | select((.values.partition_access[0].role // "") != "admin")
  | "WARNING: user \"\(.values.name)\" has TMOS role \"\(.values.partition_access[0].role // "none")\" -- approximated to F5OS role \"operator\" (no exact equivalent exists); review and adjust manually if a different F5OS role is required."
' "${IN_FILE}" >&2

# The "admin" username is never emitted into the output `users` map: F5OS
# ships with a pre-existing built-in "admin" account created during
# initial device deployment, and f5os_user's Create issues a POST that
# fails with an "already exists" conflict against that account. This is
# not a lossy approximation like the role mapping above -- an
# "admin"-named user on the source device is not a candidate for
# f5os_user at all, regardless of its TMOS role, because the account
# already exists on every F5OS target out of band. Its password (also
# never present in the source extraction -- see the header comment) can
# still be set via f5os_user_password_change instead; see
# https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/configure-system-settings-from-iseries.
ADMIN_COUNT="$(jq '[ .[] | select(.type == "bigip_auth_user") | select(.values.name == "admin") ] | length' "${IN_FILE}")"
if [ "${ADMIN_COUNT}" -gt 0 ]; then
  echo "NOTE: source device has an \"admin\" user -- skipped from the output \`users\` map." >&2
  echo "      F5OS already has a built-in \"admin\" account; f5os_user cannot create it (device" >&2
  echo "      rejects the create with an \"already exists\" conflict). Use f5os_user_password_change" >&2
  echo "      to manage its password instead -- see https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/configure-system-settings-from-iseries." >&2
fi

jq '
  {
    dns_servers: [ .[] | select(.type == "bigip_sys_dns") | .values.name_servers[]? ],
    dns_search_domains: [ .[] | select(.type == "bigip_sys_dns") | .values.search[]? ],
    ntp_servers: [ .[] | select(.type == "bigip_sys_ntp") | .values.servers[]? ],
    snmp_sys_contact: ([ .[] | select(.type == "bigip_sys_snmp") | .values.sys_contact ] | first // null),
    snmp_sys_location: ([ .[] | select(.type == "bigip_sys_snmp") | .values.sys_location ] | first // null),
    auth_order: ["local"],
    users: (
      [ .[] | select(.type == "bigip_auth_user") | select(.values.name != "admin") ]
      | map({
          (.values.name): {
            f5os_role: (if (.values.partition_access[0].role // "") == "admin" then "admin" else "operator" end),
            password: "REPLACE-ME"
          }
        })
      | add // {}
    )
  }
' "${IN_FILE}" >"${OUT_FILE}"

echo "==> Wrote system settings to ${OUT_FILE}:" >&2
echo "    $(jq '.dns_servers | length' "${OUT_FILE}") DNS server(s), $(jq '.dns_search_domains | length' "${OUT_FILE}") search domain(s)" >&2
echo "    $(jq '.ntp_servers | length' "${OUT_FILE}") NTP server(s)" >&2
echo "    $(jq '.users | length' "${OUT_FILE}") user(s) -- passwords set to REPLACE-ME, MUST be changed before applying" >&2
