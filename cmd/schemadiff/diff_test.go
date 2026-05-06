package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// diffModules
// ---------------------------------------------------------------------------

func TestDiffModules_AllNew(t *testing.T) {
	base := []YANGModule{}
	new := []YANGModule{
		{Name: "mod-a", Revision: "2025-01-01", Namespace: "urn:a"},
		{Name: "mod-b", Revision: "2025-06-01", Namespace: "urn:b"},
	}
	added, removed, updated := diffModules(base, new)

	if len(added) != 2 {
		t.Errorf("added: got %d, want 2", len(added))
	}
	if len(removed) != 0 {
		t.Errorf("removed: got %d, want 0", len(removed))
	}
	if len(updated) != 0 {
		t.Errorf("updated: got %d, want 0", len(updated))
	}
}

func TestDiffModules_AllRemoved(t *testing.T) {
	base := []YANGModule{
		{Name: "mod-a", Revision: "2025-01-01"},
	}
	new := []YANGModule{}
	added, removed, updated := diffModules(base, new)

	if len(added) != 0 {
		t.Errorf("added: got %d, want 0", len(added))
	}
	if len(removed) != 1 {
		t.Errorf("removed: got %d, want 1", len(removed))
	}
	if removed[0].Name != "mod-a" {
		t.Errorf("removed[0].Name: got %q, want %q", removed[0].Name, "mod-a")
	}
	if len(updated) != 0 {
		t.Errorf("updated: got %d, want 0", len(updated))
	}
}

func TestDiffModules_RevisionChange(t *testing.T) {
	base := []YANGModule{
		{Name: "mod-a", Revision: "2025-01-01", Namespace: "urn:a"},
	}
	new := []YANGModule{
		{Name: "mod-a", Revision: "2025-06-01", Namespace: "urn:a"},
	}
	added, removed, updated := diffModules(base, new)

	if len(added) != 0 || len(removed) != 0 {
		t.Errorf("expected no added/removed, got added=%d removed=%d", len(added), len(removed))
	}
	if len(updated) != 1 {
		t.Fatalf("updated: got %d, want 1", len(updated))
	}
	if updated[0].OldRevision != "2025-01-01" || updated[0].NewRevision != "2025-06-01" {
		t.Errorf("revision: got %q->%q, want 2025-01-01->2025-06-01",
			updated[0].OldRevision, updated[0].NewRevision)
	}
}

func TestDiffModules_NoChange(t *testing.T) {
	mods := []YANGModule{
		{Name: "mod-a", Revision: "2025-01-01"},
	}
	added, removed, updated := diffModules(mods, mods)

	if len(added) != 0 || len(removed) != 0 || len(updated) != 0 {
		t.Errorf("expected no changes, got added=%d removed=%d updated=%d",
			len(added), len(removed), len(updated))
	}
}

func TestDiffModules_Mixed(t *testing.T) {
	base := []YANGModule{
		{Name: "kept", Revision: "2025-01-01"},
		{Name: "updated", Revision: "2025-01-01"},
		{Name: "removed", Revision: "2025-01-01"},
	}
	new := []YANGModule{
		{Name: "kept", Revision: "2025-01-01"},
		{Name: "updated", Revision: "2025-06-01"},
		{Name: "added", Revision: "2025-06-01"},
	}
	added, removed, updated := diffModules(base, new)

	if len(added) != 1 || added[0].Name != "added" {
		t.Errorf("added: got %v", added)
	}
	if len(removed) != 1 || removed[0].Name != "removed" {
		t.Errorf("removed: got %v", removed)
	}
	if len(updated) != 1 || updated[0].Name != "updated" {
		t.Errorf("updated: got %v", updated)
	}
}

func TestDiffModules_SortedOutput(t *testing.T) {
	base := []YANGModule{}
	new := []YANGModule{
		{Name: "z-mod", Revision: "2025-01-01"},
		{Name: "a-mod", Revision: "2025-01-01"},
		{Name: "m-mod", Revision: "2025-01-01"},
	}
	added, _, _ := diffModules(base, new)
	if len(added) != 3 {
		t.Fatalf("expected 3, got %d", len(added))
	}
	if added[0].Name != "a-mod" || added[1].Name != "m-mod" || added[2].Name != "z-mod" {
		t.Errorf("not sorted: %s, %s, %s", added[0].Name, added[1].Name, added[2].Name)
	}
}

// ---------------------------------------------------------------------------
// diffEndpoint
// ---------------------------------------------------------------------------

func TestDiffEndpoint_AddedProperties(t *testing.T) {
	base := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"name": {Type: "string", Path: "name"},
		},
	}
	new := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"name":    {Type: "string", Path: "name"},
			"version": {Type: "string", Path: "version"},
		},
	}
	change := diffEndpoint("/test", base, new)

	if len(change.AddedProperties) != 1 || change.AddedProperties[0] != "version" {
		t.Errorf("added: got %v, want [version]", change.AddedProperties)
	}
	if len(change.RemovedProperties) != 0 {
		t.Errorf("removed: got %v, want empty", change.RemovedProperties)
	}
	if len(change.TypeChanges) != 0 {
		t.Errorf("type changes: got %v, want empty", change.TypeChanges)
	}
}

func TestDiffEndpoint_RemovedProperties(t *testing.T) {
	base := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"name":   {Type: "string", Path: "name"},
			"legacy": {Type: "string", Path: "legacy"},
		},
	}
	new := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"name": {Type: "string", Path: "name"},
		},
	}
	change := diffEndpoint("/test", base, new)

	if len(change.RemovedProperties) != 1 || change.RemovedProperties[0] != "legacy" {
		t.Errorf("removed: got %v, want [legacy]", change.RemovedProperties)
	}
}

func TestDiffEndpoint_TypeChange(t *testing.T) {
	base := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"count": {Type: "string", Path: "count"},
		},
	}
	new := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"count": {Type: "number", Path: "count"},
		},
	}
	change := diffEndpoint("/test", base, new)

	if len(change.TypeChanges) != 1 {
		t.Fatalf("type changes: got %d, want 1", len(change.TypeChanges))
	}
	tc := change.TypeChanges[0]
	if tc.OldType != "string" || tc.NewType != "number" {
		t.Errorf("type change: got %s->%s, want string->number", tc.OldType, tc.NewType)
	}
}

func TestDiffEndpoint_NoChanges(t *testing.T) {
	ep := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"name": {Type: "string", Path: "name"},
		},
	}
	change := diffEndpoint("/test", ep, ep)

	if len(change.AddedProperties) != 0 || len(change.RemovedProperties) != 0 || len(change.TypeChanges) != 0 {
		t.Error("expected no changes for identical endpoints")
	}
}

func TestDiffEndpoint_EmptyTypeIgnored(t *testing.T) {
	base := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"field": {Type: "", Path: "field"},
		},
	}
	new := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"field": {Type: "string", Path: "field"},
		},
	}
	change := diffEndpoint("/test", base, new)

	// Empty-to-something should not be flagged as a type change
	if len(change.TypeChanges) != 0 {
		t.Errorf("should ignore type change from empty string, got %v", change.TypeChanges)
	}
}

func TestDiffEndpoint_SortedOutput(t *testing.T) {
	base := APIEndpoint{
		Path:       "/test",
		Properties: map[string]PropInfo{},
	}
	new := APIEndpoint{
		Path: "/test",
		Properties: map[string]PropInfo{
			"z": {Type: "string", Path: "z"},
			"a": {Type: "string", Path: "a"},
			"m": {Type: "string", Path: "m"},
		},
	}
	change := diffEndpoint("/test", base, new)

	if len(change.AddedProperties) != 3 {
		t.Fatalf("expected 3, got %d", len(change.AddedProperties))
	}
	if change.AddedProperties[0] != "a" || change.AddedProperties[1] != "m" || change.AddedProperties[2] != "z" {
		t.Errorf("not sorted: %v", change.AddedProperties)
	}
}

// ---------------------------------------------------------------------------
// computeDiff — integration-level tests for the full diff pipeline
// ---------------------------------------------------------------------------

func TestComputeDiff_NewAPI(t *testing.T) {
	baseAPIs := []APIEndpoint{
		{Path: "/existing", StatusCode: 200, Properties: map[string]PropInfo{}},
	}
	newAPIs := []APIEndpoint{
		{Path: "/existing", StatusCode: 200, Properties: map[string]PropInfo{}},
		{Path: "/new-endpoint", StatusCode: 200, Properties: map[string]PropInfo{
			"field": {Type: "string", Path: "field"},
		}},
	}
	diff := computeDiff(nil, nil, baseAPIs, newAPIs)

	if len(diff.NewAPIs) != 1 {
		t.Fatalf("NewAPIs: got %d, want 1", len(diff.NewAPIs))
	}
	if diff.NewAPIs[0].Path != "/new-endpoint" {
		t.Errorf("NewAPIs[0].Path: got %q", diff.NewAPIs[0].Path)
	}
}

func TestComputeDiff_RemovedAPI(t *testing.T) {
	baseAPIs := []APIEndpoint{
		{Path: "/removed", StatusCode: 200, Properties: map[string]PropInfo{}},
	}
	newAPIs := []APIEndpoint{
		{Path: "/removed", StatusCode: 404, Properties: map[string]PropInfo{}},
	}
	diff := computeDiff(nil, nil, baseAPIs, newAPIs)

	if len(diff.RemovedAPIs) != 1 {
		t.Fatalf("RemovedAPIs: got %d, want 1", len(diff.RemovedAPIs))
	}
	if len(diff.BreakingChanges) != 1 {
		t.Fatalf("BreakingChanges: got %d, want 1", len(diff.BreakingChanges))
	}
	if diff.BreakingChanges[0].Category != "Removed API" {
		t.Errorf("category: got %q", diff.BreakingChanges[0].Category)
	}
}

func TestComputeDiff_RemovedModule(t *testing.T) {
	baseMods := []YANGModule{{Name: "old-mod", Revision: "2025-01-01"}}
	newMods := []YANGModule{}

	diff := computeDiff(baseMods, newMods, nil, nil)

	if len(diff.RemovedModules) != 1 {
		t.Fatalf("RemovedModules: got %d, want 1", len(diff.RemovedModules))
	}
	if len(diff.BreakingChanges) != 1 || diff.BreakingChanges[0].Category != "Removed Module" {
		t.Errorf("expected 1 breaking change (Removed Module), got %v", diff.BreakingChanges)
	}
}

func TestComputeDiff_PropertyChanges(t *testing.T) {
	baseAPIs := []APIEndpoint{
		{
			Path:       "/api",
			StatusCode: 200,
			Properties: map[string]PropInfo{
				"kept":      {Type: "string", Path: "kept"},
				"removed":   {Type: "string", Path: "removed"},
				"retyped":   {Type: "string", Path: "retyped"},
			},
		},
	}
	newAPIs := []APIEndpoint{
		{
			Path:       "/api",
			StatusCode: 200,
			Properties: map[string]PropInfo{
				"kept":    {Type: "string", Path: "kept"},
				"added":   {Type: "number", Path: "added"},
				"retyped": {Type: "number", Path: "retyped"},
			},
		},
	}
	diff := computeDiff(nil, nil, baseAPIs, newAPIs)

	if len(diff.APIChanges) != 1 {
		t.Fatalf("APIChanges: got %d, want 1", len(diff.APIChanges))
	}
	ac := diff.APIChanges[0]
	if len(ac.AddedProperties) != 1 || ac.AddedProperties[0] != "added" {
		t.Errorf("added: got %v", ac.AddedProperties)
	}
	if len(ac.RemovedProperties) != 1 || ac.RemovedProperties[0] != "removed" {
		t.Errorf("removed: got %v", ac.RemovedProperties)
	}
	if len(ac.TypeChanges) != 1 || ac.TypeChanges[0].PropertyPath != "retyped" {
		t.Errorf("type changes: got %v", ac.TypeChanges)
	}

	// Should produce breaking changes for removed property and type change
	breakingCount := 0
	for _, bc := range diff.BreakingChanges {
		if bc.Category == "Removed Property" || bc.Category == "Type Change" {
			breakingCount++
		}
	}
	if breakingCount != 2 {
		t.Errorf("expected 2 breaking changes (removed + type), got %d", breakingCount)
	}

	// Should produce new properties entry
	if len(diff.NewProperties) != 1 || diff.NewProperties[0].PropertyPath != "added" {
		t.Errorf("NewProperties: got %v", diff.NewProperties)
	}
}

func TestComputeDiff_Non200Ignored(t *testing.T) {
	baseAPIs := []APIEndpoint{
		{Path: "/a", StatusCode: 404, Properties: map[string]PropInfo{}},
	}
	newAPIs := []APIEndpoint{
		{Path: "/a", StatusCode: 404, Properties: map[string]PropInfo{}},
	}
	diff := computeDiff(nil, nil, baseAPIs, newAPIs)

	if len(diff.NewAPIs) != 0 || len(diff.RemovedAPIs) != 0 || len(diff.APIChanges) != 0 {
		t.Error("non-200 endpoints should be ignored")
	}
}

func TestComputeDiff_EmptyInputs(t *testing.T) {
	diff := computeDiff(nil, nil, nil, nil)

	if len(diff.BreakingChanges) != 0 || len(diff.NewAPIs) != 0 ||
		len(diff.RemovedAPIs) != 0 || len(diff.NewProperties) != 0 {
		t.Error("empty inputs should produce empty diff")
	}
}

func TestComputeDiff_FullScenario(t *testing.T) {
	baseMods := []YANGModule{
		{Name: "mod-kept", Revision: "2025-01-01"},
		{Name: "mod-updated", Revision: "2025-01-01"},
		{Name: "mod-removed", Revision: "2025-01-01"},
	}
	newMods := []YANGModule{
		{Name: "mod-kept", Revision: "2025-01-01"},
		{Name: "mod-updated", Revision: "2025-06-01"},
		{Name: "mod-added", Revision: "2025-06-01"},
	}
	baseAPIs := []APIEndpoint{
		{Path: "/stable", StatusCode: 200, Properties: map[string]PropInfo{
			"field": {Type: "string", Path: "field"},
		}},
		{Path: "/gone", StatusCode: 200, Properties: map[string]PropInfo{}},
	}
	newAPIs := []APIEndpoint{
		{Path: "/stable", StatusCode: 200, Properties: map[string]PropInfo{
			"field":     {Type: "string", Path: "field"},
			"new_field": {Type: "bool", Path: "new_field"},
		}},
		{Path: "/gone", StatusCode: 404, Properties: map[string]PropInfo{}},
		{Path: "/brand-new", StatusCode: 200, Properties: map[string]PropInfo{}},
	}
	diff := computeDiff(baseMods, newMods, baseAPIs, newAPIs)

	if len(diff.NewModules) != 1 {
		t.Errorf("NewModules: got %d, want 1", len(diff.NewModules))
	}
	if len(diff.RemovedModules) != 1 {
		t.Errorf("RemovedModules: got %d, want 1", len(diff.RemovedModules))
	}
	if len(diff.UpdatedModules) != 1 {
		t.Errorf("UpdatedModules: got %d, want 1", len(diff.UpdatedModules))
	}
	if len(diff.NewAPIs) != 1 {
		t.Errorf("NewAPIs: got %d, want 1", len(diff.NewAPIs))
	}
	if len(diff.RemovedAPIs) != 1 {
		t.Errorf("RemovedAPIs: got %d, want 1", len(diff.RemovedAPIs))
	}
	if len(diff.NewProperties) != 1 {
		t.Errorf("NewProperties: got %d, want 1", len(diff.NewProperties))
	}
	// Breaking: 1 removed module + 1 removed API = 2
	if len(diff.BreakingChanges) != 2 {
		t.Errorf("BreakingChanges: got %d, want 2", len(diff.BreakingChanges))
	}
}
