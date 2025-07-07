package provider

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestAccTenantImageCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantImageCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant_image.test", "id", "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_tenant_image.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTenantImageCreateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantImageCreateTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant_image.test", "id", "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_tenant_image.test",
				ImportState:       true,
				ImportStateVerify: false,
			},
		},
	})
}

func TestAccTenantImageCreateUnitTC3Resource(t *testing.T) {
	testAccPreUnitCheck(t)
	var count = 0
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method == "GET" && count == 0 {
			_, _ = fmt.Fprintf(w, "%s", "")
			count++
		} else {
			_, _ = fmt.Fprintf(w, "%s", `{"f5-tenant-images:image": [{
            "name": "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle",
            "in-use": false,
            "type": "vm-image",
            "status": "replicated",
            "date": "2023-3-27",
            "size": "2.27 GB"}]}`)
		}
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/import", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/transfer-operations/transfer-operation", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_image_transfer_status.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/remove", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", `{
    "f5-tenant-images:output": {
        "result": "Successful."
    }
}`)
	})
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantImageCreateTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant_image.test", "id", "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"),
				),
			},
			{
				Config: testAccTenantImageCreateTC2ModifyResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant_image.test", "id", "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"),
				),
			},
		},
	})
}

func TestAccTenantImageCreateUnitTC4Resource(t *testing.T) {
	testAccPreUnitCheck(t)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image=BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", `
{
    "f5-tenant-images:image": [
        {
            "name": "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle",
            "in-use": false,
            "type": "vm-image",
            "status": "replicated",
            "date": "2023-3-27",
            "size": "2.27 GB"
        }
    ]
}
`)
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/import", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", "")
	})
	mux.HandleFunc("/restconf/data/f5-utils-file-transfer:file/transfer-operations/transfer-operation", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/tenant_image_transfer_status.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/remove", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", `{
    "f5-tenant-images:output": {
        "result": "Successful."
    }
}`)
	})
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccTenantImageCreateTC2ResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tenant_image.test", "id", "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_tenant_image.test",
				ImportState:       true,
				ImportStateVerify: false,
			},
		},
	})
}

// func TestUnitTenantImageUpload(t *testing.T) {
//     testAccPreUnitCheck(t)
//     t.Logf("Server URL: %s", server.URL)
//
//     mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
//         assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
//         w.Header().Set("Content-Type", "application/yang-data+json")
//         w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
//         _, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
//     })
//
//     mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
//         w.WriteHeader(http.StatusNotFound)
//         log.Print("platform url success")
//         _, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
//     })
//
//     mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
//         assert.Equal(t, "/restconf/data/openconfig-vlan:vlans", r.URL.String(), "Expected method 'GET', got %s", r.URL.String())
//         w.WriteHeader(http.StatusNoContent)
//         _, _ = fmt.Fprintf(w, "")
//     })
//
//     getImageUrl := "/restconf/data/f5-tenant-images:images/image=dumm_image_unit_test.qcow2.zip.bundle"
//     uploadIdUrl := "/restconf/data/f5-utils-file-transfer:file/f5-file-upload-meta-data:upload/start-upload"
//     uploadImageUrl := "/restconf/data/openconfig-system:system/f5-image-upload:image/upload-image"
//     deleteImageUrl := "/restconf/data/f5-tenant-images:images/remove"
//     imageExists := false
//     mux.HandleFunc(getImageUrl, func(w http.ResponseWriter, r *http.Request) {
//         assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
//         if !imageExists {
//             _, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
//             imageExists = true
//         }
//         if imageExists {
//             _, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/getExistingImage.json"))
//         }
//     })
//
//     mux.HandleFunc(uploadIdUrl, func(w http.ResponseWriter, r *http.Request) {
//         assert.Equal(t, http.MethodPost, r.Method, "Expected method 'POST', got %s", r.Method)
//         body, err := io.ReadAll(r.Body)
//         if err != nil {
//             t.Errorf("unable to read post request's payload: %s", err)
//         }
//
//         payloadJson := make(map[string]any)
//         t.Logf("upload ID payload: %s", string(body))
//         err = json.NewDecoder(bytes.NewReader(body)).Decode(&payloadJson)
//         if err != nil {
//             t.Errorf("structure of the payload for upload Id endpoint is not correct: %s", err)
//         }
//
//         fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/uploadIdResp.json"))
//     })
//
//     mux.HandleFunc(uploadImageUrl, func(w http.ResponseWriter, r *http.Request) {
//         assert.Equal(t, http.MethodPost, r.Method, "Expected method 'POST', got %s", r.Method)
//         t.Logf("upload image response: %s", loadFixtureString("./fixtures/uploadSuccessful.json"))
//         fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/uploadSuccessful.json"))
//     })
//
//     mux.HandleFunc(deleteImageUrl, func(w http.ResponseWriter, r *http.Request) {
//         assert.Equal(t, http.MethodPost, r.Method, "Expected method 'POST', got %s", r.Method)
//         fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/deleteImageSuccess.json"))
//     })
//
//     defer teardown()
//     tfCfg := `
// resource "f5os_tenant_image" "test" {
//  image_name       = "dumm_image_unit_test.qcow2.zip.bundle"
//  upload_from_path = "./fixtures"
// }
// `
//     resource.Test(t, resource.TestCase{
//         IsUnitTest:               true,
//         ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
//         Steps: []resource.TestStep{
//             // Read testing
//             {
//                 Config:  tfCfg,
//                 Destroy: false,
//             },
//         },
//     })
// }

const testAccTenantImageCreateResourceConfig = `
resource "f5os_tenant_image" "test" {
  image_name  = "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"
  remote_host = "spkapexsrvc01.olympus.f5net.com"
  remote_path = "v17.1.0.1/daily/current/VM"
  local_path  = "images/tenant"
  timeout = 360
}
`

const testAccTenantImageCreateTC2ResourceConfig = `
resource "f5os_tenant_image" "test" {
  image_name  = "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"
  remote_host = "spkapexsrvc01.olympus.f5net.com"
  remote_path = "v17.1.0.1/daily/current/VM"
  local_path  = "images"
  timeout = 360
}
`

const testAccTenantImageCreateTC2ModifyResourceConfig = `
resource "f5os_tenant_image" "test" {
  image_name  = "BIGIP-17.1.0.1-0.0.4.ALL-F5OS.qcow2.zip.bundle"
  remote_host = "spkapexsrvc01.olympus.f5net.com"
  remote_path = "v17.1.0.1/daily/current/VM"
  local_path  = "images"
  timeout = 380
}
`
