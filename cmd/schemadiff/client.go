package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// f5osClient is a lightweight RESTCONF client for schema discovery.
// It mirrors the auth flow in the vendored f5osclient but is self-contained
// so this tool can be built independently.
type f5osClient struct {
	host            string // full https://host:port
	uriRoot         string // /restconf/data or /api/data
	token           string
	transport       *http.Transport
	timeout         time.Duration
	platformType    string
	platformVersion string
}

// newF5osClient authenticates to an F5OS device and detects its platform.
func newF5osClient(host, user, pass string) (*f5osClient, error) {
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse host: %w", err)
	}
	_, port, _ := net.SplitHostPort(u.Host)
	uriRoot := "/restconf/data"
	if port == "443" {
		uriRoot = "/api/data"
	}

	c := &f5osClient{
		host:    host,
		uriRoot: uriRoot,
		transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // lab devices
		},
		timeout: 30 * time.Second,
	}

	// authenticate – GET /restconf/data/openconfig-system:system/aaa with basic auth
	loginURL := fmt.Sprintf("%s%s/openconfig-system:system/aaa", c.host, c.uriRoot)
	req, err := http.NewRequest("GET", loginURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(user, pass)
	req.Header.Set("Content-Type", "application/yang-data+json")

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login returned %d: %s", resp.StatusCode, body)
	}
	c.token = resp.Header.Get("X-Auth-Token")
	if c.token == "" {
		return nil, fmt.Errorf("no X-Auth-Token in login response")
	}

	// detect platform type + version
	if err := c.detectPlatform(); err != nil {
		return nil, fmt.Errorf("detect platform: %w", err)
	}

	return c, nil
}

// get performs an authenticated GET and returns the raw body bytes.
func (c *f5osClient) get(path string) ([]byte, int, error) {
	u := fmt.Sprintf("%s%s%s", c.host, c.uriRoot, path)
	req, err := http.NewRequest("GET", u, bytes.NewBuffer(nil))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Auth-Token", c.token)
	req.Header.Set("Content-Type", "application/yang-data+json")

	resp, err := c.do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func (c *f5osClient) do(req *http.Request) (*http.Response, error) {
	client := &http.Client{Transport: c.transport, Timeout: c.timeout}
	return client.Do(req)
}

// detectPlatform mirrors f5osclient.setPlatformType to identify the device.
func (c *f5osClient) detectPlatform() error {
	body, status, err := c.get("/openconfig-platform:components/component")
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("platform detection returned %d", status)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("unmarshal platform: %w", err)
	}

	components, ok := data["openconfig-platform:component"].([]interface{})
	if !ok {
		return fmt.Errorf("missing openconfig-platform:component array")
	}

	if len(components) > 1 {
		for _, comp := range components {
			m, _ := comp.(map[string]interface{})
			if m["name"] == "platform" {
				if state, ok := m["state"].(map[string]interface{}); ok {
					if desc, ok := state["description"].(string); ok {
						c.platformType = desc
					}
				}
			}
		}
		// fetch rSeries version
		c.fetchRSeriesVersion()
	} else if len(components) == 1 {
		c.platformType = "Velos Partition"
		c.fetchPartitionVersion(components[0])
	}

	if c.platformType == "" {
		c.platformType = "Unknown"
	}
	if c.platformVersion == "" {
		c.platformVersion = "unknown"
	}
	return nil
}

func (c *f5osClient) fetchRSeriesVersion() {
	body, status, err := c.get("/openconfig-system:system/f5-system-image:image/state/install")
	if err != nil || status != 200 {
		return
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return
	}
	if install, ok := data["f5-system-image:install"].(map[string]interface{}); ok {
		if v, ok := install["install-os-version"].(string); ok {
			c.platformVersion = v
		}
	}
}

func (c *f5osClient) fetchPartitionVersion(comp interface{}) {
	m, ok := comp.(map[string]interface{})
	if !ok {
		return
	}
	sw, ok := m["f5-platform:software"].(map[string]interface{})
	if !ok {
		return
	}
	state, ok := sw["state"].(map[string]interface{})
	if !ok {
		return
	}
	scs, ok := state["software-components"].(map[string]interface{})
	if !ok {
		return
	}
	scList, ok := scs["software-component"].([]interface{})
	if !ok || len(scList) == 0 {
		return
	}
	sc, ok := scList[0].(map[string]interface{})
	if !ok {
		return
	}
	scState, ok := sc["state"].(map[string]interface{})
	if !ok {
		return
	}
	if idx, ok := sc["software-index"].(string); ok && idx == "blade-os" {
		if v, ok := scState["version"].(string); ok {
			c.platformVersion = v
		}
	}
}

// ── YANG library ──────────────────────────────────────────────────────

// YANGModule represents a single module from ietf-yang-library.
type YANGModule struct {
	Name      string `json:"name"`
	Revision  string `json:"revision"`
	Namespace string `json:"namespace"`
	Schema    string `json:"schema,omitempty"`
}

// fetchYANGModules retrieves the YANG library module list per RFC 8525.
// Falls back to the older RFC 7895 modules-state if the newer endpoint is unavailable.
func (c *f5osClient) fetchYANGModules() ([]YANGModule, error) {
	// Try RFC 8525 first: /restconf/data/ietf-yang-library:yang-library
	modules, err := c.tryYANGLibrary("/ietf-yang-library:yang-library")
	if err == nil && len(modules) > 0 {
		return modules, nil
	}
	log.Printf("  RFC 8525 yang-library not available (%v), trying RFC 7895 modules-state...", err)

	// Fallback: RFC 7895 /restconf/data/ietf-yang-library:modules-state
	modules, err = c.tryModulesState("/ietf-yang-library:modules-state")
	if err == nil && len(modules) > 0 {
		return modules, nil
	}

	return nil, fmt.Errorf("YANG library unavailable: %w", err)
}

func (c *f5osClient) tryYANGLibrary(path string) ([]YANGModule, error) {
	body, status, err := c.get(path)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("status %d", status)
	}

	// RFC 8525 structure:
	// { "ietf-yang-library:yang-library": { "module-set": [ { "module": [...] } ] } }
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var modules []YANGModule
	lib, ok := data["ietf-yang-library:yang-library"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected yang-library structure")
	}
	moduleSets, ok := lib["module-set"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no module-set found")
	}
	for _, ms := range moduleSets {
		msMap, ok := ms.(map[string]interface{})
		if !ok {
			continue
		}
		mods, ok := msMap["module"].([]interface{})
		if !ok {
			continue
		}
		for _, m := range mods {
			mm, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			mod := YANGModule{
				Name:      strVal(mm, "name"),
				Revision:  strVal(mm, "revision"),
				Namespace: strVal(mm, "namespace"),
			}
			if mod.Name != "" {
				modules = append(modules, mod)
			}
		}
	}
	return modules, nil
}

func (c *f5osClient) tryModulesState(path string) ([]YANGModule, error) {
	body, status, err := c.get(path)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("status %d", status)
	}

	// RFC 7895 structure:
	// { "ietf-yang-library:modules-state": { "module": [...] } }
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var modules []YANGModule
	state, ok := data["ietf-yang-library:modules-state"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected modules-state structure")
	}
	mods, ok := state["module"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no module list found")
	}
	for _, m := range mods {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		mod := YANGModule{
			Name:      strVal(mm, "name"),
			Revision:  strVal(mm, "revision"),
			Namespace: strVal(mm, "namespace"),
			Schema:    strVal(mm, "schema"),
		}
		if mod.Name != "" {
			modules = append(modules, mod)
		}
	}
	return modules, nil
}

// ── API endpoint crawling ─────────────────────────────────────────────

// APIEndpoint holds the crawl result for one RESTCONF path.
type APIEndpoint struct {
	Path       string              // RESTCONF URI relative to uriRoot
	StatusCode int                 // HTTP status (200, 404, etc.)
	TopKeys    []string            // top-level JSON keys in the response
	Properties map[string]PropInfo // flattened property map
}

// PropInfo describes a single property discovered in an API response.
type PropInfo struct {
	Type     string // "string", "number", "bool", "array", "object", "null"
	Path     string // dot-delimited path within the response
	HasValue bool   // whether a non-null value was present
}

// endpointsToScan is the list of well-known F5OS RESTCONF paths we crawl.
// This covers the APIs the Terraform provider manages.
var endpointsToScan = []string{
	// system
	"/openconfig-system:system/aaa",
	"/openconfig-system:system/dns",
	"/openconfig-system:system/ntp",
	"/openconfig-system:system/f5-system-snmp:snmp",
	"/openconfig-system:system/f5-system-logging:logging",
	"/openconfig-system:system/f5-system-licensing:licensing",
	"/openconfig-system:system/f5-system-image:image",
	"/openconfig-system:system/f5-database:database",

	// interfaces
	"/openconfig-interfaces:interfaces",
	"/openconfig-lacp:lacp/interfaces",

	// vlans
	"/openconfig-vlan:vlans",

	// platform
	"/openconfig-platform:components",

	// tenants
	"/f5-tenants:tenants",
	"/f5-tenant-images:images",

	// partitions (Velos)
	"/f5-system-partition:partitions",

	// slots
	"/f5-system-slot:slots",

	// file transfer
	"/f5-utils-file-transfer:file/list",
	"/f5-utils-file-transfer:file/transfer-operations/transfer-operation",

	// users / auth
	"/openconfig-system:system/aaa/authentication",
	"/openconfig-system:system/aaa/f5-system-aaa:primary-key",
	"/openconfig-system:system/aaa/f5-openconfig-aaa-tls:tls",

	// SNMPv2 MIB
	"/SNMPv2-MIB:SNMPv2-MIB/system",
}

// crawlEndpoints fetches each well-known endpoint and records its structure.
func (c *f5osClient) crawlEndpoints() []APIEndpoint {
	var results []APIEndpoint
	for _, path := range endpointsToScan {
		ep := c.crawlOne(path)
		results = append(results, ep)
	}
	return results
}

func (c *f5osClient) crawlOne(path string) APIEndpoint {
	ep := APIEndpoint{
		Path:       path,
		Properties: make(map[string]PropInfo),
	}

	body, status, err := c.get(path)
	ep.StatusCode = status
	if err != nil {
		log.Printf("  crawl %s: error %v", path, err)
		return ep
	}
	if status != 200 {
		return ep
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("  crawl %s: unmarshal error %v", path, err)
		return ep
	}

	for k := range data {
		ep.TopKeys = append(ep.TopKeys, k)
	}

	// flatten the entire response tree into properties
	flattenJSON("", data, ep.Properties)

	return ep
}

// flattenJSON recursively walks a JSON object and records every leaf and branch.
func flattenJSON(prefix string, data interface{}, props map[string]PropInfo) {
	switch v := data.(type) {
	case map[string]interface{}:
		for k, val := range v {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			props[path] = PropInfo{
				Type:     jsonType(val),
				Path:     path,
				HasValue: val != nil,
			}
			flattenJSON(path, val, props)
		}
	case []interface{}:
		// Record the array itself, then walk only the first element (if
		// any) to discover the shape of array items.
		//
		// NOTE: This assumes array items are homogeneous — every element
		// has the same set of keys. This is a safe assumption for
		// YANG-modeled RESTCONF data (list entries share the same schema
		// node), but would miss properties that only appear in later
		// elements if items were heterogeneous.
		if len(v) > 0 {
			flattenJSON(prefix+"[]", v[0], props)
		}
	}
}

func jsonType(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func strVal(m map[string]interface{}, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}
