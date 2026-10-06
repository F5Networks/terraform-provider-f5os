#!/usr/bin/env bash
#
# interfaces-from-iseries.sh -- Phase 4 (input prep) of the i-Series ->
# r-Series (F5OS) migration workflow. Converts the interface/VLAN
# membership portion of the `extracted-sys-settings.json` produced by
# terraform-provider-bigip's `scripts/extract-sys-settings.sh` (that
# repo's Phase 1) into an `interfaces.auto.tfvars.json` file shaped for
# this repo's examples/migration/vlans-from-iseries/ configuration, which
# creates the equivalent f5os_interface resources via
# `for_each = var.interfaces`.
#
# Usage:
#   ./scripts/interfaces-from-iseries.sh [extracted-sys-settings.json] [interfaces.auto.tfvars.json]
#
# Arguments (both optional, positional):
#   extracted-sys-settings.json    Input file produced by
#                                   terraform-provider-bigip's
#                                   extract-sys-settings.sh. Defaults to
#                                   extracted-sys-settings.json in the
#                                   current directory.
#   interfaces.auto.tfvars.json    Output file. Defaults to
#                                   interfaces.auto.tfvars.json in the
#                                   current directory. Terraform
#                                   automatically loads any
#                                   *.auto.tfvars(.json) file found in
#                                   the working directory, so no
#                                   explicit -var-file flag is needed if
#                                   this output is placed alongside
#                                   main.tf.
#
# Input shapes consumed (from terraform-provider-bigip's
# extract-sys-settings.sh):
#   - data.bigip_net_interfaces.extracted.values.interfaces[] --
#     physical interface name/enabled state.
#   - data.bigip_net_vlans.extracted.values.vlans[] -- each VLAN's tag
#     and its tagged/untagged interfaces[] membership list.
#   - data.bigip_net_trunks.extracted.values.trunks[]?.name -- trunk
#     (LAG) names, used only to identify which VLAN-membership entries
#     belong to a LAG rather than a physical interface (see "Trunks
#     (LAGs) are out of scope" below).
#
# Output shape (this repo's examples/migration/vlans-from-iseries
# var.interfaces): a map keyed by the rSeries interface name (see
# "Interface name mapping" below), each value shaped for
# f5os_interface's native_vlan/trunk_vlans/enabled arguments directly:
#   {
#     "interfaces": {
#       "1.0": { "native_vlan": 100, "trunk_vlans": [], "enabled": true },
#       "2.0": { "native_vlan": 101, "trunk_vlans": [], "enabled": true }
#     }
#   }
#
# Interface name mapping (TMOS i-Series -> F5OS-A rSeries):
#   TMOS names physical interfaces `<blade>.<port>` (e.g. `1.1`, `1.2`);
#   single-appliance i-Series/VE platforms are always blade `1`. rSeries
#   (F5OS-A) has no blade concept at all -- it maps to `<port>.0` with
#   the blade number dropped entirely (not reformatted -- dropped). See
#   https://registry.terraform.io/providers/F5Networks/f5os/latest/docs/guides/create-vlans-from-iseries
#   and terraform-provider-bigip's
#   https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping
#   for the full naming-convention reference, including the VELOS
#   (`<blade>/<port>.<subport>`) form this script does NOT produce.
#
#   Any physical interface name that doesn't match the expected TMOS
#   `<blade>.<port>` pattern is passed through unchanged into the output
#   map (so it isn't silently dropped) and reported separately under
#   `.unmapped_names_encountered` printed to stderr, for manual review
#   -- it will not be a valid rSeries name and will need to be renamed
#   by hand before applying.
#
# The dedicated management interface ("mgmt") is excluded entirely: it
# is not part of TMOS's numbered `<blade>.<port>` interface scheme, has
# no rSeries data-plane equivalent to map to, and (per BIG-IP's own
# addressing model) is never a member of a VLAN's tagged/untagged
# interface list, so it never needs native_vlan/trunk_vlans translation
# in the first place.
#
# Ambiguous native VLANs: a physical interface with more than one
# *untagged* VLAN membership is an invalid TMOS configuration this
# script can't silently resolve into a single native_vlan -- those are
# left with native_vlan=null in the output map and reported separately
# under `.ambiguous_native_vlans_encountered` (printed to stderr) for
# manual review, matching terraform-provider-bigip's
# extract-sys-settings.sh interface_vlans derivation behavior.
#
# Trunks (LAGs) are out of scope: F5OS represents a LAG as an
# `f5os_lag` resource, not `f5os_interface` -- a TMOS trunk name has no
# corresponding entry in bigip_net_interfaces (it isn't a physical
# interface) and is therefore never emitted into this script's
# `interfaces` output map. Any such name's VLAN membership (native/trunk
# VLANs it would need on the eventual f5os_lag resource) is still
# surfaced separately under `.trunk_only_names_encountered` (printed to
# stderr) so it isn't silently lost -- see
# terraform-provider-bigip's
# https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping
# for
# translating trunk membership to `f5os_lag`.
#
# rSeries name collisions: dropping the blade number means two TMOS
# interfaces on *different* blades but the same port (e.g. `1.1` and
# `2.1` on a multi-blade/chassis-style source) both map to the same
# rSeries name (`1.0`), and only one survives in the output map (the
# other's native/trunk VLAN config is silently lost). This mapping is
# only meaningful for genuinely single-blade i-Series/VE sources in the
# first place (see "Interface name mapping" above) -- any such
# collision is reported under `.rseries_name_collisions_encountered`
# (printed to stderr) so it isn't a silent data loss, and must be
# resolved manually (this script cannot guess which TMOS interface
# should own the resulting rSeries name).

set -euo pipefail

command -v jq >/dev/null || {
  echo "jq not found in PATH" >&2
  exit 1
}

IN_FILE="${1:-extracted-sys-settings.json}"
OUT_FILE="${2:-interfaces.auto.tfvars.json}"

if [ ! -f "${IN_FILE}" ]; then
  echo "ERROR: input file not found: ${IN_FILE}" >&2
  exit 1
fi

IFACE_COUNT="$(jq '[ .[] | select(.type == "bigip_net_interfaces") | .values.interfaces[]? ] | length' "${IN_FILE}")"

if [ "${IFACE_COUNT}" -eq 0 ]; then
  echo "WARNING: no interfaces found under a bigip_net_interfaces entry in ${IN_FILE}." >&2
  echo "         (extract-sys-settings.sh in registry-fallback mode omits bigip_net_interfaces" >&2
  echo "         entirely -- re-run it with a locally built provider to include interfaces.)" >&2
fi

VLAN_COUNT="$(jq '[ .[] | select(.type == "bigip_net_vlans") | .values.vlans[]? ] | length' "${IN_FILE}")"

if [ "${VLAN_COUNT}" -eq 0 ]; then
  echo "WARNING: no VLANs found under a bigip_net_vlans entry in ${IN_FILE}." >&2
  echo "         (extract-sys-settings.sh in registry-fallback mode omits bigip_net_vlans" >&2
  echo "         entirely -- every interface will be emitted with native_vlan=null and an" >&2
  echo "         empty trunk_vlans, not because the source device has no VLAN memberships." >&2
  echo "         Re-run it with a locally built provider to include VLAN membership data.)" >&2
fi

# The jq program is intentionally kept in one pass so every derived
# value (mapped name, native/trunk VLAN classification, diagnostics)
# comes from a single consistent read of the input, and written to a
# temp file so the diagnostics jq calls below don't need to re-derive
# any of it.
DERIVED="$(mktemp)"
trap 'rm -f "${DERIVED}"' EXIT

jq '
  ( [ .[] | select(.type == "bigip_net_vlans") | .values.vlans[]? ] ) as $vlans
  | ( [ .[] | select(.type == "bigip_net_trunks") | .values.trunks[]?.name ] ) as $trunk_names
  | ( [ .[] | select(.type == "bigip_net_interfaces") | .values.interfaces[]? ] ) as $phys
  | ( [ $vlans[] as $vlan | ($vlan.interfaces // [])[] | {interface: .name, tag: $vlan.tag, tagged: .tagged} ] ) as $memberships
  | [ $phys[] | select(.name != "mgmt") ] as $data_phys
  | ( $data_phys | map(.name) ) as $data_phys_names
  # Trunk (LAG) names with no bigip_net_interfaces entry of their own
  # map to f5os_lag, not f5os_interface -- surfaced separately, not
  # emitted into the interfaces map (see header comment above).
  | (
      [ $trunk_names[] | select(. as $t | $data_phys_names | index($t) == null) ]
      | map(
          . as $name
          | {
              name: $name,
              native_vlans: ( [ $memberships[] | select(.interface == $name and .tagged == false) | .tag ] | unique ),
              trunk_vlans: ( [ $memberships[] | select(.interface == $name and .tagged == true) | .tag ] | unique )
            }
        )
    ) as $trunk_only
  | (
      $data_phys
      | map(
          . as $iface
          # capture() produces no output at all (not null) on a
          # non-match; wrapping in [ ... ] | first turns that into a
          # real null instead of silently vanishing from the map()
          # output (which would otherwise drop this interface from the
          # result entirely rather than reporting it as unmapped).
          | ( [ $iface.name | capture("^(?<blade>[0-9]+)\\.(?<port>[0-9]+)$") ] | first ) as $parts
          | {
              tmos_name: $iface.name,
              rseries_name: (if $parts != null then "\($parts.port).0" else $iface.name end),
              mapped: ($parts != null),
              enabled: $iface.enabled,
              native_vlans: ( [ $memberships[] | select(.interface == $iface.name and .tagged == false) | .tag ] | unique ),
              trunk_vlans: ( [ $memberships[] | select(.interface == $iface.name and .tagged == true) | .tag ] | unique )
            }
        )
    ) as $rows
  | {
      interfaces: (
        $rows
        | map({
            (.rseries_name): {
              native_vlan: (if (.native_vlans | length) > 1 then null else (.native_vlans[0] // null) end),
              trunk_vlans: .trunk_vlans,
              enabled: .enabled
            }
          })
        | add // {}
      ),
      ambiguous_native_vlans_encountered: [ $rows[] | select((.native_vlans | length) > 1) | { interface: .rseries_name, tmos_name: .tmos_name, native_vlans: .native_vlans } ],
      unmapped_names_encountered: [ $rows[] | select(.mapped == false) | .tmos_name ],
      trunk_only_names_encountered: $trunk_only,
      # Two (or more) distinct TMOS names mapping to the same
      # rseries_name (see "rSeries name collisions" in the header
      # comment) -- group by rseries_name and keep only groups with
      # more than one member.
      rseries_name_collisions_encountered: (
        $rows
        | group_by(.rseries_name)
        | map(select(length > 1))
        | map({ rseries_name: .[0].rseries_name, tmos_names: [ .[].tmos_name ] })
      )
    }
' "${IN_FILE}" >"${DERIVED}"

jq '{ interfaces: .interfaces }' "${DERIVED}" >"${OUT_FILE}"

AMBIGUOUS_COUNT="$(jq '.ambiguous_native_vlans_encountered | length' "${DERIVED}")"
if [ "${AMBIGUOUS_COUNT}" -gt 0 ]; then
  echo "==> WARNING: ${AMBIGUOUS_COUNT} interface(s) have more than one untagged VLAN (invalid TMOS config)" >&2
  echo "    and were left with native_vlan=null in ${OUT_FILE}; review before applying:" >&2
  jq -c '.ambiguous_native_vlans_encountered[]' "${DERIVED}" | while IFS= read -r line; do
    echo "      - ${line}" >&2
  done
fi

UNMAPPED_COUNT="$(jq '.unmapped_names_encountered | length' "${DERIVED}")"
if [ "${UNMAPPED_COUNT}" -gt 0 ]; then
  echo "==> WARNING: ${UNMAPPED_COUNT} interface name(s) did not match the expected TMOS <blade>.<port>" >&2
  echo "    pattern and were passed through unchanged -- these are NOT valid rSeries names and must" >&2
  echo "    be renamed manually in ${OUT_FILE} before applying:" >&2
  jq -r '.unmapped_names_encountered[] | "      - \(.)"' "${DERIVED}" >&2
fi

TRUNK_ONLY_COUNT="$(jq '.trunk_only_names_encountered | length' "${DERIVED}")"
if [ "${TRUNK_ONLY_COUNT}" -gt 0 ]; then
  echo "==> NOTE: ${TRUNK_ONLY_COUNT} trunk (LAG) name(s) had VLAN membership but map to f5os_lag," >&2
  echo "    not f5os_interface, so they were NOT included in ${OUT_FILE}. Translate these to" >&2
  echo "    f5os_lag's native_vlan/trunk_vlans manually:" >&2
  jq -c '.trunk_only_names_encountered[]' "${DERIVED}" | while IFS= read -r line; do
    echo "      - ${line}" >&2
  done
fi

COLLISION_COUNT="$(jq '.rseries_name_collisions_encountered | length' "${DERIVED}")"
if [ "${COLLISION_COUNT}" -gt 0 ]; then
  echo "==> WARNING: ${COLLISION_COUNT} rSeries interface name(s) were produced by more than one" >&2
  echo "    distinct TMOS interface (dropping the blade number collapsed them together -- this" >&2
  echo "    mapping assumes a single-blade i-Series/VE source). Only ONE of each group's" >&2
  echo "    native_vlan/trunk_vlans/enabled ended up in ${OUT_FILE}; the others were DROPPED." >&2
  echo "    Resolve manually before applying:" >&2
  jq -c '.rseries_name_collisions_encountered[]' "${DERIVED}" | while IFS= read -r line; do
    echo "      - ${line}" >&2
  done
fi

echo "==> Wrote $(jq '.interfaces | length' "${OUT_FILE}") interface(s) to ${OUT_FILE}" >&2
