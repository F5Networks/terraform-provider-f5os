package f5os

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	UriAAAServerGroups       = "/openconfig-system:system/aaa/server-groups"
	UriAAAServerGroup        = "/openconfig-system:system/aaa/server-groups/server-group=%s"
	uriAAAServerGroupServers = "/openconfig-system:system/aaa/server-groups/server-group=%s/servers"
	uriAAAServerGroupServer  = "/openconfig-system:system/aaa/server-groups/server-group=%s/servers/server=%s"
)

type LdapConfig struct {
	BaseDN           interface{} `json:"base,omitempty"`
	BindDN           interface{} `json:"binddn,omitempty"`
	BindPW           interface{} `json:"bindpw,omitempty"`
	BindTimelimit    interface{} `json:"bind_timelimit,omitempty"`
	IdleTimelimit    interface{} `json:"idle_timelimit,omitempty"`
	Timelimit        interface{} `json:"timelimit,omitempty"`
	LDAPVersion      interface{} `json:"ldap_version,omitempty"`
	ChaseReferrals   interface{} `json:"chase-referrals,omitempty"`
	SSL              interface{} `json:"ssl,omitempty"`
	ActiveDirectory  interface{} `json:"active_directory,omitempty"`
	UserObjectClass  []string    `json:"user-object-class"`
	GroupObjectClass []string    `json:"group-object-class"`
	UnixAttributes   interface{} `json:"unix_attributes,omitempty"`
	IgnoreCase       interface{} `json:"ignore-case,omitempty"`
}

type LdapServerConfig struct {
	Address  string `json:"address"`
	AuthPort *int64 `json:"f5-openconfig-aaa-ldap:auth-port,omitempty"`
	Type     string `json:"f5-openconfig-aaa-ldap:type,omitempty"`
}

// CreateLdapServer creates a new LDAP server within a server group using the
// supplied low-level client methods. It mirrors the vendor client's behavior
// but is colocated in internal/provider so tests and resources don't need to
// modify the vendored client.
func (client *F5os) CreateLdapServer(serverGroup, address string, port *int64, serverType string) error {
	uri := fmt.Sprintf(uriAAAServerGroupServers, url.PathEscape(serverGroup))

	ldapConfig := map[string]interface{}{}
	if port != nil {
		ldapConfig["auth-port"] = *port
	}
	if serverType != "" {
		ldapConfig["type"] = serverType
	}

	payload := map[string]interface{}{
		"openconfig-system:server": []map[string]interface{}{
			{
				"address":                     address,
				"config":                      map[string]interface{}{"address": address},
				"f5-openconfig-aaa-ldap:ldap": map[string]interface{}{"config": ldapConfig},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal LDAP server: %w", err)
	}

	_, err = client.PostRequest(uri, body)
	if err == nil {
		tflog.Info(context.Background(), fmt.Sprintf("LdapServer created: group=%s address=%s port=%v type=%s", serverGroup, address, port, serverType))
		return nil
	}

	// If the device reports the servers container is missing, attempt to
	// PATCH the server-group to include an empty servers container and retry
	// the POST once. Devices sometimes require the parent container to exist
	// before accepting POST to the child collection.
	errStr := err.Error()
	if strings.Contains(strings.ToLower(errStr), "missing element: servers") || strings.Contains(strings.ToLower(errStr), "servers") {
		patchPayload := map[string]interface{}{
			"openconfig-system:server-group": []map[string]interface{}{
				{
					"name":    serverGroup,
					"config":  map[string]interface{}{"name": serverGroup, "type": "f5-openconfig-aaa-ldap:LDAP"},
					"servers": map[string]interface{}{"server": []interface{}{}},
				},
			},
		}
		patchBody, _ := json.Marshal(patchPayload)
		if _, perr := client.PatchRequest(fmt.Sprintf(UriAAAServerGroup, url.PathEscape(serverGroup)), patchBody); perr == nil {
			_, err2 := client.PostRequest(uri, body)
			if err2 == nil {
				tflog.Info(context.Background(), fmt.Sprintf("LdapServer created after patch: group=%s address=%s port=%v type=%s", serverGroup, address, port, serverType))
				return nil
			}
			return fmt.Errorf("POST LDAP server failed after patch retry: %w", err2)
		}
	}

	return fmt.Errorf("POST LDAP server failed: %w", err)
}

// UpdateLdapServer updates an existing LDAP server.
func (client *F5os) UpdateLdapServer(serverGroup, address string, port *int64, serverType string) error {
	uri := fmt.Sprintf(uriAAAServerGroupServer, url.PathEscape(serverGroup), url.PathEscape(address))

	ldapConfig := map[string]interface{}{}
	if port != nil {
		ldapConfig["auth-port"] = *port
	}
	if serverType != "" {
		ldapConfig["type"] = serverType
	}

	payload := map[string]interface{}{
		"openconfig-system:server": []map[string]interface{}{
			{
				"address":                     address,
				"config":                      map[string]interface{}{"address": address},
				"f5-openconfig-aaa-ldap:ldap": map[string]interface{}{"config": ldapConfig},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal LDAP server update payload: %w", err)
	}

	_, err = client.PatchRequest(uri, body)
	if err != nil {
		return fmt.Errorf("failed to update LDAP server %s in group %s: %w", address, serverGroup, err)
	}
	tflog.Info(context.Background(), fmt.Sprintf("LdapServer updated: group=%s address=%s port=%v type=%s", serverGroup, address, port, serverType))
	return nil
}

// DeleteLdapServer removes an LDAP server from a server group.
func (client *F5os) DeleteLdapServer(serverGroup string, address string) error {
	uri := fmt.Sprintf(uriAAAServerGroupServer, url.PathEscape(serverGroup), url.PathEscape(address))
	if err := client.DeleteRequest(uri); err != nil {
		return fmt.Errorf("DELETE LDAP server %s failed: %w", address, err)
	}
	tflog.Info(context.Background(), fmt.Sprintf("LdapServer deleted: group=%s address=%s", serverGroup, address))
	return nil
}

// GetLdapServer retrieves an individual LDAP server from a server group.
func (client *F5os) GetLdapServer(serverGroup string, address string) (*LdapServerConfig, error) {
	uri := fmt.Sprintf(uriAAAServerGroupServer, url.PathEscape(serverGroup), url.PathEscape(address))
	resp, err := client.GetRequest(uri)
	if err != nil {
		return nil, fmt.Errorf("GET LDAP server failed: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(resp, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON for LDAP server: %w", err)
	}

	var serverEntry map[string]interface{}
	for _, key := range []string{"openconfig-system:server", "server"} {
		if val, ok := raw[key]; ok {
			if list, ok := val.([]interface{}); ok && len(list) > 0 {
				for _, item := range list {
					if entryMap, ok := item.(map[string]interface{}); ok {
						entryAddr, _ := entryMap["address"].(string)
						if entryAddr == "" {
							if cfgMap, ok := entryMap["config"].(map[string]interface{}); ok {
								entryAddr, _ = cfgMap["address"].(string)
							}
						}
						if entryAddr == address || len(list) == 1 {
							serverEntry = entryMap
							break
						}
					}
				}
				if serverEntry != nil {
					break
				}
			} else if entryMap, ok := val.(map[string]interface{}); ok {
				serverEntry = entryMap
				break
			}
		}
	}

	if serverEntry == nil {
		return nil, fmt.Errorf("LDAP server %s not found in response", address)
	}

	cfg := &LdapServerConfig{Address: address}

	if addr, ok := serverEntry["address"].(string); ok && addr != "" {
		cfg.Address = addr
	} else if configMap, ok := serverEntry["config"].(map[string]interface{}); ok {
		if addr, ok := configMap["address"].(string); ok && addr != "" {
			cfg.Address = addr
		}
	}

	var ldapMap map[string]interface{}
	for _, k := range []string{"f5-openconfig-aaa-ldap:ldap", "ldap"} {
		if v, ok := serverEntry[k].(map[string]interface{}); ok {
			ldapMap = v
			break
		}
	}

	if ldapMap != nil {
		targetMap := ldapMap
		if nestedConfig, ok := ldapMap["config"].(map[string]interface{}); ok {
			targetMap = nestedConfig
		}

		for _, portKey := range []string{"auth-port", "f5-openconfig-aaa-ldap:auth-port"} {
			if pVal, ok := targetMap[portKey]; ok {
				if pNum, ok := pVal.(float64); ok {
					pInt := int64(pNum)
					cfg.AuthPort = &pInt
					break
				}
			}
		}

		for _, typeKey := range []string{"type", "f5-openconfig-aaa-ldap:type"} {
			if tVal, ok := targetMap[typeKey]; ok {
				if tStr, ok := tVal.(string); ok {
					if idx := strings.LastIndex(tStr, ":"); idx >= 0 {
						cfg.Type = tStr[idx+1:]
					} else {
						cfg.Type = tStr
					}
					break
				}
			}
		}
	}

	return cfg, nil
}

// --- LDAP common config helpers (merged from ldap_common_client.go) ---
const uriAAALdap = "/openconfig-system:system/aaa/authentication/f5-openconfig-aaa-ldap:ldap"

// GetLdapConfig reads the current LDAP container config from the device.
// Returns the vendor's LdapConfig type so callers that expect that type
// (e.g., ldapConfigToModel) can operate without additional conversion.
func (client *F5os) GetLdapConfig() (*LdapConfig, error) {
	resp, err := client.GetRequest(uriAAALdap)
	if err != nil {
		return nil, fmt.Errorf("GET ldap config failed: %w", err)
	}

	var parsed struct {
		Ldap *LdapConfig `json:"f5-openconfig-aaa-ldap:ldap"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON for ldap config: %w", err)
	}
	if parsed.Ldap == nil {
		// Attempt to parse vendor-local LdapConfig (in partition.go) as a
		// fallback so both shapes are accepted during the refactor.
		var parsedLocal struct {
			Ldap *struct {
				BaseDN           interface{} `json:"base,omitempty"`
				BindDN           interface{} `json:"binddn,omitempty"`
				BindPW           interface{} `json:"bindpw,omitempty"`
				BindTimelimit    interface{} `json:"bind_timelimit,omitempty"`
				IdleTimelimit    interface{} `json:"idle_timelimit,omitempty"`
				Timelimit        interface{} `json:"timelimit,omitempty"`
				LDAPVersion      interface{} `json:"ldap_version,omitempty"`
				ChaseReferrals   interface{} `json:"chase-referrals,omitempty"`
				SSL              interface{} `json:"ssl,omitempty"`
				ActiveDirectory  interface{} `json:"active_directory,omitempty"`
				UserObjectClass  []string    `json:"user-object-class,omitempty"`
				GroupObjectClass []string    `json:"group-object-class,omitempty"`
				UnixAttributes   interface{} `json:"unix_attributes,omitempty"`
				IgnoreCase       interface{} `json:"ignore-case,omitempty"`
			} `json:"f5-openconfig-aaa-ldap:ldap"`
		}
		if perr := json.Unmarshal(resp, &parsedLocal); perr == nil && parsedLocal.Ldap != nil {
			out := &LdapConfig{}
			out.BaseDN = parsedLocal.Ldap.BaseDN
			out.BindDN = parsedLocal.Ldap.BindDN
			out.BindPW = parsedLocal.Ldap.BindPW
			out.BindTimelimit = parsedLocal.Ldap.BindTimelimit
			out.IdleTimelimit = parsedLocal.Ldap.IdleTimelimit
			out.Timelimit = parsedLocal.Ldap.Timelimit
			out.LDAPVersion = parsedLocal.Ldap.LDAPVersion
			out.ChaseReferrals = parsedLocal.Ldap.ChaseReferrals
			out.SSL = parsedLocal.Ldap.SSL
			out.ActiveDirectory = parsedLocal.Ldap.ActiveDirectory
			out.UserObjectClass = parsedLocal.Ldap.UserObjectClass
			out.GroupObjectClass = parsedLocal.Ldap.GroupObjectClass
			out.UnixAttributes = parsedLocal.Ldap.UnixAttributes
			out.IgnoreCase = parsedLocal.Ldap.IgnoreCase
			return out, nil
		}
		return &LdapConfig{}, nil
	}
	return parsed.Ldap, nil
}

// SetLdapConfig updates the LDAP configuration on the device.
func (client *F5os) SetLdapConfig(config *LdapConfig) error {
	payload := struct {
		Ldap *LdapConfig `json:"f5-openconfig-aaa-ldap:ldap"`
	}{Ldap: config}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal ldap config: %w", err)
	}
	if _, err := client.PatchRequest(uriAAALdap, body); err != nil {
		return fmt.Errorf("PATCH ldap config failed: %w", err)
	}
	return nil
}
