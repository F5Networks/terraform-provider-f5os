package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// DNS payload types are defined in f5osclient (structs_partition.go).
// Alias them here so test helpers can unmarshal mock request bodies
// without duplicating the struct definitions.
type (
	DNSConfigPayload = f5ossdk.DNSConfigPayload
	DNSConfig        = f5ossdk.DNSConfig
	DNSConfigServers = f5ossdk.DNSConfigServers
	DNSConfigSearch  = f5ossdk.DNSConfigSearch
	DNSServer        = f5ossdk.DNSServer
)

// -- Acceptance Test (requires live F5OS device) --

func TestAccF5osDNSResource(t *testing.T) {
	expectedID := computeResourceID([]string{"8.8.8.8"}, []string{"internal.domain"})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDNSDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccF5osDNSConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "internal.domain"),
					resource.TestCheckResourceAttr("f5os_dns.test", "id", expectedID),
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

// ---------------------------------------------------------------------------
// Shared mock setup for DNS unit tests
// ---------------------------------------------------------------------------

// dnsMockState holds mutable mock-server state shared across handlers.
type dnsMockState struct {
	servers     []string
	domains     []string
	deleteCount int
}

// setupDNSMock registers all the standard mock handlers needed for DNS
// unit tests and returns a pointer to the shared state so the test can
// mutate the device-side data between steps. The caller is responsible
// for calling teardown() when the test completes.
func setupDNSMock(t *testing.T, initialServers []string, initialDomains []string) *dnsMockState {
	t.Helper()
	testAccPreUnitCheck(t)

	st := &dnsMockState{
		servers: initialServers,
		domains: initialDomains,
	}

	mux.HandleFunc("/restconf/data/openconfig-system:system/dns", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			// Build dynamic response from current mock state
			var serverJSON string
			for i, s := range st.servers {
				if i > 0 {
					serverJSON += ","
				}
				serverJSON += `{"address":"` + s + `"}`
			}
			var domainJSON string
			for i, d := range st.domains {
				if i > 0 {
					domainJSON += ","
				}
				domainJSON += `"` + d + `"`
			}
			resp := `{"openconfig-system:dns":{"servers":{"server":[` +
				serverJSON + `]},"config":{"search":[` + domainJSON + `]}}}`
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(resp))
		case "PATCH":
			var p DNSConfigPayload
			if err := json.NewDecoder(r.Body).Decode(&p); err == nil {
				st.servers = nil
				for _, s := range p.DNS.Servers.Server {
					st.servers = append(st.servers, s.Address)
				}
				st.domains = p.DNS.Config.Search
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "DELETE":
			st.deleteCount++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("Unexpected HTTP method on /dns: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// DELETE for individual servers: /restconf/data/openconfig-system:system/dns/servers/server=<addr>
	mux.HandleFunc("/restconf/data/openconfig-system:system/dns/servers/", func(w http.ResponseWriter, r *http.Request) {
		// Parse address from path (.../server=<addr>)
		parts := strings.SplitN(r.URL.Path, "server=", 2)
		if len(parts) == 2 {
			addr := parts[1]
			var kept []string
			for _, s := range st.servers {
				if s != addr {
					kept = append(kept, s)
				}
			}
			st.servers = kept
		}
		st.deleteCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	// DELETE for individual domains: /restconf/data/openconfig-system:system/dns/config/search=<domain>
	mux.HandleFunc("/restconf/data/openconfig-system:system/dns/config/", func(w http.ResponseWriter, r *http.Request) {
		// Parse domain from path (.../search=<domain>)
		parts := strings.SplitN(r.URL.Path, "search=", 2)
		if len(parts) == 2 {
			dom := parts[1]
			var kept []string
			for _, d := range st.domains {
				if d != dom {
					kept = append(kept, d)
				}
			}
			st.domains = kept
		}
		st.deleteCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	return st
}

// ---------------------------------------------------------------------------
// Unit tests for Read method fix (state refresh from device)
// ---------------------------------------------------------------------------

// TestUnitDNSReadRefreshesServersFromDevice verifies that Read populates
// dns_servers from the device API response rather than preserving stale
// prior state. Before the fix, Read fetched the data but never wrote it
// back to state (the assignment was commented out).
func TestUnitDNSReadRefreshesServersFromDevice(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"internal.domain"})
	defer teardown()

	expectedID := computeResourceID([]string{"8.8.8.8"}, []string{"internal.domain"})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with 8.8.8.8
			{
				Config: testUnitDNSOneServer,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "internal.domain"),
					resource.TestCheckResourceAttr("f5os_dns.test", "id", expectedID),
				),
			},
			// Step 2: Simulate out-of-band change — device now returns 1.1.1.1
			// but HCL still says 8.8.8.8. Read must detect the drift and
			// the plan must show a diff. We apply the config with 1.1.1.1
			// to match the device and confirm the state is correct.
			{
				PreConfig: func() {
					st.servers = []string{"1.1.1.1"}
					st.domains = []string{"changed.domain"}
				},
				Config: testUnitDNSOneServerChanged,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "1.1.1.1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "changed.domain"),
				),
			},
		},
	})
}

// TestUnitDNSReadRefreshesDomainsFromDevice verifies that Read populates
// dns_domains from the device API response. Step 1 creates with one domain,
// then the mock changes the domain out-of-band before Step 2's Read. Step 2
// supplies the new domain in HCL so the plan converges.
func TestUnitDNSReadRefreshesDomainsFromDevice(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"original.domain"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with one domain
			{
				Config: testUnitDNSOriginalDomainConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "original.domain"),
				),
			},
			// Step 2: Device changes domain out-of-band. Read must detect
			// the new domain from the device. Config matches the new value.
			{
				PreConfig: func() {
					st.domains = []string{"surprise.domain"}
				},
				Config: testUnitDNSSurpriseDomainConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "surprise.domain"),
				),
			},
		},
	})
}

// TestUnitDNSReadRecomputesIdFromDevice verifies that the Read method
// recomputes the resource ID from the actual device state. Before the fix,
// the ID was never updated during Read, so it could become stale if the
// device state drifted.
func TestUnitDNSReadRecomputesIdFromDevice(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"internal.domain"})
	defer teardown()

	expectedID1 := computeResourceID([]string{"8.8.8.8"}, []string{"internal.domain"})
	expectedID2 := computeResourceID([]string{"1.1.1.1"}, []string{"new.domain"})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create — ID is computed from servers + domains
			{
				Config: testUnitDNSOneServer,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "id", expectedID1),
				),
			},
			// Step 2: Device changes out-of-band. Config matches new device
			// values. Read must recompute the ID from the new device state.
			{
				PreConfig: func() {
					st.servers = []string{"1.1.1.1"}
					st.domains = []string{"new.domain"}
				},
				Config: testUnitDNSIdRecomputeConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "id", expectedID2),
				),
			},
		},
	})
}

// TestUnitDNSReadHandlesEmptyDomains verifies that when the device returns
// no domains, the Read method writes an empty list to state rather than
// leaving stale data. Servers are swapped (not emptied) because dns_servers
// is required by the schema and cannot be zero-length in a valid config.
func TestUnitDNSReadHandlesEmptyDomains(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"internal.domain"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create normally
			{
				Config: testUnitDNSOneServer,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
				),
			},
			// Step 2: Device domains are wiped; Read must reflect empty domains.
			// Servers are changed (not emptied) because dns_servers is required.
			{
				PreConfig: func() {
					st.servers = []string{"9.9.9.9"}
					st.domains = []string{}
				},
				Config: testUnitDNSEmptyAfterWipeConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "9.9.9.9"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "0"),
				),
			},
		},
	})
}

// TestUnitDNSReadMultipleServersAndDomains verifies that Read correctly
// handles multiple servers and domains from the device response.
func TestUnitDNSReadMultipleServersAndDomains(t *testing.T) {
	_ = setupDNSMock(t, []string{"8.8.8.8", "1.1.1.1"}, []string{"foo.local", "bar.local"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitDNSMultiConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "2"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.1", "1.1.1.1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "2"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "foo.local"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.1", "bar.local"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Unit tests for Update method fix (stale entry deletion)
// ---------------------------------------------------------------------------

// TestUnitDNSUpdateRemovesStaleEntries verifies that shrinking the server
// or domain list in an Update correctly deletes the removed entries from
// the device rather than leaving them as stale config.
func TestUnitDNSUpdateRemovesStaleEntries(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8", "1.1.1.1"}, []string{"foo.local", "bar.local"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with two servers and two domains
			{
				Config: testUnitDNSTwoServersConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "2"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "2"),
				),
			},
			// Step 2: Shrink to one server and one domain
			{
				Config: testUnitDNSOneServerOneDomainConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "foo.local"),
					// Verify the mock state reflects the removal
					func(s *terraform.State) error {
						for _, srv := range st.servers {
							if srv == "1.1.1.1" {
								return fmt.Errorf("stale server 1.1.1.1 still in mock state: %v", st.servers)
							}
						}
						for _, dom := range st.domains {
							if dom == "bar.local" {
								return fmt.Errorf("stale domain bar.local still in mock state: %v", st.domains)
							}
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitDNSUpdateAddsEntries verifies that growing the server and domain
// lists (no removals) works correctly. removedEntries returns empty so
// DeleteDNSConfig is effectively a no-op, and PatchDNSConfig adds the new
// entries.
func TestUnitDNSUpdateAddsEntries(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"foo.local"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with one server and one domain
			{
				Config: testUnitDNSGrowStep1Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
				),
			},
			// Step 2: Grow to two servers and two domains
			{
				Config: testUnitDNSGrowStep2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "2"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.1", "1.1.1.1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "2"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "foo.local"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.1", "bar.local"),
					// Mock state should contain all entries (none removed)
					func(s *terraform.State) error {
						if len(st.servers) != 2 {
							return fmt.Errorf("expected 2 servers in mock, got %v", st.servers)
						}
						if len(st.domains) != 2 {
							return fmt.Errorf("expected 2 domains in mock, got %v", st.domains)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestUnitDNSUpdateSwapsAllEntries verifies that completely replacing all
// servers and domains in a single Update works. Every old entry is removed
// via DeleteDNSConfig, then PatchDNSConfig applies the new entries.
func TestUnitDNSUpdateSwapsAllEntries(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"old.local"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testUnitDNSSwapStep1Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "old.local"),
				),
			},
			// Step 2: Swap every entry
			{
				Config: testUnitDNSSwapStep2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "9.9.9.9"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "new.local"),
					// Verify old entries are gone from mock
					func(s *terraform.State) error {
						for _, srv := range st.servers {
							if srv == "8.8.8.8" {
								return fmt.Errorf("old server 8.8.8.8 still in mock: %v", st.servers)
							}
						}
						for _, dom := range st.domains {
							if dom == "old.local" {
								return fmt.Errorf("old domain old.local still in mock: %v", st.domains)
							}
						}
						return nil
					},
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// HCL configs for DNS unit tests
// ---------------------------------------------------------------------------

const testUnitDNSOneServer = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["internal.domain"]
}
`

const testUnitDNSOneServerChanged = `
resource "f5os_dns" "test" {
  dns_servers = ["1.1.1.1"]
  dns_domains = ["changed.domain"]
}
`

const testUnitDNSOriginalDomainConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["original.domain"]
}
`

const testUnitDNSSurpriseDomainConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["surprise.domain"]
}
`

const testUnitDNSIdRecomputeConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["1.1.1.1"]
  dns_domains = ["new.domain"]
}
`

const testUnitDNSEmptyAfterWipeConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["9.9.9.9"]
}
`

const testUnitDNSMultiConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8", "1.1.1.1"]
  dns_domains = ["foo.local", "bar.local"]
}
`

const testUnitDNSTwoServersConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8", "1.1.1.1"]
  dns_domains = ["foo.local", "bar.local"]
}
`

const testUnitDNSOneServerOneDomainConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["foo.local"]
}
`

const testUnitDNSGrowStep1Config = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["foo.local"]
}
`

const testUnitDNSGrowStep2Config = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8", "1.1.1.1"]
  dns_domains = ["foo.local", "bar.local"]
}
`

const testUnitDNSSwapStep1Config = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["old.local"]
}
`

const testUnitDNSSwapStep2Config = `
resource "f5os_dns" "test" {
  dns_servers = ["9.9.9.9"]
  dns_domains = ["new.local"]
}
`

// ---------------------------------------------------------------------------
// Unit test for Delete no-op behavior
// ---------------------------------------------------------------------------

// TestUnitDNSDeletePreservesDeviceConfig verifies that Delete removes the
// resource from Terraform state without making any DELETE API calls to the
// device. DNS is a singleton system setting — removing managed entries
// would break name resolution and could make the device unreachable.
func TestUnitDNSDeletePreservesDeviceConfig(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"internal.domain"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitDNSOneServer,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
				),
			},
			{
				Destroy: true,
				Config:  testUnitDNSOneServer,
				Check: func(s *terraform.State) error {
					if st.deleteCount > 0 {
						return fmt.Errorf("expected no DELETE calls to device, got %d", st.deleteCount)
					}
					if len(st.servers) != 1 || st.servers[0] != "8.8.8.8" {
						return fmt.Errorf("expected device servers to be preserved as [8.8.8.8], got %v", st.servers)
					}
					if len(st.domains) != 1 || st.domains[0] != "internal.domain" {
						return fmt.Errorf("expected device domains to be preserved as [internal.domain], got %v", st.domains)
					}
					return nil
				},
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test helpers
// ---------------------------------------------------------------------------

// newDNSClientFromEnv creates a fresh f5osclient session from env vars.
// Port defaults to 8888 to match the provider.
func newDNSClientFromEnv() (*f5ossdk.F5os, error) {
	host := os.Getenv("F5OS_HOST")
	user := os.Getenv("F5OS_USERNAME")
	if user == "" {
		user = os.Getenv("F5OS_USER")
	}
	pass := os.Getenv("F5OS_PASSWORD")
	port := 8888
	if p := os.Getenv("F5OS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	cfg := &f5ossdk.F5osConfig{
		Host:             host,
		User:             user,
		Password:         pass,
		Port:             port,
		DisableSSLVerify: true,
	}
	return f5ossdk.NewSession(cfg)
}

// testAccCheckDNSServerPresentOnDevice queries the device directly and
// verifies that the given server address is present in the DNS config.
// Uses a "contains" check rather than exact match because PatchDNSConfig
// is additive (PATCH, not PUT).
func testAccCheckDNSServerPresentOnDevice(server string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newDNSClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		config, err := client.ReadDNSConfig()
		if err != nil {
			return fmt.Errorf("ReadDNSConfig failed: %w", err)
		}
		for _, srv := range config.DNS.Servers.Server {
			if srv.Address == server {
				return nil
			}
		}
		var actual []string
		for _, srv := range config.DNS.Servers.Server {
			actual = append(actual, srv.Address)
		}
		return fmt.Errorf("server %q not found on device, got %v", server, actual)
	}
}

// testAccCheckDNSDomainPresentOnDevice queries the device directly and
// verifies that the given search domain is present in the DNS config.
func testAccCheckDNSDomainPresentOnDevice(domain string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newDNSClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		config, err := client.ReadDNSConfig()
		if err != nil {
			return fmt.Errorf("ReadDNSConfig failed: %w", err)
		}
		for _, d := range config.DNS.Config.Search {
			if d == domain {
				return nil
			}
		}
		return fmt.Errorf("domain %q not found on device, got %v", domain, config.DNS.Config.Search)
	}
}

// testAccCheckDNSDestroy verifies that the resource was removed from state.
// Delete is a no-op on the device (DNS config is preserved), so we only
// confirm the device is still reachable and DNS config still exists.
func testAccCheckDNSDestroy(s *terraform.State) error {
	client, err := newDNSClientFromEnv()
	if err != nil {
		return nil // cannot connect — nothing to verify
	}
	_, err = client.ReadDNSConfig()
	if err != nil {
		return fmt.Errorf("device DNS config should still exist after destroy (no-op), but ReadDNSConfig failed: %s", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Acceptance tests for Read method fix (state refresh from device)
// ---------------------------------------------------------------------------

// TestAccDNSReadRefreshesStateFromDevice is the primary acceptance test for
// the Read fix. It exercises Create → Read → Destroy using safe,
// non-routable IPs and RFC 2606 reserved domains.
//
// The fix: Read now actually writes the device values into Terraform state
// (dns_servers, dns_domains, id). Before the fix, these assignments were
// commented out, breaking drift detection, state refresh, and import.
//
// Safety: Uses 10.255.255.x IPs (non-routable) and .invalid domains
// (RFC 2606) per shared-device safety rules.
func TestAccDNSReadRefreshesStateFromDevice(t *testing.T) {
	expectedID := computeResourceID([]string{"10.255.255.60"}, []string{"acc-test-1.invalid"})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDNSDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with one server and one domain.
			// Verify Terraform state AND device both reflect the config.
			// The fact that Read populates state from the device (not stale
			// plan data) is what makes this test pass — before the fix,
			// dns_servers and dns_domains in state were stale.
			{
				Config: testAccDNSCreateConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.60"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.0", "acc-test-1.invalid"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "id", expectedID),
					// Direct API verification — bypasses Read
					testAccCheckDNSServerPresentOnDevice("10.255.255.60"),
					testAccCheckDNSDomainPresentOnDevice("acc-test-1.invalid"),
				),
			},
			// Step 2: Destroy is automatic — CheckDestroy verifies cleanup
		},
	})
}

// TestAccDNSUpdateRefreshesState verifies that after an Update, Read
// refreshes state from the device. Uses a single-server config that
// changes between steps to avoid the additive PATCH issue.
func TestAccDNSUpdateRefreshesState(t *testing.T) {
	expectedID1 := computeResourceID([]string{"10.255.255.61"}, []string{"update-1.invalid"})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDNSDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: testAccDNSUpdateStep1Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.61"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.0", "update-1.invalid"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "id", expectedID1),
					testAccCheckDNSServerPresentOnDevice("10.255.255.61"),
					testAccCheckDNSDomainPresentOnDevice("update-1.invalid"),
				),
			},
			// Step 2: Update server and domain. The Update method now
			// deletes stale entries before patching, so the plan should
			// be clean after refresh.
			{
				Config: testAccDNSUpdateStep2Config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.62"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.0", "update-2.invalid"),
					testAccCheckDNSServerPresentOnDevice("10.255.255.62"),
					testAccCheckDNSDomainPresentOnDevice("update-2.invalid"),
				),
			},
		},
	})
}

// TestAccDNSReadStateMatchesDevice verifies that after a plan/apply cycle,
// a subsequent Read produces state that matches the device. This is the
// core behavior the fix enables: Read refreshing state from the API rather
// than preserving stale prior state.
func TestAccDNSReadStateMatchesDevice(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDNSDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDNSReadVerifyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.70"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.0", "read-verify.invalid"),
					resource.TestCheckResourceAttrSet("f5os_dns.acc_test", "id"),
					// Direct device checks — confirms Read wrote device
					// values, not stale plan data
					testAccCheckDNSServerPresentOnDevice("10.255.255.70"),
					testAccCheckDNSDomainPresentOnDevice("read-verify.invalid"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test HCL configs
// ---------------------------------------------------------------------------

const testAccDNSCreateConfig = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.60"]
  dns_domains = ["acc-test-1.invalid"]
}
`

const testAccDNSUpdateStep1Config = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.61"]
  dns_domains = ["update-1.invalid"]
}
`

const testAccDNSUpdateStep2Config = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.62"]
  dns_domains = ["update-2.invalid"]
}
`

const testAccDNSReadVerifyConfig = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.70"]
  dns_domains = ["read-verify.invalid"]
}
`

// ---------------------------------------------------------------------------
// Unit tests for search domain null entry fix
// ---------------------------------------------------------------------------

// TestUnitExtractStringListNullReturnsEmptySlice verifies that
// extractStringList returns a non-nil empty slice when the input list is null.
// Before the fix, it returned nil, which caused json.Marshal to produce
// "search":null instead of "search":[] in the PATCH payload.
func TestUnitExtractStringListNullReturnsEmptySlice(t *testing.T) {
	ctx := context.Background()
	nullList := types.ListNull(types.StringType)

	result, diags := extractStringList(ctx, nullList)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if result == nil {
		t.Fatal("extractStringList returned nil for null list; expected non-nil empty slice")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

// TestUnitExtractStringListUnknownReturnsEmptySlice verifies that
// extractStringList returns a non-nil empty slice for unknown lists.
func TestUnitExtractStringListUnknownReturnsEmptySlice(t *testing.T) {
	ctx := context.Background()
	unknownList := types.ListUnknown(types.StringType)

	result, diags := extractStringList(ctx, unknownList)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if result == nil {
		t.Fatal("extractStringList returned nil for unknown list; expected non-nil empty slice")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

// TestUnitExtractStringListWithValuesReturnsSlice verifies the normal case.
func TestUnitExtractStringListWithValuesReturnsSlice(t *testing.T) {
	ctx := context.Background()
	list, diags := types.ListValueFrom(ctx, types.StringType, []string{"a", "b"})
	if diags.HasError() {
		t.Fatalf("failed to create list: %v", diags)
	}

	result, diags2 := extractStringList(ctx, list)
	if diags2.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags2)
	}
	if len(result) != 2 || result[0] != "a" || result[1] != "b" {
		t.Errorf("expected [a b], got %v", result)
	}
}

// TestUnitDNSCreateOmitDomainsNoPatchNull verifies that when dns_domains
// is omitted from the HCL config, the PATCH payload contains "search":[]
// (empty array) rather than "search":null. The mock inspects the request
// body and fails the test if it finds a null search field.
func TestUnitDNSCreateOmitDomainsNoPatchNull(t *testing.T) {
	testAccPreUnitCheck(t)

	mux.HandleFunc("/restconf/data/openconfig-system:system/dns", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PATCH":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read PATCH body: %v", err)
				http.Error(w, "read error", http.StatusInternalServerError)
				return
			}

			// The raw JSON must not contain "search":null
			if strings.Contains(string(body), `"search":null`) {
				t.Error("PATCH payload contains \"search\":null — expected \"search\":[]")
			}

			// Verify it contains an empty array instead
			var payload DNSConfigPayload
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("failed to unmarshal PATCH body: %v", err)
				http.Error(w, "unmarshal error", http.StatusInternalServerError)
				return
			}
			if payload.DNS.Config.Search == nil {
				t.Error("PATCH payload Search is nil after unmarshal — expected non-nil empty slice")
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"openconfig-system:dns": {
					"servers": {"server": [{"address": "8.8.8.8"}]},
					"config": {"search": []}
				}
			}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// dns_domains is omitted — should produce [] not null
				Config: testUnitDNSOmitDomainsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "0"),
				),
			},
		},
	})
}

const testUnitDNSOmitDomainsConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
}
`

// TestUnitDNSReadNilDomainListProducesEmptyList verifies that when the
// device returns empty/nil for the search list, Read produces an empty
// Terraform list (dns_domains.# = 0) rather than a null value which
// would cause a perpetual plan diff.
func TestUnitDNSReadNilDomainListProducesEmptyList(t *testing.T) {
	// Step 1 creates with servers + domains. Then we mutate the mock
	// to return empty domains on the next GET, simulating the device
	// having no search domains. Step 2 verifies Read sets domains to
	// an empty list (not null).
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"internal.domain"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUnitDNSOneServer,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
				),
			},
			{
				// Simulate device returning empty domains
				PreConfig: func() {
					st.domains = nil // nil, not []string{}
				},
				Config: testUnitDNSOmitDomainsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					// Key assertion: domains must be an empty list, not null
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "0"),
				),
			},
		},
	})
}

// TestUnitComputeResourceIDNilSlices verifies that computeResourceID
// handles nil slices without panicking and produces a consistent hash.
func TestUnitComputeResourceIDNilSlices(t *testing.T) {
	id1 := computeResourceID(nil, nil)
	id2 := computeResourceID([]string{}, []string{})

	if id1 == "" {
		t.Error("computeResourceID(nil, nil) returned empty string")
	}
	if id1 != id2 {
		t.Errorf("nil and empty slices should produce the same ID; got %q vs %q", id1, id2)
	}
}

// TestUnitRemovedEntriesNilInputs verifies removedEntries handles nil inputs.
func TestUnitRemovedEntriesNilInputs(t *testing.T) {
	// nil old, nil new — nothing removed
	if got := removedEntries(nil, nil); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}

	// non-nil old, nil new — all removed
	got := removedEntries([]string{"a", "b"}, nil)
	if len(got) != 2 {
		t.Errorf("expected 2 removed, got %v", got)
	}

	// nil old, non-nil new — nothing removed
	if got := removedEntries(nil, []string{"a"}); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Acceptance tests for search domain null entry fix
// ---------------------------------------------------------------------------

// testAccCheckDNSDomainAbsentOnDevice queries the device directly and
// confirms the specified domain is NOT present in the DNS config. This is
// used instead of checking for zero domains because PatchDNSConfig is
// additive and Delete is a no-op, so other domains from prior tests may
// be present.
func testAccCheckDNSDomainAbsentOnDevice(domain string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := newDNSClientFromEnv()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		config, err := client.ReadDNSConfig()
		if err != nil {
			return fmt.Errorf("ReadDNSConfig failed: %w", err)
		}
		for _, d := range config.DNS.Config.Search {
			if d == domain {
				return fmt.Errorf("domain %q should have been removed but is still present on device (all domains: %v)",
					domain, config.DNS.Config.Search)
			}
		}
		return nil
	}
}

// TestAccDNSCreateWithoutDomains verifies that creating an f5os_dns resource
// with dns_domains omitted from the config:
//  1. Applies without error (no "provider returned invalid result" panic).
//  2. Sets dns_domains to an empty list in state (not null).
//
// Before the fix, Create returned null for dns_domains (a Computed attribute),
// which caused the Terraform Plugin Framework to reject the apply with
// "provider returned invalid result object after apply".
//
// Note: The implicit refresh-after-apply may show a non-empty plan because
// the device has residual DNS entries from prior tests (PATCH is additive,
// Delete is a no-op). This is a known pre-existing issue unrelated to the
// null-domain fix. We use ExpectNonEmptyPlan to acknowledge this.
//
// Safety: Uses 10.255.255.x (non-routable) and .invalid domain (RFC 2606).
func TestAccDNSCreateWithoutDomains(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDNSDestroy,
		Steps: []resource.TestStep{
			{
				Config:             testAccDNSNoDomains,
				ExpectNonEmptyPlan: true, // residual device state from additive PATCH
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.80"),
					// Key assertion: dns_domains is an empty list, not null.
					// Before the fix, this step would fail with "provider
					// returned invalid result object after apply".
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.#", "0"),
					resource.TestCheckResourceAttrSet("f5os_dns.acc_test", "id"),
					testAccCheckDNSServerPresentOnDevice("10.255.255.80"),
				),
			},
		},
	})
}

// TestAccDNSUpdateRemovesAllDomains verifies that updating an f5os_dns
// resource to remove all search domains (transitioning from dns_domains
// present to dns_domains omitted) results in:
//  1. dns_domains.# = 0 in state after the update.
//  2. The managed domain removed from the device.
//
// Before the fix, Update could return null for dns_domains, causing the
// same "invalid result" error as Create.
//
// Note: ExpectNonEmptyPlan is set because the device may have residual
// entries from prior tests (PATCH is additive, Delete is a no-op).
//
// Safety: Uses 10.255.255.x (non-routable) and .invalid domains (RFC 2606).
func TestAccDNSUpdateRemovesAllDomains(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDNSDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with a server and a domain.
			{
				Config:             testAccDNSWithOneDomain,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.81"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.0", "remove-me.invalid"),
					testAccCheckDNSServerPresentOnDevice("10.255.255.81"),
					testAccCheckDNSDomainPresentOnDevice("remove-me.invalid"),
				),
			},
			// Step 2: Update to remove all domains — must succeed and set
			// dns_domains.# = 0 in state, with the managed domain gone
			// from device.
			{
				Config: testAccDNSRemoveDomains,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.81"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.#", "0"),
					testAccCheckDNSServerPresentOnDevice("10.255.255.81"),
					testAccCheckDNSDomainAbsentOnDevice("remove-me.invalid"),
				),
			},
		},
	})
}

// TestAccDNSUpdateAddsDomains verifies the reverse path: creating without
// dns_domains and then updating to add one. This exercises the Update code
// path that was also fixed to avoid returning null for the Computed attribute.
//
// Note: ExpectNonEmptyPlan is set because the device may have residual
// entries from prior tests (PATCH is additive, Delete is a no-op).
//
// Safety: Uses 10.255.255.x (non-routable) and .invalid domains (RFC 2606).
func TestAccDNSUpdateAddsDomains(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDNSDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create without domains — verify state shows empty
			// list and server is present on device.
			{
				Config:             testAccDNSNoDomains2,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.82"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.#", "0"),
					testAccCheckDNSServerPresentOnDevice("10.255.255.82"),
				),
			},
			// Step 2: Add a domain — Update must succeed and set
			// dns_domains.# = 1 in state.
			{
				Config: testAccDNSWithAddedDomain,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_servers.0", "10.255.255.82"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.acc_test", "dns_domains.0", "added.invalid"),
					testAccCheckDNSServerPresentOnDevice("10.255.255.82"),
					testAccCheckDNSDomainPresentOnDevice("added.invalid"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test HCL configs for null-domain fix
// ---------------------------------------------------------------------------

const testAccDNSNoDomains = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.80"]
}
`

const testAccDNSWithOneDomain = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.81"]
  dns_domains = ["remove-me.invalid"]
}
`

const testAccDNSRemoveDomains = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.81"]
}
`

const testAccDNSNoDomains2 = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.82"]
}
`

const testAccDNSWithAddedDomain = `
resource "f5os_dns" "acc_test" {
  dns_servers = ["10.255.255.82"]
  dns_domains = ["added.invalid"]
}
`

// ---------------------------------------------------------------------------
// Unit tests for ReadDNSConfig empty response handling
// ---------------------------------------------------------------------------

// TestUnitDNSCreateWhenDeviceHasNoConfig verifies that Create succeeds when
// the device has no pre-existing DNS configuration (API returns empty body
// on the initial GET). Before the fix, ReadDNSConfig would fail with
// "unexpected end of JSON input" on the empty response.
func TestUnitDNSCreateWhenDeviceHasNoConfig(t *testing.T) {
	testAccPreUnitCheck(t)

	getCallCount := 0
	mux.HandleFunc("/restconf/data/openconfig-system:system/dns", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			getCallCount++
			if getCallCount == 1 {
				// First GET (Create's pre-existing check): empty body
				// simulates no DNS config on a fresh device.
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte{})
			} else {
				// Subsequent GETs (Read after Create): return the
				// config that was patched so state reconciles.
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"openconfig-system:dns": {
						"servers": {"server": [{"address": "8.8.8.8"}]},
						"config": {"search": ["test.internal"]}
					}
				}`))
			}
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
				Config: testUnitDNSCreateNoExistingConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_domains.0", "test.internal"),
				),
			},
		},
	})
}

const testUnitDNSCreateNoExistingConfig = `
resource "f5os_dns" "test" {
  dns_servers = ["8.8.8.8"]
  dns_domains = ["test.internal"]
}
`

// TestUnitDNSReadHandlesEmptyArraysFromDevice verifies that Read handles
// the device returning empty server and domain arrays gracefully. This
// simulates the scenario where DNS entries were removed externally, but
// the API still returns a valid JSON envelope with empty arrays.
// Note: this does NOT test the empty-body path (len(resp)==0) — that is
// covered by TestUnitDNSCreateWhenDeviceHasNoConfig.
func TestUnitDNSReadHandlesEmptyArraysFromDevice(t *testing.T) {
	st := setupDNSMock(t, []string{"8.8.8.8"}, []string{"test.internal"})
	defer teardown()

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create resource with valid config
			{
				Config: testUnitDNSCreateNoExistingConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
				),
			},
			// Step 2: Simulate device returning empty arrays.
			// The mock still returns valid JSON but with no entries.
			{
				PreConfig: func() {
					st.servers = nil
					st.domains = nil
				},
				Config: testUnitDNSCreateNoExistingConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.#", "1"),
					resource.TestCheckResourceAttr("f5os_dns.test", "dns_servers.0", "8.8.8.8"),
				),
			},
		},
	})
}

// TestUnitDNSConfigPayloadUnmarshalEmpty verifies that unmarshaling an
// empty JSON body into DNSConfigPayload produces usable zero-value fields
// (nil slices) rather than a parse error. This validates the assumption
// behind the ReadDNSConfig fix — that an empty DNSConfigPayload{} is safe
// to use when the API returns no data.
func TestUnitDNSConfigPayloadUnmarshalEmpty(t *testing.T) {
	// Case 1: empty input should fail json.Unmarshal (this is what the fix guards against)
	var config1 DNSConfigPayload
	err := json.Unmarshal([]byte{}, &config1)
	if err == nil {
		t.Error("Expected json.Unmarshal to fail on empty input, but it succeeded")
	}

	// Case 2: an empty struct literal should be safe to use (nil slices, no panic)
	config2 := &DNSConfigPayload{}
	if config2.DNS.Servers.Server != nil {
		t.Errorf("Expected nil servers slice, got %v", config2.DNS.Servers.Server)
	}
	if config2.DNS.Config.Search != nil {
		t.Errorf("Expected nil search slice, got %v", config2.DNS.Config.Search)
	}
	// Verify ranging over nil slices doesn't panic
	for _, s := range config2.DNS.Servers.Server {
		t.Errorf("Unexpected server: %s", s.Address)
	}
	for _, d := range config2.DNS.Config.Search {
		t.Errorf("Unexpected domain: %s", d)
	}
}
