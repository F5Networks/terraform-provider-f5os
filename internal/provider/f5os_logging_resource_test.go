package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Helpers to get cert/key/ca_bundle from env
func getTestCerts(t *testing.T) (string, string) {
	cert := os.Getenv("F5OS_TEST_CERT")
	key := os.Getenv("F5OS_TEST_KEY")
	if cert == "" || key == "" {
		t.Skip("F5OS_TEST_CERT and F5OS_TEST_KEY must be set for this test")
	}
	return cert, key
}

func getTestCABundle(t *testing.T) string {
	cabundle := os.Getenv("F5OS_TEST_CA_BUNDLE")
	if cabundle == "" {
		t.Skip("F5OS_TEST_CA_BUNDLE must be set for this test")
	}
	return cabundle
}

// Acceptance test using env cert/key/ca_bundle
func TestAccF5osLogging_Logging(t *testing.T) {
	cert, key := getTestCerts(t)
	cabundle := getTestCABundle(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`


resource "f5os_logging" "logging" {

  include_hostname = false

  servers = [
    {
      address        = "192.168.100.1"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "local0"
          severity = "debug"
        }
      ]
    },
    {
      address        = "192.168.100.2"
      port           = 514
      protocol       = "tcp"
      authentication = false
      logs = [
        {
          facility = "authpriv"
          severity = "emergency"
        }
      ]
    }
  ]

  remote_forwarding = {
    enabled = true

    logs = [
      {
        facility = "local0"
        severity = "error"
      },
      {
        facility = "authpriv"
        severity = "critical"
      }
    ]

    files = [
      {
        name = "rseries_debug.log"
      },
      {
        name = "rseries_audit.log"
      }
    ]
  }

  tls = {
    certificate = <<EOT
%s
EOT

    key = <<EOT
%s
EOT
  }

  ca_bundles = [
    {
      name    = "rseries-ca"
      content = <<EOT
%s
EOT
    }
  ]
}
`, cert, key, cabundle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_logging.logging", "include_hostname", "false"),
					resource.TestCheckResourceAttr("f5os_logging.logging", "servers.0.address", "192.168.100.1"),
					resource.TestCheckResourceAttr("f5os_logging.logging", "servers.1.address", "192.168.100.2"),
					resource.TestCheckResourceAttr("f5os_logging.logging", "tls.certificate", cert),
					resource.TestCheckResourceAttr("f5os_logging.logging", "tls.key", key),
					resource.TestCheckResourceAttr("f5os_logging.logging", "ca_bundles.0.content", cabundle),
				),
			},
		},
	})
}
