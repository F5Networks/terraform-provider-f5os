package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// ---------------------------------------------------------------------------
// Acceptance test (requires live F5OS device)
// ---------------------------------------------------------------------------

func TestAccDeviceInfoInterfacesAndVlans(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeviceInfoInterfacesAndVlansConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("data.f5os_device_info.test", "interfaces.#", assertLengthGreaterThanZero),
					resource.TestCheckResourceAttrWith("data.f5os_device_info.test", "vlans.#", assertLengthGreaterThanZero),
					resource.TestCheckResourceAttrSet("data.f5os_device_info.test", "id"),
					// Verify interface attributes are populated
					resource.TestCheckResourceAttrSet("data.f5os_device_info.test", "interfaces.0.name"),
					resource.TestCheckResourceAttrSet("data.f5os_device_info.test", "interfaces.0.type"),
					resource.TestCheckResourceAttrSet("data.f5os_device_info.test", "interfaces.0.operational_status"),
				),
			},
		},
	})
}

func TestAccDeviceInfoTenantImages(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeviceInfoTenantImagesConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.f5os_device_info.test", "id"),
				),
			},
		},
	})
}

func TestAccDeviceInfoNegationFilter(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeviceInfoNegationConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.f5os_device_info.test", "id"),
					// With !all, no data should be gathered — lists remain empty
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "partition_images.#", "0"),
				),
			},
		},
	})
}

// ---------------------------------------------------------------------------
// Acceptance test configs
// ---------------------------------------------------------------------------

const testAccDeviceInfoInterfacesAndVlansConfig = `
data "f5os_device_info" "test" {
  gather_info_of = ["interfaces", "vlans"]
}
`

const testAccDeviceInfoTenantImagesConfig = `
data "f5os_device_info" "test" {
  gather_info_of = ["tenant_images"]
}
`

const testAccDeviceInfoNegationConfig = `
data "f5os_device_info" "test" {
  gather_info_of = ["!all"]
}
`

func assertLengthGreaterThanZero(value string) error {
	length, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("failed to parse length: %v", err)
	}
	if length <= 0 {
		return fmt.Errorf("expected length to be greater than 0, got %d", length)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Unit tests — converter functions
// ---------------------------------------------------------------------------

func TestUnitConvertIsoImagesInfo(t *testing.T) {
	t.Run("multiple images", func(t *testing.T) {
		input := f5ossdk.F5IsoImagesInfo{
			Images: []f5ossdk.F5IsoImageInfo{
				{Version: "1.5.0-10234", Service: "1.5.0-10234", Os: "1.5.0-10234"},
				{Version: "1.6.0-20345", Service: "1.6.0-20345", Os: "1.6.0-20345"},
			},
		}
		result := convertIsoImagesInfo(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 images, got %d", len(result))
		}
		if result[0].Version.ValueString() != "1.5.0-10234" {
			t.Errorf("expected version 1.5.0-10234, got %s", result[0].Version.ValueString())
		}
		if result[0].Service.ValueString() != "1.5.0-10234" {
			t.Errorf("expected service 1.5.0-10234, got %s", result[0].Service.ValueString())
		}
		if result[0].Os.ValueString() != "1.5.0-10234" {
			t.Errorf("expected os 1.5.0-10234, got %s", result[0].Os.ValueString())
		}
		if result[1].Version.ValueString() != "1.6.0-20345" {
			t.Errorf("expected version 1.6.0-20345, got %s", result[1].Version.ValueString())
		}
	})

	t.Run("empty images", func(t *testing.T) {
		input := f5ossdk.F5IsoImagesInfo{
			Images: []f5ossdk.F5IsoImageInfo{},
		}
		result := convertIsoImagesInfo(input)
		if result != nil {
			t.Fatalf("expected nil for empty input, got %v", result)
		}
	})

	t.Run("nil images slice", func(t *testing.T) {
		input := f5ossdk.F5IsoImagesInfo{}
		result := convertIsoImagesInfo(input)
		if result != nil {
			t.Fatalf("expected nil for nil input, got %v", result)
		}
	})

	t.Run("single image", func(t *testing.T) {
		input := f5ossdk.F5IsoImagesInfo{
			Images: []f5ossdk.F5IsoImageInfo{
				{Version: "2.0.0-1", Service: "svc-2.0", Os: "f5os-2.0"},
			},
		}
		result := convertIsoImagesInfo(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 image, got %d", len(result))
		}
		if result[0].Os.ValueString() != "f5os-2.0" {
			t.Errorf("expected os f5os-2.0, got %s", result[0].Os.ValueString())
		}
	})
}

func TestUnitConvertTenantImagesInfo(t *testing.T) {
	t.Run("multiple images", func(t *testing.T) {
		input := f5ossdk.F5TenantImagesInfo{
			Images: []f5ossdk.F5TenantImageInfo{
				{
					F5RespTenantImageStatus: f5ossdk.F5RespTenantImageStatus{
						Name:   "BIGIP-17.1.0.qcow2.zip.bundle",
						InUse:  true,
						Status: "verified",
					},
					Type: "vm-image",
					Date: "2024-01-15",
					Size: "2.53 GB",
				},
				{
					F5RespTenantImageStatus: f5ossdk.F5RespTenantImageStatus{
						Name:   "BIGIP-15.1.10.qcow2.zip.bundle",
						InUse:  false,
						Status: "verified",
					},
					Type: "vm-image",
					Date: "2024-02-20",
					Size: "1.93 GB",
				},
			},
		}
		result := convertTenantImagesInfo(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 images, got %d", len(result))
		}
		if result[0].ImageName.ValueString() != "BIGIP-17.1.0.qcow2.zip.bundle" {
			t.Errorf("expected name BIGIP-17.1.0.qcow2.zip.bundle, got %s", result[0].ImageName.ValueString())
		}
		if !result[0].InUse.ValueBool() {
			t.Errorf("expected in_use true for first image")
		}
		if result[0].Type.ValueString() != "vm-image" {
			t.Errorf("expected type vm-image, got %s", result[0].Type.ValueString())
		}
		if result[0].Status.ValueString() != "verified" {
			t.Errorf("expected status verified, got %s", result[0].Status.ValueString())
		}
		if result[0].Date.ValueString() != "2024-01-15" {
			t.Errorf("expected date 2024-01-15, got %s", result[0].Date.ValueString())
		}
		if result[0].Size.ValueString() != "2.53 GB" {
			t.Errorf("expected size 2.53 GB, got %s", result[0].Size.ValueString())
		}
		if result[1].InUse.ValueBool() {
			t.Errorf("expected in_use false for second image")
		}
	})

	t.Run("empty images", func(t *testing.T) {
		input := f5ossdk.F5TenantImagesInfo{}
		result := convertTenantImagesInfo(input)
		if result != nil {
			t.Fatalf("expected nil for empty input, got %v", result)
		}
	})
}

func TestUnitConvertInterfacesInfo(t *testing.T) {
	t.Run("single interface", func(t *testing.T) {
		input := f5ossdk.F5RespOpenconfigInterface{
			OpenconfigInterfacesInterface: []f5ossdk.F5RespInterface{
				{
					Name: "1.0",
					Config: struct {
						Name    string `json:"name,omitempty"`
						Type    string `json:"type,omitempty"`
						Enabled bool   `json:"enabled,omitempty"`
					}{
						Name:    "1.0",
						Type:    "iana-if-type:ethernetCsmacd",
						Enabled: true,
					},
					State: struct {
						Name       string `json:"name,omitempty"`
						Type       string `json:"type,omitempty"`
						Mtu        int    `json:"mtu,omitempty"`
						Enabled    bool   `json:"enabled,omitempty"`
						OperStatus string `json:"oper-status,omitempty"`
						Counters   struct {
							InOctets         string `json:"in-octets,omitempty"`
							InUnicastPkts    string `json:"in-unicast-pkts,omitempty"`
							InBroadcastPkts  string `json:"in-broadcast-pkts,omitempty"`
							InMulticastPkts  string `json:"in-multicast-pkts,omitempty"`
							InDiscards       string `json:"in-discards,omitempty"`
							InErrors         string `json:"in-errors,omitempty"`
							InFcsErrors      string `json:"in-fcs-errors,omitempty"`
							OutOctets        string `json:"out-octets,omitempty"`
							OutUnicastPkts   string `json:"out-unicast-pkts,omitempty"`
							OutBroadcastPkts string `json:"out-broadcast-pkts,omitempty"`
							OutMulticastPkts string `json:"out-multicast-pkts,omitempty"`
							OutDiscards      string `json:"out-discards,omitempty"`
							OutErrors        string `json:"out-errors,omitempty"`
						} `json:"counters,omitempty"`
						F5InterfaceForwardErrorCorrection string `json:"f5-interface:forward-error-correction,omitempty"`
						F5LacpLacpState                   string `json:"f5-lacp:lacp_state,omitempty"`
					}{
						Mtu:        9600,
						OperStatus: "UP",
						Counters: struct {
							InOctets         string `json:"in-octets,omitempty"`
							InUnicastPkts    string `json:"in-unicast-pkts,omitempty"`
							InBroadcastPkts  string `json:"in-broadcast-pkts,omitempty"`
							InMulticastPkts  string `json:"in-multicast-pkts,omitempty"`
							InDiscards       string `json:"in-discards,omitempty"`
							InErrors         string `json:"in-errors,omitempty"`
							InFcsErrors      string `json:"in-fcs-errors,omitempty"`
							OutOctets        string `json:"out-octets,omitempty"`
							OutUnicastPkts   string `json:"out-unicast-pkts,omitempty"`
							OutBroadcastPkts string `json:"out-broadcast-pkts,omitempty"`
							OutMulticastPkts string `json:"out-multicast-pkts,omitempty"`
							OutDiscards      string `json:"out-discards,omitempty"`
							OutErrors        string `json:"out-errors,omitempty"`
						}{
							InOctets:      "11067281",
							InMulticastPkts: "50398",
							InDiscards:    "10075",
						},
					},
					OpenconfigIfEthernetEthernet: struct {
						OpenconfigVlanSwitchedVlan struct {
							Config struct {
								NativeVlan int   `json:"native-vlan,omitempty"`
								TrunkVlans []int `json:"trunk-vlans,omitempty"`
							} `json:"config,omitempty"`
						} `json:"openconfig-vlan:switched-vlan,omitempty"`
						Config struct {
							AutoNegotiate bool   `json:"auto-negotiate,omitempty"`
							DuplexMode    string `json:"duplex-mode,omitempty"`
							PortSpeed     string `json:"port-speed,omitempty"`
						} `json:"config,omitempty"`
					}{
						Config: struct {
							AutoNegotiate bool   `json:"auto-negotiate,omitempty"`
							DuplexMode    string `json:"duplex-mode,omitempty"`
							PortSpeed     string `json:"port-speed,omitempty"`
						}{
							PortSpeed: "SPEED_100GB",
						},
					},
				},
			},
		}
		result := convertInterfacesInfo(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 interface, got %d", len(result))
		}
		if result[0].Name.ValueString() != "1.0" {
			t.Errorf("expected name 1.0, got %s", result[0].Name.ValueString())
		}
		if result[0].Type.ValueString() != "iana-if-type:ethernetCsmacd" {
			t.Errorf("expected type iana-if-type:ethernetCsmacd, got %s", result[0].Type.ValueString())
		}
		if !result[0].Enabled.ValueBool() {
			t.Errorf("expected enabled true")
		}
		if result[0].OperationalStatus.ValueString() != "UP" {
			t.Errorf("expected oper status UP, got %s", result[0].OperationalStatus.ValueString())
		}
		if result[0].Mtu.ValueInt64() != 9600 {
			t.Errorf("expected mtu 9600, got %d", result[0].Mtu.ValueInt64())
		}
		if result[0].PortSpeed.ValueString() != "SPEED_100GB" {
			t.Errorf("expected port speed SPEED_100GB, got %s", result[0].PortSpeed.ValueString())
		}
		// Check some l3 counter values
		elems := result[0].L3Counters.Elements()
		if v, ok := elems["in_octets"]; !ok {
			t.Errorf("expected in_octets in l3_counters")
		} else if v.String() != `"11067281"` {
			t.Errorf("expected in_octets 11067281, got %s", v.String())
		}
	})

	t.Run("empty interfaces", func(t *testing.T) {
		input := f5ossdk.F5RespOpenconfigInterface{}
		result := convertInterfacesInfo(input)
		if result != nil {
			t.Fatalf("expected nil for empty input, got %v", result)
		}
	})
}

func TestUnitConvertVlansInfo(t *testing.T) {
	t.Run("multiple vlans", func(t *testing.T) {
		input := f5ossdk.F5RespVlan{
			OpenconfigVlanVlan: []f5ossdk.F5RespVlanConfig{
				{Config: struct {
					VlanID int    `json:"vlan-id,omitempty"`
					Name   string `json:"name,omitempty"`
				}{VlanID: 100, Name: "vlan-100"}},
				{Config: struct {
					VlanID int    `json:"vlan-id,omitempty"`
					Name   string `json:"name,omitempty"`
				}{VlanID: 200, Name: "vlan-200"}},
			},
		}
		result := convertVlansInfo(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 vlans, got %d", len(result))
		}
		if result[0].VlanId.ValueInt64() != 100 {
			t.Errorf("expected vlan id 100, got %d", result[0].VlanId.ValueInt64())
		}
		if result[0].VlanName.ValueString() != "vlan-100" {
			t.Errorf("expected vlan name vlan-100, got %s", result[0].VlanName.ValueString())
		}
		if result[1].VlanId.ValueInt64() != 200 {
			t.Errorf("expected vlan id 200, got %d", result[1].VlanId.ValueInt64())
		}
	})

	t.Run("empty vlans", func(t *testing.T) {
		input := f5ossdk.F5RespVlan{}
		result := convertVlansInfo(input)
		if result != nil {
			t.Fatalf("expected nil for empty input, got %v", result)
		}
	})
}

// ---------------------------------------------------------------------------
// Unit tests — filterGatherSubsets
// ---------------------------------------------------------------------------

func TestUnitFilterGatherSubsets(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]bool
	}{
		{
			name:  "all expands to five subsets",
			input: []string{"all"},
			expected: map[string]bool{
				"interfaces":       true,
				"vlans":            true,
				"controller_images": true,
				"partition_images": true,
				"tenant_images":    true,
			},
		},
		{
			name:     "bang all returns empty",
			input:    []string{"!all"},
			expected: map[string]bool{},
		},
		{
			name:  "all minus interfaces",
			input: []string{"all", "!interfaces"},
			expected: map[string]bool{
				"vlans":            true,
				"controller_images": true,
				"partition_images": true,
				"tenant_images":    true,
			},
		},
		{
			name:  "all minus controller_images and partition_images",
			input: []string{"all", "!controller_images", "!partition_images"},
			expected: map[string]bool{
				"interfaces":    true,
				"vlans":         true,
				"tenant_images": true,
			},
		},
		{
			name:  "single subset",
			input: []string{"interfaces"},
			expected: map[string]bool{
				"interfaces": true,
			},
		},
		{
			name:  "multiple individual subsets",
			input: []string{"interfaces", "vlans"},
			expected: map[string]bool{
				"interfaces": true,
				"vlans":      true,
			},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: map[string]bool{},
		},
		{
			name:  "negation of non-included item is no-op",
			input: []string{"interfaces", "!vlans"},
			expected: map[string]bool{
				"interfaces": true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterGatherSubsets(tc.input)
			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d items, got %d: %v", len(tc.expected), len(result), result)
			}
			resultSet := make(map[string]bool, len(result))
			for _, item := range result {
				resultSet[item] = true
				if !tc.expected[item] {
					t.Errorf("unexpected item in result: %s", item)
				}
			}
			for key := range tc.expected {
				if !resultSet[key] {
					t.Errorf("missing expected item in result: %s", key)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Unit tests — Read method via mock server
// ---------------------------------------------------------------------------

// setupDeviceInfoMockAuth registers the auth handler on the shared mux so
// the provider can initialise the f5osclient session.
func setupDeviceInfoMockAuth() {
	mux.HandleFunc("/restconf/data/openconfig-system:system/aaa", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Auth-Token", "mock-token")
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	})
}

func TestUnitDeviceInfoReadAllSubsets(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_interfaces.json"))
	})
	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_vlans.json"))
	})
	mux.HandleFunc("/restconf/data/f5-system-image:image/controller/config/iso/iso", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_controller_images.json"))
	})
	mux.HandleFunc("/restconf/data/f5-system-image:image/partition/config/iso/iso", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_partition_images.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_tenant_images.json"))
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "f5os_device_info" "test" { gather_info_of = ["all"] }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Interfaces
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.0.name", "1.0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.0.type", "iana-if-type:ethernetCsmacd"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.0.enabled", "true"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.0.operational_status", "UP"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.0.mtu", "9600"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.0.port_speed", "openconfig-if-ethernet:SPEED_100GB"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.0.l3_counters.in_octets", "11067281"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.0.l3_counters.in_multicast_pkts", "50398"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.1.name", "2.0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.1.enabled", "false"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.1.operational_status", "DOWN"),
					// Vlans
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.0.vlan_id", "100"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.0.vlan_name", "vlan-100"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.1.vlan_id", "200"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.1.vlan_name", "vlan-200"),
					// Controller images
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.0.version", "1.5.0-10234"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.0.service", "1.5.0-10234"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.0.os", "1.5.0-10234"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.1.version", "1.6.0-20345"),
					// Partition images
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "partition_images.#", "1"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "partition_images.0.version", "1.5.0-10234"),
					// Tenant images
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.image_name", "BIGIP-17.1.0.2-0.0.2.ALL-F5OS.qcow2.zip.bundle"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.in_use", "true"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.type", "vm-image"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.status", "verified"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.date", "2024-01-15"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.size", "2.53 GB"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.1.in_use", "false"),
					// ID is set
					resource.TestCheckResourceAttrSet("data.f5os_device_info.test", "id"),
				),
			},
		},
	})
}

func TestUnitDeviceInfoReadInterfacesOnly(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_interfaces.json"))
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "f5os_device_info" "test" { gather_info_of = ["interfaces"] }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "partition_images.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.#", "0"),
				),
			},
		},
	})
}

func TestUnitDeviceInfoReadBangAll(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "f5os_device_info" "test" { gather_info_of = ["!all"] }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "partition_images.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.#", "0"),
					resource.TestCheckResourceAttrSet("data.f5os_device_info.test", "id"),
				),
			},
		},
	})
}

func TestUnitDeviceInfoReadInterfaceError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error": "internal server error"}`)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "f5os_device_info" "test" { gather_info_of = ["interfaces"] }`,
				ExpectError: regexp.MustCompile(`.*Error getting interface info.*`),
			},
		},
	})
}

func TestUnitDeviceInfoReadVlansError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error": "vlan query failed"}`)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "f5os_device_info" "test" { gather_info_of = ["vlans"] }`,
				ExpectError: regexp.MustCompile(`.*Error getting vlans info.*`),
			},
		},
	})
}

func TestUnitDeviceInfoReadControllerImagesError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/f5-system-image:image/controller/config/iso/iso", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error": "controller images query failed"}`)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "f5os_device_info" "test" { gather_info_of = ["controller_images"] }`,
				ExpectError: regexp.MustCompile(`.*Error getting controller images info.*`),
			},
		},
	})
}

func TestUnitDeviceInfoReadPartitionImagesError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/f5-system-image:image/partition/config/iso/iso", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error": "partition images query failed"}`)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "f5os_device_info" "test" { gather_info_of = ["partition_images"] }`,
				ExpectError: regexp.MustCompile(`.*Error getting partition images info.*`),
			},
		},
	})
}

func TestUnitDeviceInfoReadTenantImagesError(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error": "tenant images query failed"}`)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "f5os_device_info" "test" { gather_info_of = ["tenant_images"] }`,
				ExpectError: regexp.MustCompile(`.*Error getting tenant images info.*`),
			},
		},
	})
}

func TestUnitDeviceInfoReadAllMinusSubsets(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	// Only interfaces and tenant_images endpoints are needed
	mux.HandleFunc("/restconf/data/openconfig-interfaces:interfaces/interface", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_interfaces.json"))
	})
	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_tenant_images.json"))
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "f5os_device_info" "test" { gather_info_of = ["all", "!vlans", "!controller_images", "!partition_images"] }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "partition_images.#", "0"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.#", "2"),
				),
			},
		},
	})
}

func TestUnitDeviceInfoReadVlansOnly(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/openconfig-vlan:vlans/vlan", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_vlans.json"))
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "f5os_device_info" "test" { gather_info_of = ["vlans"] }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.0.vlan_id", "100"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "vlans.0.vlan_name", "vlan-100"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "0"),
				),
			},
		},
	})
}

func TestUnitDeviceInfoReadControllerImagesOnly(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/f5-system-image:image/controller/config/iso/iso", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_controller_images.json"))
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "f5os_device_info" "test" { gather_info_of = ["controller_images"] }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.0.version", "1.5.0-10234"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "controller_images.1.version", "1.6.0-20345"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "0"),
				),
			},
		},
	})
}

func TestUnitDeviceInfoReadPartitionImagesOnly(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/f5-system-image:image/partition/config/iso/iso", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_partition_images.json"))
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "f5os_device_info" "test" { gather_info_of = ["partition_images"] }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "partition_images.#", "1"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "partition_images.0.version", "1.5.0-10234"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "0"),
				),
			},
		},
	})
}

func TestUnitDeviceInfoReadTenantImagesOnly(t *testing.T) {
	testAccPreUnitCheck(t)
	defer teardown()

	setupDeviceInfoMockAuth()
	setupMockPlatformVersion(mux, "1.8.0")

	mux.HandleFunc("/restconf/data/f5-tenant-images:images/image", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yang-data+json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, loadFixtureString("./fixtures/device_info_tenant_images.json"))
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "f5os_device_info" "test" { gather_info_of = ["tenant_images"] }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.#", "2"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.image_name", "BIGIP-17.1.0.2-0.0.2.ALL-F5OS.qcow2.zip.bundle"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.in_use", "true"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.0.status", "verified"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "tenant_images.1.in_use", "false"),
					resource.TestCheckResourceAttr("data.f5os_device_info.test", "interfaces.#", "0"),
				),
			},
		},
	})
}
