#!/usr/bin/env bash
#
# vlans-from-iseries.sh -- Phase 3 (input prep) of the i-Series -> r-Series
# (F5OS) migration workflow. Converts the VLAN portion of the
# `extracted-sys-settings.json` produced by terraform-provider-bigip's
# `scripts/extract-sys-settings.sh` (that repo's Phase 1) into a
# `vlans.auto.tfvars.json` file shaped for this repo's
# examples/migration/vlans-from-iseries/ configuration, which creates the
# equivalent f5os_vlan resources via `for_each = var.vlans`.
#
# Usage:
#   ./scripts/vlans-from-iseries.sh [extracted-sys-settings.json] [vlans.auto.tfvars.json]
#
# Arguments (both optional, positional):
#   extracted-sys-settings.json  Input file produced by
#                                 terraform-provider-bigip's
#                                 extract-sys-settings.sh. Defaults to
#                                 extracted-sys-settings.json in the
#                                 current directory.
#   vlans.auto.tfvars.json       Output file. Defaults to
#                                 vlans.auto.tfvars.json in the current
#                                 directory. Terraform automatically loads
#                                 any *.auto.tfvars(.json) file found in
#                                 the working directory, so no explicit
#                                 -var-file flag is needed if this output
#                                 is placed alongside main.tf.
#
# Input shape (from terraform-provider-bigip's extract-sys-settings.sh):
#   the `.values.vlans` array on the `data.bigip_net_vlans.extracted`
#   entry of extracted-sys-settings.json, e.g.:
#     {
#       "address": "data.bigip_net_vlans.extracted",
#       "type": "bigip_net_vlans",
#       "values": {
#         "vlans": [
#           { "name": "extracted-external", "tag": 100, ... },
#           { "name": "extracted-internal", "tag": 101, ... }
#         ]
#       }
#     }
#
# Output shape (this repo's examples/migration/vlans-from-iseries var.vlans):
#   { "vlans": { "extracted-external": 100, "extracted-internal": 101 } }
#
# Both VLAN name and VLAN ID/tag are preserved verbatim from the source
# i-Series device -- this script performs no renaming or renumbering.
#
# Note: TMOS VLAN names are commonly full paths (e.g. "/Common/vlan100").
# f5os_vlan's `name` has no partition concept and a 58-character limit, so
# names with a leading "/Common/" (or other partition prefix) are
# stripped down to their final path segment before being used as the F5OS
# VLAN name; anything still longer than 58 characters, or that doesn't
# start with a letter, is left as-is with a warning printed to stderr so
# it can be fixed manually before applying to F5OS -- see
# docs/guides/create-vlans-from-iseries.md.

set -euo pipefail

command -v jq >/dev/null || {
  echo "jq not found in PATH" >&2
  exit 1
}

IN_FILE="${1:-extracted-sys-settings.json}"
OUT_FILE="${2:-vlans.auto.tfvars.json}"

if [ ! -f "${IN_FILE}" ]; then
  echo "ERROR: input file not found: ${IN_FILE}" >&2
  exit 1
fi

VLAN_COUNT="$(jq '[ .[] | select(.type == "bigip_net_vlans") | .values.vlans[]? ] | length' "${IN_FILE}")"

if [ "${VLAN_COUNT}" -eq 0 ]; then
  echo "WARNING: no VLANs found under a bigip_net_vlans entry in ${IN_FILE}." >&2
  echo "         (extract-sys-settings.sh in registry-fallback mode omits bigip_net_vlans" >&2
  echo "         entirely -- re-run it with a locally built provider to include VLANs.)" >&2
fi

# Strip a leading partition path (e.g. "/Common/") down to the final path
# segment, since f5os_vlan's name attribute has no partition concept.
# Warn (to stderr) on any name that still doesn't fit f5os_vlan's
# constraints (first character must be a letter, remaining characters
# alphanumeric/period/comma/hyphen/underscore only, max 58 characters --
# see internal/provider/vlan_resource.go's `name` schema description) so
# it can be reviewed manually, but still emit it -- Terraform/the device
# will surface a clear validation error on apply if left unfixed.
#
# Stripping the partition prefix can collapse two distinct source VLANs
# into the same name (e.g. "/Common/vlan100" and "/Partition2/vlan100"
# both become "vlan100"), which would otherwise silently drop one VLAN
# from the output map (the last one wins on key collision). Detect and
# warn about any such collision before writing the map, so a VLAN isn't
# dropped from migration without the operator knowing.
jq -r '
  [ .[] | select(.type == "bigip_net_vlans") | .values.vlans[]? ]
  | map({ orig: .name, name: (.name | split("/") | last), tag: .tag })
  | group_by(.name)
  | map(select(length > 1))
  | .[]
  | "WARNING: VLAN name collision after stripping partition prefix: " +
    ([ .[] | "\"\(.orig)\" (tag \(.tag))" ] | join(" and ")) +
    " all map to \"\(.[0].name)\" -- only the last one (\"\(.[-1].orig)\") will be kept, the rest will NOT be migrated."
' "${IN_FILE}" >&2

# Two (or more) distinct VLANs sharing the same numeric tag are not
# caught by the name-collision check above (they can have entirely
# different names) but will fail on `terraform apply` regardless -- F5OS
# requires VLAN IDs to be unique, while TMOS scopes VLAN IDs per route
# domain/partition and so can legitimately have two VLANs on the same
# tag under different partitions. Warn here, at the earliest point the
# conflict is knowable, instead of leaving it to surface as a confusing
# runtime error from the device.
jq -r '
  [ .[] | select(.type == "bigip_net_vlans") | .values.vlans[]? ]
  | group_by(.tag)
  | map(select(length > 1))
  | .[]
  | "WARNING: Duplicate VLAN tag \(.[0].tag): " +
    ([ .[] | "\"\(.name)\"" ] | join(" and ")) +
    " share the same tag; F5OS requires unique VLAN IDs -- resolve before applying."
' "${IN_FILE}" >&2

jq '
  [ .[] | select(.type == "bigip_net_vlans") | .values.vlans[]? ]
  | map({
      name: (.name | split("/") | last),
      tag: .tag
    })
  | { vlans: ( map({ (.name): .tag }) | add // {} ) }
' "${IN_FILE}" >"${OUT_FILE}"

jq -r '
  .vlans
  | to_entries[]
  | select((.key | test("^[A-Za-z][A-Za-z0-9.,_-]*$") | not) or (.key | length) > 58)
  | "WARNING: VLAN name \"\(.key)\" does not meet f5os_vlan constraints (must start with a letter, allowed characters: alphanumeric, \u0027.\u0027, \u0027,\u0027, \u0027-\u0027, \u0027_\u0027, max 58 characters) -- review before applying."
' "${OUT_FILE}" >&2 || true

echo "==> Wrote $(jq '.vlans | length' "${OUT_FILE}") VLAN(s) to ${OUT_FILE}" >&2
