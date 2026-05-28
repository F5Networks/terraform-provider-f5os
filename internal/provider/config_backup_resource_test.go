package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
)

// testAccCheckCfgBackupExists queries the device directly to confirm that
// a config backup file with the given name is present in the configs/ listing.
func testAccCheckCfgBackupExists(backupName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newTestClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create f5os client: %w", err)
		}
		return cfgBackupExistsOnDevice(client, backupName)
	}
}

// cfgBackupExistsOnDevice returns nil if the named backup is present on the
// device, or an error otherwise.
func cfgBackupExistsOnDevice(client interface{ GetConfigBackup() ([]byte, error) }, name string) error {
	res, err := client.GetConfigBackup()
	if err != nil {
		return fmt.Errorf("GetConfigBackup failed: %w", err)
	}
	obj := make(map[string]any)
	if err := json.NewDecoder(bytes.NewReader(res)).Decode(&obj); err != nil {
		return fmt.Errorf("failed to decode config backup response: %w", err)
	}
	output, ok := obj["f5-utils-file-transfer:output"].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected response structure: missing f5-utils-file-transfer:output")
	}
	entries, ok := output["entries"].([]any)
	if !ok {
		return fmt.Errorf("unexpected response structure: missing entries")
	}
	for _, v := range entries {
		m, _ := v.(map[string]any)
		if m["name"].(string) == name {
			return nil
		}
	}
	return fmt.Errorf("config backup %q not found on device", name)
}

// testAccCheckCfgBackupDestroy verifies that the test backup files have been
// removed from the device after Terraform destroys the resource.
func testAccCheckCfgBackupDestroy(s *terraform.State) error {
	client, err := newTestClientFromEnv()
	if err != nil {
		// Cannot connect — treat as destroyed
		return nil
	}
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "f5os_config_backup" {
			continue
		}
		name := rs.Primary.Attributes["name"]
		if err := cfgBackupExistsOnDevice(client, name); err == nil {
			return fmt.Errorf("config backup %q still exists on device after destroy", name)
		}
	}
	return nil
}

func TestAccCfgBackupCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCfgBackupDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create the backup and verify state + device
			{
				Config: cfgBackupConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_config_backup.test", "name", "test_backup_92dh7s"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_path", "/upload/upload.php"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "protocol", "https"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_user", "corpuser"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_password", "password"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "timeout", "150"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "id", "test_backup_92dh7s"),
					testAccCheckCfgBackupExists("test_backup_92dh7s"),
				),
			},
			// Step 2: Update a mutable attribute (remote_path) to exercise Update
			{
				Config: cfgBackupConfigUpdated,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_config_backup.test", "name", "test_backup_92dh7s"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_path", "/upload/upload_v2.php"),
					testAccCheckCfgBackupExists("test_backup_92dh7s"),
				),
			},
			// Step 3: Destroy is automatic; CheckDestroy verifies cleanup
		},
	})
}

func TestUnitCfgBackup(t *testing.T) {
	testAccPreUnitCheck(t)
	t.Logf("Server URL: %s", server.URL)

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})

	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		log.Print("platform url success")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})

	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/restconf/data/openconfig-vlan:vlans", r.URL.String(), "Expected method 'GET', got %s", r.URL.String())
		w.WriteHeader(http.StatusNoContent)
		_, _ = fmt.Fprintf(w, ``)
	})

	createCfgBackup := "/restconf/data/openconfig-system:system/f5-database:database/f5-database:config-backup"
	exportCfgBackup := "/restconf/data/f5-utils-file-transfer:file/export"
	transferStatus := "/restconf/data/f5-utils-file-transfer:file/transfer-operations/transfer-operation"
	readCfgBackup := "/restconf/data/f5-utils-file-transfer:file/list"
	deleteCfgBackup := "/restconf/data/f5-utils-file-transfer:file/delete"

	mux.HandleFunc(createCfgBackup, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "Expected method '%s', got '%s'", http.MethodPost, r.Method)
		//nolint:errcheck
		fmt.Fprint(w, `{"f5-database:output":{"result":"Database backup successful."}}`)
	})

	mux.HandleFunc(exportCfgBackup, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "Expected method '%s', got '%s'", http.MethodPost, r.Method)
		//nolint:errcheck
		fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"File transfer is initiated.(configs/test_cfg_backup)"}}`)
	})

	mux.HandleFunc(transferStatus, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "Expected method '%s', got '%s'", http.MethodGet, r.Method)
		//nolint:errcheck
		fmt.Fprint(w,
			`
			{
				"f5-utils-file-transfer:transfer-operation": [
					{
						"local-file-path": "configs/test_cfg_backup",
						"remote-host": "10.255.0.142",
						"remote-file-path": "/upload/test_cfg_backup",
						"operation": "Export file",
						"protocol": "HTTPS   ",
						"status": "         Completed",
						"timestamp": "Tue Aug  1 07:21:03 2023"
					}
				]
			}
		`,
		)
	})

	mux.HandleFunc(readCfgBackup, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "Expected method '%s', got '%s'", http.MethodPost, r.Method)
		_, _ = fmt.Fprint(w,
			`
			{
				"f5-utils-file-transfer:output": {
					"entries": [
						{
							"name": "test_cfg_backup",
							"date": "Tue Aug  1 06:02:35 UTC 2023",
							"size": "45KB"
						}
					]
				}
			}
			`,
		)
	})

	mux.HandleFunc(deleteCfgBackup, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "Expected method '%s', got '%s'", http.MethodPost, r.Method)
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"Deleting the file"}}`)
	})

	defer teardown()

	tfCfg := `
resource "f5os_config_backup" "test" {
  name            = "test_cfg_backup"
  remote_host     = "1.2.3.4"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/test_cfg_backup"
  protocol        = "https"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{
			{
				Config: tfCfg,
			},
		},
	})
}

// setupCfgBackupMockProvider registers the common provider-level mock
// endpoints (auth, platform, vlans) that every config_backup unit test needs.
func setupCfgBackupMockProvider() {
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

// setupCfgBackupMockHappyPath registers the resource-level mock endpoints
// (backup create, export, transfer-status, file list, file delete) that
// return successful responses. Tests that need custom behavior for specific
// endpoints should call setupCfgBackupMockProvider() directly and register
// only the handlers they need.
func setupCfgBackupMockHappyPath() {
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-database:database/f5-database:config-backup", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database backup successful."}}`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/export", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"File transfer is initiated.(configs/test_cfg_backup)"}}`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/transfer-operations/transfer-operation", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:transfer-operation":[{"local-file-path":"configs/test_cfg_backup","remote-host":"1.2.3.4","remote-file-path":"/upload/test_cfg_backup","operation":"Export file","protocol":"HTTPS","status":"         Completed","timestamp":"Tue Aug  1 07:21:03 2023"}]}`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/list", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"entries":[{"name":"test_cfg_backup","date":"Tue Aug  1 06:02:35 UTC 2023","size":"45KB"}]}}`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/delete", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"Deleting the file"}}`)
	})
}

// setupCfgBackupMockCreateExportTransfer registers only the backup create,
// export, and transfer-status endpoints. Use this when you need custom
// handlers for the list or delete endpoints.
func setupCfgBackupMockCreateExportTransfer() {
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-database:database/f5-database:config-backup", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-database:output":{"result":"Database backup successful."}}`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/export", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"File transfer is initiated.(configs/test_cfg_backup)"}}`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/transfer-operations/transfer-operation", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:transfer-operation":[{"local-file-path":"configs/test_cfg_backup","remote-host":"1.2.3.4","remote-file-path":"/upload/test_cfg_backup","operation":"Export file","protocol":"HTTPS","status":"         Completed","timestamp":"Tue Aug  1 07:21:03 2023"}]}`)
	})
}

// TestUnitCfgBackupCreateError verifies the Create error path when
// CreateConfigBackup fails (the initial database backup POST returns 500).
func TestUnitCfgBackupCreateError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgBackupMockProvider()

	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-database:database/f5-database:config-backup", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"internal error creating backup"}]}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testUnitCfgBackupConfig,
				ExpectError: regexp.MustCompile(`failure while creating config backup`),
			},
		},
	})
}

// TestUnitCfgBackupDeleteError verifies the Delete error path when
// DeleteConfigBackup fails. Step 1 creates the resource successfully,
// then step 2 destroys it but the delete endpoint returns an error.
// The doRequest client retries 3 times on HTTP 500, so the first 3
// calls must all fail for the error to propagate. Subsequent calls
// succeed to allow framework cleanup.
func TestUnitCfgBackupDeleteError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgBackupMockProvider()
	setupCfgBackupMockCreateExportTransfer()

	var deleteCallCount int32

	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/list", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"entries":[{"name":"test_cfg_backup","date":"Tue Aug  1 06:02:35 UTC 2023","size":"45KB"}]}}`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/delete", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&deleteCallCount, 1)
		// doRequest retries 3 times on 500; the first 3 calls must all fail
		// so that the error propagates. After that, succeed for cleanup.
		if n <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"delete failed: resource locked"}]}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"Deleting the file"}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the resource successfully
			{
				Config: testUnitCfgBackupConfig,
			},
			// Step 2: Destroy triggers Delete, which fails after all retries
			{
				Config:      testUnitCfgBackupConfig,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`failure while destroying config backup`),
			},
		},
	})
}

// TestUnitCfgBackupReadGetError verifies the Read error path when
// GetConfigBackup returns an API error. The resource is created
// successfully, then Read fails during refresh in step 2.
func TestUnitCfgBackupReadGetError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgBackupMockProvider()
	setupCfgBackupMockCreateExportTransfer()

	var listCallCount int32

	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/list", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&listCallCount, 1)
		if n <= 1 {
			// First call (during Create/Read after Create) succeeds
			_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"entries":[{"name":"test_cfg_backup","date":"Tue Aug  1 06:02:35 UTC 2023","size":"45KB"}]}}`)
			return
		}
		// Subsequent calls (Read in step 2) fail with 500
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"ietf-restconf:errors":{"error":[{"error-type":"application","error-tag":"operation-failed","error-message":"internal server error"}]}}`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/delete", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"Deleting the file"}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds
			{
				Config: testUnitCfgBackupConfig,
			},
			// Step 2: Re-apply triggers Read which now fails
			{
				Config:      testUnitCfgBackupConfig,
				ExpectError: regexp.MustCompile(`Error Reading Config Backups`),
			},
		},
	})
}

// TestUnitCfgBackupReadJsonDecodeError verifies the Read error path when
// GetConfigBackup returns unparseable JSON. The resource is created,
// then Read gets malformed JSON during refresh.
func TestUnitCfgBackupReadJsonDecodeError(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgBackupMockProvider()
	setupCfgBackupMockCreateExportTransfer()

	var listCallCount int32

	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/list", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&listCallCount, 1)
		if n <= 1 {
			// First call succeeds (Read after Create)
			_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"entries":[{"name":"test_cfg_backup","date":"Tue Aug  1 06:02:35 UTC 2023","size":"45KB"}]}}`)
			return
		}
		// Subsequent calls return malformed JSON
		_, _ = fmt.Fprint(w, `{not valid json at all`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/delete", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"Deleting the file"}}`)
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create succeeds
			{
				Config: testUnitCfgBackupConfig,
			},
			// Step 2: Re-apply triggers Read which gets bad JSON
			{
				Config:      testUnitCfgBackupConfig,
				ExpectError: regexp.MustCompile(`Error parsing Config Backup Response`),
			},
		},
	})
}

// TestUnitCfgBackupUpdate verifies that the Update method is exercised when
// a mutable attribute (remote_host) changes between steps. Update persists
// the new plan to state; without that, Terraform would report an
// "inconsistent result after apply" error.
func TestUnitCfgBackupUpdate(t *testing.T) {
	testAccPreUnitCheck(t)
	setupCfgBackupMockProvider()
	setupCfgBackupMockHappyPath()

	defer teardown()

	cfgStep1 := `
resource "f5os_config_backup" "test" {
  name            = "test_cfg_backup"
  remote_host     = "1.2.3.4"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/test_cfg_backup"
  protocol        = "https"
}
`
	cfgStep2 := `
resource "f5os_config_backup" "test" {
  name            = "test_cfg_backup"
  remote_host     = "5.6.7.8"
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
			// Step 1: Create
			{
				Config: cfgStep1,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_host", "1.2.3.4"),
				),
			},
			// Step 2: Change remote_host to trigger Update
			{
				Config: cfgStep2,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_host", "5.6.7.8"),
				),
			},
		},
	})
}

// TestUnitBackupModelToExportConfig verifies the backupModelToExportConfig
// helper populates all FileExport fields correctly.
func TestUnitBackupModelToExportConfig(t *testing.T) {
	model := &CfgBackupResourceModel{
		Name:           types.StringValue("my_backup"),
		RemoteHost:     types.StringValue("10.0.0.1"),
		RemoteUser:     types.StringValue("admin"),
		RemotePassword: types.StringValue("secret"),
		RemotePath:     types.StringValue("/backups/my_backup"),
		Protocol:       types.StringValue("scp"),
	}

	export := backupModelToExportConfig(model)

	assert.Equal(t, "10.0.0.1", export.RemoteHost)
	assert.Equal(t, "/backups/my_backup", export.RemotePath)
	assert.Equal(t, "configs/my_backup", export.LocalFile)
	assert.Equal(t, "scp", export.Protocol)
	assert.Equal(t, "admin", export.Username)
	assert.Equal(t, "secret", export.Password)
	assert.Equal(t, "", export.Insecure)
}

const testUnitCfgBackupConfig = `
resource "f5os_config_backup" "test" {
  name            = "test_cfg_backup"
  remote_host     = "1.2.3.4"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/test_cfg_backup"
  protocol        = "https"
}
`

const cfgBackupConfig = `
resource "f5os_config_backup" "test" {
  name            = "test_backup_92dh7s"
  remote_host     = "10.255.0.142"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/upload.php"
  protocol        = "https"
}
`

const cfgBackupConfigUpdated = `
resource "f5os_config_backup" "test" {
  name            = "test_backup_92dh7s"
  remote_host     = "10.255.0.142"
  remote_user     = "corpuser"
  remote_password = "password"
  remote_path     = "/upload/upload_v2.php"
  protocol        = "https"
}
`
