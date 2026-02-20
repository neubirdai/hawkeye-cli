package service

import (
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
