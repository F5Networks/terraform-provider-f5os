package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/assert"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// setupCfgRestoreMockProvider registers the common provider-level mock
// endpoints (auth, platform, vlans) that every config_restore unit test
// needs. Mirrors setupCfgBackupMockProvider in config_backup_resource_test.go.
func setupCfgRestoreMockProvider() {
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

const cfgRestoreConfigURI = "/restconf/data/openconfig-system:system/f5-database:database/f5-database:config-restore"
const fileImportURI = "/restconf/data/f5-utils-file-transfer:file/import"
const fileTransferStatusURI = "/restconf/data/f5-utils-file-transfer:file/transfer-operations/transfer-operation"

// TestUnitCfgRestoreLocalOnly verifies the happy path when no remote_host
// is set: only config-restore is called, no file import.
func TestUnitCfgRestoreLocalOnly(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	var importCalled bool
	mux.HandleFunc(fileImportURI, func(w http.ResponseWriter, r *http.Request) {
		importCalled = true
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"File Transfer Initiated."}}`)
	})
	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database restore successful."}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name = "test_cfg_backup"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_config_restore.test", "name", "test_cfg_backup"),
					resource.TestCheckResourceAttr("f5os_config_restore.test", "id", "test_cfg_backup"),
					resource.TestCheckResourceAttr("f5os_config_restore.test", "result", "Database restore successful."),
					resource.TestCheckResourceAttr("f5os_config_restore.test", "timeout", "150"),
				),
			},
		},
	})

	if importCalled {
		t.Error("expected file/import endpoint NOT to be called when remote_host is unset")
	}
}

// TestUnitCfgRestoreTimeoutChangeRequiresReplace verifies that changing
// only `timeout` between applies plans a destroy-then-create (Replace),
// not an in-place Update. Every attribute on this one-shot-action
// resource has RequiresReplace precisely so that any input change
// re-triggers the restore via Create (a fresh resource instance), never
// via Update -- see the resource's Update method doc comment. This is a
// regression test for a gap where `timeout` was missing its
// RequiresReplace plan modifier, which would have let a "just adjusting
// a poll timeout" edit silently reach Update and re-invoke a real
// device restore outside of Terraform's normal create/destroy signaling.
func TestUnitCfgRestoreTimeoutChangeRequiresReplace(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	var restoreCallCount int
	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		restoreCallCount++
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database restore successful."}}`)
	})

	defer teardown()

	cfgStep1 := `
resource "f5os_config_restore" "test" {
  name    = "test_cfg_backup"
  timeout = 150
}
`
	cfgStep2 := `
resource "f5os_config_restore" "test" {
  name    = "test_cfg_backup"
  timeout = 300
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfgStep1,
			},
			{
				Config: cfgStep2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("f5os_config_restore.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
		},
	})

	if restoreCallCount != 2 {
		t.Errorf("expected config-restore to be called exactly twice (Create for each of the two distinct resource instances), got %d calls", restoreCallCount)
	}
}

// TestUnitCfgRestoreInsecureRequiresRemoteHost verifies that setting
// insecure = true without remote_host is rejected by ValidateConfig
// (insecure has no effect without a remote fetch to apply it to).
func TestUnitCfgRestoreInsecureRequiresRemoteHost(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name      = "test_cfg_backup"
  insecure  = true
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`insecure requires remote_host`),
			},
		},
	})
}

// TestUnitCfgRestoreRemoteUserRequiresRemoteHost verifies that setting
// remote_user/remote_password without remote_host is rejected at plan
// time by the remote_user/remote_password schema validators, rather
// than being silently discarded by restoreModelToImportConfig (which
// returns nil -- a local-only restore -- whenever remote_host is unset).
func TestUnitCfgRestoreRemoteUserRequiresRemoteHost(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name            = "test_cfg_backup"
  remote_user     = "corpuser"
  remote_password = "password"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`(?s)remote_user.*remote_host`),
			},
		},
	})
}

// TestUnitCfgRestoreRemotePathRequiresRemoteHost verifies the reverse
// direction of the remote-fetch attribute group's AlsoRequires
// constraint: remote_path without remote_host is also rejected (not
// just remote_host without remote_path, which was already covered by
// the original remote_path validator prior to this test).
func TestUnitCfgRestoreRemotePathRequiresRemoteHost(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name        = "test_cfg_backup"
  remote_path = "/upload/test_cfg_backup"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`(?s)remote_path.*remote_host`),
			},
		},
	})
}

// TestUnitCfgRestoreConfigRestoreMalformedResult verifies that a 200
// response whose f5-database:output has no "result" key (or a
// non-string one) is treated as an error, not silently as success with
// an empty result string.
func TestUnitCfgRestoreConfigRestoreMalformedResult(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"something-else":"unexpected shape"}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name = "test_cfg_backup"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`(?s)missing or non-string\s+result`),
			},
		},
	})
}

// TestUnitCfgRestoreWithRemoteFetch verifies the happy path when
// remote_host is set: the backup file is imported from the remote host
// first (with polling to Completed), then config-restore is invoked.
func TestUnitCfgRestoreWithRemoteFetch(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	mux.HandleFunc(fileImportURI, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"File Transfer Initiated.","operation-id":"IMPORT-1"}}`)
	})
	mux.HandleFunc(fileTransferStatusURI, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:transfer-operation":[{"operation-id":"IMPORT-1","local-file-path":"configs/test_cfg_backup","remote-host":"1.2.3.4","remote-file-path":"/upload/test_cfg_backup","operation":"Import file","protocol":"HTTPS","status":"Completed","timestamp":"Tue Aug 1 07:21:03 2023"}]}`)
	})
	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database restore successful."}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name            = "test_cfg_backup"
  remote_host     = "1.2.3.4"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/test_cfg_backup"
  protocol        = "https"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_config_restore.test", "name", "test_cfg_backup"),
					resource.TestCheckResourceAttr("f5os_config_restore.test", "remote_host", "1.2.3.4"),
					resource.TestCheckResourceAttr("f5os_config_restore.test", "result", "Database restore successful."),
				),
			},
		},
	})
}

// TestUnitCfgRestoreImportTimeout verifies that a remote fetch which
// never reaches "Completed" status surfaces a timeout error. This calls
// f5ossdk.RestoreConfigBackup directly (bypassing the resource/schema
// layer, whose timeout attribute has a 150-3600 second floor via
// int64validator.Between) so the test can use a sub-second timeout and
// stay fast, following the same direct-client-call pattern as
// f5os_snmp_resource_test.go's newAdapterOrFail/TestF5osSnmpClient_*
// tests rather than driving it through a full resource.Test lifecycle.
func TestUnitCfgRestoreImportTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Auth-Token", "test-token")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc(fileImportURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"File Transfer Initiated.","operation-id":"IMPORT-2"}}`)
	})
	mux.HandleFunc(fileTransferStatusURI, func(w http.ResponseWriter, r *http.Request) {
		// Always "In Progress" -- never completes, forcing the poll loop
		// to exhaust the (short, test-only) timeout.
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:transfer-operation":[{"operation-id":"IMPORT-2","local-file-path":"configs/test_cfg_backup","remote-host":"1.2.3.4","remote-file-path":"/upload/test_cfg_backup","operation":"Import file","protocol":"HTTPS","status":"In Progress","timestamp":"Tue Aug 1 07:21:03 2023"}]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := &f5ossdk.F5osConfig{
		Host:             srv.URL,
		User:             "admin",
		Password:         "admin",
		DisableSSLVerify: true,
	}
	client, err := f5ossdk.NewSession(cfg)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	client.PollInterval = time.Millisecond

	importCfg := &f5ossdk.FileImport{
		RemoteHost: "1.2.3.4",
		RemoteFile: "/upload/test_cfg_backup",
		Protocol:   "https",
		Username:   "corpuser",
		Password:   "password",
	}
	_, err = client.RestoreConfigBackup("test_cfg_backup", 1, importCfg)
	if err == nil || !strings.Contains(err.Error(), "config backup import timed out") {
		t.Fatalf("expected a config backup import timed out error, got: %v", err)
	}
}

// TestUnitCfgRestoreImportError verifies that an error from the remote
// file import endpoint itself (not the polling loop) is surfaced.
func TestUnitCfgRestoreImportError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	mux.HandleFunc(fileImportURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"internal error initiating import"}]}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name            = "test_cfg_backup"
  remote_host     = "1.2.3.4"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/test_cfg_backup"
  protocol        = "https"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`failure while restoring config backup`),
			},
		},
	})
}

// TestUnitCfgRestoreAlreadyExistsError verifies the "Aborted: local-file
// already exists" special case from the import endpoint is surfaced as
// an error rather than proceeding to config-restore.
func TestUnitCfgRestoreAlreadyExistsError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	var restoreCalled bool
	mux.HandleFunc(fileImportURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `Aborted: local-file already exists`)
	})
	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		restoreCalled = true
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database restore successful."}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name            = "test_cfg_backup"
  remote_host     = "1.2.3.4"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/test_cfg_backup"
  protocol        = "https"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`already exists`),
			},
		},
	})

	if restoreCalled {
		t.Error("expected config-restore NOT to be called after an 'already exists' import error")
	}
}

// TestUnitCfgRestoreConfigRestoreError verifies that a device-side
// config-restore failure (e.g. the backup file not present under
// configs/) is surfaced as an error. This mirrors the real HTTP 400
// response confirmed against a live F5OS device when restoring a
// nonexistent backup name (see f5os.go's RestoreConfigBackup comment).
func TestUnitCfgRestoreConfigRestoreError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"malformed-message","error-message":"couldn't open file configs/test_cfg_backup: no such file or directory\nDatabase config-restore failed."}]}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name = "test_cfg_backup"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`failure while restoring config backup`),
			},
		},
	})
}

// TestUnitCfgRestoreResultContainsFailed verifies that a 200-status
// response whose result text nonetheless contains "failed" is treated
// as an error rather than success (defensive guard -- see
// RestoreConfigBackup's comment on the unconfirmed success string).
func TestUnitCfgRestoreResultContainsFailed(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database config-restore failed. bad juju"}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name = "test_cfg_backup"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`failed`),
			},
		},
	})
}

// TestUnitCfgRestoreReadPreservesState verifies Read is a no-op that
// preserves prior state across a refresh (config-restore has no
// persistent, independently re-readable device representation).
func TestUnitCfgRestoreReadPreservesState(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	var restoreCallCount int
	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		restoreCallCount++
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database restore successful."}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name = "test_cfg_backup"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_config_restore.test", "result", "Database restore successful."),
				),
			},
			// Refresh-only plan: Read must not re-trigger a restore or
			// clear the previously-recorded result.
			{
				Config:             tfCfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})

	if restoreCallCount != 1 {
		t.Errorf("expected config-restore to be called exactly once (Create only), got %d calls", restoreCallCount)
	}
}

// TestUnitCfgRestoreDelete verifies Delete succeeds without making any
// device call (restoring cannot be undone -- see the resource's Delete
// comment).
func TestUnitCfgRestoreDelete(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgRestoreMockProvider()

	mux.HandleFunc(cfgRestoreConfigURI, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database restore successful."}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_restore" "test" {
  name = "test_cfg_backup"
}
`
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
			},
			{
				Config:  tfCfg,
				Destroy: true,
			},
		},
	})
}

// TestUnitRestoreModelToImportConfig verifies the restoreModelToImportConfig
// helper: nil when remote_host is unset, fully populated otherwise.
func TestUnitRestoreModelToImportConfig(t *testing.T) {
	localOnly := &CfgRestoreResourceModel{
		Name: types.StringValue("my_backup"),
	}
	if got := restoreModelToImportConfig(localOnly); got != nil {
		t.Errorf("expected nil FileImport when remote_host is unset, got %+v", got)
	}

	withRemote := &CfgRestoreResourceModel{
		Name:           types.StringValue("my_backup"),
		RemoteHost:     types.StringValue("10.0.0.1"),
		RemoteUser:     types.StringValue("admin"),
		RemotePassword: types.StringValue("secret"),
		RemotePath:     types.StringValue("/backups/my_backup"),
		Protocol:       types.StringValue("scp"),
	}
	got := restoreModelToImportConfig(withRemote)
	if got == nil {
		t.Fatal("expected non-nil FileImport when remote_host is set")
	}
	assert.Equal(t, "10.0.0.1", got.RemoteHost)
	assert.Equal(t, "/backups/my_backup", got.RemoteFile)
	assert.Equal(t, "scp", got.Protocol)
	assert.Equal(t, "admin", got.Username)
	assert.Equal(t, "secret", got.Password)
	assert.Nil(t, got.Insecure)
}

// TestAccCfgRestoreLocalOnly is intentionally NOT implemented as a
// success-path acceptance test against a real device. f5-database's
// YANG source (grouping config-check, `tailf:hidden debug`) notes that
// "reset-to-default remains a required prerequisite for config-restore"
// -- i.e. invoking a real restore against a device that already has
// non-default configuration (which describes every shared acceptance
// DUT used by this provider's test suite) is not confirmed to be the
// safe, idempotent no-op that a freshly-taken self-backup-then-restore
// round trip might otherwise suggest, and could disrupt other testers'
// in-flight state or leave the DUT in an unexpected condition.
//
// Only the error path is exercised for real below (restoring a backup
// name that does not exist), which is safe: it is confirmed (against
// both a live F5OS 1.8.3 and 2.0.0 device) to fail cleanly with an HTTP
// 400 "no such file or directory" response before any device state is
// touched.
func TestAccCfgRestoreNonexistentBackupFails(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set; skipping acceptance test")
	}

	tfCfg := `
resource "f5os_config_restore" "test" {
  name = "definitely_does_not_exist_xyz"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      tfCfg,
				ExpectError: regexp.MustCompile(`failure while restoring config backup`),
			},
		},
	})
}
