package service

import (
	"fmt"
	"strings"

	"hawkeye-cli/internal/api"
)

// FilterSystemProjects removes SystemGlobalProject entries from the list.
// This logic is shared between CLI and TUI.
func FilterSystemProjects(projects []api.ProjectSpec) []api.ProjectSpec {
	var filtered []api.ProjectSpec
	for _, p := range projects {
		if !strings.Contains(p.Name, "SystemGlobalProject") {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// FindProject searches a project list for a matching UUID.
// Returns nil if not found.
func FindProject(projects []api.ProjectSpec, uuid string) *api.ProjectSpec {
	for i := range projects {
		if projects[i].UUID == uuid {
			return &projects[i]
		}
	}
	return nil
}

// FindProjectByName searches by name. It returns all case-insensitive matches
// and reports whether any match was exact (case-sensitive).
func FindProjectByName(projects []api.ProjectSpec, name string) (matches []*api.ProjectSpec, exactFound bool) {
	// First pass: exact (case-sensitive) match
	for i := range projects {
		if projects[i].Name == name {
			return []*api.ProjectSpec{&projects[i]}, true
		}
	}
	// Second pass: case-insensitive
	lower := strings.ToLower(name)
	for i := range projects {
		if strings.ToLower(projects[i].Name) == lower {
			matches = append(matches, &projects[i])
		}
	}
	return matches, false
}

// ResolveProject finds a project by UUID or name. It tries UUID first, then
// falls back to name lookup. Returns a descriptive error for not-found or
// ambiguous cases.
func ResolveProject(projects []api.ProjectSpec, value string) (*api.ProjectSpec, error) {
	// Try UUID first
	if proj := FindProject(projects, value); proj != nil {
		return proj, nil
	}

	// Try name
	matches, _ := FindProjectByName(projects, value)
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("project %q not found. Run: hawkeye projects", value)
	case 1:
		return matches[0], nil
	default:
		lines := fmt.Sprintf("multiple projects match %q — use the UUID instead:", value)
		for _, m := range matches {
			lines += fmt.Sprintf("\n  %s  %s", m.UUID, m.Name)
		}
		return nil, fmt.Errorf("%s", lines)
	}
}
