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

// FindProject searches for a project by UUID or name (case-insensitive).
// Returns nil if no match is found.
func FindProject(projects []api.ProjectSpec, value string) *api.ProjectSpec {
	for i := range projects {
		p := &projects[i]
		if p.UUID == value || strings.EqualFold(p.Name, value) {
			return p
		}
	}
	return nil
}

// ProjectDetailDisplay holds display-ready project detail info.
type ProjectDetailDisplay struct {
	UUID        string
	Name        string
	Description string
	Ready       bool
	CreateTime  string
	UpdateTime  string
}

// FormatProjectDetail maps a raw ProjectDetail to a display-ready struct.
func FormatProjectDetail(p *api.ProjectDetail) ProjectDetailDisplay {
	if p == nil {
		return ProjectDetailDisplay{}
	}
	name := p.Name
	if name == "" {
		name = "(unnamed)"
	}
	return ProjectDetailDisplay{
		UUID:        p.UUID,
		Name:        name,
		Description: p.Description,
		Ready:       p.Ready,
		CreateTime:  p.CreateTime,
		UpdateTime:  p.UpdateTime,
	}
}
