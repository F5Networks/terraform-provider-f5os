package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSystemCreateTC1Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccSystemCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Todays weather is great!"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Welcome to the system."),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "UTC"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3600"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "15"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.1", "aes256-gcm@openssh.com"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_mac_alg.0", "hmac-sha1-96"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_hkey_alg.0", "ssh-rsa"),
				),
			},
			// ImportState testing
			// {
			// 	ResourceName:      "f5os_system.system_settings",
			// 	ImportState:       true,
			// 	ImportStateVerify: true,
			// },
		},
	})
}

func TestAccSystemUpdateTC2Resource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccSystemCreateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Todays weather is great!"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Welcome to the system."),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "UTC"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3600"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "15"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_idle_timeout", "1800"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.1", "aes256-gcm@openssh.com"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_mac_alg.0", "hmac-sha1-96"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_hkey_alg.0", "ssh-rsa"),
				),
			},
			{
				Config: testAccSystemUpdateResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system.example.net"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Todays weather is great Update!"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Welcome to the updated system."),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "Poland"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3500"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "16"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.1", "ecdh-sha2-nistp521"),
				),
			},
		},
	})
}

// func TestAccSystemCreateUnitTC1Resource(t *testing.T) {
// 	testAccPreUnitCheck(t)

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method == "GET" {
// 			assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
// 			w.Header().Set("Content-Type", "application/yang-data+json")
// 			w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9.eyJhdXRoaW5mbyI6ImFkbWluIDEwMDAgOTAwMCBcL3ZhclwvRjVcL3BhcnRpdGlvbiIsImV4cCI6MTY4MDcyMDc4MiwiaWF0IjoxNjgwNzE5ODgyLCJyZW5ld2xpbWl0IjoiNSIsInVzZXJpbmZvIjoiYWRtaW4gMTcyLjE4LjIzMy4yMiJ9.c6Fw4AVm9dN4F-rRJZ1655Ks3xEWCzdAvum-Q3K7cwU")
// 			_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
// 		}
// 		if r.Method == "PATCH" {
// 			w.WriteHeader(http.StatusOK)
// 			_, _ = fmt.Fprintf(w, ``)
// 		}
// 		count++
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusOK)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-system-settings:settings", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=httpd/config", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/ciphers", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/kexalgorithms", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/macs", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/f5-security-ciphers:security/services/service=sshd/config/host-key-algorithms", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusNoContent)
// 		_, _ = fmt.Fprintf(w, ``)
// 	})

// 	mux.HandleFunc("/restconf/data/openconfig-system:system/config", func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusOK)
// 		_, _ = fmt.Fprintf(w, `{
// 			"openconfig-system:config": {
// 				"hostname": "system.example.net",
// 				"login-banner": "Welcome to the system.",
// 				"motd-banner": "Todays weather is great!"
// 			}
// 		}`,
// 		)
// 	})
// 	defer teardown()
// 	resource.Test(t, resource.TestCase{
// 		IsUnitTest:               true,
// 		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
// 		Steps: []resource.TestStep{
// 			// Read testing
// 			{
// 				Config: testAccSystemCreateResourceConfig,
// 				Check: resource.ComposeAggregateTestCheckFunc(
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "id", "system.example.net"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "motd", "Todays weather is great!"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "login_banner", "Welcome to the system."),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "timezone", "UTC"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "cli_timeout", "3600"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "token_lifetime", "15"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "httpd_ciphersuite", "ECDHE-RSA-AES256-GCM-SHA384"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_idle_timeout", "1800"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.0", "aes256-ctr"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_ciphers.1", "aes256-gcm@openssh.com"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_kex_alg.0", "ecdh-sha2-nistp384"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_mac_alg.0", "hmac-sha1-96"),
// 					resource.TestCheckResourceAttr("f5os_system.system_settings", "sshd_hkey_alg.0", "ssh-rsa"),
// 				),
// 			},
// 		},
// 	})
// }

const testAccSystemCreateResourceConfig = `
resource "f5os_system" "system_settings" {
  hostname = "system.example.net"
  motd = "Todays weather is great!"
  login_banner = "Welcome to the system."
  timezone = "UTC"
  cli_timeout = 3600
  token_lifetime = 15
  sshd_idle_timeout = 1800
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers = ["aes256-ctr", "aes256-gcm@openssh.com"]
  sshd_kex_alg = ["ecdh-sha2-nistp384"]
  sshd_mac_alg = ["hmac-sha1-96"]
  sshd_hkey_alg = ["ssh-rsa"]
}`

const testAccSystemUpdateResourceConfig = `
resource "f5os_system" "system_settings" {
  hostname = "system.example.net"
  motd = "Todays weather is great Update!"
  login_banner = "Welcome to the updated system."
  timezone = "Poland"
  cli_timeout = 3500
  token_lifetime = 16
  sshd_idle_timeout = 1800
  httpd_ciphersuite = "ECDHE-RSA-AES256-GCM-SHA384"
  sshd_ciphers = ["aes256-ctr"]
  sshd_kex_alg = ["ecdh-sha2-nistp384", "ecdh-sha2-nistp521"]
  sshd_mac_alg = ["hmac-sha1-96"]
  sshd_hkey_alg = ["ssh-rsa"]
}`
