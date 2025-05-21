package provider

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestAccPrimaryKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "passphrase", "test-pass"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "salt", "test-salt"),
				),
			},
		},
	})
}

func TestUnitPrimaryKeyResource(t *testing.T) {
	testAccPreUnitCheck(t)

	// Combined handler for GET and PATCH on the same path
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-primary-key:primary-key", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			assert.Equal(t, "GET", r.Method)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"f5-primary-key:primary-key": {
					"f5-primary-key:state": {
						"f5-primary-key:hash": "abc123hash",
						"f5-primary-key:status": "COMPLETE"
					}
				}
			}`))
		case "PATCH":
			assert.Equal(t, "PATCH", r.Method)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("Unexpected HTTP method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPrimaryKeyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_primarykey.default", "id", "primary-key"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "hash", "abc123hash"),
					resource.TestCheckResourceAttr("f5os_primarykey.default", "status", "COMPLETE"),
				),
			},
		},
	})
}

const testAccPrimaryKeyResourceConfig = `
resource "f5os_primarykey" "default" {
  passphrase   = "test-pass"
  salt         = "test-salt"
  force_update = true
}
`
