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
	uriRoot           = "/restconf/data"
	uriLogin          = "/restconf/data/openconfig-system:system/aaa"
	contentTypeHeader = "application/yang-data+json"
	uriPlatformType   = "/openconfig-platform:components/component=platform/state/description"
	uriInterface      = "/openconfig-interfaces:interfaces"
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

type OpenconfigInterfacesInterface struct {
	Name   string `json:"name,omitempty"`
	Config struct {
		Name    string `json:"name,omitempty"`
		Type    string `json:"type,omitempty"`
		Enabled bool   `json:"enabled"`
	} `json:"config,omitempty"`
	OpenconfigIfEthernetEthernet struct {
		// Config struct {
		// 	PortSpeed string `json:"port-speed,omitempty"`
		// } `json:"config,omitempty"`
		// State struct {
		// 	PortSpeed    string `json:"port-speed,omitempty"`
		// 	HwMacAddress string `json:"hw-mac-address,omitempty"`
		// } `json:"state,omitempty"`
		OpenconfigVlanSwitchedVlan struct {
			Config struct {
				NativeVlan int   `json:"native-vlan,omitempty"`
				TrunkVlans []int `json:"trunk-vlans,omitempty"`
			} `json:"config,omitempty"`
		} `json:"openconfig-vlan:switched-vlan,omitempty"`
	} `json:"openconfig-if-ethernet:ethernet,omitempty"`
}

type F5osInterface struct {
	OpenconfigInterfacesInterfaces struct {
		Interface []OpenconfigInterfacesInterface `json:"interface,omitempty"`
	} `json:"openconfig-interfaces:interfaces,omitempty"`
}

type F5osVlanSwitchedVlan struct {
	OpenconfigVlanSwitchedVlan struct {
		Config struct {
			NativeVlan int   `json:"native-vlan,omitempty"`
			TrunkVlans []int `json:"trunk-vlans,omitempty"`
		} `json:"config,omitempty"`
	} `json:"openconfig-vlan:switched-vlan,omitempty"`
}

type F5osInterfaces struct {
	OpenconfigInterfacesInterfaces []OpenconfigInterfacesInterface `json:"openconfig-interfaces:interface,omitempty"`
}

// APIRequest builds our request before sending it to the server.
type APIRequest struct {
	Method      string
	URL         string
	Body        string
	ContentType string
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
	// fmt.Println("Welcome to init() function")
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

	if f5osObj.Port != 0 && port == "" {
		urlString = fmt.Sprintf("%s:%d", urlString, f5osObj.Port)
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
	urlString = fmt.Sprintf("%s%s", urlString, uriLogin)

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
	f5osSession.setPlaformType()
	f5osLogger.Info("[NewSession] Session creation Success")
	return f5osSession, nil
}

// // APICall is used to query the BIG-IP web API.
// func (p *F5os) APICall(options *APIRequest) ([]byte, error) {
// 	var req *http.Request
// 	client := &http.Client{
// 		Transport: p.Transport,
// 		Timeout:   p.ConfigOptions.APICallTimeout,
// 	}
// 	url := fmt.Sprintf("%s%s", p.Host, options.URL)
// 	body := bytes.NewReader([]byte(options.Body))
// 	req, _ = http.NewRequest(strings.ToUpper(options.Method), url, body)
// 	req.Header.Set("X-Auth-Token", p.Token)
// 	if len(options.ContentType) > 0 {
// 		req.Header.Set("Content-Type", options.ContentType)
// 	}
// 	res, err := client.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer res.Body.Close()
// 	return io.ReadAll(res.Body)
// }

func (p *F5os) doRequest(op, path string, body []byte) ([]byte, error) {
	f5osLogger.Debug("[doRequest]", "Request path", hclog.Fmt("%+v", path))
	// log.Trace().Msgf("path = %s", path)
	if len(body) > 0 {
		f5osLogger.Debug("[doRequest]", "Request body", hclog.Fmt("%+v", string(body)))
		// log.Trace().Msgf("body = %s", string(body))
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
	// req.Header.Add("Content-Type", "application/yang-data+json")
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
	// if resp.StatusCode == 400 {
	// 	return io.ReadAll(resp.Body)
	// 	// var f5osError F5osError
	// 	// bodyResp, err := io.ReadAll(resp.Body)
	// 	// if err != nil {
	// 	// 	return bodyResp, err
	// 	// }
	// 	// json.Unmarshal(bodyResp, &f5osError)
	// 	// if f5osError.IetfRestconfErrors.Error[0].ErrorMessage == "" {
	// 	// 	return
	// 	// }
	// }
	if resp.StatusCode >= 400 {
		byteData, _ := io.ReadAll(resp.Body)
		var errorNew F5osError
		json.Unmarshal(byteData, &errorNew)
		return nil, errorNew.Error()
	}
	return nil, nil
}

func (p *F5os) GetRequest(path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, uriRoot, path)
	f5osLogger.Info("[GetRequest]", "Request path", hclog.Fmt("%+v", url))
	return p.doRequest("GET", url, nil)
}

func (p *F5os) DeleteRequest(path string) error {
	url := fmt.Sprintf("%s%s%s", p.Host, uriRoot, path)
	f5osLogger.Debug("[DeleteRequest]", "Request path", hclog.Fmt("%+v", url))
	if resp, err := p.doRequest("DELETE", url, nil); err != nil {
		return err
	} else if len(resp) > 0 {
		f5osLogger.Trace("[DeleteRequest]", "Response", hclog.Fmt("%+v", string(resp)))
	}
	return nil
}

func (p *F5os) PatchRequest(path string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, uriRoot, path)
	f5osLogger.Debug("[PatchRequest]", "Request path", hclog.Fmt("%+v", url))
	return p.doRequest("PATCH", url, body)
}

func (p *F5os) PostRequest(path string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, uriRoot, path)
	f5osLogger.Debug("[PostRequest]", "Request path", hclog.Fmt("%+v", url))
	return p.doRequest("POST", url, body)
}

func (p *F5os) GetInterface(intf string) (*F5osInterfaces, error) {
	intfnew := fmt.Sprintf("/interface=%s", intf)
	url := fmt.Sprintf("%s%s", uriInterface, intfnew)
	f5osLogger.Info("[GetInterface]", "Request path", hclog.Fmt("%+v", url))
	intFace := &F5osInterfaces{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(byteData, intFace)
	f5osLogger.Debug("[GetInterface]", "intFace", hclog.Fmt("%+v", intFace))
	return intFace, nil
}

func (p *F5os) UpdateInterface(intf string, body *F5osInterface) ([]byte, error) {
	f5osLogger.Debug("[UpdateInterface]", "Request path", hclog.Fmt("%+v", uriInterface))
	vlans, err := p.getSwitchedVlans(intf)
	nativeVlan := vlans.OpenconfigVlanSwitchedVlan.Config.NativeVlan
	trunkVlans := vlans.OpenconfigVlanSwitchedVlan.Config.TrunkVlans
	for _, val := range body.OpenconfigInterfacesInterfaces.Interface {
		innativeVlan := val.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.NativeVlan
		_ = val.OpenconfigIfEthernetEthernet.OpenconfigVlanSwitchedVlan.Config.TrunkVlans
		if nativeVlan != 0 && innativeVlan != nativeVlan {
			p.RemoveNativeVlans(intf)
		}
		for _, intfVal := range trunkVlans {
			p.RemoveTrunkVlans(intf, intfVal)
		}
	}
	f5osLogger.Trace("[UpdateInterface]", "nativeVlan", hclog.Fmt("%+v", nativeVlan))
	f5osLogger.Trace("[UpdateInterface]", "trunkVlans", hclog.Fmt("%+v", trunkVlans))
	byteBody, err := json.Marshal(body)
	if err != nil {
		return byteBody, err
	}
	resp, err := p.PatchRequest(uriInterface, byteBody)
	if err != nil {
		return resp, err
	}
	f5osLogger.Debug("[UpdateInterface]", "Resp:", hclog.Fmt("%+v", string(resp)))
	return resp, nil
}
func (p *F5os) getSwitchedVlans(intf string) (*F5osVlanSwitchedVlan, error) {
	intfnew := fmt.Sprintf("/interface=%s/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan", intf)
	url := fmt.Sprintf("%s%s", uriInterface, intfnew)
	f5osLogger.Debug("[getSwitchedVlans]", "Request path", hclog.Fmt("%+v", url))
	intFace := &F5osVlanSwitchedVlan{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	// f5osLogger.Debug("[getSwitchedVlans]", "SwitchedVlans", hclog.Fmt("%+v", string(byteData)))
	json.Unmarshal(byteData, intFace)
	f5osLogger.Debug("[getSwitchedVlans]", "intFace", hclog.Fmt("%+v", intFace))
	return intFace, nil
}

func (p *F5os) RemoveNativeVlans(intf string) error {
	intfnew := fmt.Sprintf("/interface=%s/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:native-vlan", intf)
	url := fmt.Sprintf("%s%s", uriInterface, intfnew)
	f5osLogger.Debug("[removeNativeVlans]", "Request path", hclog.Fmt("%+v", url))
	// intFace := &F5osVlanSwitchedVlan{}
	err := p.DeleteRequest(url)
	if err != nil {
		return err
	}
	return nil
}

func (p *F5os) RemoveTrunkVlans(intf string, vlanId int) error {
	intfnew := fmt.Sprintf("/interface=%s/openconfig-if-ethernet:ethernet/openconfig-vlan:switched-vlan/openconfig-vlan:config/openconfig-vlan:trunk-vlans=%d", intf, vlanId)
	url := fmt.Sprintf("%s%s", uriInterface, intfnew)
	f5osLogger.Debug("[removeNativeVlans]", "Request path", hclog.Fmt("%+v", url))
	// intFace := &F5osVlanSwitchedVlan{}
	err := p.DeleteRequest(url)
	if err != nil {
		return err
	}
	return nil
}

func (p *F5os) setPlaformType() ([]byte, error) {
	url := fmt.Sprintf("%s%s%s", p.Host, uriRoot, uriPlatformType)
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
		url1 := fmt.Sprintf("%s%s%s", p.Host, uriRoot, uriVlan)
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
