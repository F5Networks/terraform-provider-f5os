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
