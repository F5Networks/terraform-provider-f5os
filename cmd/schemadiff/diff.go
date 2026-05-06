package main

import "sort"

// ── Diff result structures ─────────────────────────────────────────────

// DiffResult holds the complete comparison between two F5OS versions.
type DiffResult struct {
	// Module-level changes (from YANG library)
	NewModules     []YANGModule       // modules present only in the new version
	RemovedModules []YANGModule       // modules present only in the base version
	UpdatedModules []ModuleUpdate     // modules with revision changes

	// API-level changes (from endpoint crawling)
	NewAPIs        []APIEndpoint      // endpoints reachable in new but 404/missing in base
	RemovedAPIs    []APIEndpoint      // endpoints reachable in base but 404/missing in new
	APIChanges     []APIChange        // endpoints present in both with structural differences

	// Aggregated lists for the report
	BreakingChanges []BreakingChange  // anything that would break existing consumers
	NewProperties   []NewProperty     // new fields added to existing APIs
}

// ModuleUpdate records a YANG module whose revision changed between versions.
type ModuleUpdate struct {
	Name        string
	Namespace   string
	OldRevision string
	NewRevision string
}

// APIChange records structural differences within a single API endpoint.
type APIChange struct {
	Path              string
	AddedProperties   []string // property paths present in new but not base
	RemovedProperties []string // property paths present in base but not new
	TypeChanges       []TypeChange
}

// TypeChange records a property whose JSON type changed between versions.
type TypeChange struct {
	PropertyPath string
	OldType      string
	NewType      string
}

// BreakingChange is a human-readable record of something that could break consumers.
type BreakingChange struct {
	Category    string // "Removed Module", "Removed API", "Removed Property", "Type Change"
	Path        string // affected API or module
	Description string
}

// NewProperty records a property added to an existing API in the new version.
type NewProperty struct {
	APIPath      string
	PropertyPath string
	Type         string
}

// ── Comparison logic ───────────────────────────────────────────────────

func computeDiff(baseModules, newModules []YANGModule, baseAPIs, newAPIs []APIEndpoint) DiffResult {
	var diff DiffResult

	// ── YANG module comparison ─────────────────────────────────────────
	diff.NewModules, diff.RemovedModules, diff.UpdatedModules = diffModules(baseModules, newModules)

	// removed modules are breaking
	for _, m := range diff.RemovedModules {
		diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
			Category:    "Removed Module",
			Path:        m.Name,
			Description: "YANG module \"" + m.Name + "\" (revision " + m.Revision + ") was removed",
		})
	}

	// ── API endpoint comparison ────────────────────────────────────────
	baseByPath := indexAPIs(baseAPIs)
	newByPath := indexAPIs(newAPIs)

	// new APIs (reachable in new, absent/404 in base)
	for path, nep := range newByPath {
		if nep.StatusCode != 200 {
			continue
		}
		bep, exists := baseByPath[path]
		if !exists || bep.StatusCode != 200 {
			diff.NewAPIs = append(diff.NewAPIs, nep)
		}
	}

	// removed APIs (reachable in base, absent/404 in new)
	for path, bep := range baseByPath {
		if bep.StatusCode != 200 {
			continue
		}
		nep, exists := newByPath[path]
		if !exists || nep.StatusCode != 200 {
			diff.RemovedAPIs = append(diff.RemovedAPIs, bep)
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Category:    "Removed API",
				Path:        path,
				Description: "API endpoint \"" + path + "\" is no longer reachable (was HTTP 200, now missing or non-200)",
			})
		}
	}

	// structural diff for endpoints present in both
	for path, bep := range baseByPath {
		if bep.StatusCode != 200 {
			continue
		}
		nep, exists := newByPath[path]
		if !exists || nep.StatusCode != 200 {
			continue
		}

		change := diffEndpoint(path, bep, nep)
		if len(change.AddedProperties) > 0 || len(change.RemovedProperties) > 0 || len(change.TypeChanges) > 0 {
			diff.APIChanges = append(diff.APIChanges, change)
		}

		// removed properties are breaking
		for _, prop := range change.RemovedProperties {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Category:    "Removed Property",
				Path:        path,
				Description: "Property \"" + prop + "\" was removed from endpoint \"" + path + "\"",
			})
		}

		// type changes are breaking
		for _, tc := range change.TypeChanges {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Category:    "Type Change",
				Path:        path,
				Description: "Property \"" + tc.PropertyPath + "\" changed type from " + tc.OldType + " to " + tc.NewType + " in \"" + path + "\"",
			})
		}

		// added properties -> new properties
		for _, prop := range change.AddedProperties {
			pType := "unknown"
			if pi, ok := nep.Properties[prop]; ok {
				pType = pi.Type
			}
			diff.NewProperties = append(diff.NewProperties, NewProperty{
				APIPath:      path,
				PropertyPath: prop,
				Type:         pType,
			})
		}
	}

	// sort everything for deterministic output
	sortBreakingChanges(diff.BreakingChanges)
	sortNewAPIs(diff.NewAPIs)
	sortNewProperties(diff.NewProperties)
	sortAPIChanges(diff.APIChanges)

	return diff
}

// ── Module diffing ─────────────────────────────────────────────────────

func diffModules(base, new []YANGModule) (added, removed []YANGModule, updated []ModuleUpdate) {
	baseMap := make(map[string]YANGModule)
	for _, m := range base {
		baseMap[m.Name] = m
	}
	newMap := make(map[string]YANGModule)
	for _, m := range new {
		newMap[m.Name] = m
	}

	for name, nm := range newMap {
		bm, exists := baseMap[name]
		if !exists {
			added = append(added, nm)
		} else if bm.Revision != nm.Revision {
			updated = append(updated, ModuleUpdate{
				Name:        name,
				Namespace:   nm.Namespace,
				OldRevision: bm.Revision,
				NewRevision: nm.Revision,
			})
		}
	}
	for name, bm := range baseMap {
		if _, exists := newMap[name]; !exists {
			removed = append(removed, bm)
		}
	}

	sort.Slice(added, func(i, j int) bool { return added[i].Name < added[j].Name })
	sort.Slice(removed, func(i, j int) bool { return removed[i].Name < removed[j].Name })
	sort.Slice(updated, func(i, j int) bool { return updated[i].Name < updated[j].Name })
	return
}

// ── Endpoint structural diff ───────────────────────────────────────────

func diffEndpoint(path string, base, new APIEndpoint) APIChange {
	change := APIChange{Path: path}

	for prop, npi := range new.Properties {
		bpi, exists := base.Properties[prop]
		if !exists {
			change.AddedProperties = append(change.AddedProperties, prop)
		} else if bpi.Type != npi.Type && bpi.Type != "" && npi.Type != "" {
			change.TypeChanges = append(change.TypeChanges, TypeChange{
				PropertyPath: prop,
				OldType:      bpi.Type,
				NewType:      npi.Type,
			})
		}
	}

	for prop := range base.Properties {
		if _, exists := new.Properties[prop]; !exists {
			change.RemovedProperties = append(change.RemovedProperties, prop)
		}
	}

	sort.Strings(change.AddedProperties)
	sort.Strings(change.RemovedProperties)
	sort.Slice(change.TypeChanges, func(i, j int) bool {
		return change.TypeChanges[i].PropertyPath < change.TypeChanges[j].PropertyPath
	})

	return change
}

// ── Helpers ────────────────────────────────────────────────────────────

func indexAPIs(apis []APIEndpoint) map[string]APIEndpoint {
	m := make(map[string]APIEndpoint, len(apis))
	for _, ep := range apis {
		m[ep.Path] = ep
	}
	return m
}

func sortBreakingChanges(s []BreakingChange) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Category != s[j].Category {
			return s[i].Category < s[j].Category
		}
		return s[i].Path < s[j].Path
	})
}

func sortNewAPIs(s []APIEndpoint) {
	sort.Slice(s, func(i, j int) bool { return s[i].Path < s[j].Path })
}

func sortNewProperties(s []NewProperty) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].APIPath != s[j].APIPath {
			return s[i].APIPath < s[j].APIPath
		}
		return s[i].PropertyPath < s[j].PropertyPath
	})
}

func sortAPIChanges(s []APIChange) {
	sort.Slice(s, func(i, j int) bool { return s[i].Path < s[j].Path })
}
