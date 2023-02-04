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
	// f5osLogger.Debug("[ImportImage]", "Import Image Resp: ", hclog.Fmt("%+v", string(respData)))
	t1 := time.Now()
	for {
		check, err := p.importWait()
		if err != nil {
			return []byte(""), nil
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
		f5osLogger.Debug("[importWait]", "Trans Status: ", hclog.Fmt("%+v", transStatus))
		if err != nil {
			return true, nil
		}
		if strings.Contains(transStatus, "Completed") {
			return false, nil
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
