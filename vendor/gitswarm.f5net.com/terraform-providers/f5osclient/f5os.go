/*
Copyright 2022 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
// Package f5os interacts with F5OS systems using the OPEN API.
package f5os

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	uriRoot               = "/restconf/data"
	uriLogin              = "/openconfig-system:system/aaa"
	contentTypeHeader     = "application/yang-data+json"
	uriPlatformType       = "/openconfig-platform:components/component=platform/state/description"
	uriInterface          = "/openconfig-interfaces:interfaces"
	uriConfigBackup       = "/openconfig-system:system/f5-database:database/f5-database:config-backup"
	uriFileExport         = "/f5-utils-file-transfer:file/export"
	uriFileDelete         = "/f5-utils-file-transfer:file/delete"
	uriFileList           = "/f5-utils-file-transfer:file/list"
	uriFileTransferStatus = "/f5-utils-file-transfer:file/transfer-operations/transfer-operation"
)

var f5osLogger hclog.Logger

var defaultConfigOptions = &ConfigOptions{
	APICallTimeout: 60 * time.Second,
}

type ConfigOptions struct {
	APICallTimeout time.Duration
}

type F5osConfig struct {
	Host      string
	User      string
	Password  string
	Port      int
	Transport *http.Transport
	// UserAgent is an optional field that specifies the caller of this request.
	UserAgent     string
	Teem          bool
	ConfigOptions *ConfigOptions
}

// F5os is a container for our session state.
type F5os struct {
	Host      string
	Token     string // if set, will be used instead of User/Password
	Transport *http.Transport
	// UserAgent is an optional field that specifies the caller of this request.
	UserAgent     string
	Teem          bool
	ConfigOptions *ConfigOptions
	PlatformType  string
	UriRoot       string
}
type F5osError struct {
	IetfRestconfErrors struct {
		Error []struct {
			ErrorType    string `json:"error-type"`
			ErrorTag     string `json:"error-tag"`
			ErrorPath    string `json:"error-path"`
			ErrorMessage string `json:"error-message"`
		} `json:"error"`
	} `json:"ietf-restconf:errors"`
}

// Upload contains information about a file upload status
type Upload struct {
	RemainingByteCount int64          `json:"remainingByteCount"`
	UsedChunks         map[string]int `json:"usedChunks"`
	TotalByteCount     int64          `json:"totalByteCount"`
	LocalFilePath      string         `json:"localFilePath"`
	TemporaryFilePath  string         `json:"temporaryFilePath"`
	Generation         int            `json:"generation"`
	LastUpdateMicros   int            `json:"lastUpdateMicros"`
}

type FileExport struct {
	RemoteHost string `json:"remote-host"`
	RemotePath string `json:"remote-file"`
	LocalFile  string `json:"local-file"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Protocol   string `json:"protocol"`
	Insecure   string `json:"insecure"`
}

// RequestError contains information about any error we get from a request.
type RequestError struct {
	Code       int      `json:"code,omitempty"`
	Message    string   `json:"message,omitempty"`
	ErrorStack []string `json:"errorStack,omitempty"`
}

// Error returns the error message.
func (r *F5osError) Error() error {
	if len(r.IetfRestconfErrors.Error) > 0 {
		return errors.New(r.IetfRestconfErrors.Error[0].ErrorMessage)
	}
	return nil
}

func init() {
	val, ok := os.LookupEnv("TF_LOG")
	if !ok {
		val, ok = os.LookupEnv("TF_LOG_PROVIDER_F5OS")
		if !ok {
			val = "INFO"
		}
	}
	f5osLogger = hclog.New(&hclog.LoggerOptions{
		Name:  "[F5OS]",
		Level: hclog.LevelFromString(val),
	})
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}

// NewSession sets up connection to the F5os system.
func NewSession(f5osObj *F5osConfig) (*F5os, error) {
	f5osLogger.Info("[NewSession] Session creation Starts...")
	var urlString string
	f5osSession := &F5os{}
	if !strings.HasPrefix(f5osObj.Host, "http") {
		urlString = fmt.Sprintf("https://%s", f5osObj.Host)
	} else {
		urlString = f5osObj.Host
	}
	f5osLogger.Info("[NewSession]", "URL", hclog.Fmt("%+v", urlString))
	u, _ := url.Parse(urlString)
	_, port, _ := net.SplitHostPort(u.Host)
	f5osSession.UriRoot = uriRoot
	if port == "443" {
		f5osSession.UriRoot = "/api/data"
	}
	if f5osObj.Port != 0 && port == "" {
		urlString = fmt.Sprintf("%s:%d", urlString, f5osObj.Port)
		if f5osObj.Port == 443 {
			f5osSession.UriRoot = "/api/data"
		}
	}
	if f5osObj.ConfigOptions == nil {
		f5osObj.ConfigOptions = defaultConfigOptions
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	f5osSession.Host = urlString
	f5osSession.Transport = tr
	f5osSession.ConfigOptions = f5osObj.ConfigOptions
	client := &http.Client{
		Transport: tr,
	}
	method := "GET"
	urlString = fmt.Sprintf("%s%s%s", urlString, f5osSession.UriRoot, uriLogin)

	f5osLogger.Debug("[NewSession]", "URL", hclog.Fmt("%+v", urlString))
	req, err := http.NewRequest(method, urlString, nil)
	req.Header.Set("Content-Type", contentTypeHeader)
	req.SetBasicAuth(f5osObj.User, f5osObj.Password)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	f5osSession.Token = res.Header.Get("X-Auth-Token")
	respData, err := io.ReadAll(res.Body)
	if res.StatusCode == 401 {
		return nil, fmt.Errorf("%+v with error:%+v", res.Status, string(respData))
	}
	if err != nil {
		return nil, err
	}
	if strings.Contains(fmt.Sprintf("%s", string(respData)), "enable JavaScript to run this app") {
		return nil, fmt.Errorf("Failed with %s", string(respData))
	}
	f5osSession.setPlaformType()
	f5osLogger.Info("[NewSession] Session creation Success")
	return f5osSession, nil
}

func (p *F5os) doRequest(op, path string, body []byte) ([]byte, error) {
	f5osLogger.Debug("[doRequest]", "Request path", hclog.Fmt("%+v", path))
	if len(body) > 0 {
		f5osLogger.Debug("[doRequest]", "Request body", hclog.Fmt("%+v", string(body)))
	}
	req, err := http.NewRequest(op, path, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", p.Token)
	req.Header.Set("Content-Type", contentTypeHeader)
	client := &http.Client{
		Transport: p.Transport,
		Timeout:   p.ConfigOptions.APICallTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	f5osLogger.Debug("[doRequest]", "Resp CODE", hclog.Fmt("%+v", resp.StatusCode))
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return io.ReadAll(resp.Body)
	}
	if resp.StatusCode == 404 {
		// byteData, err := io.ReadAll(resp.Body)
		// if err != nil {
		// 	return nil, err
		// }
		// f5osLogger.Debug("[doRequest]", "Resp CODE", hclog.Fmt("%+v", string(byteData)))
		return io.ReadAll(resp.Body)
	}
	if resp.StatusCode >= 400 {
		byteData, _ := io.ReadAll(resp.Body)
		var errorNew F5osError
		json.Unmarshal(byteData, &errorNew)
		return nil, errorNew.Error()
	}
	return nil, nil
}

func (p *F5os) GetRequest(path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, p.UriRoot, path)
	f5osLogger.Info("[GetRequest]", "Request path", hclog.Fmt("%+v", url))
	return p.doRequest("GET", url, nil)
}

func (p *F5os) DeleteRequest(path string) error {
	url := fmt.Sprintf("%s%s%s", p.Host, p.UriRoot, path)
	f5osLogger.Debug("[DeleteRequest]", "Request path", hclog.Fmt("%+v", url))
	if resp, err := p.doRequest("DELETE", url, nil); err != nil {
		return err
	} else if len(resp) > 0 {
		f5osLogger.Trace("[DeleteRequest]", "Response", hclog.Fmt("%+v", string(resp)))
	}
	return nil
}

func (p *F5os) PutRequest(path string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, p.UriRoot, path)
	f5osLogger.Debug("[PutRequest]", "Request path", hclog.Fmt("%+v", url))
	return p.doRequest("PUT", url, body)
}

func (p *F5os) PatchRequest(path string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, p.UriRoot, path)
	f5osLogger.Debug("[PatchRequest]", "Request path", hclog.Fmt("%+v", url))
	return p.doRequest("PATCH", url, body)
}

func (p *F5os) PostRequest(path string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, p.UriRoot, path)
	f5osLogger.Debug("[PostRequest]", "Request path", hclog.Fmt("%+v", url))
	return p.doRequest("POST", url, body)
}

func (p *F5os) GetInterface(intf string) (*F5RespOpenconfigInterface, error) {
	intfnew := fmt.Sprintf("/interface=%s", intf)
	url := fmt.Sprintf("%s%s", uriInterface, intfnew)
	f5osLogger.Info("[GetInterface]", "Request path", hclog.Fmt("%+v", url))
	intFace := &F5RespOpenconfigInterface{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(byteData, intFace)
	f5osLogger.Debug("[GetInterface]", "intFace", hclog.Fmt("%+v", intFace))
	return intFace, nil
}

func (p *F5os) UpdateInterface(intf string, body *F5ReqOpenconfigInterface) ([]byte, error) {
	f5osLogger.Debug("[UpdateInterface]", "Request path", hclog.Fmt("%+v", uriInterface))
	vlans, err := p.getSwitchedVlans(intf)
	if err != nil {
		return []byte(""), err
	}
	nativeVlan := vlans.OpenconfigVlanSwitchedVlan.Config.NativeVlan
	trunkVlans := vlans.OpenconfigVlanSwitchedVlan.Config.TrunkVlans
	for _, val := range body.OpenconfigInterfacesInterfaces.Interface {
		innativeVlan := val.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.NativeVlan
		newTrunkvlans := val.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.TrunkVlans
		diffTrunkvlans := listDifference(trunkVlans, newTrunkvlans)
		if nativeVlan != 0 && innativeVlan != nativeVlan {
			p.RemoveNativeVlans(intf)
		}
		for _, intfVal := range diffTrunkvlans {
			p.RemoveTrunkVlans(intf, intfVal)
		}
	}
	byteBody, err := json.Marshal(body)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[UpdateInterface]", "Request Body", hclog.Fmt("%+v", body))
	resp, err := p.PatchRequest(uriInterface, byteBody)
	if err != nil {
		return resp, err
	}
	f5osLogger.Debug("[UpdateInterface]", "Resp:", hclog.Fmt("%+v", string(resp)))
	return resp, nil
}
func (p *F5os) getSwitchedVlans(intf string) (*F5ReqVlanSwitchedVlan, error) {
	intfnew := fmt.Sprintf("/interface=%s/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", intf)
	url := fmt.Sprintf("%s%s", uriInterface, intfnew)
	f5osLogger.Debug("[getSwitchedVlans]", "Request path", hclog.Fmt("%+v", url))
	intFace := &F5ReqVlanSwitchedVlan{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(byteData, intFace)
	f5osLogger.Debug("[getSwitchedVlans]", "intFace", hclog.Fmt("%+v", intFace))
	return intFace, nil
}

func (p *F5os) RemoveNativeVlans(intf string) error {
	intfnew := fmt.Sprintf("/interface=%s/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:native-vlan", intf)
	url := fmt.Sprintf("%s%s", uriInterface, intfnew)
	f5osLogger.Debug("[RemoveNativeVlans]", "Request path", hclog.Fmt("%+v", url))
	err := p.DeleteRequest(url)
	if err != nil {
		return err
	}
	return nil
}

func (p *F5os) RemoveTrunkVlans(intf string, vlanId int) error {
	intfnew := fmt.Sprintf("/interface=%s/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=%d", intf, vlanId)
	url := fmt.Sprintf("%s%s", uriInterface, intfnew)
	f5osLogger.Debug("[RemoveTrunkVlans]", "Request path", hclog.Fmt("%+v", url))
	err := p.DeleteRequest(url)
	if err != nil {
		return err
	}
	return nil
}

func (p *F5os) UploadImagePostRequest(path string, formData io.Reader, headers map[string]string) ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, p.UriRoot, path)
	req, err := http.NewRequest(
		http.MethodPost,
		url,
		formData,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("File-Upload-Id", headers["File-Upload-Id"])
	req.Header.Set("Content-Type", headers["Content-Type"])
	req.Header.Set("X-Auth-Token", p.Token)

	client := &http.Client{
		Transport: p.Transport,
		Timeout:   p.ConfigOptions.APICallTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(resp.Body)
}

func (p *F5os) CreateConfigBackup(backupName string, timeout int64, exportCfg FileExport) ([]byte, error) {
	f5osLogger.Debug("[CreateConfigBackup]", "Request path", hclog.Fmt("%+v", uriConfigBackup))

	payload := map[string]string{"f5-database:name": backupName}
	byteBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := p.PostRequest(uriConfigBackup, byteBody)
	if err != nil {
		return nil, err
	}

	obj := make(map[string]any)
	err = json.NewDecoder(bytes.NewReader(resp)).Decode(&obj)

	if err != nil {
		return nil, err
	}

	backupResult := obj["f5-database:output"].(map[string]any)["result"].(string)
	if !strings.HasPrefix(backupResult, "Database backup successful.") {
		return nil, fmt.Errorf("failed to create database config backup")
	} else {
		f5osLogger.Debug("[CreateConfigBackup]", "successfull created backup file: ", hclog.Fmt("%+v", backupName))
	}

	resp, err = p.ExportConfigBackup(exportCfg)

	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(bytes.NewReader(resp)).Decode(&obj)
	if err != nil {
		return nil, fmt.Errorf("unable to decode response from file export endpoint")
	}
	f5osLogger.Debug("[CreateConfigBackup]", "file transfer response: ", hclog.Fmt("%s", string(resp)))

	result := obj["f5-utils-file-transfer:output"].(map[string]any)["result"].(string)
	if !strings.HasPrefix(result, "File transfer is initiated") {
		return nil, fmt.Errorf("unable to initiate backup file transfer")
	}

	var transferId string
	key := "operation-id"
	transferId, ok := obj["f5-utils-file-transfer:output"].(map[string]any)["operation-id"].(string)

	if !ok {
		transferId = fmt.Sprintf("configs/%s", backupName)
		key = "local-file-path"
	}

	f5osLogger.Debug("[CreateConfigBackup]", "transferId and key are ", hclog.Fmt("%+v, %+v", transferId, key))
	waitTime := time.Second * time.Duration(timeout)
	for start := time.Now(); time.Since(start).Seconds() < waitTime.Seconds(); time.Sleep(5 * time.Second) {
		status, err := p.fileTransferStatus(key, transferId)
		if err != nil {
			return nil, err
		}

		if status == "Completed" {
			f5osLogger.Debug("[CreateConfigBackup]", "successfully exported backup file to host", hclog.Fmt("%+v", exportCfg.RemoteHost))
			return nil, nil
		}
	}

	return nil, fmt.Errorf("export operation timed out")
}

func (p *F5os) DeleteConfigBackup(backup string) error {
	f5osLogger.Debug("[DeleteConfigBackup]", "Request path", hclog.Fmt("%+v", uriFileDelete))
	payload, err := json.Marshal(map[string]string{
		"f5-utils-file-transfer:file-name": backup,
	})

	if err != nil {
		return err
	}

	resp, err := p.PostRequest(uriFileDelete, payload)

	if err != nil {
		return err
	}

	obj := make(map[string]any)
	json.NewDecoder(bytes.NewReader(resp)).Decode(&obj)
	msg := obj["f5-utils-file-transfer:output"].(map[string]any)["result"].(string)

	if msg != "Deleting the file" {
		return fmt.Errorf("unable to delete the config backup file")
	} else {
		f5osLogger.Info("[DeleteConfigBackup]", "successfully deleted config backup file", hclog.Fmt("%+v", backup))
	}
	return nil
}

func (p *F5os) GetConfigBackup() ([]byte, error) {
	f5osLogger.Debug("[ReadConfigBackup]", "Request path", hclog.Fmt("%+v", uriFileList))
	payload, err := json.Marshal(map[string]string{
		"f5-utils-file-transfer:path": "configs/",
	})

	if err != nil {
		return nil, err
	}

	resp, err := p.PostRequest(uriFileList, payload)

	if err != nil {
		return nil, err
	}

	f5osLogger.Debug("[ReadConfigBackup]", fmt.Sprintf("Response from %s: ", uriFileList), hclog.Fmt("%+v", resp))

	return resp, nil
}

func (p *F5os) ExportConfigBackup(exportCfg FileExport) ([]byte, error) {
	f5osLogger.Debug("[ExportConfigBackup]", "Request path", hclog.Fmt("%+v", uriFileExport))
	payload, err := json.Marshal(exportCfg)

	if err != nil {
		return nil, err
	}

	return p.PostRequest(uriFileExport, payload)
}

func (p *F5os) fileTransferStatus(key, transferId string) (string, error) {
	f5osLogger.Debug("[fileTransferStatus]", "Request path", hclog.Fmt("%+v", uriFileTransferStatus))
	resp, err := p.GetRequest(uriFileTransferStatus)
	if err != nil {
		return "", err
	}

	obj := make(map[string]any)

	err = json.NewDecoder(bytes.NewReader(resp)).Decode(&obj)
	if err != nil {
		return "", fmt.Errorf("unable to read file transfer status")
	}

	transfers := obj["f5-utils-file-transfer:transfer-operation"].([]any)
	for _, v := range transfers {
		m := v.(map[string]any)
		opID, ok := m[key].(string)
		if ok && opID == transferId {
			return strings.Trim(m["status"].(string), " "), nil
		}
	}

	return "", fmt.Errorf("no transfer status available for the file/operation-id: %s", transferId)
}

func (p *F5os) setPlaformType() ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, p.UriRoot, uriPlatformType)
	f5osLogger.Debug("[setPlaformType]", "Request path", hclog.Fmt("%+v", url))
	req, err := http.NewRequest("GET", url, bytes.NewBuffer(nil))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", p.Token)
	req.Header.Set("Content-Type", contentTypeHeader)
	client := &http.Client{
		Transport: p.Transport,
		Timeout:   p.ConfigOptions.APICallTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		p.PlatformType = "rSeries Platform"
		return io.ReadAll(resp.Body)
	}
	if resp.StatusCode == 404 {
		url1 := fmt.Sprintf("%s%s%s", p.Host, p.UriRoot, uriVlan)
		req, err := http.NewRequest("GET", url1, bytes.NewBuffer(nil))
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Auth-Token", p.Token)
		req.Header.Set("Content-Type", contentTypeHeader)
		client := &http.Client{
			Transport: p.Transport,
			Timeout:   p.ConfigOptions.APICallTimeout,
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 || resp.StatusCode == 204 {
			p.PlatformType = "Velos Partition"
		}
		if resp.StatusCode == 404 {
			bytes, _ := io.ReadAll(resp.Body)
			var mymap map[string]interface{}
			json.Unmarshal(bytes, &mymap)
			intfVal := mymap["ietf-restconf:errors"].(map[string]interface{})["error"].([]interface{})[0].(map[string]interface{})["error-message"]
			if intfVal == "uri keypath not found" {
				p.PlatformType = "Velos Controller"
			}
		}
	}
	return nil, nil
}

// contains checks if a int is present in
// a slice
func contains(s []int, str int) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func listDifference(s1 []int, s2 []int) []int {
	difference := make([]int, 0)
	map1 := make(map[int]bool)
	map2 := make(map[int]bool)
	for _, val := range s1 {
		map1[val] = true
	}
	for _, val := range s2 {
		map2[val] = true
	}
	for key := range map1 {
		if _, ok := map2[key]; !ok {
			difference = append(difference, key) //if element not present in map2 append elements in difference slice
		}
	}
	return difference
}
