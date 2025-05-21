package provider

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// -- Structs used in DNS config patch payload --

type DNSServer struct {
	Address string `json:"address"`
}

type DNSConfigServers struct {
	Server []DNSServer `json:"server"`
}

type DNSConfigSearch struct {
	Search []string `json:"search"`
}

type DNSConfig struct {
	Servers DNSConfigServers `json:"servers"`
	Config  DNSConfigSearch  `json:"config"`
}

type DNSConfigPayload struct {
	DNS DNSConfig `json:"openconfig-system:dns"`
}

// -- Mock F5os struct --

type F5os struct {
	UriRoot string
}

// -- Acceptance Test (requires live F5OS device) --

func TestAccF5osDNSResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccF5osDNSConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "internal.domain"), // fix this index
					resource.TestCheckResourceAttr("f5os_dns.test", "id", "dns"),
				),
			},
		},
	})
}

const testAccF5osDNSConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["internal.domain"]
}
`

// This is a simple test case to verify the basic behavior of DNS resource
func TestUnitF5osDNSResource(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/dns", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:dns": {
					"servers": {
						"server": [
							{ "address": "8.8.8.8" }
						]
					},
					"config": {
						"search": ["internal.domain"]
					}
				}
			}`))
		case "PATCH":
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
				Config: testAccF5osDNSConfigSingle,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "internal.domain"),

					// resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					// resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "internal.domain"),
				),
			},
		},
	})
}

const testAccF5osDNSConfigSingle = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["internal.domain"]
}`
