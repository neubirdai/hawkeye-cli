package service

import (
	"testing"

	"hawkeye-cli/internal/api"
)

func TestFindProject(t *testing.T) {
	projects := []api.ProjectSpec{
		{Name: "Production", UUID: "prod-uuid"},
		{Name: "Staging", UUID: "staging-uuid"},
	}

	tests := []struct {
		name     string
		projects []api.ProjectSpec
		uuid     string
		wantName string
		wantNil  bool
	}{
		{
			name:     "found",
			projects: projects,
			uuid:     "staging-uuid",
			wantName: "Staging",
		},
		{
			name:     "not found",
			projects: projects,
			uuid:     "bogus-uuid",
			wantNil:  true,
		},
		{
			name:     "empty list",
			projects: nil,
			uuid:     "any-uuid",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindProject(tt.projects, tt.uuid)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if got.Name != tt.wantName {
				t.Errorf("got name %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

func TestFilterSystemProjects(t *testing.T) {
	tests := []struct {
		name    string
		input   []api.ProjectSpec
		wantLen int
	}{
		{
			name:    "nil input",
			input:   nil,
			wantLen: 0,
		},
		{
			name:    "empty input",
			input:   []api.ProjectSpec{},
			wantLen: 0,
		},
		{
			name: "no system projects",
			input: []api.ProjectSpec{
				{Name: "Production", UUID: "p1"},
				{Name: "Staging", UUID: "p2"},
			},
			wantLen: 2,
		},
		{
			name: "with system project",
			input: []api.ProjectSpec{
				{Name: "Production", UUID: "p1"},
				{Name: "SystemGlobalProject", UUID: "sys"},
				{Name: "Staging", UUID: "p2"},
			},
			wantLen: 2,
		},
		{
			name: "system project substring",
			input: []api.ProjectSpec{
				{Name: "my-SystemGlobalProject-thing", UUID: "sys2"},
				{Name: "Real Project", UUID: "p1"},
			},
			wantLen: 1,
		},
		{
			name: "all system projects",
			input: []api.ProjectSpec{
				{Name: "SystemGlobalProject", UUID: "sys1"},
				{Name: "SystemGlobalProject-backup", UUID: "sys2"},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterSystemProjects(tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("got %d projects, want %d", len(got), tt.wantLen)
			}
			// Verify no system projects remain
			for _, p := range got {
				if p.Name == "SystemGlobalProject" {
					t.Errorf("SystemGlobalProject still in result: %+v", p)
				}
			}
		})
	}
}
