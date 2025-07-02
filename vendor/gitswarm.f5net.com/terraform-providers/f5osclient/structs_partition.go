/*
Copyright 2023 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
// Package f5os interacts with F5OS systems using the OPEN API.

package f5os

import "github.com/hashicorp/terraform-plugin-framework/types"

type EulaPayload struct {
	RegKey    string   `json:"f5-system-licensing-install:registration-key,omitempty"`
	AddonKeys []string `json:"f5-system-licensing-install:add-on-keys,omitempty"`
}

type LicenseInstallPayload struct {
	RegKey    string   `json:"f5-system-licensing-install:registration-key,omitempty"`
	AddonKeys []string `json:"f5-system-licensing-install:add-on-keys,omitempty"`
	Output    struct {
		Result string `json:"result,omitempty"`
	} `json:"f5-system-licensing-install:output,omitempty"`
}

type License struct {
	Licensing struct {
		State struct {
			RegKey struct {
				Base string `json:"base,omitempty"`
			} `json:"registration-key,omitempty"`
			License    string `json:"license,omitempty"`
			RawLicense string `json:"raw-license,omitempty"`
		}
	} `json:"f5-system-licensing:licensing,omitempty"`
}

type F5ReqPartition struct {
	Name   string               `json:"name,omitempty"`
	Config F5ReqPartitionConfig `json:"config,omitempty"`
}

type F5ReqPartitions struct {
	Partition F5ReqPartition `json:"partition,omitempty"`
}

type F5ReqPartitionConfig struct {
	Enabled             bool   `json:"enabled,omitempty"`
	IsoVersion          string `json:"iso-version,omitempty"`
	ConfigurationVolume int    `json:"configuration-volume,omitempty"`
	ImagesVolume        int    `json:"images-volume,omitempty"`
	SharedVolume        int    `json:"shared-volume,omitempty"`
	MgmtIp              struct {
		Ipv4 struct {
			Address      string `json:"address,omitempty"`
			PrefixLength int    `json:"prefix-length,omitempty"`
			Gateway      string `json:"gateway,omitempty"`
		} `json:"ipv4,omitempty"`
		Ipv6 struct {
			Address      string `json:"address,omitempty"`
			PrefixLength int    `json:"prefix-length,omitempty"`
			Gateway      string `json:"gateway,omitempty"`
		} `json:"ipv6,omitempty"`
	} `json:"mgmt-ip,omitempty"`
}

type F5RespPartitions struct {
	Partition []F5RespPartition `json:"f5-system-partition:partition,omitempty"`
}

type F5RespPartition struct {
	Name   string `json:"name,omitempty"`
	Config struct {
		Enabled             bool   `json:"enabled,omitempty"`
		IsoVersion          string `json:"iso-version,omitempty"`
		ConfigurationVolume int    `json:"configuration-volume,omitempty"`
		ImagesVolume        int    `json:"images-volume,omitempty"`
		SharedVolume        int    `json:"shared-volume,omitempty"`
		MgmtIp              struct {
			Ipv4 struct {
				Address      string `json:"address,omitempty"`
				PrefixLength int    `json:"prefix-length,omitempty"`
				Gateway      string `json:"gateway,omitempty"`
			} `json:"ipv4,omitempty"`
			Ipv6 struct {
				Address      string `json:"address,omitempty"`
				PrefixLength int    `json:"prefix-length,omitempty"`
				Gateway      string `json:"gateway,omitempty"`
			} `json:"ipv6,omitempty"`
		} `json:"mgmt-ip,omitempty"`
	} `json:"config,omitempty"`
	State struct {
		Id                    int    `json:"id,omitempty"`
		OsVersion             string `json:"os-version,omitempty"`
		ServiceVersion        string `json:"service-version,omitempty"`
		InstallOsVersion      string `json:"install-os-version,omitempty"`
		InstallServiceVersion string `json:"install-service-version,omitempty"`
		InstallStatus         string `json:"install-status,omitempty"`
		Controllers           struct {
			Controller []struct {
				Controller            int    `json:"controller,omitempty"`
				PartitionId           int    `json:"partition-id,omitempty"`
				PartitionStatus       string `json:"partition-status,omitempty"`
				RunningServiceVersion string `json:"running-service-version,omitempty"`
				StatusSeconds         string `json:"status-seconds,omitempty"`
				StatusAge             string `json:"status-age,omitempty"`
				Volumes               struct {
					Volume []struct {
						VolumeName    string `json:"volume-name,omitempty"`
						TotalSize     string `json:"total-size,omitempty"`
						AvailableSize string `json:"available-size,omitempty"`
					} `json:"volume,omitempty"`
				} `json:"volumes,omitempty"`
			} `json:"controller,omitempty"`
		} `json:"controllers,omitempty"`
	} `json:"state,omitempty"`
}

type F5ReqPartitionPassChange struct {
	OldPassword     string `json:"f5-system-aaa:old-password,omitempty"`
	NewPassword     string `json:"f5-system-aaa:new-password,omitempty"`
	ConfirmPassword string `json:"f5-system-aaa:confirm-password,omitempty"`
}

type F5ReqVlanConfig struct {
	VlanId string `json:"vlan-id,omitempty"`
	Config struct {
		VlanId int    `json:"vlan-id,omitempty"`
		Name   string `json:"name,omitempty"`
	} `json:"config,omitempty"`
	Members struct {
		Member []struct {
			State struct {
				Interface string `json:"interface,omitempty"`
			} `json:"state,omitempty"`
		} `json:"member,omitempty"`
	} `json:"members,omitempty"`
}

type F5ReqVlansConfig struct {
	OpenconfigVlanVlans struct {
		Vlan []F5ReqVlanConfig `json:"vlan,omitempty"`
	} `json:"openconfig-vlan:vlans,omitempty"`
}

type F5RespVlanConfig struct {
	VlanID int `json:"vlan-id,omitempty"`
	Config struct {
		VlanID int    `json:"vlan-id,omitempty"`
		Name   string `json:"name,omitempty"`
	} `json:"config,omitempty"`
}
type F5RespVlan struct {
	OpenconfigVlanVlan []F5RespVlanConfig `json:"openconfig-vlan:vlan,omitempty"`
}

type F5ReqInterface struct {
	Name   string `json:"name,omitempty"`
	Config struct {
		Name    string `json:"name,omitempty"`
		Type    string `json:"type,omitempty"`
		Enabled bool   `json:"enabled"`
	} `json:"config,omitempty"`
	OpenconfigIfEthernetEthernet struct {
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
		} `json:"config,omitempty,omitempty"`
	} `json:"openconfig-if-ethernet:ethernet,omitempty"`
}

type F5ReqOpenconfigInterface struct {
	OpenconfigInterfacesInterfaces struct {
		Interface []F5ReqInterface `json:"interface,omitempty"`
	} `json:"openconfig-interfaces:interfaces,omitempty"`
}

type F5ReqTokenLifetime struct {
	OpenConfigSystem struct {
		RestConfigToken struct {
			Config struct {
				Lifetime int `json:"lifetime,omitempty"`
			} `json:"config,omitempty"`
		} `json:"f5-aaa-confd-restconf-token:restconf-token,omitempty"`
	} `json:"openconfig-system:aaa,omitempty"`
}

type F5ReqSystemSettingConfig struct {
	Settings struct {
		Config struct {
			SshdIdleTimeout string `json:"sshd-idle-timeout,omitempty"`
			CliTimeout      int    `json:"idle-timeout,omitempty"`
		} `json:"config,omitempty"`
	} `json:"settings,omitempty"`
}
type F5ReqHttpCipherConfig struct {
	Config struct {
		Name           string `json:"name,omitempty"`
		SslCipherSuite string `json:"ssl-ciphersuite,omitempty"`
	} `json:"config,omitempty"`
}
type F5ReqSshdCipherConfig struct {
	Ciphers []string `json:"ciphers,omitempty"`
}

type F5ReqSshdKeyAlgConfig struct {
	KeyExchangeAlgorithms []string `json:"kexalgorithms,omitempty"`
}
type F5ReqSshdMacConfig struct {
	Macs []string `json:"macs,omitempty"`
}
type F5ReqSshdHkeyAlgConfig struct {
	HostKeyAlgorithms []string `json:"host-key-algorithms,omitempty"`
}

type F5ReqSystemConfig struct {
	OpenConfigSystem struct {
		Clock struct {
			Config struct {
				TimezoneName string `json:"timezone-name,omitempty"`
			} `json:"config,omitempty"`
		} `json:"clock,omitempty"`
		Config struct {
			Hostname    string `json:"hostname,omitempty"`
			Motd        string `json:"motd-banner,omitempty"`
			LoginBanner string `json:"login-banner,omitempty"`
		} `json:"config,omitempty"`
	} `json:"openconfig-system:system,omitempty"`
}

type F5ResSystemConfig struct {
	OpenConfigSystem struct {
		Hostname    string `json:"hostname,omitempty"`
		Motd        string `json:"motd-banner,omitempty"`
		LoginBanner string `json:"login-banner,omitempty"`
	} `json:"openconfig-system:config,omitempty"`
}

type F5ResClockConfig struct {
	OpenConfigClock struct {
		Config struct {
			TimeZoneName string `json:"timezone-name,omitempty"`
		} `json:"config,omitempty"`
	} `json:"openconfig-system:clock,omitempty"`
}

type F5ResSettingsConfig struct {
	Settings struct {
		Config struct {
			SshdIdleTimeout any `json:"sshd-idle-timeout,omitempty"`
			CliTimeout      any `json:"idle-timeout,omitempty"`
		} `json:"config,omitempty"`
	} `json:"f5-system-settings:settings,omitempty"`
}

type F5ResTokenLifetime struct {
	Lifetime int `json:"f5-aaa-confd-restconf-token:lifetime,omitempty"`
}

type HttpdConfig struct {
	Name           string `json:"name"`
	SSLCipherSuite string `json:"ssl-ciphersuite"`
}

type HttpdBlock struct {
	Name   string      `json:"name"`
	Config HttpdConfig `json:"config"`
}

type SshdConfig struct {
	Name          string   `json:"name"`
	Ciphers       []string `json:"ciphers"`
	MACs          []string `json:"macs"`
	KexAlgorithms []string `json:"kexalgorithms"`
	HostKeyAlgos  []string `json:"host-key-algorithms"`
}

type SshdBlock struct {
	Name   string     `json:"name"`
	Config SshdConfig `json:"config"`
}

type F5ReqVlanSwitchedVlan struct {
	OpenconfigVlanSwitchedVlan struct {
		Config struct {
			NativeVlan int   `json:"native-vlan,omitempty"`
			TrunkVlans []int `json:"trunk-vlans,omitempty"`
		} `json:"config,omitempty"`
	} `json:"openconfig-vlan:switched-vlan,omitempty"`
}

type F5ReqLagInterfaces struct {
	OpenconfigInterfacesInterfaces struct {
		Interface []F5ReqLagInterface `json:"interface,omitempty"`
	} `json:"openconfig-interfaces:interfaces,omitempty"`
}

type F5ReqLagInterfacesConfig struct {
	OpenconfigInterfacesInterfaces struct {
		OpenConfigLacp struct {
			Interfaces struct {
				Interface []F5ReqLagInterfaceConfig `json:"interface,omitempty"`
			} `json:"interfaces,omitempty"`
		} `json:"openconfig-lacp:lacp,omitempty"`
	} `json:"ietf-restconf:data,omitempty"`
}

type F5ReqLagInterfaceConfig struct {
	Name   string            `json:"name,omitempty"`
	Config LagIntervalConfig `json:"config,omitempty"`
}

type LagIntervalConfig struct {
	Name     string `json:"name,omitempty"`
	Interval string `json:"interval,omitempty"`
	Mode     string `json:"lacp-mode,omitempty"`
}

type LacpInterfaceResponses struct {
	OpenConfigLacpInterface []LacpInterfaceResponse `json:"openconfig-lacp:interface,omitempty"`
}
type LacpInterfaceResponse struct {
	Name   string `json:"name,omitempty"`
	Config struct {
		Name     string `json:"name,omitempty"`
		Interval string `json:"interval,omitempty"`
		Mode     string `json:"lacp-mode,omitempty"`
	} `json:"config,omitempty"`
	State struct {
		Name        string `json:"name,omitempty"`
		Interval    string `json:"interval,omitempty"`
		Mode        string `json:"lacp-mode,,omitempty"`
		SystemIdMac string `json:"system-id-mac,omitempty"`
	}
	Members struct {
		Member []MemberConfig `json:"member,omitempty"`
	} `json:"members,omitempty"`
}

type MemberConfig struct {
	Interface string `json:"interface,omitempty"`
	State     struct {
		Interface       string `json:"interface,omitempty"`
		Activity        string `json:"activity,omitempty"`
		Timeout         string `json:"timeout,omitempty"`
		Synchronization string `json:"synchronization,omitempty"`
		Aggregatable    bool   `json:"aggregatable,omitempty"`
		Collecting      bool   `json:"collecting,omitempty"`
		Distributing    bool   `json:"distributing,omitempty"`
		SystemId        string `json:"system-id,omitempty"`
		OperKey         int    `json:"oper-key,omitempty"`
		PartnerId       string `json:"partner-id,omitempty"`
		PartnerKey      int    `json:"partner-key,omitempty"`
		PortNum         int    `json:"port-num,omitempty"`
		PartnerPortNum  int    `json:"partner-port-num,omitempty"`
		Counters        struct {
			LacpInPkts   int `json:"lacp-in-pkts,omitempty"`
			LacpOutPkts  int `json:"lacp-out-pkts,omitempty"`
			LacpRxErrors int `json:"lacp-rx-errors,omitempty"`
		} `json:"counters,omitempty"`
	} `json:"state,omitempty"`
}

type F5ReqLagInterface struct {
	Name   string `json:"name,omitempty"`
	Config struct {
		Name    string `json:"name,omitempty"`
		Type    string `json:"type,omitempty"`
		Enabled bool   `json:"enabled,omitempty"`
	} `json:"config,omitempty"`
	OpenconfigIfAggregateAggregation struct {
		OpenconfigVlanSwitchedVlan struct {
			Config struct {
				NativeVlan int   `json:"native-vlan,omitempty"`
				TrunkVlans []int `json:"trunk-vlans,omitempty"`
			} `json:"config,omitempty"`
		} `json:"openconfig-vlan:switched-vlan,omitempty"`
		Config struct {
			LagType         string `json:"lag-type,omitempty"`
			DistributioHash string `json:"f5-if-aggregate:distribution-hash,omitempty"`
		} `json:"config,omitempty"`
	} `json:"openconfig-if-aggregate:aggregation,omitempty"`
	OpenconfigIfEthernetEthernet struct {
		Config struct {
			Name string `json:"openconfig-if-aggregate:aggregate-id,omitempty"`
		} `json:"config,omitempty"`
	} `json:"openconfig-if-ethernet:ethernet,omitempty"`
}

type F5RespLagInterfaces struct {
	OpenconfigInterfacesInterface []F5RespLagInterface `json:"openconfig-interfaces:interface,omitempty"`
}

type F5RespLagInterface struct {
	Name   string `json:"name,omitempty"`
	Config struct {
		Name        string `json:"name,omitempty"`
		Type        string `json:"type,omitempty"`
		Description string `json:"description,omitempty"`
		Enabled     bool   `json:"enabled,omitempty"`
	} `json:"config,omitempty"`
	State struct {
		Name       string `json:"name,omitempty"`
		Type       string `json:"type,omitempty"`
		Mtu        int    `json:"mtu,omitempty"`
		Enabled    bool   `json:"enabled,omitempty"`
		OperStatus string `json:"oper-status,omitempty"`
	} `json:"state,omitempty"`
	OpenconfigIfAggregateAggregation struct {
		Config struct {
			LagType         string `json:"lag-type,omitempty"`
			DistributioHash string `json:"f5-if-aggregate:distribution-hash,omitempty"`
		} `json:"config,omitempty"`
		State struct {
			LagType         string `json:"lag-type,omitempty"`
			LagSpeed        int    `json:"lag-speed,omitempty"`
			DistributioHash string `json:"f5-if-aggregate:distribution-hash,omitempty"`
			Members         struct {
				Member []F5RespLagMembers `json:"member,omitempty"`
			} `json:"f5-if-aggregate:members,omitempty"`
			MacAddress string `json:"f5-if-aggregate:mac-address,omitempty"`
			LagId      int    `json:"f5-if-aggregate:lagid,omitempty"`
		} `json:"state,omitempty"`
		OpenconfigVlanSwitchedVlan struct {
			Config struct {
				NativeVlan int   `json:"native-vlan,omitempty"`
				TrunkVlans []int `json:"trunk-vlans,omitempty"`
			} `json:"config,omitempty"`
		} `json:"openconfig-vlan:switched-vlan,omitempty"`
	} `json:"openconfig-if-aggregate:aggregation,omitempty"`
}

type F5RespLagMembers struct {
	Name   string `json:"member-name,omitempty"`
	Status string `json:"member-status,omitempty"`
}

type F5RespOpenconfigInterface struct {
	OpenconfigInterfacesInterface []F5RespInterface `json:"openconfig-interfaces:interface,omitempty"`
}
type F5RespInterface struct {
	Name   string `json:"name,omitempty"`
	Config struct {
		Name    string `json:"name,omitempty"`
		Type    string `json:"type,omitempty"`
		Enabled bool   `json:"enabled,omitempty"`
	} `json:"config,omitempty"`
	State struct {
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
	} `json:"state,omitempty"`
	OpenconfigIfEthernetEthernet struct {
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
	} `json:"openconfig-if-ethernet:ethernet,omitempty"`
}

type TlsCertKey struct {
	Name                   string `json:"f5-openconfig-aaa-tls:name,omitempty"`
	SubjectAlternativeName string `json:"f5-openconfig-aaa-tls:san,omitempty"`
	DaysValid              int64  `json:"f5-openconfig-aaa-tls:days-valid,omitempty"`
	Email                  string `json:"f5-openconfig-aaa-tls:email,omitempty"`
	City                   string `json:"f5-openconfig-aaa-tls:city,omitempty"`
	Province               string `json:"f5-openconfig-aaa-tls:region,omitempty"`
	Country                string `json:"f5-openconfig-aaa-tls:country,omitempty"`
	Organization           string `json:"f5-openconfig-aaa-tls:organization,omitempty"`
	Unit                   string `json:"f5-openconfig-aaa-tls:unit,omitempty"`
	Version                int64  `json:"f5-openconfig-aaa-tls:version,omitempty"`
	KeyType                string `json:"f5-openconfig-aaa-tls:key-type,omitempty"`
	KeySize                int64  `json:"f5-openconfig-aaa-tls:key-size,omitempty"`
	KeyCurve               string `json:"f5-openconfig-aaa-tls:curve-name,omitempty"`
	KeyPassphrase          string `json:"f5-openconfig-aaa-tls:key-passphrase,omitempty"`
	ConfirmKeyPassphrase   string `json:"f5-openconfig-aaa-tls:confirm-key-passphrase,omitempty"`
	StoreTls               bool   `json:"f5-openconfig-aaa-tls:store-tls,omitempty"`
}

type F5ReqDNS struct {
	DNS struct {
		Servers struct {
			Server []struct {
				Address string `json:"address"`
			} `json:"server"`
		} `json:"servers"`
		Config struct {
			Search []string `json:"search"`
		} `json:"config"`
	} `json:"openconfig-system:dns"`
}

// DNSConfigPayload is the struct for PATCHing the full DNS config.
type DNSConfigPayload struct {
	DNS DNSConfig `json:"openconfig-system:dns"`
}

type DNSConfig struct {
	Servers DNSConfigServers `json:"servers"`
	Config  DNSConfigSearch  `json:"config"`
}

type DNSConfigServers struct {
	Server []DNSServer `json:"server"`
}

type DNSConfigSearch struct {
	Search []string `json:"search"`
}

// DNSConfigRequest represents the structure of a DNS configuration request.
type DNSConfigRequest struct {
	Config  DNSConfig     `json:"config"`
	Servers DNSServerList `json:"servers"`
}

type DNSServer struct {
	Address string `json:"address"`
}

type DNSServerList struct {
	Server []DNSServer `json:"server"`
}

const DNSConfigURI = "/openconfig-system:system/dns"

type F5ReqPrimaryKey struct {
	PrimaryKey PrimaryKeyConfig `json:"f5-primary-key:set"`
}

type PrimaryKeyConfig struct {
	Passphrase        string `json:"f5-primary-key:passphrase"`
	ConfirmPassphrase string `json:"f5-primary-key:confirm-passphrase"`
	Salt              string `json:"f5-primary-key:salt"`
	ConfirmSalt       string `json:"f5-primary-key:confirm-salt"`
}

type F5RespPrimaryKey struct {
	PrimaryKey struct {
		State struct {
			Hash   string `json:"f5-primary-key:hash"`
			Status string `json:"f5-primary-key:status"`
		} `json:"f5-primary-key:state"`
	} `json:"f5-primary-key:primary-key"`
}

type PrimaryKeyState struct {
	State PrimaryKeyStatus `json:"state"`
}

type PrimaryKeyStatus struct {
	Status string `json:"status"`
}

// NTPServerModel is the Terraform resource schema representation
// using Terraform Plugin Framework types.
// type NTPServerModel struct {
// 	Id                string `tfsdk:"id"`
// 	Server            string `tfsdk:"server"`
// 	KeyID             int    `tfsdk:"key_id"`
// 	Prefer            bool   `tfsdk:"prefer"`
// 	IBurst            bool   `tfsdk:"iburst"`
// 	NTPService        bool   `tfsdk:"ntp_service"`
// 	NTPAuthentication bool   `tfsdk:"ntp_authentication"`
// }

type NTPServerModel struct {
	ID                types.String `tfsdk:"id"` // ✅ Corrected
	Server            types.String `tfsdk:"server"`
	KeyID             types.Int64  `tfsdk:"key_id"`
	Prefer            types.Bool   `tfsdk:"prefer"`
	IBurst            types.Bool   `tfsdk:"iburst"`
	NTPService        types.Bool   `tfsdk:"ntp_service"`
	NTPAuthentication types.Bool   `tfsdk:"ntp_authentication"`
}

// NTPServerStruct is the internal Go representation
// used for HTTP payloads and response parsing.
type NTPServerStruct struct {
	Address           string `json:"address"`
	KeyID             int64  `json:"key_id,omitempty"`
	Prefer            bool   `json:"prefer,omitempty"`
	IBurst            bool   `json:"iburst,omitempty"`
	NTPService        bool   `json:"ntp_service,omitempty"`
	NTPAuthentication bool   `json:"ntp_authentication,omitempty"`
}
