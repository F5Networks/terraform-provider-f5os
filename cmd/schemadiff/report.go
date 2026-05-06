package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// generateReport produces a Markdown report from the diff result.
func generateReport(
	basePlatform, baseVersion string,
	newPlatform, newVersion string,
	diff DiffResult,
) string {
	var b strings.Builder

	// ── Header ─────────────────────────────────────────────────────────
	b.WriteString("# F5OS Schema Diff Report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))
	b.WriteString("| | Base | New |\n")
	b.WriteString("|---|---|---|\n")
	fmt.Fprintf(&b, "| **Platform** | %s | %s |\n", basePlatform, newPlatform)
	fmt.Fprintf(&b, "| **Version** | %s | %s |\n\n", baseVersion, newVersion)

	// ── Executive summary ──────────────────────────────────────────────
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Count |\n")
	b.WriteString("| --- | --- |\n")
	fmt.Fprintf(&b, "| Breaking changes | **%d** |\n", len(diff.BreakingChanges))
	fmt.Fprintf(&b, "| New APIs (endpoints) | %d |\n", len(diff.NewAPIs))
	fmt.Fprintf(&b, "| New properties on existing APIs | %d |\n", len(diff.NewProperties))
	fmt.Fprintf(&b, "| New YANG modules | %d |\n", len(diff.NewModules))
	fmt.Fprintf(&b, "| Removed YANG modules | %d |\n", len(diff.RemovedModules))
	fmt.Fprintf(&b, "| Updated YANG modules (revision change) | %d |\n", len(diff.UpdatedModules))
	fmt.Fprintf(&b, "| APIs with structural changes | %d |\n\n", len(diff.APIChanges))

	// ── Breaking changes ───────────────────────────────────────────────
	b.WriteString("---\n\n")
	b.WriteString("## Breaking Changes\n\n")
	if len(diff.BreakingChanges) == 0 {
		b.WriteString("No breaking changes detected.\n\n")
	} else {
		b.WriteString("> **WARNING:** The following changes may break existing Terraform configurations or API consumers.\n\n")
		// group by category
		grouped := groupBreakingChanges(diff.BreakingChanges)
		for _, cat := range []string{"Removed Module", "Removed API", "Removed Property", "Type Change"} {
			items, ok := grouped[cat]
			if !ok || len(items) == 0 {
				continue
			}
			fmt.Fprintf(&b, "### %ss (%d)\n\n", cat, len(items))
			for _, bc := range items {
				fmt.Fprintf(&b, "- `%s`: %s\n", bc.Path, bc.Description)
			}
			b.WriteString("\n")
		}
	}

	// ── New APIs ───────────────────────────────────────────────────────
	b.WriteString("---\n\n")
	b.WriteString("## New APIs\n\n")
	if len(diff.NewAPIs) == 0 {
		b.WriteString("No new API endpoints detected.\n\n")
	} else {
		b.WriteString("The following RESTCONF endpoints are available in the new version but were not reachable in the base version:\n\n")
		b.WriteString("| Endpoint | Top-Level Keys |\n")
		b.WriteString("| --- | --- |\n")
		for _, api := range diff.NewAPIs {
			keys := strings.Join(api.TopKeys, ", ")
			if keys == "" {
				keys = "(empty)"
			}
			fmt.Fprintf(&b, "| `%s` | %s |\n", api.Path, keys)
		}
		b.WriteString("\n")

		// detail: list properties for each new API
		for _, api := range diff.NewAPIs {
			if len(api.Properties) == 0 {
				continue
			}
			fmt.Fprintf(&b, "### `%s`\n\n", api.Path)
			fmt.Fprintf(&b, "<details>\n<summary>Properties (%d)</summary>\n\n", len(api.Properties))
			b.WriteString("| Property Path | Type |\n")
			b.WriteString("| --- | --- |\n")
			for _, prop := range sortedProps(api.Properties) {
				fmt.Fprintf(&b, "| `%s` | %s |\n", prop.Path, prop.Type)
			}
			b.WriteString("\n</details>\n\n")
		}
	}

	// ── New Properties on Existing APIs ────────────────────────────────
	b.WriteString("---\n\n")
	b.WriteString("## New Properties\n\n")
	if len(diff.NewProperties) == 0 {
		b.WriteString("No new properties detected on existing API endpoints.\n\n")
	} else {
		b.WriteString("The following properties were added to existing API endpoints in the new version:\n\n")
		// group by API path
		grouped := groupNewProperties(diff.NewProperties)
		apiPaths := sortedKeys(grouped)
		for _, apiPath := range apiPaths {
			props := grouped[apiPath]
			fmt.Fprintf(&b, "### `%s` (%d new properties)\n\n", apiPath, len(props))
			b.WriteString("| Property Path | Type |\n")
			b.WriteString("| --- | --- |\n")
			for _, np := range props {
				fmt.Fprintf(&b, "| `%s` | %s |\n", np.PropertyPath, np.Type)
			}
			b.WriteString("\n")
		}
	}

	// ── YANG Module Changes ────────────────────────────────────────────
	b.WriteString("---\n\n")
	b.WriteString("## YANG Module Changes\n\n")

	if len(diff.NewModules) > 0 {
		fmt.Fprintf(&b, "### New Modules (%d)\n\n", len(diff.NewModules))
		b.WriteString("| Module | Revision | Namespace |\n")
		b.WriteString("| --- | --- | --- |\n")
		for _, m := range diff.NewModules {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", m.Name, m.Revision, m.Namespace)
		}
		b.WriteString("\n")
	}

	if len(diff.RemovedModules) > 0 {
		fmt.Fprintf(&b, "### Removed Modules (%d)\n\n", len(diff.RemovedModules))
		b.WriteString("| Module | Revision | Namespace |\n")
		b.WriteString("| --- | --- | --- |\n")
		for _, m := range diff.RemovedModules {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", m.Name, m.Revision, m.Namespace)
		}
		b.WriteString("\n")
	}

	if len(diff.UpdatedModules) > 0 {
		fmt.Fprintf(&b, "### Updated Modules (%d)\n\n", len(diff.UpdatedModules))
		b.WriteString("| Module | Old Revision | New Revision | Namespace |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
		for _, m := range diff.UpdatedModules {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", m.Name, m.OldRevision, m.NewRevision, m.Namespace)
		}
		b.WriteString("\n")
	}

	if len(diff.NewModules) == 0 && len(diff.RemovedModules) == 0 && len(diff.UpdatedModules) == 0 {
		b.WriteString("No YANG module changes detected.\n\n")
	}

	// ── Detailed API Structural Changes ────────────────────────────────
	b.WriteString("---\n\n")
	b.WriteString("## Detailed API Changes\n\n")
	if len(diff.APIChanges) == 0 {
		b.WriteString("No structural changes detected in existing API endpoints.\n\n")
	} else {
		for _, ac := range diff.APIChanges {
			fmt.Fprintf(&b, "### `%s`\n\n", ac.Path)

			if len(ac.AddedProperties) > 0 {
				fmt.Fprintf(&b, "**Added properties** (%d):\n", len(ac.AddedProperties))
				for _, p := range ac.AddedProperties {
					fmt.Fprintf(&b, "- `%s`\n", p)
				}
				b.WriteString("\n")
			}
			if len(ac.RemovedProperties) > 0 {
				fmt.Fprintf(&b, "**Removed properties** (%d):\n", len(ac.RemovedProperties))
				for _, p := range ac.RemovedProperties {
					fmt.Fprintf(&b, "- `%s` (**BREAKING**)\n", p)
				}
				b.WriteString("\n")
			}
			if len(ac.TypeChanges) > 0 {
				fmt.Fprintf(&b, "**Type changes** (%d):\n", len(ac.TypeChanges))
				for _, tc := range ac.TypeChanges {
					fmt.Fprintf(&b, "- `%s`: %s -> %s (**BREAKING**)\n", tc.PropertyPath, tc.OldType, tc.NewType)
				}
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

// ── Report helpers ─────────────────────────────────────────────────────

func groupBreakingChanges(changes []BreakingChange) map[string][]BreakingChange {
	grouped := make(map[string][]BreakingChange)
	for _, bc := range changes {
		grouped[bc.Category] = append(grouped[bc.Category], bc)
	}
	return grouped
}

func groupNewProperties(props []NewProperty) map[string][]NewProperty {
	grouped := make(map[string][]NewProperty)
	for _, np := range props {
		grouped[np.APIPath] = append(grouped[np.APIPath], np)
	}
	return grouped
}

func sortedKeys(m map[string][]NewProperty) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedProps(props map[string]PropInfo) []PropInfo {
	list := make([]PropInfo, 0, len(props))
	for _, pi := range props {
		list = append(list, pi)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
	return list
}
