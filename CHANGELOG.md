## 1.13.0 (Unreleased)

BREAKING CHANGES:
FEATURES:
* `f5os_tenant`: Added `max_nodes` (Optional/Computed) attribute for F5OS 2.0.0+ tenants; version-gated so it is not sent to devices below 2.0.0. Also exposes read-only `mgmt_vlan`, `mgmt_vlan_accessible`, and `clustering_as_service` state attributes reported by F5OS 2.0.0+
* `f5os_auth`: Added `login_policy` nested block (`admin_role_limit`, `restconf_max_session_limit`, `ssh_max_session_limit`) backed by the `f5-openconfig-aaa-login-policy` module; version-gated to F5OS 2.0.0+ (configuring it on older devices returns a clear error)
* `f5os_auth`: Extended `password_policy` with F5OS 2.0.0+ fields `min_days`, `remember`, and `warn_age`; version-gated so configuring them on devices below 2.0.0 returns a clear error
* `f5os_auth`: Added `ldap` nested block (`user_object_class`, `group_object_class` lists) backed by the `f5-openconfig-aaa-ldap` module; version-gated to F5OS 2.0.0+ (configuring it on older devices returns a clear error)
* `f5os_ntp_server`: Added `association_type` (String), `version` (Int64), and `port` (Int64) config attributes backed by the F5OS 2.0.0+ additive NTP server leaves; version-gated so configuring them on devices below 2.0.0 returns a clear "Unsupported attribute" error before any RESTCONF payload is sent. Also exposes read-only `stratum` (Int64), `authenticated` (Bool), and `state_address` (String) attributes populated from the device's NTP server state container on 2.0.0+ (null on older devices)
BUG FIXES:
* `f5os_logging`: On destroy, explicitly reset `include_hostname` to `false` via PUT instead of relying on DELETE of the `f5-openconfig-system-logging:config` container. On some F5OS versions DELETE did not clear the leaf, leaving `include-hostname` set to `true` after the resource was destroyed. Includes a DELETE fallback for older F5OS versions where PUT on the container leaf is not supported
IMPROVEMENTS:
* CI/CD: Bumped Go toolchain from 1.25.10 to 1.25.12 to remediate standard library vulnerabilities GO-2026-5856 (crypto/tls), GO-2026-5039 (net/textproto), and GO-2026-5037 (crypto/x509) flagged by `govulncheck`
* CI/CD: Re-enabled the GitHub Actions `govulncheck` job as blocking (removed `continue-on-error`) now that the toolchain upgrade clears all known standard library vulnerabilities
* CI/CD: Pinned `GOTOOLCHAIN=go1.25.12` on the `govulncheck` install step in both GitHub Actions and GitLab CI so the scanner is built with a toolchain able to parse the module's `go 1.25` sources

SECURITY:
* Upgraded `golang.org/x/net` from v0.55.0 to v0.56.0 to remediate GO-2026-5942 (panic parsing invalid SVCB/HTTPS DNS records in `golang.org/x/net/dns/dnsmessage`)
* Upgraded `golang.org/x/text` from v0.24.0 to v0.39.0 to remediate GO-2026-5970 (infinite loop on invalid input in `golang.org/x/text`)
* Upgraded transitive dependencies pulled in by the above: `golang.org/x/crypto` v0.37.0 → v0.53.0, `golang.org/x/sys` v0.44.0 → v0.46.0, `golang.org/x/mod` v0.17.0 → v0.37.0, `google.golang.org/protobuf` v1.34.1 → v1.36.10, `github.com/google/go-cmp` v0.6.0 → v0.7.0

## 1.12.0

BREAKING CHANGES:
* Minimum Go version is now 1.25 (upgraded from 1.23). Contributors and CI environments must use Go 1.25+.

FEATURES:
* `f5os_lag`: LAG resource now accepts LACP or STATIC mode
* CI/CD: Split acceptance tests into 22 sequential per-resource jobs with `resource_group` mutex for safe shared-device testing
* CI/CD: Added coverage threshold enforcement (75%) to both GitHub Actions and GitLab CI via shared `scripts/check-coverage.sh`
* CI/CD: Added `cve-scan` job using `govulncheck` — runs on every MR, push to default branch, tag, and schedule; failures block the pipeline

BUG FIXES:
* `f5os_user`: Password update now uses admin set-password endpoint, fixing failures when provider does not have the user's old password

IMPROVEMENTS:
* Aligned GitHub Actions unit test job timeout with GitLab CI (30m to 65m)
* CI/CD: Made GitHub Actions `govulncheck` job non-blocking (`continue-on-error`) pending Go toolchain upgrade to 1.25.11
* Removed redundant `go:build` CI job (compilation already covered by lint and unit test jobs)
* Added unit tests for `common.go` utility functions
* Added unit tests for `f5os_tenant_image` data source
* Increased unit test coverage across all resources to 80%+ target:
  - `f5os_auth_resource`: 80%
  - `f5os_logging_resource`: 80%
  - `f5os_snmp_resource`: 0.8% → 80%
  - `f5os_ntp_server_resource`: 2.8% → 80%
  - `f5os_qkview_resource`: 1.5% → 80%
  - `f5os_system_resource`: 0.8% → 80%
  - `f5os_tls_cert_key_resource`: 4.9% → 80%
  - `attribute_plan_modifier`: 0% → 80%
  - `config_backup_resource`: 74.2% → 80%
  - `device_info_data_source`: 73.1% → 80%
  - `interface_resource`: 2.9% → 80%
  - `lag_resource`: 1.8% → 80%
  - `license_resource`: 4.8% → 80%
  - `partition_resource`: 1.4% → 80%
  - `partition_change_password_resource`: added coverage
  - `primarykey_resource`: 2.7% → 80%
  - `tenant_resource`: 1.4% → 80%
  - `tenant_image_resource`: 2.8% → 80%
  - `user_resource`: expanded coverage
  - `user_password_change_resource`: added coverage
  - `vlan_resource`: 3.3% → 80%

SECURITY:
* Upgraded Go from 1.23.6 to 1.25.10
* Upgraded `golang.org/x/net` from v0.39.0 to v0.55.0 to remediate CVE vulnerabilities
* Upgraded `golang.org/x/sys` from v0.32.0 to v0.44.0 to remediate CVE vulnerabilities
* Upgraded `google.golang.org/grpc` from v1.65.0 to v1.79.3 to remediate CVE vulnerabilities

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