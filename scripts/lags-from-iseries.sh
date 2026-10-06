#!/usr/bin/env bash
#
# lags-from-iseries.sh -- Phase 5 (input prep) of the i-Series ->
# r-Series (F5OS) migration workflow. Converts the trunk/VLAN membership
# portion of the `extracted-sys-settings.json` produced by
# terraform-provider-bigip's `scripts/extract-sys-settings.sh` (that
# repo's Phase 1) into a `lags.auto.tfvars.json` file shaped for this
# repo's examples/migration/vlans-from-iseries/ configuration, which
# creates the equivalent f5os_lag resources via `for_each = var.lags`.
#
# Usage:
#   ./scripts/lags-from-iseries.sh [extracted-sys-settings.json] [lags.auto.tfvars.json]
#
# Arguments (both optional, positional):
#   extracted-sys-settings.json    Input file produced by
#                                   terraform-provider-bigip's
#                                   extract-sys-settings.sh. Defaults to
#                                   extracted-sys-settings.json in the
#                                   current directory.
#   lags.auto.tfvars.json          Output file. Defaults to
#                                   lags.auto.tfvars.json in the current
#                                   directory. Terraform automatically
#                                   loads any *.auto.tfvars(.json) file
#                                   found in the working directory, so
#                                   no explicit -var-file flag is needed
#                                   if this output is placed alongside
#                                   main.tf.
#
# Input shapes consumed (from terraform-provider-bigip's
# extract-sys-settings.sh):
#   - data.bigip_net_trunks.extracted.values.trunks[] -- trunk name,
#     lacp/lacp_mode/lacp_timeout, and interfaces[] (TMOS-named member
#     list, owned by the trunk).
#   - data.bigip_net_vlans.extracted.values.vlans[] -- each VLAN's tag
#     and its tagged/untagged interfaces[] membership list (a trunk name
#     appears here exactly like a physical interface name would).
#
# Output shape (this repo's examples/migration/vlans-from-iseries
# var.lags): a map keyed by the trunk name (preserved verbatim -- F5OS
# LAG names are free-form identifiers, not numeric IDs, so unlike
# physical interfaces there is no name-mapping step here), each value
# shaped for f5os_lag's lag_type/mode/interval/members/native_vlan/
# trunk_vlans arguments directly:
#   {
#     "lags": {
#       "lag1": {
#         "lag_type": "LACP", "mode": "ACTIVE", "interval": "SLOW",
#         "members": ["1.0", "2.0"], "native_vlan": null, "trunk_vlans": [200]
#       }
#     }
#   }
#
# Field mapping (TMOS bigip_net_trunks -> F5OS f5os_lag):
#   name                          -> lags key (preserved verbatim)
#   lacp ("enabled"/"disabled")   -> lag_type ("LACP"/"STATIC")
#   lacp_mode ("active"/"passive") -> mode ("ACTIVE"/"PASSIVE"), LACP only.
#                                      A missing or unrecognized value maps
#                                      to null (f5os_lag's mode is Optional,
#                                      so null accepts the device default)
#                                      rather than guessing "ACTIVE" --
#                                      see unrecognized_lacp_settings_encountered
#                                      below for values that were present
#                                      but not recognized.
#   lacp_timeout ("long"/"short") -> interval ("SLOW"/"FAST"), LACP only.
#                                      Same null-on-unrecognized/missing
#                                      treatment as lacp_mode above.
#   interfaces (TMOS member names) -> members (F5OS-mapped member names)
#   distribution_hash             -> (not migrated -- f5os_lag hardcodes
#                                      "src-dst-ipport"; see
#                                      internal/provider/lag_resource.go)
#   bandwidth, working_member_count, stp, type
#                                  -> (not migrated -- runtime
#                                      state/counters on F5OS, not
#                                      something configured to match a
#                                      TMOS value; see
#                                      terraform-provider-bigip's
#                                      https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping)
#
# Member interface name mapping: identical blade-drop rule as
# scripts/interfaces-from-iseries.sh ("<blade>.<port>" -> "<port>.0");
# a member name that doesn't match the expected TMOS pattern is passed
# through unchanged (so it isn't silently dropped) and reported
# separately under `.unmapped_member_names_encountered` printed to
# stderr, for manual review -- it will not be a valid rSeries name and
# will need to be renamed by hand before applying.
#
# Ambiguous native VLANs and rSeries member-name collisions are
# detected and reported to stderr exactly as in
# scripts/interfaces-from-iseries.sh -- see that script's header
# comment for the rationale. Unlike interfaces, a member-name collision
# here means two *different* TMOS interfaces that both map to the same
# rSeries name end up in the same LAG's `members` list de-duplicated
# (Terraform's `members` is a Set) rather than one silently overwriting
# the other -- still reported, since it signals the blade-drop mapping
# assumption (single-blade i-Series/VE source) doesn't hold.
#
# LACP LAGs whose lacp_mode/lacp_timeout is present but doesn't match a
# recognized TMOS value are reported separately under
# `.unrecognized_lacp_settings_encountered` (printed to stderr) --
# mode/interval are emitted as null for that LAG (accepting the F5OS
# device default, since f5os_lag itself marks both attributes
# Optional) rather than silently defaulting to "ACTIVE"/"SLOW", which
# would mask incomplete or malformed extraction data.

set -euo pipefail

command -v jq >/dev/null || {
  echo "jq not found in PATH" >&2
  exit 1
}

IN_FILE="${1:-extracted-sys-settings.json}"
OUT_FILE="${2:-lags.auto.tfvars.json}"

if [ ! -f "${IN_FILE}" ]; then
  echo "ERROR: input file not found: ${IN_FILE}" >&2
  exit 1
fi

TRUNK_COUNT="$(jq '[ .[] | select(.type == "bigip_net_trunks") | .values.trunks[]? ] | length' "${IN_FILE}")"

if [ "${TRUNK_COUNT}" -eq 0 ]; then
  echo "NOTE: no trunks found under a bigip_net_trunks entry in ${IN_FILE} -- lags will be empty." >&2
fi

VLAN_COUNT="$(jq '[ .[] | select(.type == "bigip_net_vlans") | .values.vlans[]? ] | length' "${IN_FILE}")"

if [ "${VLAN_COUNT}" -eq 0 ]; then
  echo "WARNING: no VLANs found under a bigip_net_vlans entry in ${IN_FILE}." >&2
  echo "         (extract-sys-settings.sh in registry-fallback mode omits bigip_net_vlans" >&2
  echo "         entirely -- every LAG will be emitted with native_vlan=null and an empty" >&2
  echo "         trunk_vlans, not because the source trunk has no VLAN memberships." >&2
  echo "         Re-run it with a locally built provider to include VLAN membership data.)" >&2
fi

# The jq program is intentionally kept in one pass, matching
# scripts/interfaces-from-iseries.sh's structure, so every derived value
# (mapped member names, native/trunk VLAN classification, diagnostics)
# comes from a single consistent read of the input.
DERIVED="$(mktemp)"
trap 'rm -f "${DERIVED}"' EXIT

jq '
  ( [ .[] | select(.type == "bigip_net_vlans") | .values.vlans[]? ] ) as $vlans
  | ( [ .[] | select(.type == "bigip_net_trunks") | .values.trunks[]? ] ) as $trunks
  | ( [ $vlans[] as $vlan | ($vlan.interfaces // [])[] | {interface: .name, tag: $vlan.tag, tagged: .tagged} ] ) as $memberships
  | (
      $trunks
      | map(
          . as $trunk
          | {
              name: $trunk.name,
              lag_type: (if $trunk.lacp == "enabled" then "LACP" else "STATIC" end),
              # Only "active"/"passive" (mode) and "long"/"short"
              # (interval) are recognized TMOS values; anything else
              # (missing field, unexpected value from a malformed or
              # future extract-sys-settings.sh) resolves to null here
              # rather than silently defaulting to "ACTIVE"/"SLOW" --
              # mode/interval are Optional on f5os_lag, so null lets
              # var.lags own validation block (variables.tf) accept
              # the device default instead of this script asserting a
              # value it cannot actually be sure of. raw_mode/raw_timeout
              # are carried through member_rows-adjacent fields below
              # so a null caused by an *unrecognized* value (as opposed
              # to a genuinely absent field) can still be reported to
              # stderr for manual review.
              mode: (
                if $trunk.lacp_mode == "active" then "ACTIVE"
                elif $trunk.lacp_mode == "passive" then "PASSIVE"
                else null
                end
              ),
              interval: (
                if $trunk.lacp_timeout == "long" then "SLOW"
                elif $trunk.lacp_timeout == "short" then "FAST"
                else null
                end
              ),
              raw_lacp_mode: ($trunk.lacp_mode // null),
              raw_lacp_timeout: ($trunk.lacp_timeout // null),
              member_rows: (
                ($trunk.interfaces // [])
                | map(
                    . as $tmos_name
                    # capture() produces no output at all (not null) on a
                    # non-match; wrapping in [ ... ] | first turns that
                    # into a real null instead of silently vanishing from
                    # the map() output.
                    | ( [ $tmos_name | capture("^(?<blade>[0-9]+)\\.(?<port>[0-9]+)$") ] | first ) as $parts
                    | {
                        tmos_name: $tmos_name,
                        rseries_name: (if $parts != null then "\($parts.port).0" else $tmos_name end),
                        mapped: ($parts != null)
                      }
                  )
              ),
              native_vlans: ( [ $memberships[] | select(.interface == $trunk.name and .tagged == false) | .tag ] | unique ),
              trunk_vlans: ( [ $memberships[] | select(.interface == $trunk.name and .tagged == true) | .tag ] | unique )
            }
        )
    ) as $rows
  | {
      lags: (
        $rows
        | map({
            (.name): {
              lag_type: .lag_type,
              mode: (if .lag_type == "LACP" then .mode else null end),
              interval: (if .lag_type == "LACP" then .interval else null end),
              members: (.member_rows | map(.rseries_name) | unique),
              native_vlan: (if (.native_vlans | length) > 1 then null else (.native_vlans[0] // null) end),
              trunk_vlans: .trunk_vlans
            }
          })
        | add // {}
      ),
      ambiguous_native_vlans_encountered: [ $rows[] | select((.native_vlans | length) > 1) | { lag: .name, native_vlans: .native_vlans } ],
      unmapped_member_names_encountered: [ $rows[] | .name as $lag | .member_rows[] | select(.mapped == false) | { lag: $lag, tmos_name: .tmos_name } ],
      # Only flags LACP LAGs whose lacp_mode/lacp_timeout is present but
      # unrecognized (mode/interval resolved to null despite raw_lacp_*
      # being non-null) -- a genuinely absent field is expected and not
      # worth a warning, since mode/interval are Optional on f5os_lag
      # and null there means "accept the device default", exactly as
      # intended.
      unrecognized_lacp_settings_encountered: [
        $rows[]
        | select(.lag_type == "LACP")
        | select((.mode == null and .raw_lacp_mode != null) or (.interval == null and .raw_lacp_timeout != null))
        | { lag: .name, raw_lacp_mode: .raw_lacp_mode, raw_lacp_timeout: .raw_lacp_timeout }
      ],
      member_name_collisions_encountered: (
        $rows
        | map(
            . as $row
            | ($row.member_rows | group_by(.rseries_name) | map(select(length > 1)) | map({ rseries_name: .[0].rseries_name, tmos_names: [ .[].tmos_name ] })) as $collisions
            | select(($collisions | length) > 0)
            | { lag: $row.name, collisions: $collisions }
          )
      )
    }
' "${IN_FILE}" >"${DERIVED}"

jq '{ lags: .lags }' "${DERIVED}" >"${OUT_FILE}"

AMBIGUOUS_COUNT="$(jq '.ambiguous_native_vlans_encountered | length' "${DERIVED}")"
if [ "${AMBIGUOUS_COUNT}" -gt 0 ]; then
  echo "==> WARNING: ${AMBIGUOUS_COUNT} LAG(s) have more than one untagged VLAN (invalid TMOS config)" >&2
  echo "    and were left with native_vlan=null in ${OUT_FILE}; review before applying:" >&2
  jq -c '.ambiguous_native_vlans_encountered[]' "${DERIVED}" | while IFS= read -r line; do
    echo "      - ${line}" >&2
  done
fi

UNMAPPED_COUNT="$(jq '.unmapped_member_names_encountered | length' "${DERIVED}")"
if [ "${UNMAPPED_COUNT}" -gt 0 ]; then
  echo "==> WARNING: ${UNMAPPED_COUNT} LAG member name(s) did not match the expected TMOS" >&2
  echo "    <blade>.<port> pattern and were passed through unchanged -- these are NOT valid" >&2
  echo "    rSeries names and must be renamed manually in ${OUT_FILE} before applying:" >&2
  jq -r '.unmapped_member_names_encountered[] | "      - LAG \(.lag): \(.tmos_name)"' "${DERIVED}" >&2
fi

UNRECOGNIZED_LACP_COUNT="$(jq '.unrecognized_lacp_settings_encountered | length' "${DERIVED}")"
if [ "${UNRECOGNIZED_LACP_COUNT}" -gt 0 ]; then
  echo "==> WARNING: ${UNRECOGNIZED_LACP_COUNT} LACP LAG(s) had an lacp_mode/lacp_timeout value" >&2
  echo "    that did not match the expected TMOS values (\"active\"/\"passive\" for lacp_mode," >&2
  echo "    \"long\"/\"short\" for lacp_timeout) -- mode/interval were set to null in ${OUT_FILE}" >&2
  echo "    (accepting the F5OS device default) rather than guessing. Review and set explicitly" >&2
  echo "    if a specific mode/interval is required:" >&2
  jq -r '.unrecognized_lacp_settings_encountered[] | "      - LAG \(.lag): lacp_mode=\(.raw_lacp_mode // "<absent>"), lacp_timeout=\(.raw_lacp_timeout // "<absent>")"' "${DERIVED}" >&2
fi

COLLISION_COUNT="$(jq '.member_name_collisions_encountered | length' "${DERIVED}")"
if [ "${COLLISION_COUNT}" -gt 0 ]; then
  echo "==> WARNING: ${COLLISION_COUNT} LAG(s) had two or more distinct TMOS member interfaces" >&2
  echo "    collapse to the same rSeries name after the blade number was dropped (this mapping" >&2
  echo "    assumes a single-blade i-Series/VE source). The colliding names were de-duplicated" >&2
  echo "    into a single members entry in ${OUT_FILE}. Resolve manually before applying:" >&2
  jq -c '.member_name_collisions_encountered[]' "${DERIVED}" | while IFS= read -r line; do
    echo "      - ${line}" >&2
  done
fi

echo "==> Wrote $(jq '.lags | length' "${OUT_FILE}") LAG(s) to ${OUT_FILE}" >&2
