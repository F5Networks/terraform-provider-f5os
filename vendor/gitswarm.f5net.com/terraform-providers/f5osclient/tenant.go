/*
Copyright 2022 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
// Package f5os interacts with F5OS systems using the OPEN API.
package f5os

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

const (
	uriComponent    = "/openconfig-platform:components"
	uriTenantImage  = "/f5-tenant-images:images"
	uriTenantImport = "/f5-utils-file-transfer:file/import"
	uriFileTransfer = "/f5-utils-file-transfer:file"
	uriTenant       = "/f5-tenants:tenants"
)

type ImageTenant struct {
	Name string `json:"name"`
}
type TenantImages struct {
	ImageTenants []ImageTenant
}
type TenantImageStatus struct {
	Name   string `json:"name,omitempty"`
	InUse  bool   `json:"in-use"`
	Status string `json:"status,omitempty"`
}
type F5TenantImagesStatus struct {
	TenantImages []TenantImageStatus `json:"f5-tenant-images:image,omitempty"`
}

type F5TenantImage struct {
	Insecure   string `json:"insecure"`
	LocalFile  string `json:"local-file,omitempty"`
	RemoteFile string `json:"remote-file,omitempty"`
	RemoteHost string `json:"remote-host,omitempty"`
}

type F5UtilsFileTransferStatus struct {
	LocalFilePath  string `json:"local-file-path,omitempty"`
	RemoteHost     string `json:"remote-host,omitempty"`
	RemoteFilePath string `json:"remote-file-path,omitempty"`
	Operation      string `json:"operation,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	Status         string `json:"status,omitempty"`
}

type F5UtilsFileTransfersStatus struct {
	F5UtilsFileTransfers []F5UtilsFileTransferStatus `json:"f5-utils-file-transfer:transfer-operation,omitempty"`
}

func (p *F5os) GetImage(imageName string) (*F5TenantImagesStatus, error) {
	imagenew := fmt.Sprintf("/image=%s", imageName)
	url := fmt.Sprintf("%s%s", uriTenantImage, imagenew)
	f5osLogger.Info("[GetImage]", "Request path", hclog.Fmt("%+v", url))
	imagesStatus := &F5TenantImagesStatus{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	// f5osLogger.Debug("[GetImage]", "Image Info:", hclog.Fmt("%+v", string(byteData)))
	json.Unmarshal(byteData, imagesStatus)
	f5osLogger.Debug("[GetImage]", "Image Struct:", hclog.Fmt("%+v", imagesStatus))
	return imagesStatus, nil
}

func (p *F5os) ImportImage(tenantImage *F5TenantImage, timeOut int) ([]byte, error) {
	f5osLogger.Debug("[ImportImage]", "Image struct:", hclog.Fmt("%+v", tenantImage))
	byteBody, err := json.Marshal(tenantImage)
	if err != nil {
		return byteBody, err
	}
	respData, err := p.PostRequest(uriTenantImport, byteBody)
	if err != nil {
		return respData, err
	}
	f5osLogger.Info("[ImportImage]", "Import Image Resp: ", hclog.Fmt("%+v", string(respData)))
	if strings.Contains(string(respData), "Aborted: local-file already exists") {
		return []byte(""), fmt.Errorf("%s", string(respData))
	}

	t1 := time.Now()
	for {
		check, err := p.importWait()
		if err != nil {
			return []byte(""), err
		}
		t2 := time.Now()
		timeDiff := t2.Sub(t1)
		if timeDiff.Seconds() > float64(timeOut) {
			return []byte(""), fmt.Errorf("Image Import transfer still in In Progress with Timeout Period,please increse timeout")
		}
		if check {
			time.Sleep(20 * time.Second)
			continue
		} else {
			time.Sleep(20 * time.Second)
			return []byte("Import Image Transfer Success"), nil
		}
	}
	return []byte("Import Image Transfer Success"), nil
}

func (p *F5os) importWait() (bool, error) {
	transferMap, err := p.getImporttransferStatus()
	for _, val := range transferMap["f5-utils-file-transfer:transfer-operation"].([]interface{}) {
		transStatus := val.(map[string]interface{})["status"].(string)
		f5osLogger.Info("[importWait]", "Trans Status: ", hclog.Fmt("%+v", transStatus))
		if err != nil {
			return true, nil
		}
		if strings.Contains(transStatus, "Completed") {
			return false, nil
		}
		if strings.Contains(transStatus, "HTTP Error") {
			return false, fmt.Errorf("%s", transStatus)
		}
		if strings.Contains(transStatus, "Couldn't resolve host") {
			return false, fmt.Errorf("%s", transStatus)
		}
		if strings.Contains(transStatus, "Failure") {
			return false, fmt.Errorf("%s", transStatus)
		}
		for strings.HasPrefix(transStatus, "In Progress") {
			return true, nil
		}
	}
	return true, nil
}

func (p *F5os) getImporttransferStatus() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/transfer-operations/transfer-operation", uriFileTransfer)
	f5osLogger.Info("[getImporttransferStatus]", "Request path", hclog.Fmt("%+v", url))
	// fileTransStatus := &F5UtilsFileTransferStatus{}
	var ss map[string]interface{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	// err := json.Unmarshal(byteData, fileTransStatus)
	err = json.Unmarshal(byteData, &ss)
	if err != nil {
		return nil, err
	}
	return ss, nil
	// return fileTransStatus, nil
}

func (p *F5os) IsImported(imageName string) (*map[string]interface{}, error) {
	url := fmt.Sprintf("%s/image=%s/status", uriTenantImage, imageName)
	f5osLogger.Debug("[isImported]", "Request path", hclog.Fmt("%+v", url))
	// fileTransStatus := &F5UtilsFileTransferStatus{}
	var ss map[string]interface{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	// err := json.Unmarshal(byteData, fileTransStatus)
	err = json.Unmarshal(byteData, &ss)
	if err != nil {
		return nil, err
	}
	return &ss, nil
	// return fileTransStatus, nil
}
func (p *F5os) DeleteTenantImage(tenantImage string) error {
	url := fmt.Sprintf("%s%s%s/remove", p.Host, uriRoot, uriTenantImage)
	f5osLogger.Info("[DeleteTenantImage]", "Request path", hclog.Fmt("%+v", url))
	image := &ImageTenant{}
	image.Name = tenantImage
	imagesList := []*ImageTenant{}
	imagesList = append(imagesList, image)
	byteBody, err := json.Marshal(image)
	if err != nil {
		return err
	}
	var respMap map[string]interface{}
	resp, err := p.doRequest("POST", url, byteBody)
	if err != nil {
		return err
	}
	err = json.Unmarshal(resp, &respMap)
	if err != nil {
		return err
	}
	if respMap["f5-tenant-images:output"].(map[string]interface{})["result"] == "Successful." {
		return nil
	}
	return fmt.Errorf("Delete Tenant Image failed with:%+v", respMap)
}

func (p *F5os) CreatebackupTenant(tenantObj *TenantObj) ([]byte, error) {
	tenants := &Tenants{}
	tenants.Tenant = append(tenants.Tenant, *tenantObj)
	url := fmt.Sprintf("%s", uriTenant)
	f5osLogger.Info("[CreateTenant]", "Request path", hclog.Fmt("%+v", url))
	byteBody, err := json.Marshal(tenants)
	if err != nil {
		return byteBody, err
	}
	respData, err := p.PostRequest(uriTenant, byteBody)
	if err != nil {
		return respData, err
	}
	f5osLogger.Info("[CreateTenant]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	return []byte("CreateTenant is in progress"), nil
}

func (p *F5os) CreateTenant(tenantObj *TenantsObj, timeOut int) ([]byte, error) {
	url := uriTenant
	f5osLogger.Info("[CreateTenant]", "Request path", hclog.Fmt("%+v", url))
	byteBody, err := json.Marshal(tenantObj)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Info("[CreateTenant]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PostRequest(uriTenant, byteBody)
	if err != nil {
		return respData, err
	}
	f5osLogger.Info("[CreateTenant]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	t1 := time.Now()
	for {
		check, err := p.tenantWait(tenantObj.F5TenantsTenant[0].Name)
		if err != nil {
			return []byte(""), err
		}
		t2 := time.Now()
		timeDiff := t2.Sub(t1)
		if timeDiff.Seconds() > float64(timeOut) {
			return []byte(""), fmt.Errorf("Tenant Deployment still in In Progress with Timeout Period,please increse timeout")
		}
		if check {
			time.Sleep(20 * time.Second)
			continue
		} else {
			time.Sleep(20 * time.Second)
			return []byte("Tenant Deployment Success"), nil
		}
	}
	return []byte("Tenant Deployment Success"), nil
}

func (p *F5os) UpdateTenant(tenantObj *TenantsPatchObj, timeOut int) ([]byte, error) {
	url := fmt.Sprintf("%s", uriTenant)
	f5osLogger.Info("[UpdateTenant]", "Request path", hclog.Fmt("%+v", url))
	byteBody, err := json.Marshal(tenantObj)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Info("[UpdateTenant]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PatchRequest(uriTenant, byteBody)
	if err != nil {
		return respData, err
	}
	f5osLogger.Info("[UpdateTenant]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	t1 := time.Now()
	for {
		check, err := p.tenantWait(tenantObj.F5TenantsTenants.Tenant[0].Name)
		if err != nil {
			return []byte(""), err
		}
		t2 := time.Now()
		timeDiff := t2.Sub(t1)
		if timeDiff.Seconds() > float64(timeOut) {
			return []byte(""), fmt.Errorf("Tenant Deployment still in In Progress with Timeout Period,please increse timeout")
		}
		if check {
			time.Sleep(20 * time.Second)
			continue
		} else {
			time.Sleep(20 * time.Second)
			return []byte("Tenant Deployment Success"), nil
		}
	}
	return []byte("Tenant Deployment Success"), nil
}

func (p *F5os) GetTenant(tenantName string) (*TenantsStatusObj, error) {
	tenantNameurl := fmt.Sprintf("/tenant=%s", tenantName)
	url := fmt.Sprintf("%s%s", uriTenant, tenantNameurl)
	f5osLogger.Info("[GetTenant]", "Request path", hclog.Fmt("%+v", url))
	tenantStatus := &TenantsStatusObj{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	f5osLogger.Info("[GetTenant]", "Tenant Info:", hclog.Fmt("%+v", string(byteData)))
	json.Unmarshal(byteData, tenantStatus)
	f5osLogger.Info("[GetTenant]", "Tenant Struct:", hclog.Fmt("%+v", tenantStatus))
	return tenantStatus, nil
}

func (p *F5os) DeleteTenant(tenantName string) error {
	url := fmt.Sprintf("%s%s%s/tenant=%s", p.Host, uriRoot, uriTenant, tenantName)
	f5osLogger.Info("[DeleteTenantImage]", "Request path", hclog.Fmt("%+v", url))
	_, err := p.doRequest("DELETE", url, []byte(""))
	if err != nil {
		return err
	}
	return nil
}
func (p *F5os) tenantWait(tenantName string) (bool, error) {
	tenantMap, err := p.getTenantDeployStatus(tenantName)
	if err != nil {
		return true, err
	}
	if tenantMap["f5-tenants:state"].(map[string]interface{})["status"] == nil {
		return true, nil
	}
	tenantStatus := tenantMap["f5-tenants:state"].(map[string]interface{})["status"].(string)
	if strings.Contains(tenantStatus, "Running") {
		return false, nil
	}
	if strings.Contains(tenantStatus, "Configured") {
		return false, nil
	}
	if strings.Contains(tenantStatus, "Pending") {
		if tenantMap["f5-tenants:state"].(map[string]interface{})["instances"] != nil {
			return false, fmt.Errorf("%v", tenantMap["f5-tenants:state"].(map[string]interface{})["instances"])
		}
	}
	return true, nil
}
func (p *F5os) getTenantDeployStatus(tenantName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/tenant=%s/state", uriTenant, tenantName)
	f5osLogger.Info("[getTenantDeployStatus]", "Request path", hclog.Fmt("%+v", url))
	var ss map[string]interface{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(byteData, &ss)
	if err != nil {
		return nil, err
	}
	return ss, nil
}

func (p *F5os) GetSoftwareComponentVersions() ([]byte, error) {
	// retmap := map[string]string{}
	var result []byte
	result, err := p.GetRequest(uriComponent)
	if err != nil {
		f5osLogger.Error("[GetSoftwareComponentVersions]Get failed", "err", hclog.Fmt("%+v", err))
		return result, err
		// log.Error().Msgf("Get failed, err = %v", err)
	}
	f5osLogger.Debug("[GetSoftwareComponentVersions]", "Response", hclog.Fmt("%+v", string(result)))
	// log.Trace().Msgf("%+v", string(result))
	return result, err
}

type TenantConfig struct {
	Image            string `json:"image"`
	Nodes            []int  `json:"nodes,omitempty"`
	MgmtIp           string `json:"mgmt-ip"`
	Gateway          string `json:"gateway"`
	PrefixLength     int    `json:"prefix-length"`
	Vlans            []int  `json:"vlans,omitempty"`
	VcpuCoresPerNode int    `json:"vcpu-cores-per-node"`
	Memory           int    `json:"memory,omitempty"`
	Cryptos          string `json:"cryptos,omitempty"`
	Storage          struct {
		Size int `json:"size,omitempty"`
	} `json:"storage"`
	RunningState string `json:"running-state,omitempty"`
}

type TenantObj struct {
	Name   string       `json:"name"`
	Config TenantConfig `json:"config"`
}
type Tenants struct {
	Tenant []TenantObj `json:"tenant"`
}

type TenantObjs struct {
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
}

type TenantStatusObjs struct {
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
		Cryptos          string `json:"cryptos,omitempty"`
		VcpuCoresPerNode int    `json:"vcpu-cores-per-node,omitempty"`
		Memory           string `json:"memory,omitempty"`
		Storage          struct {
			Size int `json:"size,omitempty"`
		} `json:"storage,omitempty"`
		RunningState string `json:"running-state,omitempty"`
		TrustMode    bool   `json:"trust-mode,omitempty"`
		MacData      struct {
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

type TenantsObj struct {
	F5TenantsTenant []TenantObjs `json:"f5-tenants:tenant"`
}

type TenantsStatusObj struct {
	F5TenantsTenant []TenantStatusObjs `json:"f5-tenants:tenant"`
}

type TenantsPatchObj struct {
	F5TenantsTenants struct {
		Tenant []TenantObjs `json:"tenant"`
	} `json:"f5-tenants:tenants"`
}
