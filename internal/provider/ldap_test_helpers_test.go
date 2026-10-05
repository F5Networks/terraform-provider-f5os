package provider

// Test-only helpers for LDAP server-group creation/deletion. These functions
// exist solely to allow acceptance/unit tests to provision and clean up the
// LDAP-type server-group that f5os_ldap_server requires as a precondition.
// Production code (f5os_ldap_server_resource.go) must never create, patch,
// or delete server-groups; it only verifies the named server-group already
// exists. Because this file ends in _test.go, these helpers are compiled
// only when running `go test` and are excluded from production builds.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// EnsureLdapServerGroupCreated ensures a server-group exists. Returns true if
// the caller actually created the server-group, false if it already existed
// (possibly patched to correct shape). Test-only: used by acceptance/unit
// tests to provision the server-group that f5os_ldap_server requires.
func EnsureLdapServerGroupCreated(client *f5ossdk.F5os, name string) (bool, error) {
	uri := fmt.Sprintf(f5ossdk.UriAAAServerGroup, url.PathEscape(name))

	resp, err := client.GetRequest(uri)
	// If GET returned an error or empty body, treat as absent and create.
	if err == nil && len(resp) > 0 {
		var parsed map[string]interface{}
		if jerr := json.Unmarshal(resp, &parsed); jerr == nil {
			var entry map[string]interface{}
			keys := []string{"openconfig-system:server-group", "server-group", "openconfig-system:server-groups", "server-groups"}
			for _, k := range keys {
				if v, ok := parsed[k]; ok {
					switch vt := v.(type) {
					case []interface{}:
						for _, item := range vt {
							if m, ok := item.(map[string]interface{}); ok {
								if n, _ := m["name"].(string); n == name {
									entry = m
									break
								}
							}
						}
					case map[string]interface{}:
						if inner, ok := vt["server-group"]; ok {
							if list, ok := inner.([]interface{}); ok && len(list) > 0 {
								if m, ok := list[0].(map[string]interface{}); ok {
									entry = m
								}
							}
						}
						if entry == nil {
							entry = vt
						}
					}
					if entry != nil {
						break
					}
				}
			}

			if entry != nil {
				if cfg, ok := entry["config"].(map[string]interface{}); ok {
					if t, ok := cfg["type"].(string); ok && t == "f5-openconfig-aaa-ldap:LDAP" {
						// present and correct
						tflog.Info(context.Background(), fmt.Sprintf("ServerGroup exists: name=%s", name))
						return false, nil
					}
				}
				// Attempt to patch the type only (do not modify servers)
				patchPayload := map[string]interface{}{
					"openconfig-system:server-group": []map[string]interface{}{
						{"name": name, "config": map[string]interface{}{"name": name, "type": "f5-openconfig-aaa-ldap:LDAP"}},
					},
				}
				body, _ := json.Marshal(patchPayload)
				if _, perr := client.PatchRequest(uri, body); perr == nil {
					tflog.Info(context.Background(), fmt.Sprintf("ServerGroup patched: name=%s", name))
				}
				return false, nil
			}
		}
	}

	// Create the server-group (collection-level POST)
	createURI := f5ossdk.UriAAAServerGroups
	payload := map[string]interface{}{
		"openconfig-system:server-group": []map[string]interface{}{
			{
				"name": name,
				"config": map[string]interface{}{
					"name": name,
					"type": "f5-openconfig-aaa-ldap:LDAP",
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	_, err = client.PostRequest(createURI, body)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			// Race: someone else created it. Treat as not-created by us.
			return false, nil
		}
		return false, fmt.Errorf("create server group %q failed: %w", name, err)
	}
	// Log creation and poll for visibility
	tflog.Info(context.Background(), fmt.Sprintf("ServerGroup created: name=%s", name))
	const (
		pollInterval = 3 * time.Second
		pollTimeout  = 15 * time.Second
	)
	start := time.Now()
	for {
		resp, gerr := client.GetRequest(uri)
		if gerr == nil && len(resp) > 0 {
			return true, nil
		}
		if time.Since(start) >= pollTimeout {
			return true, fmt.Errorf("server group %q not visible after %v: last error: %v", name, pollTimeout, gerr)
		}
		time.Sleep(pollInterval)
	}
}

// DeleteServerGroup removes a server group from the device. Test-only: used
// by acceptance/unit test cleanup to remove a server-group that was created
// by EnsureLdapServerGroupCreated during test setup.
func DeleteServerGroup(client *f5ossdk.F5os, name string) error {
	uri := fmt.Sprintf(f5ossdk.UriAAAServerGroup, url.PathEscape(name))
	if err := client.DeleteRequest(uri); err != nil {
		return err
	}
	tflog.Info(context.Background(), fmt.Sprintf("ServerGroup deleted: name=%s", name))
	return nil
}
