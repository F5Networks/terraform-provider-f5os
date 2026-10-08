---
page_title: "iSeries to rSeries migration operator checklist"
description: |-
  Quick operator checklist for executing the target-side F5OS phases of an iSeries to rSeries migration.
---

# iSeries to rSeries migration operator checklist

Use this as the short operational companion to the full
[migration flow](iseries-to-rseries-migration-flow.html).

## Prerequisites

1. Obtain the source-side outputs from `terraform-provider-bigip`:
   - inventory output
   - extracted settings JSON
   - interface/VLAN mapping JSON
   - UCS backup
2. Confirm target F5OS/rSeries admin access.
3. Obtain a new registration key for the target device.
4. Stage or download the tenant image selected from source Phase 0.
5. Have the Phase 2 UCS archive ready to upload into the deployed BIG-IP tenant.

## Target-side sequence

1. Apply the F5OS license.
2. Apply system settings if they are being migrated.
3. Create VLANs.
4. Configure interfaces.
5. Configure LAGs.
6. Deploy tenant resources.
7. Upload the UCS into BIG-IP and load it with the platform-migrate option.

## Inputs to prepare

1. System settings variables from `scripts/system-settings-from-iseries.sh`
2. VLAN variables from `scripts/vlans-from-iseries.sh`
3. Interface variables from `scripts/interfaces-from-iseries.sh`
4. LAG variables from `scripts/lags-from-iseries.sh`
5. Tenant sizing, management IP, image, and VLAN attachment values chosen by
   the operator

## Validation checkpoints

1. License activation succeeded.
2. DNS/NTP/SNMP/auth/local users are present as intended.
3. VLANs exist with the expected names and IDs.
4. Interfaces use the expected rSeries naming and VLAN assignments.
5. LAGs use the expected members and VLAN assignments.
6. Tenant image/version matches the migration inventory decision.
7. Tenant management IPs are reachable.
8. Tenant VLAN attachments align with the intended platform networking.
9. UCS load completed successfully inside BIG-IP with platform migrate enabled.

## Common migration cautions

1. rSeries interface names differ from TMOS/iSeries names.
2. Port-group planning may be required before interface use on rSeries.
3. Tenant sizing is a target-side design decision, not an extracted source
   field.
4. Licensing is a new target-device action, not a converted source artifact.
5. The UCS restore happens only after the tenant is deployed, and BIG-IP must
   load it with the platform-migrate option for this migration path.

## Related guides

1. [Overall iSeries to rSeries migration flow](iseries-to-rseries-migration-flow.html)
2. [Applying a license to F5OS as part of an iSeries migration](apply-license-from-iseries.html)
3. [Configuring system settings on F5OS from discovered iSeries configuration](configure-system-settings-from-iseries.html)
4. [Creating VLANs on F5OS from discovered iSeries configuration](create-vlans-from-iseries.html)
5. [Configuring F5OS interfaces from discovered iSeries configuration](configure-interfaces-from-iseries.html)
6. [Configuring F5OS LAGs from discovered iSeries configuration](configure-lags-from-iseries.html)
7. [Deploying F5OS tenants from discovered iSeries configuration](deploy-tenants-from-iseries.html)
