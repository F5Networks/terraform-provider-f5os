package main

import (
	"reflect"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// flattenJSON
// ---------------------------------------------------------------------------

func TestFlattenJSON_SimpleObject(t *testing.T) {
	data := map[string]interface{}{
		"name":    "test",
		"enabled": true,
		"count":   float64(42),
	}
	props := make(map[string]PropInfo)
	flattenJSON("", data, props)

	want := map[string]string{
		"name":    "string",
		"enabled": "bool",
		"count":   "number",
	}
	for path, wantType := range want {
		pi, ok := props[path]
		if !ok {
			t.Errorf("missing property %q", path)
			continue
		}
		if pi.Type != wantType {
			t.Errorf("property %q: got type %q, want %q", path, pi.Type, wantType)
		}
		if !pi.HasValue {
			t.Errorf("property %q: HasValue should be true", path)
		}
	}
}

func TestFlattenJSON_NestedObject(t *testing.T) {
	data := map[string]interface{}{
		"config": map[string]interface{}{
			"hostname": "device1",
		},
	}
	props := make(map[string]PropInfo)
	flattenJSON("", data, props)

	if _, ok := props["config"]; !ok {
		t.Error("missing top-level 'config' property")
	}
	pi, ok := props["config.hostname"]
	if !ok {
		t.Fatal("missing nested 'config.hostname' property")
	}
	if pi.Type != "string" {
		t.Errorf("config.hostname type: got %q, want %q", pi.Type, "string")
	}
}

func TestFlattenJSON_ArrayWalksFirstElement(t *testing.T) {
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"name": "admin",
				"role": "superuser",
			},
			map[string]interface{}{
				"name": "operator",
				"role": "viewer",
			},
		},
	}
	props := make(map[string]PropInfo)
	flattenJSON("", data, props)

	// The array itself should be recorded
	pi, ok := props["users"]
	if !ok {
		t.Fatal("missing 'users' property")
	}
	if pi.Type != "array" {
		t.Errorf("users type: got %q, want %q", pi.Type, "array")
	}

	// First element's keys should be flattened with [] notation
	if _, ok := props["users[].name"]; !ok {
		t.Error("missing 'users[].name' — first element keys should be walked")
	}
	if _, ok := props["users[].role"]; !ok {
		t.Error("missing 'users[].role' — first element keys should be walked")
	}
}

func TestFlattenJSON_EmptyArray(t *testing.T) {
	data := map[string]interface{}{
		"items": []interface{}{},
	}
	props := make(map[string]PropInfo)
	flattenJSON("", data, props)

	// The array is recorded at the parent level but nothing inside
	if _, ok := props["items"]; !ok {
		t.Error("missing 'items' property")
	}
	// No items[].xxx keys should exist
	for path := range props {
		if path != "items" {
			t.Errorf("unexpected property %q from empty array", path)
		}
	}
}

func TestFlattenJSON_NullValue(t *testing.T) {
	data := map[string]interface{}{
		"value": nil,
	}
	props := make(map[string]PropInfo)
	flattenJSON("", data, props)

	pi, ok := props["value"]
	if !ok {
		t.Fatal("missing 'value' property")
	}
	if pi.Type != "null" {
		t.Errorf("type: got %q, want %q", pi.Type, "null")
	}
	if pi.HasValue {
		t.Error("HasValue should be false for nil")
	}
}

func TestFlattenJSON_WithPrefix(t *testing.T) {
	data := map[string]interface{}{
		"key": "val",
	}
	props := make(map[string]PropInfo)
	flattenJSON("root", data, props)

	if _, ok := props["root.key"]; !ok {
		t.Error("expected property 'root.key' when prefix is set")
	}
}

func TestFlattenJSON_DeeplyNested(t *testing.T) {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": map[string]interface{}{
					"leaf": "deep",
				},
			},
		},
	}
	props := make(map[string]PropInfo)
	flattenJSON("", data, props)

	if _, ok := props["a.b.c.leaf"]; !ok {
		t.Error("missing deeply nested property 'a.b.c.leaf'")
	}
}

// ---------------------------------------------------------------------------
// jsonType
// ---------------------------------------------------------------------------

func TestJsonType(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{"hello", "string"},
		{float64(1.5), "number"},
		{true, "bool"},
		{false, "bool"},
		{[]interface{}{}, "array"},
		{map[string]interface{}{}, "object"},
		{nil, "null"},
	}
	for _, tt := range tests {
		got := jsonType(tt.input)
		if got != tt.want {
			t.Errorf("jsonType(%v): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// strVal
// ---------------------------------------------------------------------------

func TestStrVal(t *testing.T) {
	m := map[string]interface{}{
		"name":  "test",
		"count": float64(5),
	}

	if got := strVal(m, "name"); got != "test" {
		t.Errorf("strVal(name): got %q, want %q", got, "test")
	}
	if got := strVal(m, "count"); got != "" {
		t.Errorf("strVal(count): got %q, want empty (not a string)", got)
	}
	if got := strVal(m, "missing"); got != "" {
		t.Errorf("strVal(missing): got %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// sortedProps
// ---------------------------------------------------------------------------

func TestSortedProps(t *testing.T) {
	props := map[string]PropInfo{
		"z": {Path: "z", Type: "string"},
		"a": {Path: "a", Type: "number"},
		"m": {Path: "m", Type: "bool"},
	}
	sorted := sortedProps(props)
	if len(sorted) != 3 {
		t.Fatalf("expected 3 props, got %d", len(sorted))
	}
	if sorted[0].Path != "a" || sorted[1].Path != "m" || sorted[2].Path != "z" {
		t.Errorf("unexpected order: %v", sorted)
	}
}

// ---------------------------------------------------------------------------
// indexAPIs
// ---------------------------------------------------------------------------

func TestIndexAPIs(t *testing.T) {
	apis := []APIEndpoint{
		{Path: "/a", StatusCode: 200},
		{Path: "/b", StatusCode: 404},
	}
	idx := indexAPIs(apis)
	if len(idx) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(idx))
	}
	if idx["/a"].StatusCode != 200 {
		t.Error("/a should have status 200")
	}
	if idx["/b"].StatusCode != 404 {
		t.Error("/b should have status 404")
	}
}

// ---------------------------------------------------------------------------
// envOr
// ---------------------------------------------------------------------------

func TestEnvOr_Fallback(t *testing.T) {
	// Use a key that is extremely unlikely to be set
	got := envOr("SCHEMADIFF_TEST_UNSET_KEY_12345", "default")
	if got != "default" {
		t.Errorf("expected fallback 'default', got %q", got)
	}
}

func TestEnvOr_EnvSet(t *testing.T) {
	t.Setenv("SCHEMADIFF_TEST_KEY", "fromenv")
	got := envOr("SCHEMADIFF_TEST_KEY", "fallback")
	if got != "fromenv" {
		t.Errorf("expected 'fromenv', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// sort helpers (ensure deterministic output)
// ---------------------------------------------------------------------------

func TestSortBreakingChanges(t *testing.T) {
	changes := []BreakingChange{
		{Category: "Type Change", Path: "/z"},
		{Category: "Removed API", Path: "/a"},
		{Category: "Removed API", Path: "/b"},
		{Category: "Removed Module", Path: "m1"},
	}
	sortBreakingChanges(changes)

	// should sort by category first, then path
	if changes[0].Category != "Removed API" || changes[0].Path != "/a" {
		t.Errorf("index 0: got %s %s", changes[0].Category, changes[0].Path)
	}
	if changes[1].Category != "Removed API" || changes[1].Path != "/b" {
		t.Errorf("index 1: got %s %s", changes[1].Category, changes[1].Path)
	}
	if changes[2].Category != "Removed Module" {
		t.Errorf("index 2: got category %s", changes[2].Category)
	}
	if changes[3].Category != "Type Change" {
		t.Errorf("index 3: got category %s", changes[3].Category)
	}
}

func TestSortNewAPIs(t *testing.T) {
	apis := []APIEndpoint{
		{Path: "/z"},
		{Path: "/a"},
		{Path: "/m"},
	}
	sortNewAPIs(apis)
	if apis[0].Path != "/a" || apis[1].Path != "/m" || apis[2].Path != "/z" {
		t.Errorf("unexpected sort order: %v", apis)
	}
}

func TestSortNewProperties(t *testing.T) {
	props := []NewProperty{
		{APIPath: "/b", PropertyPath: "z"},
		{APIPath: "/a", PropertyPath: "y"},
		{APIPath: "/a", PropertyPath: "x"},
	}
	sortNewProperties(props)
	if props[0].APIPath != "/a" || props[0].PropertyPath != "x" {
		t.Errorf("index 0: got %s %s", props[0].APIPath, props[0].PropertyPath)
	}
	if props[1].APIPath != "/a" || props[1].PropertyPath != "y" {
		t.Errorf("index 1: got %s %s", props[1].APIPath, props[1].PropertyPath)
	}
	if props[2].APIPath != "/b" {
		t.Errorf("index 2: got %s", props[2].APIPath)
	}
}

func TestSortAPIChanges(t *testing.T) {
	changes := []APIChange{
		{Path: "/z"},
		{Path: "/a"},
	}
	sortAPIChanges(changes)
	if changes[0].Path != "/a" || changes[1].Path != "/z" {
		paths := []string{changes[0].Path, changes[1].Path}
		t.Errorf("unexpected sort order: %v", paths)
	}
}

// ---------------------------------------------------------------------------
// sortedKeys
// ---------------------------------------------------------------------------

func TestSortedKeys(t *testing.T) {
	m := map[string][]NewProperty{
		"/z": nil,
		"/a": nil,
		"/m": nil,
	}
	got := sortedKeys(m)
	want := []string{"/a", "/m", "/z"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
