# AGENTS.md

## Project

Terraform provider for F5OS (VELOS partitions and rSeries appliances). Uses the
Terraform Plugin Framework (not SDKv2). The F5OS API client lives in a vendored
module at `vendor/gitswarm.f5net.com/terraform-providers/f5osclient/`.

- Module path: `gitswarm.f5net.com/terraform-providers/terraform-provider-f5os`
- Go 1.25.0, Plugin Protocol 6
- Provider registry address: `registry.terraform.io/f5networks/f5os`

## Layout

```
main.go                          # Entrypoint; registers provider via providerserver
internal/provider/               # All resources, data sources, provider config
internal/provider/fixtures/      # JSON fixtures for unit/mock tests
cmd/schemadiff/                  # Tool to diff and report schema changes between F5OS versions
vendor/                          # Vendored deps (committed to repo)
docs/                            # Auto-generated; do not hand-edit
templates/                       # Templates for doc generation
examples/                        # Example HCL consumed by doc generator
.agents/skills/                  # OpenCode skill definitions (acc-test, release, qrt)
```

All resources and data sources are in `internal/provider/` as flat files
(`<name>_resource.go` + `<name>_resource_test.go`). There are no sub-packages
for individual resources.

## Commands

```bash
make build          # go build -v ./...
make test           # Unit tests only (no device needed)
make testacc        # Acceptance tests (TF_ACC=1, requires real device)
make lint           # golangci-lint run --timeout=3m
make fmt            # gofmt -s -w ./internal
make generate       # go generate ./... (re-generates docs + formats examples)
make install        # go install for local dev override
```

### Run a single test

```bash
# Unit test (no device)
go test -v -run TestUnitTenantImageImportPayloadIncludesAllFields -timeout 5m ./internal/provider/

# Acceptance test (needs F5OS_HOST, F5OS_USERNAME, F5OS_PASSWORD, TF_ACC=1)
TF_ACC=1 go test -v -run TestAccTenantImageCreateTC1Resource -timeout 5m ./internal/provider/
```

## CI

- **GitHub Actions** (`.github/workflows/test.yml`): build + lint + unit tests
  only. Does NOT run acceptance tests (`TF_ACC` is not set). Tests run against
  Terraform 1.0/1.1/1.2 matrix.
- **GitLab CI** (`.gitlab-ci.yml`): lint, build, unit tests, goreleaser for
  tagged releases, then publishes to GitHub for the Terraform Registry.
- `go generate ./...` drift is checked in CI. Run `make generate` before
  committing if you change schemas, examples, or templates.

### Coverage enforcement

Both CI pipelines enforce a minimum unit test coverage threshold using the
shared script `scripts/check-coverage.sh`. The current threshold is **75%**.

- The script parses `cover.out` (produced by `make test`), extracts the total
  coverage, and fails the build if it falls below the threshold. The 75%
  threshold provides a 5% grace margin below the 80% coverage target.
- On GitHub Actions it writes a Markdown summary to the job summary page and
  uploads `cover.out` as an artifact.
- On GitLab CI the `coverage:` keyword extracts the percentage for the
  pipeline badge.
- To change the threshold, update the `46` argument in both
  `.github/workflows/test.yml` and `.gitlab-ci.yml`, or set the
  `COVERAGE_THRESHOLD` environment variable.

```bash
# Check coverage locally
make test
./scripts/check-coverage.sh cover.out 75
```

## Vendoring

Dependencies are vendored. After changing `go.mod`:

```bash
go mod tidy && go mod vendor
```

The f5osclient at `vendor/gitswarm.f5net.com/terraform-providers/f5osclient/`
is the sole API client. If you need to change client behavior, edit the vendored
copy directly (or update the upstream module and re-vendor).

## Testing patterns

### Unit tests (mock server)

Unit tests use `testAccPreUnitCheck(t)` which starts an `httptest.Server` and
sets `F5OS_HOST`/`F5OS_USERNAME`/`F5OS_PASSWORD` env vars to point at it. Mock
handlers are registered on the global `mux`. Always `defer teardown()` when
using the mock server. Set `IsUnitTest: true` in the `resource.TestCase`.

Test helpers in `provider_test.go`: `loadFixtureBytes`, `loadFixtureString`,
`setup`, `teardown`, `testAccPreCheck`, `testAccPreUnitCheck`.

### F5OS 2.0.0 mock helpers and fixtures

For unit tests that need a 2.0.0-shaped device surface, use the helpers in
`setup_mock_2_0_0_test.go` instead of hand-registering platform version and
AAA endpoints:

- `setupMockPlatformVersion2_0_0(mux)` — canonical 2.0.0 build (`2.0.0-22925`).
- `setupMock2_0_0LoginPolicy(mux)` — mounts the login-policy GET handler.
- `setupMock2_0_0PasswordPolicy(mux)` — mounts the password-policy GET handler
  (returns v1.7 fields plus 2.0.0 additions `min-days`, `remember`, `warn-age`).
- `setupMock2_0_0LdapObjectClasses(mux, empty bool)` — LDAP object-class
  leaf-lists. Pass `true` to serve the empty-list variant that exercises
  the client's "cleared to empty" vs "unmanaged" distinction, or `false`
  for the populated variant.
- `setupMock2_0_0AAA(mux)` — bundle: platform version + focused
  login-policy / password-policy / LDAP endpoints (populated variant).
  Deliberately does NOT register the wide `/aaa` container path — many
  callers already mock it and `http.ServeMux` panics on duplicate
  registration.
- `setupMock2_0_0AAAContainer(t, mux)` — opt-in helper that serves the
  full 2.0.0 aaa container (`f5os_auth_v2_0.json`) on
  `/restconf/data/openconfig-system:system/aaa`. Takes `*testing.T` so
  a duplicate registration is surfaced as `t.Fatalf` instead of a
  binary-wide panic. Call it explicitly ONLY when your test does not
  already register a handler on that path.

Fixture files:

- `fixtures/f5os_auth.json` — baseline aaa container; `tally-count` was
  removed in 2.0.0 and is not present. Safe to use across all mock tests.
- `fixtures/f5os_auth_v17.json` — v1.7 password-policy shape (has
  `max-letter-repeat`/`max-sequence-repeat`/`max-class-repeat`, does not
  have the 2.0.0 leaves). Also no `tally-count`.
- `fixtures/f5os_auth_v2_0.json` — full 2.0.0 aaa container with expanded
  password-policy, login-policy, and LDAP object-class leaf-lists.
- `fixtures/aaa_login_policy_2_0_0.json` / `aaa_password_policy_2_0_0.json`
  / `aaa_ldap_object_classes_2_0_0.json` (+ `_empty.json` variant) —
  focused container responses for tests that only need one endpoint.
- `fixtures/tenant_get_status_2_0_0_shape.json` — 2.0.0 tenant state:
  carries `max-nodes`, `f5-tenant-mgmt-vlan:mgmt-vlan(-accessible)`,
  `feature-flags`; deliberately omits pre-2.0 legacy leaves `instances`,
  `cpu-allocations`, `primary-slot`, `image-version`, `mac-block`.

Shape assertions in `TestUnitFixtures2_0_0Shape` (in
`setup_mock_2_0_0_smoke_test.go`) guard against fixture drift — if you
edit any of these files, run that test to confirm the required leaves
are still present or absent as documented.

### Acceptance tests (real device)

Require env vars: `F5OS_HOST`, `F5OS_USERNAME`, `F5OS_PASSWORD`, and `TF_ACC=1`.
Port defaults to **8888** (RESTCONF API), not 443. Use `testAccPreCheck(t)` as
the PreCheck.

### Known testing issue: Some Read methods may skip device queries

Historically, many resources' `Read` methods preserved plan state instead of
reading from the device after Create/Update. Most resources have been fixed
(dns, auth, tenant, tenant_image, snmp, ntp_server) but some may still exhibit
this behavior (logging, qkview, partition, vlan, interface, lag, license).
When testing resources not confirmed as fixed, `TestCheckResourceAttr` may only
verify what Terraform thinks — not the device state. Write custom check
functions that create a fresh `f5osclient` session and query the API directly.
See `.agents/skills/f5os-acc-test/SKILL.md` for templates and detailed guidance.

## Resource implementation conventions

- Each resource implements `resource.Resource` and `resource.ResourceWithImportState`.
- The provider validates platform type in Create (some resources reject
  "Velos Controller" and require partition-level or rSeries).
- `ImportState` must set both `id` and the primary identifier attribute (e.g.,
  `image_name`) via `resp.State.SetAttribute`.
- Error handling: always return early after `AddError`. Check that errors from
  API calls are not silently discarded (a recurring bug pattern in this codebase).

## Code comments

- **Do not include line number references in comments** when writing tests or
  adding code. Line numbers become stale as code evolves and create maintenance
  burden. Instead, describe what code path or function is being exercised
  without referencing specific line numbers (e.g., write "exercises the
  fetchTLS else branch" not "exercises line 761").

## Unit test coverage

Coverage has improved significantly since the April 2026 testing push.
Re-run the commands below to get current numbers.

Well-covered (>60%): `primarykey`, `tenant`, `tls_cert_key`, `provider`,
`config_backup`, `auth`, `ntp_server`, `snmp`, `dns`, `user`, `tenant_image`,
`partition_change_password`.

No unit tests (acc-only): `logging`, `qkview`, `partition`, `vlan`,
`interface`, `lag`, `license`, `user_password_change`.

Full report: `reports/unit_test_coverage.md`. Regenerate with:

```bash
make test                                 # writes cover.out
go tool cover -func=cover.out | tail -1   # overall %
go tool cover -html=cover.out             # interactive HTML
```

## Doc generation

Docs in `docs/` are generated from `templates/` and `examples/` via
`terraform-plugin-docs`. Do not edit `docs/` directly. Run `make generate` to
regenerate.

## Skills

The `.agents/skills/` directory contains detailed workflows for specific tasks:

- **f5os-acc-test**: Writing and running acceptance tests against real devices.
  Includes safety rules for shared DUTs, verification templates, and report
  formats.
- **f5os-dual-acc-test**: Comparative acceptance testing across two devices.
- **f5os-release**: Release process for GitHub + Terraform Registry.
- **qrt**: Reserving BIG-IP Classic test machines via the QRT API.

Load these via the `skill` tool when the task matches.
