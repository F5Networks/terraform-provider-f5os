/*
Copyright 2023 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
// Package f5os interacts with F5OS systems using the OPEN API.

package f5os

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
