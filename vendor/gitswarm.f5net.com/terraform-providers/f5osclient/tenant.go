/*
Copyright 2022 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
// Package f5os interacts with F5OS systems using the OPEN API.
package f5os

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

const (
	uriComponent    = "/openconfig-platform:components"
	uriTenantImage  = "/f5-tenant-images:images"
	uriStartUpload  = "/f5-utils-file-transfer:file/f5-file-upload-meta-data:upload/start-upload"
	uriImageUpload  = "/openconfig-system:system/f5-image-upload:image/upload-image"
	uriTenantImport = "/f5-utils-file-transfer:file/import"
	uriFileTransfer = "/f5-utils-file-transfer:file"
	uriTenant       = "/f5-tenants:tenants"
)

func (p *F5os) GetImageVerify(imageName string) (*F5RespTenantImagesStatus, error) {
	imgFldr := strings.Join(strings.Split(imageName, ".")[0:7], ".")
	imgYaml := fmt.Sprintf("%s.yaml", imgFldr)
	imgList := []string{imageName, imgFldr, imgYaml}
	timOut := 360
	t1 := time.Now()
	for _, img := range imgList {
		// add timeout for each image
		for {
			t2 := time.Now()
			timeDiff := t2.Sub(t1)
			if timeDiff.Seconds() > float64(timOut) {
				return nil, fmt.Errorf("image verification still in In Progress with Timeout Period, please increase timeout")
			}
			imagenew := fmt.Sprintf("/image=%s", img)
			url := fmt.Sprintf("%s%s", uriTenantImage, imagenew)
			f5osLogger.Info("[GetImageVerify]", "Request path", hclog.Fmt("%+v", url))
			imagesStatus := &F5RespTenantImagesStatus{}
			byteData, err := p.GetTenantRequest(url)
			f5osLogger.Debug("[GetImageVerify]", "Image Resp:", hclog.Fmt("%+v", string(byteData)))
			if err != nil {
				if strings.Contains(err.Error(), "uri keypath not found") {
					continue
				}
			}
			for _, val := range imagesStatus.TenantImages {
				if val.Name == img && (val.Status == "replicated" || val.Status == "processed" || val.Status == "verified") {
					continue
				}
				// return imagesStatus, nil
			}
		}
	}
	return nil, nil
}

func (p *F5os) GetImage(imageName string) (*F5RespTenantImagesStatus, error) {
	imagenew := fmt.Sprintf("/image=%s", imageName)
	url := fmt.Sprintf("%s%s", uriTenantImage, imagenew)
	f5osLogger.Info("[GetImage]", "Request path", hclog.Fmt("%+v", url))
	imagesStatus := &F5RespTenantImagesStatus{}
	byteData, err := p.GetTenantRequest(url)
	if err != nil {
		if strings.Contains(err.Error(), "uri keypath not found") {
			errorNew := struct {
				Status  string          `json:"status"`
				Message string          `json:"message"`
				Details json.RawMessage `json:"details"`
			}{
				Status:  "404 Not Found",
				Message: fmt.Sprintf("Tenant Image (%s) not found", imageName),
				Details: json.RawMessage(err.Error()),
			}
			jsonData, _ := json.Marshal(errorNew)
			return nil, fmt.Errorf("%+v", string(jsonData))
			// return nil, fmt.Errorf("Tenant Image (%s) not found", imageName)
		}
		return nil, err
	}
	f5osLogger.Debug("[GetImage]", "Image Resp:", hclog.Fmt("%+v", string(byteData)))
	json.Unmarshal(byteData, imagesStatus)
	f5osLogger.Debug("[GetImage]", "Image Struct:", hclog.Fmt("%+v", imagesStatus))
	return imagesStatus, nil
}

func (p *F5os) UploadImage(filePath string) ([]byte, error) {
	fileObj, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := fileObj.Stat()
	if err != nil {
		return nil, err
	}

	uploadId, err := p.getUploadId(fileObj)
	f5osLogger.Debug("[Upload Image]", "Upload ID:", hclog.Fmt(uploadId))
	if err != nil {
		return nil, err
	}
	if uploadId == "" {
		return nil, fmt.Errorf("failed to get the upload ID")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	formData, err := writer.CreateFormFile("image", fileInfo.Name())
	if err != nil {
		return nil, err
	}

	io.Copy(formData, fileObj)
	writer.Close()

	headers := map[string]string{
		"File-Upload-Id": uploadId,
		"Content-Type":   writer.FormDataContentType(),
	}

	resp, err := p.UploadImagePostRequest(uriImageUpload, body, headers)
	if err != nil {
		return nil, err
	}
	time.Sleep(time.Second * 10)
	return resp, nil
}

func (p *F5os) getUploadId(fileObj *os.File) (string, error) {
	fileStat, err := fileObj.Stat()
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(
		map[string]any{
			"size":      fileStat.Size(),
			"name":      fileStat.Name(),
			"file-path": "images/",
		},
	)
	if err != nil {
		return "", err
	}

	ret := make(map[string]map[string]string)
	resp, err := p.PostRequest(uriStartUpload, payload)
	if err != nil {
		return "", err
	}

	json.NewDecoder(bytes.NewReader(resp)).Decode(&ret)
	return ret["f5-file-upload-meta-data:output"]["upload-id"], nil
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
		check, err := p.importWait(tenantImage)
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

func (p *F5os) importWait(tenantImage *F5ReqTenantImage) (bool, error) {
	transferMap, err := p.getImporttransferStatus()
	for _, val := range transferMap["f5-utils-file-transfer:transfer-operation"].([]interface{}) {
		if val.(map[string]interface{})["remote-file-path"].(string) != tenantImage.RemoteFile {
			continue
		}
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
	ss := make(map[string]interface{})
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
	url := fmt.Sprintf("%s%s%s/remove", p.Host, p.UriRoot, uriTenantImage)
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

// https://{{velos_chassis1_system_controller_ip}}:443/api

func (p *F5os) GetApi() ([]byte, error) {
	url := fmt.Sprintf("%s%s", p.Host, "/api")
	url = strings.Replace(url, "8888", "443", -1)
	byteData, err := p.doTenantRequest("GET", url, []byte(""))
	if err != nil {
		return byteData, err
	}
	f5osLogger.Debug("[GetApi]", "Api Resp", hclog.Fmt("%+v", string(byteData)))
	return byteData, nil
}

func (p *F5os) F5OsKeepAlive(delay time.Duration) chan bool {
	stop := make(chan bool)
	go func() {
		for {
			select {
			case <-time.After(delay):
				_, _ = p.GetApi()
				// f5osLogger.Info("[schedule]", "RequestGetApi ", hclog.Fmt("%+v", string(resp)))
			case <-stop:
				return
			}
		}
	}()
	return stop
}

func (p *F5os) CreateTenantAndGetApi(tenantObj *F5ReqTenants, timeOut int) ([]byte, error) {
	// create a channel to run the CreateTenant function
	chan1 := make(chan []byte)
	err1 := make(chan error)
	// create a channel to run the GetApi function
	chan2 := make(chan []byte)
	err2 := make(chan error)

	go func() {
		resp, err := p.CreateTenant(tenantObj, timeOut)
		chan1 <- resp
		err1 <- err
	}()
	go func() {
		for {
			resp, err := p.GetApi()
			f5osLogger.Info("[CreateTenantAndGetApi]", "RequestGetApi ", hclog.Fmt("%+v", string(resp)))
			time.Sleep(15 * time.Second)
			chan2 <- resp
			err2 <- err
		}
	}()
	if <-err1 != nil {
		return <-chan1, <-err1
	}
	if <-err2 == nil {
		f5osLogger.Info("[CreateTenantAndGetApi]", "RequestGetApi-Out", hclog.Fmt("%+v", <-chan2))
	}
	return <-chan1, <-err1
}

func (p *F5os) CreateTenant(tenantObj *F5ReqTenants, timeOut int) ([]byte, error) {
	// url := uriTenant
	f5osLogger.Info("[CreateTenant]", "Request path", hclog.Fmt("%+v", uriTenant))
	byteBody, err := json.Marshal(tenantObj)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Info("[CreateTenant]", "Body", hclog.Fmt("%+v", string(byteBody)))
	// stop := p.schedule(15 * time.Second)
	respData, err := p.PostTenantRequest(uriTenant, byteBody)
	if err != nil {
		// stop <- true
		return respData, err
	}
	f5osLogger.Info("[CreateTenant]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	t1 := time.Now()
	for {
		check, err := p.tenantWait(tenantObj.F5TenantsTenant[0].Name, tenantObj.F5TenantsTenant[0].Config.RunningState)
		if err != nil {
			if err.Error() == "tenant status not found" {
				time.Sleep(30 * time.Second)
				t1 = time.Now()
				continue
			}
			// stop <- true
			return []byte(""), err
		}
		t2 := time.Now()
		timeDiff := t2.Sub(t1)
		f5osLogger.Info("[CreateTenant]", "timeDiff: ", hclog.Fmt("%+v", timeDiff))
		if timeDiff.Seconds() > float64(timeOut) {
			tenantMap, _ := p.getTenantDeployStatus(tenantObj.F5TenantsTenant[0].Name)
			tenantResp, _ := json.Marshal(tenantMap)
			tenantStatus := tenantMap["f5-tenants:state"].(map[string]interface{})["status"].(string)
			errorNew := struct {
				Status  string          `json:"status"`
				Message string          `json:"message"`
				Details json.RawMessage `json:"details"`
			}{
				Status:  "200 status OK",
				Message: fmt.Sprintf("tenant deployment status is still in (%+v) within in %d seconds timeout period", tenantStatus, timeOut),
				Details: json.RawMessage(string(tenantResp)),
			}
			jsonData, _ := json.Marshal(errorNew)
			// stop <- true
			return []byte(""), fmt.Errorf("%+v", string(jsonData))
			//return []byte(""), fmt.Errorf("[TF-100]tenant deployment still in In Progress with in Timeout Period, please increase timeout")
		}
		if check {
			time.Sleep(80 * time.Second)
			continue
		} else {
			time.Sleep(20 * time.Second)
			// stop <- true
			return []byte("Tenant Deployment Success"), nil
		}
	}
	// return []byte("Tenant Deployment Success"), nil
}

func (p *F5os) UpdateTenant(tenantObj *F5ReqTenantsPatch, timeOut int) ([]byte, error) {
	// url := fmt.Sprintf("%s", uriTenant)
	f5osLogger.Info("[UpdateTenant]", "Request path", hclog.Fmt("%+v", uriTenant))
	byteBody, err := json.Marshal(tenantObj)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Info("[UpdateTenant]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PutRequest(uriTenant, byteBody)
	if err != nil {
		return respData, err
	}
	f5osLogger.Info("[UpdateTenant]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	t1 := time.Now()
	for {
		check, err := p.tenantWait(tenantObj.F5TenantsTenants.Tenant[0].Name, tenantObj.F5TenantsTenants.Tenant[0].Config.RunningState)
		if err != nil {
			if err.Error() == "tenant status not found" {
				time.Sleep(30 * time.Second)
				t1 = time.Now()
				continue
			}
			return []byte(""), err
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
	// return []byte("Tenant Deployment Success"), nil
}

func (p *F5os) GetTenant(tenantName string) (*F5RespTenants, error) {
	tenantNameurl := fmt.Sprintf("/tenant=%s", tenantName)
	url := fmt.Sprintf("%s%s", uriTenant, tenantNameurl)
	f5osLogger.Info("[GetTenant]", "Request path", hclog.Fmt("%+v", url))
	tenantStatus := &F5RespTenants{}
	byteData, err := p.GetTenantRequest(url)
	if err != nil {
		errorNew := struct {
			Status  string          `json:"status"`
			Message string          `json:"message"`
			Details json.RawMessage `json:"details"`
		}{
			Status:  "404 Not Found",
			Message: fmt.Sprintf("Tenant (%s) not found", tenantName),
			Details: json.RawMessage(err.Error()),
		}
		jsonData, _ := json.Marshal(errorNew)
		return nil, fmt.Errorf("%+v", string(jsonData))
		// return nil, err
	}
	f5osLogger.Info("[GetTenant]", "Tenant Info:", hclog.Fmt("%+v", string(byteData)))
	json.Unmarshal(byteData, tenantStatus)
	if len(tenantStatus.F5TenantsTenant) == 0 {
		errorNew := struct {
			Status  string          `json:"status"`
			Message string          `json:"message"`
			Details json.RawMessage `json:"details"`
		}{
			Status:  "404 Not Found",
			Message: fmt.Sprintf("Tenant (%s) not found", tenantName),
			Details: json.RawMessage(string(byteData)),
		}
		jsonData, _ := json.Marshal(errorNew)
		return nil, fmt.Errorf("%+v", string(jsonData))
		// return nil, fmt.Errorf("GetTenant failed with :%+v", string(byteData))
	}
	// f5osLogger.Info("[GetTenant]", "Instances Length:", hclog.Fmt("%+v", len(tenantStatus.F5TenantsTenant[0].State.Instances.Instance)))
	return tenantStatus, nil
}

func (p *F5os) CheckTenantnotexist(tenantName string) bool {
	tenantNameurl := fmt.Sprintf("/tenant=%s", tenantName)
	url := fmt.Sprintf("%s%s", uriTenant, tenantNameurl)
	f5osLogger.Info("[CheckTenantnotexist]", "Request path", hclog.Fmt("%+v", url))
	byteData, err := p.GetRequest(url)
	if err != nil {
		return false
	}
	var tenantStatus map[string]interface{}
	// {
	// 	"ietf-restconf:errors": {
	// 		"error": [
	// 			{
	// 				"error-type": "application",
	// 				"error-tag": "invalid-value",
	// 				"error-message": "uri keypath not found"
	// 			}
	// 		]
	// 	}
	// }
	json.Unmarshal(byteData, &tenantStatus)
	// check error-message
	f5osLogger.Info("[CheckTenantnotexist]", "Tenant", hclog.Fmt("%+v uri result :%+v", tenantName, tenantStatus["ietf-restconf:errors"].(map[string]interface{})["error"].([]interface{})[0].(map[string]interface{})["error-message"].(string)))
	return tenantStatus["ietf-restconf:errors"].(map[string]interface{})["error"].([]interface{})[0].(map[string]interface{})["error-message"].(string) == "uri keypath not found"
}

func (p *F5os) DeleteTenant(tenantName string) error {
	url := fmt.Sprintf("%s%s%s/tenant=%s", p.Host, p.UriRoot, uriTenant, tenantName)
	f5osLogger.Info("[DeleteTenant]", "Request path", hclog.Fmt("%+v", url))
	_, err := p.doTenantRequest("DELETE", url, []byte(""))
	if err != nil {
		return err
	}
	f5osLogger.Debug("[DeleteTenant]", "wait for 50 sec", hclog.Fmt("%d", 10))
	time.Sleep(50 * time.Second)
	p.CheckTenantnotexist(tenantName)
	return nil
}
func (p *F5os) tenantWait(tenantName, runningState string) (bool, error) {
	tenantMap, err := p.getTenantDeployStatus(tenantName)
	if err != nil {
		return true, err
	}
	if tenantMap["f5-tenants:state"].(map[string]interface{})["status"] == nil {
		return true, fmt.Errorf("tenant status not found")
	}
	tenantStatus := tenantMap["f5-tenants:state"].(map[string]interface{})["status"].(string)
	f5osLogger.Info("[tenantWait]", "tenantName:", hclog.Fmt("%+v", tenantName))
	f5osLogger.Info("[tenantWait]", "f5-tenants:state", hclog.Fmt("%+v", tenantStatus))
	if strings.Contains(tenantStatus, "Running") && runningState == "deployed" {
		return false, nil
	}
	if strings.Contains(tenantStatus, "Configured") && runningState == "configured" {
		return false, nil
	}
	if strings.Contains(tenantStatus, "Starting") {
		return true, nil
	}
	if strings.Contains(tenantStatus, "Pending") {
		// map[instance:[map[creation-time: instance-id:2 node:2 phase:Insufficient slots to deploy tenant pod-name:test-tenant22-2 ready-time: status:Tenant deployment will be processed when the slot available in partition]]]
		jsonDataold, _ := json.Marshal(tenantMap["f5-tenants:state"].(map[string]interface{})["instances"].(map[string]interface{})["instance"].([]interface{})[0])
		if tenantMap["f5-tenants:state"].(map[string]interface{})["instances"] != nil {
			errorNew := struct {
				Status  string          `json:"status"`
				Message string          `json:"message"`
				Details json.RawMessage `json:"details"`
			}{
				Status:  "Tenant Deployment Pending",
				Message: tenantMap["f5-tenants:state"].(map[string]interface{})["instances"].(map[string]interface{})["instance"].([]interface{})[0].(map[string]interface{})["phase"].(string),
				Details: json.RawMessage(string(jsonDataold)),
			}
			jsonData, _ := json.Marshal(errorNew)
			return false, fmt.Errorf("%v", string(jsonData))
		}
	}
	return true, nil
}
func (p *F5os) getTenantDeployStatus(tenantName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/tenant=%s/state", uriTenant, tenantName)
	f5osLogger.Info("[getTenantDeployStatus]", "Request path", hclog.Fmt("%+v", url))
	var ss map[string]interface{}
	byteData, err := p.GetTenantRequest(url)
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
