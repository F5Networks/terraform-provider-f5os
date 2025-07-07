/*
Copyright 2023 F5 Networks Inc.
This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
*/
// Package f5os interacts with F5OS systems using the OPEN API.

package f5os

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
)

const (
	uriSlot          = "/f5-system-slot:slots/slot"
	uriSlots         = "/f5-system-slot:slots"
	uriPartition     = "/f5-system-partition:partitions"
	uriNodes         = "/f5-cluster:cluster/nodes/node"
	uriVlan          = "/openconfig-vlan:vlans"
	uriAuth          = "/openconfig-system:system/aaa"
	uriCreateCertKey = "/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls/f5-openconfig-aaa-tls:create-self-signed-cert"
	uriSystemDNS     = "/openconfig-system:system/dns"
	uriBase          = "/openconfig-system:system"
)

func (p *F5os) CreatePartition(partitionObj *F5ReqPartitions) ([]byte, error) {
	url := fmt.Sprintf("%s", uriPartition)
	f5osLogger.Debug("[CreatePartition]", "Request path", hclog.Fmt("%+v", url))
	byteBody, err := json.Marshal(partitionObj)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[CreatePartition]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PostRequest(url, byteBody)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[CreatePartition]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	return byteBody, nil
}

func (p *F5os) UpdatePartition(partitionName string, partitionObj *F5ReqPartition) ([]byte, error) {
	url := fmt.Sprintf("%s/partition=%s/config", uriPartition, partitionName)
	f5osLogger.Debug("[UpdatePartition]", "Request path", hclog.Fmt("%+v", url))
	byteBody, err := json.Marshal(partitionObj)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[UpdatePartition]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PatchRequest(url, byteBody)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[UpdatePartition]", "Resp: ", hclog.Fmt("%+v", string(respData)))

	return byteBody, nil
}

func (p *F5os) DeletePartition(partitionName string) error {
	url := fmt.Sprintf("%s/partition=%s", uriPartition, partitionName)
	f5osLogger.Debug("[DeletePartition]", "Request path", hclog.Fmt("%+v", url))
	err := p.DeleteRequest(url)
	if err != nil {
		return err
	}
	return nil
}

func (p *F5os) GetPartition(partitionName string) (*F5RespPartitions, error) {
	url := fmt.Sprintf("%s/partition=%s", uriPartition, partitionName)
	f5osLogger.Debug("[GetPartition]", "Request path", hclog.Fmt("%+v", url))
	partitionStatus := &F5RespPartitions{}
	byteData, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}
	f5osLogger.Debug("[GetPartition]", "Partition Info:", hclog.Fmt("%+v", string(byteData)))
	err = json.Unmarshal(byteData, partitionStatus)
	if err != nil {
		return nil, err
	}
	if len(partitionStatus.Partition) == 0 {
		return nil, fmt.Errorf("%s", string(byteData))
	}
	f5osLogger.Debug("[GetPartition]", "Partition Struct:", hclog.Fmt("%+v", partitionStatus))
	return partitionStatus, nil
}

func (p *F5os) GetPartitionSlots(partitionName string) ([]int64, error) {
	f5osLogger.Debug("[GetPartitionSlots]", "Request path", hclog.Fmt("%+v", uriSlot))
	var ss map[string]interface{}
	byteData, err := p.GetRequest(uriSlot)
	if err != nil {
		return nil, err
	}
	f5osLogger.Debug("[GetPartitionSlots]", "Resp", hclog.Fmt("%+v", string(byteData)))
	err = json.Unmarshal(byteData, &ss)
	if err != nil {
		return nil, err
	}
	var partitionSlots []int64
	allSlots := ss["f5-system-slot:slot"].([]interface{})
	for _, slot := range allSlots {
		if slot.(map[string]interface{})["partition"].(string) == partitionName {
			partitionSlots = append(partitionSlots, int64(slot.(map[string]interface{})["slot-num"].(float64)))
		}
	}
	if len(partitionSlots) == 0 {
		return nil, nil
	}
	return partitionSlots, nil
}

func (p *F5os) GetPartitionNode() (*int64, error) {
	f5osLogger.Debug("[GetPartitionNodes]", "Request path", hclog.Fmt("%+v", uriNodes))
	var ss map[string]interface{}
	byteData, err := p.GetRequest(uriNodes)
	if err != nil {
		return nil, err
	}
	f5osLogger.Debug("[GetPartitionNodes]", "Resp", hclog.Fmt("%+v", string(byteData)))
	err = json.Unmarshal(byteData, &ss)
	if err != nil {
		return nil, err
	}
	var partitionNode int64
	allNodes := ss["f5-cluster:node"].([]interface{})
	for _, node := range allNodes {
		if node.(map[string]interface{})["state"].(map[string]interface{})["assigned"].(bool) {
			partitionNode = int64(node.(map[string]interface{})["state"].(map[string]interface{})["slot-number"].(int))
		}
	}
	return &partitionNode, nil
}

func (p *F5os) CheckPartitionState(partitionName string, timeOut int) ([]byte, error) {
	t1 := time.Now()
	for {
		check, err := p.partitionWait(partitionName)
		if err != nil {
			return []byte(""), err
		}
		t2 := time.Now()
		timeDiff := t2.Sub(t1)
		if timeDiff.Seconds() > float64(timeOut) {
			return []byte(""), fmt.Errorf("partition deployment still in in progress with timeout period, please increase timeout")
		}
		if check {
			time.Sleep(20 * time.Second)
			continue
		} else {
			time.Sleep(20 * time.Second)
			return []byte("Partition Deployment Success."), nil
		}
	}
}

// a quick and dirty all() python style function implementation for golang
func all(condition func(interface{}) bool, items []interface{}) bool {
	for _, item := range items {
		if !condition(item) {
			return false
		}
	}
	return true
}
func (p *F5os) partitionWait(partitionName string) (bool, error) {
	partitionMap, err := p.getPartitionDeployStatus(partitionName)
	if err != nil {
		return true, err
	}

	partitionStatusSlice := make([]interface{}, 0)

	// Loop over each controller and add its partition status to the slice
	controllers := partitionMap["f5-system-partition:state"].(map[string]interface{})["controllers"].(map[string]interface{})["controller"].([]interface{})
	for _, controller := range controllers {
		if controller.(map[string]interface{}) != nil && controller.(map[string]interface{})["partition-status"] != nil {
			partitionStatus := controller.(map[string]interface{})["partition-status"].(string)
			partitionStatusSlice = append(partitionStatusSlice, partitionStatus)
		}
	}
	f5osLogger.Debug("[partitionWait]", "partitionStatusSlice", hclog.Fmt("%+v", partitionStatusSlice))

	// Define a function to check if a partition status is valid
	partitionStatusIsValid := func(status interface{}) bool {
		validStatuses := []string{"running", "running-active", "running-standby"}
		for _, validStatus := range validStatuses {
			if status.(string) == validStatus {
				return true
			}
		}
		return false
	}

	// Check if all partition statuses are valid using the all() function
	allPartitionStatusesValid := all(partitionStatusIsValid, partitionStatusSlice)

	if allPartitionStatusesValid {
		return false, nil
	} else {
		return true, nil
	}
}

func (p *F5os) getPartitionDeployStatus(partitionName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/partition=%s/state", uriPartition, partitionName)
	f5osLogger.Debug("[getPartitionDeployStatus]", "Request path", hclog.Fmt("%+v", url))
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

func (p *F5os) UpdatePartitionIso(partitionName string, osVersion string) (bool, error) {
	var isoData = map[string]interface{}{
		"f5-system-partition:set-version": map[string]string{
			"iso-version": osVersion,
		},
	}
	byteBody, err := json.Marshal(isoData)
	if err != nil {
		return false, err
	}
	f5osLogger.Debug("[UpdateIsoVersion]", "Body", hclog.Fmt("%+v", string(byteBody)))
	url := fmt.Sprintf("%s/partition=%s/set-version", uriPartition, partitionName)
	f5osLogger.Debug("[UpdateIsoVersion]", "Request path", hclog.Fmt("%+v", url))
	respData, err := p.PostRequest(url, byteBody)
	if err != nil {
		return false, err
	}
	f5osLogger.Debug("[UpdateIsoVersion]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	return true, nil
}

func (p *F5os) SetSlot(partitionName string, slots []int64) ([]byte, error) {
	var slotData = map[string]interface{}{
		"f5-system-slot:slots": map[string]interface{}{
			"slot": func() []interface{} {
				var result []interface{}
				for _, slot := range slots {
					result = append(result, map[string]interface{}{
						"slot-num":  int(slot),
						"partition": partitionName,
					})
				}
				return result
			}(),
		},
	}

	byteBody, err := json.Marshal(slotData)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[SetSlot]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PatchRequest(uriSlots, byteBody)
	if err != nil {
		return respData, err
	}
	f5osLogger.Debug("[SetSlot]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	return respData, nil
}

func (p *F5os) PartitionPasswordChange(userName string, passwordChangeConfig *F5ReqPartitionPassChange) ([]byte, error) {
	url := fmt.Sprintf("%s/authentication/f5-system-aaa:users/f5-system-aaa:user=%s/f5-system-aaa:config/f5-system-aaa:change-password", uriAuth, userName)
	f5osLogger.Debug("[PartitionPasswordChange]", "Request path", hclog.Fmt("%+v", url))
	byteBody, err := json.Marshal(passwordChangeConfig)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[PartitionPasswordChange]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PostRequest(url, byteBody)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[PartitionPasswordChange]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	return byteBody, nil
}

func (p *F5os) VlanConfig(vlanConfig *F5ReqVlansConfig) ([]byte, error) {
	url := fmt.Sprintf("%s", uriVlan)
	f5osLogger.Debug("[VlanConfig]", "Request path", hclog.Fmt("%+v", url))
	byteBody, err := json.Marshal(vlanConfig)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[VlanConfig]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PatchRequest(url, byteBody)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[VlanConfig]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	return byteBody, nil
}

func (p *F5os) GetVlan(vlanId int) (*F5RespVlan, error) {
	url := fmt.Sprintf("%s/vlan=%d", uriVlan, vlanId)
	f5osVlan := &F5RespVlan{}
	byteData, err := p.GetTenantRequest(url)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(byteData, f5osVlan)
	f5osLogger.Info("[GetVlan]", "f5osVlan", hclog.Fmt("%+v", f5osVlan))
	return f5osVlan, nil
}

//
//func (p *F5os) AddVlan(vlanId int) ([]byte, error) {
//	f5osVlanid := F5osVlanId{}
//	f5osVlanid.VlanID = vlanId
//	f5osVlanid.Config.Name = fmt.Sprintf("vlan-%d", vlanId)
//	f5osVlanid.Config.VlanID = vlanId
//	f5osVlan := &F5osVlan{}
//	f5osVlan.OpenconfigVlanVlan = append(f5osVlan.OpenconfigVlanVlan, f5osVlanid)
//	f5osLogger.Debug("[AddVlan]", "AddVlan", hclog.Fmt("%+v", f5osVlan))
//	byteBody, err := json.Marshal(f5osVlan)
//	if err != nil {
//		return byteBody, err
//	}
//	respData, err := p.PostRequest(uriVlan, byteBody)
//	if err != nil {
//		return respData, err
//	}
//	f5osLogger.Debug("[AddVlan]", "f5osVlan", hclog.Fmt("%+v", string(respData)))
//	return respData, nil
//}

func (p *F5os) DeleteVlan(vlanId int) error {
	url := fmt.Sprintf("%s/vlan=%d", uriVlan, vlanId)
	f5osLogger.Info("[DeleteVlan]", "Path", hclog.Fmt("%+v", url))
	err := p.DeleteRequest(url)
	if err != nil {
		return err
	}
	return nil
}

func (p *F5os) InterfaceConfig(interfaceConfig *F5ReqOpenconfigInterface) ([]byte, error) {
	url := fmt.Sprintf("%s", uriVlan)
	f5osLogger.Debug("[InterfaceConfig]", "Request path", hclog.Fmt("%+v", url))
	byteBody, err := json.Marshal(interfaceConfig)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[InterfaceConfig]", "Body", hclog.Fmt("%+v", string(byteBody)))
	respData, err := p.PatchRequest(url, byteBody)
	if err != nil {
		return byteBody, err
	}
	f5osLogger.Debug("[InterfaceConfig]", "Resp: ", hclog.Fmt("%+v", string(respData)))
	return byteBody, nil
}

func (p *F5os) CreateTlsCertKey(config *TlsCertKey) error {
	f5osLogger.Debug("[CreateTlsCertKey]", "Request path", hclog.Fmt("%+v", uriCreateCertKey))
	byteBody, err := json.Marshal(config)
	if err != nil {
		return err
	}
	f5osLogger.Debug("[CreateTlsCertKey]", "Body", hclog.Fmt("%+v", string(byteBody)))
	_, err = p.PostRequest(uriCreateCertKey, byteBody)
	if err != nil {
		return err
	}
	return nil
}

func (p *F5os) DeleteTlsCertKey(certKeyName string) error {
	uri := "/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls"
	f5osLogger.Debug("[DeleteTlsCertKey]", "Request path", hclog.Fmt("%+v", uri))

	err := p.DeleteRequest(uri)

	return err
}

// PatchDNSConfig sets DNS config using PATCH to /system/dns
func (c *F5os) PatchDNSConfig(dnsServers []string, searchDomains []string) error {
	var servers []DNSServer
	for _, s := range dnsServers {
		servers = append(servers, DNSServer{Address: s})
	}

	payload := DNSConfigPayload{
		DNS: DNSConfig{
			Servers: DNSConfigServers{Server: servers},
			Config:  DNSConfigSearch{Search: searchDomains},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal DNS config payload: %w", err)
	}

	// 👇 Correct usage — relative to UriRoot
	_, err = c.PatchRequest(uriSystemDNS, body)
	if err != nil {
		return fmt.Errorf("failed to patch DNS config: %w", err)
	}
	return nil
}

// DeleteSearchDomain deletes a specific search domain entry
func (c *F5os) DeleteSearchDomain(domain string) error {
	path := fmt.Sprintf("/openconfig-system:system/dns/config/search=%s", domain)
	return c.DeleteRequest(path)
}

// DeleteDNSServer deletes a specific DNS server entry
func (c *F5os) DeleteDNSServer(address string) error {
	path := fmt.Sprintf("/openconfig-system:system/dns/servers/server=%s", address)
	return c.DeleteRequest(path)
}

// DeleteDNSConfig removes provided servers and domains (idempotent)
func (c *F5os) DeleteDNSConfig(dnsServers []string, searchDomains []string) error {
	for _, s := range dnsServers {
		if err := c.DeleteDNSServer(s); err != nil {
			return fmt.Errorf("delete DNS server %s failed: %w", s, err)
		}
	}
	for _, d := range searchDomains {
		if err := c.DeleteSearchDomain(d); err != nil {
			return fmt.Errorf("delete search domain %s failed: %w", d, err)
		}
	}
	return nil
}

// ReadDNSConfig fetches the current DNS configuration from the device
func (c *F5os) ReadDNSConfig() (*DNSConfigPayload, error) {
	path := "/openconfig-system:system/dns" // Relative path
	resp, err := c.GetRequest(path)
	if err != nil {
		return nil, fmt.Errorf("failed to GET DNS config: %w", err)
	}

	var config DNSConfigPayload
	if err := json.Unmarshal(resp, &config); err != nil {
		return nil, fmt.Errorf("invalid JSON in DNS read: %w", err)
	}
	return &config, nil
}

func (p *F5os) SetPrimaryKey(config *F5ReqPrimaryKey) ([]byte, error) {
	url := fmt.Sprintf("%s/aaa/f5-primary-key:primary-key/f5-primary-key:set", uriBase)
	f5osLogger.Debug("[SetPrimaryKey]", "Request path", hclog.Fmt("%+v", url))

	reqBody, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	f5osLogger.Debug("[SetPrimaryKey]", "Request Body", hclog.Fmt("%+v", string(reqBody)))

	respData, err := p.PostRequest(url, reqBody)
	if err != nil {
		return nil, err
	}
	f5osLogger.Debug("[SetPrimaryKey]", "Response", hclog.Fmt("%+v", string(respData)))

	return respData, nil
}

func (p *F5os) GetPrimaryKey() (*F5RespPrimaryKey, error) {
	url := fmt.Sprintf("%s/aaa/f5-primary-key:primary-key", uriBase)
	f5osLogger.Debug("[GetPrimaryKey]", "Request URL", hclog.Fmt("%+v", url))

	body, err := p.GetRequest(url)
	if err != nil {
		return nil, err
	}

	var resp F5RespPrimaryKey
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	f5osLogger.Debug("[GetPrimaryKey]", "Parsed Response", hclog.Fmt("%+v", resp))
	return &resp, nil
}

func (p *F5os) UpdatePrimaryKey(req *F5ReqPrimaryKey) ([]byte, error) {
	url := fmt.Sprintf("%s/aaa/f5-primary-key:primary-key/f5-primary-key:set", uriBase)
	f5osLogger.Debug("[UpdatePrimaryKey]", "Request path", hclog.Fmt("%+v", url))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	f5osLogger.Debug("[UpdatePrimaryKey]", "Body", hclog.Fmt("%+v", string(body)))

	respData, err := p.PostRequest(url, body) // Use POST instead of PATCH as per your API spec
	if err != nil {
		return nil, err
	}

	f5osLogger.Debug("[UpdatePrimaryKey]", "Response", hclog.Fmt("%+v", string(respData)))
	return respData, nil
}

const (
	uriNTPServer      = "/openconfig-system:system/ntp/openconfig-system:servers/server=%s"
	uriNTPServerBase  = "/openconfig-system:system/ntp/openconfig-system:servers"
	uriNTPConfigPatch = "/openconfig-system:system/ntp/config"
)

type ntpServerConfig struct {
	Address string `json:"address"`
	KeyID   int64  `json:"f5-openconfig-system-ntp:key-id,omitempty"`
	Prefer  bool   `json:"prefer,omitempty"`
	Iburst  bool   `json:"iburst,omitempty"`
}

type ntpServerPayload struct {
	Server []struct {
		Address string          `json:"address"`
		Config  ntpServerConfig `json:"config"`
	} `json:"server"`
}

type ntpConfigPatch struct {
	Config struct {
		Enabled       *bool `json:"enabled,omitempty"`
		EnableNTPAuth *bool `json:"enable-ntp-auth,omitempty"`
	} `json:"config"`
}

func (c *F5os) CreateNTPServer(server string, payload []byte) error {
	uri := fmt.Sprintf("/openconfig-system:system/ntp/openconfig-system:servers")

	resp, err := c.PostRequest(uri, payload)
	if err != nil {
		return fmt.Errorf("failed to create NTP server %s: %w", server, err)
	}
	f5osLogger.Debug("[CreatePartition]", "Resp: ", hclog.Fmt("%+v", string(resp)))

	return nil
}

func (c *F5os) CreateNTPServerPayload(server string, plan NTPServerModel) ([]byte, error) {
	payload := ntpServerPayload{
		Server: []struct {
			Address string          `json:"address"`
			Config  ntpServerConfig `json:"config"`
		}{
			{
				Address: server,
				Config: ntpServerConfig{
					Address: server,
					KeyID:   plan.KeyID.ValueInt64(),
					Prefer:  plan.Prefer.ValueBool(),
					Iburst:  plan.IBurst.ValueBool(),
				},
			},
		},
	}
	return json.Marshal(payload)
}

func (c *F5os) GetNTPServer(server string) (*NTPServerStruct, error) {
	uri := fmt.Sprintf(uriNTPServer, server)
	resp, err := c.GetRequest(uri)
	if err != nil {
		return nil, fmt.Errorf("GET NTP server failed: %w", err)
	}
	var ntpResp NTPServerStruct
	if err := json.Unmarshal(resp, &ntpResp); err != nil {
		return nil, fmt.Errorf("invalid JSON for NTP server: %w", err)
	}
	return &ntpResp, nil
}

func (c *F5os) UpdateNTPServer(server string, payload []byte) error {
	uri := fmt.Sprintf(uriNTPServer, server)
	_, err := c.PatchRequest(uri, payload)
	if err != nil {
		return fmt.Errorf("PATCH NTP server failed: %w", err)
	}
	return nil
}

func (c *F5os) DeleteNTPServer(server string) error {
	uri := fmt.Sprintf(uriNTPServer, server)
	err := c.DeleteRequest(uri)
	if err != nil {
		return fmt.Errorf("DELETE NTP server failed: %w", err)
	}
	return nil
}

// Optional: Patch global NTP settings like service and authentication
func (c *F5os) PatchNTPGlobalConfig(service, auth *bool) error {
	var payload ntpConfigPatch
	payload.Config.Enabled = service
	payload.Config.EnableNTPAuth = auth
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal global NTP config: %w", err)
	}
	_, err = c.PatchRequest(uriNTPConfigPatch, body)
	if err != nil {
		return fmt.Errorf("PATCH NTP global config failed: %w", err)
	}
	return nil
}
