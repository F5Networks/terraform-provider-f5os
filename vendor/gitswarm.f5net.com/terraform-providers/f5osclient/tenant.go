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

func (p *F5os) GetImage(imageName string) (*F5RespTenantImagesStatus, error) {
	imagenew := fmt.Sprintf("/image=%s", imageName)
	url := fmt.Sprintf("%s%s", uriTenantImage, imagenew)
	f5osLogger.Info("[GetImage]", "Request path", hclog.Fmt("%+v", url))
	imagesStatus := &F5RespTenantImagesStatus{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(byteData, imagesStatus)
	f5osLogger.Debug("[GetImage]", "Image Struct:", hclog.Fmt("%+v", imagesStatus))
	return imagesStatus, nil
}

func (p *F5os) ImportImage(tenantImage *F5ReqTenantImage, timeOut int) ([]byte, error) {
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
			return []byte(""), fmt.Errorf("image Import transfer still in In Progress with Timeout Period, please increase timeout")
		}
		if check {
			time.Sleep(20 * time.Second)
			continue
		} else {
			time.Sleep(20 * time.Second)
			return []byte("Import Image Transfer Success"), nil
		}
	}
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

func (p *F5os) IsImported(imageName string) (*map[string]interface{}, error) {
	url := fmt.Sprintf("%s/image=%s/status", uriTenantImage, imageName)
	f5osLogger.Debug("[isImported]", "Request path", hclog.Fmt("%+v", url))
	var ss map[string]interface{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(byteData, &ss)
	if err != nil {
		return nil, err
	}
	return &ss, nil
}
func (p *F5os) DeleteTenantImage(tenantImage string) error {
	url := fmt.Sprintf("%s%s%s/remove", p.Host, uriRoot, uriTenantImage)
	f5osLogger.Info("[DeleteTenantImage]", "Request path", hclog.Fmt("%+v", url))
	image := &F5ReqImageTenant{}
	image.Name = tenantImage
	var imagesList []*F5ReqImageTenant
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
	return fmt.Errorf("delete Tenant Image failed with:%+v", respMap)
}

func (p *F5os) CreateTenant(tenantObj *F5ReqTenants, timeOut int) ([]byte, error) {
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
			return []byte(""), nil
		}
		t2 := time.Now()
		timeDiff := t2.Sub(t1)
		if timeDiff.Seconds() > float64(timeOut) {
			return []byte(""), fmt.Errorf("tenant deployment still in In Progress with Timeout Period, please increase timeout")
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

func (p *F5os) UpdateTenant(tenantObj *F5ReqTenantsPatch, timeOut int) ([]byte, error) {
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
			return []byte(""), nil
		}
		t2 := time.Now()
		timeDiff := t2.Sub(t1)
		if timeDiff.Seconds() > float64(timeOut) {
			return []byte(""), fmt.Errorf("tenant deployment still in In Progress with Timeout Period, please incraese timeout")
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

func (p *F5os) GetTenant(tenantName string) (*F5RespTenants, error) {
	tenantNameurl := fmt.Sprintf("/tenant=%s", tenantName)
	url := fmt.Sprintf("%s%s", uriTenant, tenantNameurl)
	f5osLogger.Info("[GetTenant]", "Request path", hclog.Fmt("%+v", url))
	tenantStatus := &F5RespTenants{}
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
	f5osLogger.Info("[DeleteTenant]", "Request path", hclog.Fmt("%+v", url))
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
	var result []byte
	result, err := p.GetRequest(uriComponent)
	if err != nil {
		f5osLogger.Error("[GetSoftwareComponentVersions]Get failed", "err", hclog.Fmt("%+v", err))
		return result, err
	}
	f5osLogger.Debug("[GetSoftwareComponentVersions]", "Response", hclog.Fmt("%+v", string(result)))
	return result, err
}
