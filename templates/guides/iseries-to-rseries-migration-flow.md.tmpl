---
page_title: "i-Series to r-Series migration flow across terraform-provider-bigip and terraform-provider-f5os"
description: |-
  End-to-end phase flow for migrating a BIG-IP i-Series device to an r-Series/F5OS target using source-side discovery in terraform-provider-bigip and target-side configuration in terraform-provider-f5os.
---

# i-Series to r-Series migration flow across terraform-provider-bigip and terraform-provider-f5os

This document describes the end-to-end i-Series to r-Series migration flow
using the phases already defined in:

- `terraform-provider-bigip`
- `terraform-provider-f5os`

The workflow is split across the two repos by design:

- `terraform-provider-bigip` handles source-device inventory, extraction, and
  backup.
- `terraform-provider-f5os` handles target-platform licensing,
  system-settings, network-layer configuration, and tenant deployment.

## Phase map

| Phase | Repo | Purpose |
|---|---|---|
| Phase 0 | `terraform-provider-bigip` | Inventory TMOS version/build and hardware; select the correct tenant image |
| Phase 1 | `terraform-provider-bigip` | Extract i-Series system and network settings into JSON artifacts |
| Phase 2 | `terraform-provider-bigip` | Generate and download a UCS backup from the source device |
| Licensing phase | `terraform-provider-f5os` | Apply a new F5OS/r-Series platform license |
| System-settings phase | `terraform-provider-f5os` | Configure target DNS, NTP, SNMP, auth order, and local platform users |
| Phase 3 | `terraform-provider-f5os` | Create target VLANs |
| Phase 4 | `terraform-provider-f5os` | Configure target interfaces |
| Phase 5 | `terraform-provider-f5os` | Configure target LAGs |
| Phase 6 | `terraform-provider-f5os` | Deploy BIG-IP tenants |

## Cross-repo flow

### Source-side phases in terraform-provider-bigip

#### Phase 0: inventory TMOS version and hardware

- run `scripts/inventory-tmos-version.sh`
- identify the exact source TMOS version/build and hardware model
- derive the recommended r-Series tenant image filename pattern
- confirm whether the source TMOS train is suitable for r-Series tenant use

#### Phase 1: extract source settings

- run `scripts/extract-sys-settings.sh`
- extract system and network settings including VLANs, interfaces, and trunks
- produce JSON artifacts consumed by F5OS conversion scripts

#### Phase 2: generate UCS backup

- run `scripts/generate-ucs-backup.sh`
- create and download a UCS archive from the source i-Series device

## Target-side phases in terraform-provider-f5os

### Licensing phase

- apply a new target-device registration key using
  `examples/migration/license-from-iseries/`
- this is independent of the source extraction output

### System-settings phase

- use `examples/migration/system-settings-from-iseries/`
- feed it converted output from Phase 1 via
  `scripts/system-settings-from-iseries.sh`
- configure target DNS, NTP, SNMP, auth order, and local users

### Phase 3: create VLANs

- use `examples/migration/vlans-from-iseries/main.tf`
- feed it converted output from Phase 1 via `scripts/vlans-from-iseries.sh`

### Phase 4: configure interfaces

- use `examples/migration/vlans-from-iseries/interfaces.tf`
- feed it converted output from Phase 1 via
  `scripts/interfaces-from-iseries.sh`
- depends on Phase 3 VLAN creation

### Phase 5: configure LAGs

- use `examples/migration/vlans-from-iseries/lags.tf`
- feed it converted output from Phase 1 via `scripts/lags-from-iseries.sh`
- depends on Phase 3 VLAN creation

### Phase 6: deploy tenants

- use `examples/migration/vlans-from-iseries/tenant.tf`
- use the tenant image/version chosen in Phase 0
- attach the VLANs created in Phase 3
- supply operator-chosen sizing and management settings

## Recommended execution order

1. Phase 0 in `terraform-provider-bigip`
2. Phase 1 in `terraform-provider-bigip`
3. Phase 2 in `terraform-provider-bigip`
4. licensing phase in `terraform-provider-f5os`
5. system-settings phase in `terraform-provider-f5os`
6. Phase 3 VLAN creation
7. Phase 4 interface configuration
8. Phase 5 LAG configuration
9. Phase 6 tenant deployment

## Parallel work that is safe

- licensing can be done before or in parallel with the system-settings and
  VLAN/interface/LAG workflows
- system-settings is independent of the VLAN/interface/LAG workflow
- interface and LAG preparation both depend on VLAN creation, but the exact
  operational sequencing can still be planned around the target design

## Important caveats

- i-Series license keys are not reused on r-Series; obtain new registration
  keys for the target device.
- TMOS interface names do not map directly to r-Series names; use the
  interface/trunk mapping guidance from `terraform-provider-bigip`.
- tenant sizing is not auto-derived from the source i-Series device and must
  be planned explicitly.
- the target data plane is only fully useful once VLANs, interfaces/LAGs, and
  tenant attachments are all aligned.

## Related guides

- [Inventorying TMOS version and hardware](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/inventory-tmos-version) (Phase 0, `terraform-provider-bigip`)
- [Extracting i-Series system settings](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/extract-sys-settings) (Phase 1, `terraform-provider-bigip`)
- [Generating and downloading a UCS backup](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/generate-ucs-backup) (Phase 2, `terraform-provider-bigip`)
- [Interface and trunk naming: TMOS (i-Series) vs F5OS (r-Series/VELOS)](https://registry.terraform.io/providers/F5Networks/bigip/latest/docs/guides/interface-trunk-mapping) (`terraform-provider-bigip`)
- [Applying a license to F5OS as part of an i-Series migration](apply-license-from-iseries.html) (`terraform-provider-f5os`)
- [Configuring system settings on F5OS from discovered i-Series configuration](configure-system-settings-from-iseries.html) (`terraform-provider-f5os`)
- [Creating VLANs on F5OS from discovered i-Series configuration](create-vlans-from-iseries.html) (`terraform-provider-f5os`)
- [Configuring F5OS interfaces from discovered i-Series configuration](configure-interfaces-from-iseries.html) (`terraform-provider-f5os`)
- [Configuring F5OS LAGs from discovered i-Series configuration](configure-lags-from-iseries.html) (`terraform-provider-f5os`)
- [Deploying F5OS tenants from discovered i-Series configuration](deploy-tenants-from-iseries.html) (`terraform-provider-f5os`)
