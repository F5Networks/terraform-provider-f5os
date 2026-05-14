## 1.12.0 (Unreleased)

BREAKING CHANGES:
FEATURES:
BUG FIXES:
IMPROVEMENTS:

## 1.11.1

FEATURES:
* `f5os_auth`: Added `password_policy` support for managing password policy configuration
* `schemadiff`: New tool to diff and report on schema changes between F5OS versions
* CI/CD: Added support for scheduled releases

BUG FIXES:
* `f5os_system`: Only manage optional system settings that are explicitly configured
* `f5os_system`: Prevent panic on nil SshdIdleTimeout type assertion during state read
* `f5os_dns`: Read method now properly refreshes state from device
* `f5os_dns`: Delete preserves device config instead of incorrectly removing entries
* `f5os_dns`: Delete stale entries on update before patching
* `f5os_dns`: Fix null search domain entry handling
* `f5os_dns`: Fix null read response handling
* `f5os_tenant`: Deployment file now properly mapped in Read/state refresh
* `f5os_tenant`: Type attribute now properly mapped in Read/state refresh
* `f5os_tenant`: VLANs now properly mapped in Read/state refresh
* `f5os_tenant_image`: Fix panic on nil map during importWait
* `f5os_tenant_image`: Read/Import now preserves all config attributes in state
* `f5os_tenant_image`: GetImage error no longer silently swallowed during Create
* `f5os_tenant_image`: Fix broken timeout calculation for upload path
* `f5os_tenant_image`: Add conflict validator between upload_from_path and remote_path
* `f5os_tenant_image`: Update is no longer a silent no-op
* `f5os_tenant_image`: Fix Remote Import insecure attribute handling
* `f5os_tenant_image`: Fix protocol, remote_user, remote_password, and remote_port properties
* `f5os_ntp_server`: Fix duplicate NTPServerModel type definition
* `f5os_ntp_server`: CreateNTPServerPayload no longer drops key_id=0 due to omitempty
* `f5os_ntp_server`: Added Terraform import support
* `f5os_ntp_server`: Fix ntp_service and ntp_authentication not being written to device
* `f5os_primarykey`: Fix force_update=false skip logic that never triggered
* `f5os_primarykey`: Fix SDK JSON deserialization bug causing empty status
* `f5os_primarykey`: Stabilize post-apply refresh for async primary key migration
* `f5os_user`: Role update now removes old role assignment, eliminating state drift
* `f5os_user`: Fix revert of role GID changes during delete
* `f5os_auth`: Fix auth_order not populating during import
* `f5os_auth`: Fix SetRoleConfig failure
* `f5os_auth`: Fix restore of original auth_order during delete
* `f5os_auth`: Fix device role filtering
* `f5os_auth`: Query auth resource after create/update for accurate state
* `f5os_auth`: Fix JSON parsing on F5OS 1.8.3
* `f5os_snmp`: Delete now properly resets MIB fields

IMPROVEMENTS:
* Added unit and acceptance tests for partition_change_password resource
* Added configurable PollInterval to f5osclient, reducing unit test runtime from ~24 minutes to ~7 minutes
* Documentation updates

## 1.5.1

BREAKING CHANGES:
FEATURES:
* **resources/f5os_user_password_change:** New Resource added for changing F5OS user passwords.
IMPROVEMENTS:

## 1.5.0

BREAKING CHANGES:
FEATURES:
* **data-sources/f5os_device_info:** New Data source added.
IMPROVEMENTS: