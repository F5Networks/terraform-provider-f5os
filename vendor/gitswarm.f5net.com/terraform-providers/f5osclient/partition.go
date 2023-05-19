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
	"github.com/hashicorp/go-hclog"
	"time"
)

const (
	uriSlot      = "/f5-system-slot:slots/slot"
	uriSlots     = "/f5-system-slot:slots"
	uriPartition = "/f5-system-partition:partitions"
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
