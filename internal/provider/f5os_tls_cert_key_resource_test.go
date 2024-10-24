package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestUnitCreateCertKey(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/rseries_platform_state_ok.json"))
	})
	mux.HandleFunc("/restconf/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls/f5-openconfig-aaa-tls:create-self-signed-cert", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method, "Expected method 'POST', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method, "Expected method 'DELETE', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusNoContent)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:  createCfg,
				Destroy: false,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "name", "testcert"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "days_valid", "40"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "email", "user@org.com"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "city", "Hyd"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "province", "Telangana"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "country", "IN"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "organization", "F7"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "unit", "IT"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "key_type", "encrypted-rsa"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "key_size", "2048"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "key_passphrase", "test123"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "confirm_key_passphrase", "test123"),
				),
			},
			{
				Config: updateCfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "name", "testcert"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "days_valid", "400"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "email", "user@org.com"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "city", "Hyd"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "province", "Telangana"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "country", "IN"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "organization", "F8"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "unit", "IT"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "key_type", "encrypted-rsa"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "key_size", "2048"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "key_passphrase", "test123"),
					resource.TestCheckResourceAttr("f5os_tls_cert_key.testcert", "confirm_key_passphrase", "test123"),
				),
			},
		},
	})
}

func TestUnitCreateCertKeySANNotSupportedError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/rseries_platform_state_ok.json"))
	})
	mux.HandleFunc("/restconf/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls/f5-openconfig-aaa-tls:create-self-signed-cert", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method, "Expected method 'POST', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method, "Expected method 'DELETE', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusNoContent)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      errorCfg,
				ExpectError: regexp.MustCompile("subject_alternative_name is not supported for platform version below v1.8"),
			},
		},
	})
}

func TestUnitCreateCertKeySANRequiredError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/rseries_platform_state_ok.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-image:image/state/install", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"f5-system-image:install": {"install-os-version": "1.8.0-3518","install-service-version": "1.8.0-3518","install-status": "success"}}`)

	})
	mux.HandleFunc("/restconf/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls/f5-openconfig-aaa-tls:create-self-signed-cert", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method, "Expected method 'POST', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method, "Expected method 'DELETE', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusNoContent)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      createCfg,
				ExpectError: regexp.MustCompile("subject_alternative_name is required for platform version v1.8 and above"),
			},
		},
	})
}

const createCfg = `
resource "f5os_tls_cert_key" "testcert" {
  name                     = "testcert"
  days_valid               = 40
  email                    = "user@org.com"
  city                     = "Hyd"
  province                 = "Telangana"
  country                  = "IN"
  organization             = "F7"
  unit                     = "IT"
  key_type                 = "encrypted-rsa"
  key_size                 = 2048
  key_passphrase           = "test123"
  confirm_key_passphrase   = "test123"
}
`

const updateCfg = `
resource "f5os_tls_cert_key" "testcert" {
  name                     = "testcert"
  days_valid               = 400
  email                    = "user@org.com"
  city                     = "Hyd"
  province                 = "Telangana"
  country                  = "IN"
  organization             = "F8"
  unit                     = "IT"
  key_type                 = "encrypted-rsa"
  key_size                 = 2048
  key_passphrase           = "test123"
  confirm_key_passphrase   = "test123"
}
`

const errorCfg = `
resource "f5os_tls_cert_key" "testcert" {
  name                     = "testcert"
  subject_alternative_name = "DNS:www.example.com"
  days_valid               = 400
  email                    = "user@org.com"
  city                     = "Hyd"
  province                 = "Telangana"
  country                  = "IN"
  organization             = "F8"
  unit                     = "IT"
  key_type                 = "encrypted-rsa"
  key_size                 = 2048
  key_passphrase           = "test123"
  confirm_key_passphrase   = "test123"
}
`
