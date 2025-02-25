/*
Copyright 2023 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
// Package f5os interacts with F5OS systems using the OPEN API.

package f5os

import "time"

type F5ReqImageTenant struct {
	Name string `json:"name"`
}

type F5IsoImagesInfo struct {
	Images []F5IsoImageInfo `json:"f5-system-image:iso"`
}

type F5IsoImageInfo struct {
	Version string `json:"version"`
	Service string `json:"service"`
	Os      string `json:"os"`
}

type F5TenantImagesInfo struct {
	Images []F5TenantImageInfo `json:"f5-tenant-images:image"`
}

type F5TenantImageInfo struct {
	F5RespTenantImageStatus
	Type string `json:"type"`
	Date string `json:"date"`
	Size string `json:"size"`
}

type F5RespTenantImageStatus struct {
	Name   string `json:"name,omitempty"`
	InUse  bool   `json:"in-use"`
	Status string `json:"status,omitempty"`
}
type F5RespTenantImagesStatus struct {
	TenantImages []F5RespTenantImageStatus `json:"f5-tenant-images:image,omitempty"`
}

type F5ReqTenantImage struct {
	Insecure   string `json:"insecure"`
	LocalFile  string `json:"local-file,omitempty"`
	RemoteFile string `json:"remote-file,omitempty"`
	RemoteHost string `json:"remote-host,omitempty"`
}
type F5ReqTenant struct {
	Name           string `json:"name,omitempty"`
	Image          string `json:"image,omitempty"`
	DeploymentFile string `json:"deployment-file,omitempty"`
	Proceed        string `json:"proceed,omitempty"`
	Config         struct {
		Name                    string `json:"name,omitempty"`
		TenantID                int    `json:"tenantID,omitempty"`
		UnitKey                 string `json:"unit-key,omitempty"`
		UnitKeyHash             string `json:"unit-key-hash,omitempty"`
		TenantOp                string `json:"tenant-op,omitempty"`
		Type                    string `json:"type,omitempty"`
		Image                   string `json:"image,omitempty"`
		DeploymentFile          string `json:"deployment-file,omitempty"`
		DeploymentSpecification string `json:"deployment-specification,omitempty"`
		TargetImage             string `json:"target-image,omitempty"`
		TargetDeploymentFile    string `json:"target-deployment-file,omitempty"`
		UpgradeStatus           string `json:"upgrade-status,omitempty"`
		Nodes                   []int  `json:"nodes,omitempty"`
		MgmtIp                  string `json:"mgmt-ip,omitempty"`
		PrefixLength            int    `json:"prefix-length,omitempty"`
		Gateway                 string `json:"gateway,omitempty"`
		MacData                 struct {
			F5TenantL2InlineMacBlockSize string `json:"f5-tenant-l2-inline:mac-block-size,omitempty"`
		} `json:"mac-data,omitempty"`
		DagIpv6PrefixLength int `json:"dag-ipv6-prefix-length,omitempty"`
		MacNdiSet           []struct {
			Ndi string `json:"ndi,omitempty"`
			Mac string `json:"mac,omitempty"`
		} `json:"mac-ndi-set,omitempty"`
		Vlans            []int  `json:"vlans,omitempty"`
		Cryptos          string `json:"cryptos,omitempty"`
		VcpuCoresPerNode int    `json:"vcpu-cores-per-node,omitempty"`
		ReservedCpus     string `json:"reserved-cpus,omitempty"`
		Memory           int    `json:"memory,omitempty"`
		SEPCount         int    `json:"SEP-count,omitempty"`
		Storage          struct {
			Image    string `json:"image,omitempty"`
			Name     string `json:"name,omitempty"`
			Location string `json:"location,omitempty"`
			Address  string `json:"address,omitempty"`
			Size     int    `json:"size,omitempty"`
		} `json:"storage,omitempty"`
		Hugepages []struct {
			Slot int    `json:"slot,omitempty"`
			Path string `json:"pathomitempty"`
		} `json:"hugepages,omitempty"`
		RunningState  string `json:"running-state,omitempty"`
		TrustMode     string `json:"trust-mode,omitempty"`
		ApplianceMode struct {
			Enabled string `json:"enabled,omitempty"`
		} `json:"appliance-mode,omitempty"`
		HaState                   string   `json:"ha-state,omitempty"`
		FloatingAddress           string   `json:"floating-address,omitempty"`
		F5TenantVwireVirtualWires []string `json:"f5-tenant-vwire:virtual-wires,omitempty"`
	} `json:"config,omitempty"`
}

type F5RespTenant struct {
	Name           string `json:"name,omitempty"`
	Image          string `json:"image,omitempty"`
	DeploymentFile string `json:"deployment-file,omitempty"`
	Proceed        string `json:"proceed,omitempty"`
	Config         struct {
		Name                    string `json:"name,omitempty"`
		TenantID                int    `json:"tenantID,omitempty"`
		UnitKey                 string `json:"unit-key,omitempty"`
		UnitKeyHash             string `json:"unit-key-hash,omitempty"`
		TenantOp                string `json:"tenant-op,omitempty"`
		Type                    string `json:"type,omitempty"`
		Image                   string `json:"image,omitempty"`
		DeploymentFile          string `json:"deployment-file,omitempty"`
		DeploymentSpecification string `json:"deployment-specification,omitempty"`
		TargetImage             string `json:"target-image,omitempty"`
		TargetDeploymentFile    string `json:"target-deployment-file,omitempty"`
		UpgradeStatus           string `json:"upgrade-status,omitempty"`
		Nodes                   []int  `json:"nodes,omitempty"`
		MgmtIp                  string `json:"mgmt-ip,omitempty"`
		PrefixLength            int    `json:"prefix-length,omitempty"`
		Gateway                 string `json:"gateway,omitempty"`
		MacNdiSet               []struct {
			Ndi string `json:"ndi,omitempty"`
			Mac string `json:"mac,omitempty"`
		} `json:"mac-ndi-set,omitempty"`
		Vlans            []int  `json:"vlans,omitempty"`
		Cryptos          string `json:"cryptos,omitempty"`
		VcpuCoresPerNode int    `json:"vcpu-cores-per-node,omitempty"`
		ReservedCpus     string `json:"reserved-cpus,omitempty"`
		Memory           int    `json:"memory,omitempty"`
		SEPCount         int    `json:"SEP-count,omitempty"`
		Storage          struct {
			Image    string `json:"image,omitempty"`
			Name     string `json:"name,omitempty"`
			Location string `json:"location,omitempty"`
			Address  string `json:"address,omitempty"`
			Size     int    `json:"size,omitempty"`
		} `json:"storage,omitempty"`
		Hugepages []struct {
			Slot int    `json:"slot,omitempty"`
			Path string `json:"pathomitempty"`
		} `json:"hugepages,omitempty"`
		RunningState  string `json:"running-state,omitempty"`
		TrustMode     string `json:"trust-mode,omitempty"`
		ApplianceMode struct {
			Enabled string `json:"enabled,omitempty"`
		} `json:"appliance-mode,omitempty"`
		HaState                   string   `json:"ha-state,omitempty"`
		FloatingAddress           string   `json:"floating-address,omitempty"`
		F5TenantVwireVirtualWires []string `json:"f5-tenant-vwire:virtual-wires,omitempty"`
	} `json:"config,omitempty"`
	State struct {
		Name             string `json:"name,omitempty"`
		UnitKeyHash      string `json:"unit-key-hash,omitempty"`
		Type             string `json:"type,omitempty"`
		Image            string `json:"image,omitempty"`
		MgmtIp           string `json:"mgmt-ip,omitempty"`
		PrefixLength     int    `json:"prefix-length,omitempty"`
		Gateway          string `json:"gateway,omitempty"`
		Nodes            []int  `json:"nodes"`
		Cryptos          string `json:"cryptos,omitempty"`
		VcpuCoresPerNode int    `json:"vcpu-cores-per-node,omitempty"`
		Memory           string `json:"memory,omitempty"`
		Storage          struct {
			Size int `json:"size,omitempty"`
		} `json:"storage,omitempty"`
		RunningState        string `json:"running-state,omitempty"`
		TrustMode           bool   `json:"trust-mode,omitempty"`
		DagIpv6PrefixLength int    `json:"dag-ipv6-prefix-length,omitempty"`
		MacData             struct {
			BaseMac                  string `json:"base-mac,omitempty"`
			MacPoolSize              int    `json:"mac-pool-size,omitempty"`
			F5TenantL2InlineMacBlock []struct {
				Mac string `json:"mac,omitempty"`
			} `json:"f5-tenant-l2-inline:mac-block,omitempty"`
		} `json:"mac-data,omitempty"`
		ApplianceMode struct {
			Enabled bool `json:"enabled,omitempty"`
		} `json:"appliance-mode,omitempty"`
		CpuAllocations struct {
			CpuAllocation []struct {
				Node int   `json:"node,omitempty"`
				Cpus []int `json:"cpus,omitempty"`
			} `json:"cpu-allocation,omitempty"`
		} `json:"cpu-allocations,omitempty"`
		Status       string `json:"status,omitempty"`
		PrimarySlot  int    `json:"primary-slot,omitempty"`
		ImageVersion string `json:"image-version,omitempty"`
		Instances    struct {
			Instance []struct {
				Node         int       `json:"node,omitempty"`
				PodName      string    `json:"pod-name,omitempty"`
				InstanceId   int       `json:"instance-id,omitempty"`
				Phase        string    `json:"phase,omitempty"`
				CreationTime time.Time `json:"creation-time,omitempty"`
				ReadyTime    time.Time `json:"ready-time,omitempty"`
				Status       string    `json:"status,omitempty"`
				MgmtMac      string    `json:"mgmt-mac,omitempty"`
			} `json:"instance,omitempty"`
		} `json:"instances,omitempty"`
	} `json:"state,omitempty"`
}

type F5ReqTenants struct {
	F5TenantsTenant []F5ReqTenant `json:"f5-tenants:tenant"`
}

type F5RespTenants struct {
	F5TenantsTenant []F5RespTenant `json:"f5-tenants:tenant"`
}

type F5ReqTenantsPatch struct {
	F5TenantsTenants struct {
		Tenant []F5ReqTenant `json:"tenant"`
	} `json:"f5-tenants:tenants"`
}
