package f5os

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// PortGroupConfig mirrors the vendor client's struct for portgroup payloads.
type PortGroupConfig struct {
	Name string              `json:"name"`
	Mode string              `json:"mode"`
	DDM  *PortGroupDDMConfig `json:"f5-ddm:ddm,omitempty"`
}

type PortGroupDDMConfig struct {
	PollFrequency *int64 `json:"f5-ddm:ddm-poll-frequency,omitempty"`
}

type portGroupResponse struct {
	PortGroup []struct {
		PortGroupName string          `json:"portgroup_name"`
		Config        PortGroupConfig `json:"config"`
	} `json:"f5-portgroup:portgroup"`
	PortGroups struct {
		PortGroup []struct {
			PortGroupName string          `json:"portgroup_name"`
			Config        PortGroupConfig `json:"config"`
		} `json:"portgroup"`
	} `json:"f5-portgroup:portgroups"`
}

const uriPortGroups = "/f5-portgroup:portgroups"

func portGroupURI(name string) string {
	return fmt.Sprintf("%s/portgroup=%s", uriPortGroups, url.PathEscape(name))
}

// GetPortGroup retrieves a hardware-defined rSeries port group using the raw
// f5os client methods. This duplicates logic previously in the vendored
// client but keeps test-only orchestration out of vendor.
func (client *F5os) GetPortGroup(name string) (*PortGroupConfig, error) {
	resp, err := client.GetRequest(portGroupURI(name))
	if err != nil {
		return nil, fmt.Errorf("GET port group %q failed: %w", name, err)
	}

	var parsed portGroupResponse
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON for port group %q: %w", name, err)
	}

	var cfg *PortGroupConfig
	if len(parsed.PortGroup) > 0 {
		cfg = &parsed.PortGroup[0].Config
	} else if len(parsed.PortGroups.PortGroup) > 0 {
		cfg = &parsed.PortGroups.PortGroup[0].Config
	}

	if cfg == nil {
		return nil, fmt.Errorf("port group %q was not returned by the device", name)
	}

	// Flexible parsing for DDM poll frequency (optional)
	if cfg.DDM == nil || cfg.DDM.PollFrequency == nil {
		var raw map[string]interface{}
		if err := json.Unmarshal(resp, &raw); err == nil {
			var item map[string]interface{}
			for _, rootKey := range []string{"f5-portgroup:portgroup", "portgroup", "f5-portgroup:portgroups", "portgroups"} {
				if v, ok := raw[rootKey]; ok {
					if list, ok := v.([]interface{}); ok && len(list) > 0 {
						if m, ok := list[0].(map[string]interface{}); ok {
							item = m
							break
						}
					} else if m, ok := v.(map[string]interface{}); ok {
						if subList, ok := m["portgroup"].([]interface{}); ok && len(subList) > 0 {
							if sm, ok := subList[0].(map[string]interface{}); ok {
								item = sm
								break
							}
						} else {
							item = m
							break
						}
					}
				}
			}

			if item != nil {
				sources := []map[string]interface{}{}
				if cfgMap, ok := item["config"].(map[string]interface{}); ok {
					sources = append(sources, cfgMap)
				}
				sources = append(sources, item)

				for _, src := range sources {
					for _, ddmKey := range []string{"f5-ddm:ddm", "ddm"} {
						if dVal, ok := src[ddmKey].(map[string]interface{}); ok {
							targets := []map[string]interface{}{dVal}
							if nested, ok := dVal["config"].(map[string]interface{}); ok {
								targets = append(targets, nested)
							}
							for _, target := range targets {
								for _, freqKey := range []string{"f5-ddm:ddm-poll-frequency", "ddm-poll-frequency", "poll-frequency"} {
									if fVal, ok := target[freqKey]; ok {
										if fNum, ok := fVal.(float64); ok {
											fInt := int64(fNum)
											if cfg.DDM == nil {
												cfg.DDM = &PortGroupDDMConfig{}
											}
											cfg.DDM.PollFrequency = &fInt
											break
										}
									}
								}
								if cfg.DDM != nil && cfg.DDM.PollFrequency != nil {
									break
								}
							}
						}
						if cfg.DDM != nil && cfg.DDM.PollFrequency != nil {
							break
						}
					}
					if cfg.DDM != nil && cfg.DDM.PollFrequency != nil {
						break
					}
				}
			}
		}
	}

	return cfg, nil
}

func (client *F5os) SetPortGroupConfig(name string, config *PortGroupConfig) error {
	payload := struct {
		Config PortGroupConfig `json:"f5-portgroup:config"`
	}{Config: *config}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal port group %q payload: %w", name, err)
	}
	if _, err := client.PatchRequest(portGroupURI(name)+"/config", body); err != nil {
		return fmt.Errorf("PATCH port group %q failed: %w", name, err)
	}
	tflog.Info(context.Background(), fmt.Sprintf("PortGroup updated: name=%s", name))
	return nil
}

// ClearPortGroupDDM clears the DDM poll frequency for a port group, resetting it to device default
func (client *F5os) ClearPortGroupDDM(name string) error {
	if err := client.DeleteRequest(portGroupURI(name) + "/config/f5-ddm:ddm-poll-frequency"); err != nil {
		return fmt.Errorf("DELETE port group %q DDM poll frequency failed: %w", name, err)
	}
	tflog.Info(context.Background(), fmt.Sprintf("PortGroup DDM cleared: name=%s", name))
	return nil
}


