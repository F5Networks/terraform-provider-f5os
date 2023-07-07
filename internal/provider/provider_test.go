package provider

import (
	"net/http"
	"net/http/httptest"
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
// f5osURI = "https://localhost:60155"
// f5osURI = "http://192.168.10.10:8888"
)

var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// server is a test HTTP server used to provide mock API responses
	server *httptest.Server
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

func testAccPreUnitCheck(t *testing.T) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
	setup()
	_ = os.Setenv("F5OS_HOST", server.URL)
	_ = os.Setenv("F5OS_USERNAME", "testuser")
	_ = os.Setenv("F5OS_PASSWORD", "testpass")
	//defer teardown()
}

func setup() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)
}

func teardown() {
	server.Close()
}

// loadFixtureBytes returns the entire contents of the given file as a byte slice
func loadFixtureBytes(path string) []byte {
	contents, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// loadFixtureString returns the entire contents of the given file as a string
func loadFixtureString(path string) string {
	return string(loadFixtureBytes(path))
}
