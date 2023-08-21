package provider

import (
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestAccCfgBackupCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfgBackupConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_config_backup.test", "name", "test_backup_92dh7s"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_path", "/upload/upload.php"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "protocol", "https"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_user", "corpuser"),
					resource.TestCheckResourceAttr("f5os_config_backup.test", "remote_password", "password"),
				),
			},
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
		fmt.Fprint(w, `{"f5-database:output":{"result":"Database backup successful."}}`)
	})

	mux.HandleFunc(exportCfgBackup, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "Expected method '%s', got '%s'", http.MethodPost, r.Method)
		fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"File transfer is initiated.(configs/test_cfg_backup)"}}`)
	})

	mux.HandleFunc(transferStatus, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "Expected method '%s', got '%s'", http.MethodGet, r.Method)
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
		fmt.Fprint(w,
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
		fmt.Fprint(w, `{"f5-utils-file-transfer:output":{"result":"Deleting the file"}}`)
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
