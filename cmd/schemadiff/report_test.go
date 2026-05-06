package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// generateReport — structure and content verification
// ---------------------------------------------------------------------------

func TestGenerateReport_EmptyDiff(t *testing.T) {
	report := generateReport("r5900", "1.5.4", "r12800", "1.8.3", DiffResult{})

	mustContain := []string{
		"# F5OS Schema Diff Report",
		"| **Platform** | r5900 | r12800 |",
		"| **Version** | 1.5.4 | 1.8.3 |",
		"| Breaking changes | **0** |",
		"No breaking changes detected.",
		"No new API endpoints detected.",
		"No new properties detected",
		"No YANG module changes detected.",
		"No structural changes detected",
	}
	for _, s := range mustContain {
		if !strings.Contains(report, s) {
			t.Errorf("report missing expected string: %q", s)
		}
	}
}

func TestGenerateReport_BreakingChanges(t *testing.T) {
	diff := DiffResult{
		BreakingChanges: []BreakingChange{
			{Category: "Removed API", Path: "/old", Description: "API /old was removed"},
			{Category: "Removed Property", Path: "/api", Description: "Property gone from /api"},
			{Category: "Type Change", Path: "/api", Description: "field changed string to number"},
		},
	}
	report := generateReport("r5900", "1.5.4", "r12800", "1.8.3", diff)

	if !strings.Contains(report, "| Breaking changes | **3** |") {
		t.Error("report should show 3 breaking changes in summary")
	}
	if !strings.Contains(report, "**WARNING:**") {
		t.Error("report should contain WARNING banner for breaking changes")
	}
	if !strings.Contains(report, "### Removed APIs (1)") {
		t.Error("report should group Removed API changes")
	}
	if !strings.Contains(report, "### Removed Propertys (1)") {
		t.Error("report should group Removed Property changes")
	}
	if !strings.Contains(report, "### Type Changes (1)") {
		t.Error("report should group Type Change changes")
	}
}

func TestGenerateReport_NewAPIs(t *testing.T) {
	diff := DiffResult{
		NewAPIs: []APIEndpoint{
			{
				Path:     "/new-api",
				TopKeys:  []string{"key1", "key2"},
				Properties: map[string]PropInfo{
					"key1": {Type: "string", Path: "key1"},
				},
			},
		},
	}
	report := generateReport("r5900", "1.5.4", "r12800", "1.8.3", diff)

	if !strings.Contains(report, "| New APIs (endpoints) | 1 |") {
		t.Error("summary should show 1 new API")
	}
	if !strings.Contains(report, "`/new-api`") {
		t.Error("report should contain the new API path")
	}
	if !strings.Contains(report, "key1, key2") {
		t.Error("report should list top-level keys")
	}
}

func TestGenerateReport_NewProperties(t *testing.T) {
	diff := DiffResult{
		NewProperties: []NewProperty{
			{APIPath: "/api", PropertyPath: "new_field", Type: "bool"},
		},
	}
	report := generateReport("r5900", "1.5.4", "r12800", "1.8.3", diff)

	if !strings.Contains(report, "| New properties on existing APIs | 1 |") {
		t.Error("summary should show 1 new property")
	}
	if !strings.Contains(report, "`/api` (1 new properties)") {
		t.Error("report should group properties by API path")
	}
	if !strings.Contains(report, "| `new_field` | bool |") {
		t.Error("report should list the new property")
	}
}

func TestGenerateReport_YANGModuleChanges(t *testing.T) {
	diff := DiffResult{
		NewModules:     []YANGModule{{Name: "new-mod", Revision: "2026-01-01", Namespace: "urn:new"}},
		RemovedModules: []YANGModule{{Name: "old-mod", Revision: "2025-01-01", Namespace: "urn:old"}},
		UpdatedModules: []ModuleUpdate{{
			Name: "upd-mod", OldRevision: "2025-01-01", NewRevision: "2026-01-01", Namespace: "urn:upd",
		}},
		// Removed module should also appear in BreakingChanges
		BreakingChanges: []BreakingChange{
			{Category: "Removed Module", Path: "old-mod", Description: "old-mod removed"},
		},
	}
	report := generateReport("r5900", "1.5.4", "r12800", "1.8.3", diff)

	if !strings.Contains(report, "### New Modules (1)") {
		t.Error("report should have New Modules section")
	}
	if !strings.Contains(report, "| `new-mod` | 2026-01-01 | urn:new |") {
		t.Error("report should list new module")
	}
	if !strings.Contains(report, "### Removed Modules (1)") {
		t.Error("report should have Removed Modules section")
	}
	if !strings.Contains(report, "### Updated Modules (1)") {
		t.Error("report should have Updated Modules section")
	}
	if !strings.Contains(report, "| `upd-mod` | 2025-01-01 | 2026-01-01 | urn:upd |") {
		t.Error("report should list updated module with old and new revisions")
	}
}

func TestGenerateReport_DetailedAPIChanges(t *testing.T) {
	diff := DiffResult{
		APIChanges: []APIChange{
			{
				Path:              "/api",
				AddedProperties:   []string{"new_prop"},
				RemovedProperties: []string{"old_prop"},
				TypeChanges: []TypeChange{
					{PropertyPath: "count", OldType: "string", NewType: "number"},
				},
			},
		},
		BreakingChanges: []BreakingChange{
			{Category: "Removed Property", Path: "/api", Description: "old_prop removed"},
			{Category: "Type Change", Path: "/api", Description: "count changed type"},
		},
	}
	report := generateReport("r5900", "1.5.4", "r12800", "1.8.3", diff)

	if !strings.Contains(report, "## Detailed API Changes") {
		t.Error("report should have Detailed API Changes section")
	}
	if !strings.Contains(report, "**Added properties** (1)") {
		t.Error("should list added properties")
	}
	if !strings.Contains(report, "- `new_prop`") {
		t.Error("should list new_prop")
	}
	if !strings.Contains(report, "**Removed properties** (1)") {
		t.Error("should list removed properties")
	}
	if !strings.Contains(report, "- `old_prop` (**BREAKING**)") {
		t.Error("removed property should be marked BREAKING")
	}
	if !strings.Contains(report, "**Type changes** (1)") {
		t.Error("should list type changes")
	}
	if !strings.Contains(report, "- `count`: string -> number (**BREAKING**)") {
		t.Error("type change should show old->new and BREAKING")
	}
}

func TestGenerateReport_EmptyTopKeysShowsPlaceholder(t *testing.T) {
	diff := DiffResult{
		NewAPIs: []APIEndpoint{
			{Path: "/empty", TopKeys: nil, Properties: map[string]PropInfo{}},
		},
	}
	report := generateReport("a", "1", "b", "2", diff)

	if !strings.Contains(report, "(empty)") {
		t.Error("empty top keys should show (empty) placeholder")
	}
}

// ---------------------------------------------------------------------------
// groupBreakingChanges
// ---------------------------------------------------------------------------

func TestGroupBreakingChanges(t *testing.T) {
	changes := []BreakingChange{
		{Category: "Removed API", Path: "/a"},
		{Category: "Removed API", Path: "/b"},
		{Category: "Type Change", Path: "/c"},
	}
	grouped := groupBreakingChanges(changes)

	if len(grouped["Removed API"]) != 2 {
		t.Errorf("Removed API: got %d, want 2", len(grouped["Removed API"]))
	}
	if len(grouped["Type Change"]) != 1 {
		t.Errorf("Type Change: got %d, want 1", len(grouped["Type Change"]))
	}
	if len(grouped["Removed Module"]) != 0 {
		t.Error("Removed Module should be empty")
	}
}

// ---------------------------------------------------------------------------
// groupNewProperties
// ---------------------------------------------------------------------------

func TestGroupNewProperties(t *testing.T) {
	props := []NewProperty{
		{APIPath: "/a", PropertyPath: "x"},
		{APIPath: "/a", PropertyPath: "y"},
		{APIPath: "/b", PropertyPath: "z"},
	}
	grouped := groupNewProperties(props)

	if len(grouped["/a"]) != 2 {
		t.Errorf("/a: got %d, want 2", len(grouped["/a"]))
	}
	if len(grouped["/b"]) != 1 {
		t.Errorf("/b: got %d, want 1", len(grouped["/b"]))
	}
}
