package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
// providerConfig is a shared configuration to combine with the actual
// test configuration so the HashiCups client is properly configured.
// It is also possible to use the HASHICUPS_ environment variables instead,
// such as updating the Makefile and running the testing through that tool.
// providerConfig = “
)

var (
	// testAccProtoV6ProviderFactories are used to instantiate a provider during
	// acceptance testing. The factory function will be invoked for every Terraform
	// CLI command executed to create a provider server to which the CLI can
	// reattach.
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"f5os": providerserver.NewProtocol6WithError(New("devel")()),
	}
)

func testAccPreCheck(t *testing.T) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
	for _, s := range [...]string{"F5OS_HOST", "F5OS_USERNAME", "F5OS_PASSWORD"} {
		if os.Getenv(s) == "" {
			t.Fatal("F5OS_HOST, F5OS_USERNAME and F5OS_PASSWORD are required for tests.")
			return
		}
	}
}
