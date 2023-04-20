package provider

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestAccPartitionDeployResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		//IsUnitTest:               true,
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccPartitionDeployResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "id", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "name", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "os_version", "1.3.1-5968"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "ipv4_mgmt_address", "10.144.140.125/24"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "ipv4_mgmt_gateway", "10.144.140.253"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "ipv6_mgmt_address", "2001:db8:3333:4444:5555:6666:7777:8888/64"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "ipv6_mgmt_gateway", "2001:db8:3333:4444::"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "f5os_partition.velos-part",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestUnitPartitionDeployResource(t *testing.T) {
	testAccPreUnitCheck(t)
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method, "Expected method 'GET', got %s", r.Method)
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.Header().Set("X-Auth-Token", "eyJhbGciOiJIXzI2NiIsInR6cCI6IkcXVCJ9")
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/f5os_auth.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-platform:components/component=platform/state/description", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/platform_state.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/restconf/data/openconfig-vlan:vlans", r.URL.String(), "Expected method 'GET', got %s", r.URL.String())
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/velos_ctrl_resp.json"))
	})

	// device calls to create resource
	mux.HandleFunc("/restconf/data/f5-system-partition:partitions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("restconf/data/f5-system-slot:slots", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, ``)
	})
	mux.HandleFunc("/restconf/data/f5-system-partition:partitions/partition=TerraformPartition/state", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/partition_get_status.json"))
	})

	mux.HandleFunc("/restconf/data/f5-system-partition:partitions/partition=TerraformPartition", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/partition_config.json"))
	})

	mux.HandleFunc("/restconf/data/f5-system-slot:slots/slot", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "%s", loadFixtureString("./fixtures/partition_get_slots.json"))
	})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccPartitionDeployResourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "id", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "name", "TerraformPartition"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "os_version", "1.3.1-5968"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "ipv4_mgmt_address", "10.144.140.125/24"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "ipv4_mgmt_gateway", "10.144.140.253"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "ipv6_mgmt_address", "2001:db8:3333:4444:5555:6666:7777:8888/64"),
					resource.TestCheckResourceAttr("f5os_partition.velos-part", "ipv6_mgmt_gateway", "2001:db8:3333:4444::"),
				),
			},
		},
	})
}

const testAccPartitionDeployResourceConfig = `
resource "f5os_partition" "velos-part" {
  name = "TerraformPartition" 
  os_version = "1.3.1-5968"
  ipv4_mgmt_address = "10.144.140.125/24"
  ipv4_mgmt_gateway = "10.144.140.253"
  ipv6_mgmt_address = "2001:db8:3333:4444:5555:6666:7777:8888/64"
  ipv6_mgmt_gateway = "2001:db8:3333:4444::"
  slots = [1,2]
}
`
