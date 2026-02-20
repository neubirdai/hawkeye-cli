package service

import (
	"strings"
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

func TestFindProjectByName(t *testing.T) {
	projects := []api.ProjectSpec{
		{Name: "Production", UUID: "prod-uuid"},
		{Name: "Staging", UUID: "staging-uuid"},
		{Name: "staging", UUID: "staging-lower-uuid"},
	}

	tests := []struct {
		name      string
		projects  []api.ProjectSpec
		input     string
		wantCount int
		wantExact bool
		wantUUID  string // only checked when wantCount == 1
	}{
		{
			name:      "exact match",
			projects:  projects,
			input:     "Production",
			wantCount: 1,
			wantExact: true,
			wantUUID:  "prod-uuid",
		},
		{
			name:      "case-insensitive single match",
			projects:  projects,
			input:     "production",
			wantCount: 1,
			wantExact: false,
			wantUUID:  "prod-uuid",
		},
		{
			name:      "exact match takes priority over case-insensitive duplicates",
			projects:  projects,
			input:     "Staging",
			wantCount: 1,
			wantExact: true,
			wantUUID:  "staging-uuid",
		},
		{
			name:      "case-insensitive returns multiple when no exact match",
			projects:  projects,
			input:     "STAGING",
			wantCount: 2,
			wantExact: false,
		},
		{
			name:      "not found",
			projects:  projects,
			input:     "Nonexistent",
			wantCount: 0,
			wantExact: false,
		},
		{
			name:      "empty list",
			projects:  nil,
			input:     "anything",
			wantCount: 0,
			wantExact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, exact := FindProjectByName(tt.projects, tt.input)
			if len(matches) != tt.wantCount {
				t.Errorf("got %d matches, want %d", len(matches), tt.wantCount)
			}
			if exact != tt.wantExact {
				t.Errorf("got exact=%v, want %v", exact, tt.wantExact)
			}
			if tt.wantCount == 1 && tt.wantUUID != "" {
				if matches[0].UUID != tt.wantUUID {
					t.Errorf("got UUID %q, want %q", matches[0].UUID, tt.wantUUID)
				}
			}
		})
	}
}

func TestResolveProject(t *testing.T) {
	projects := []api.ProjectSpec{
		{Name: "Production", UUID: "prod-uuid"},
		{Name: "Staging", UUID: "staging-uuid"},
		{Name: "staging", UUID: "staging-lower-uuid"},
	}

	tests := []struct {
		name     string
		projects []api.ProjectSpec
		input    string
		wantUUID string
		wantErr  string // substring match on error
	}{
		{
			name:     "resolve by UUID",
			projects: projects,
			input:    "prod-uuid",
			wantUUID: "prod-uuid",
		},
		{
			name:     "resolve by exact name",
			projects: projects,
			input:    "Production",
			wantUUID: "prod-uuid",
		},
		{
			name:     "resolve by case-insensitive name",
			projects: projects,
			input:    "production",
			wantUUID: "prod-uuid",
		},
		{
			name:     "exact name wins over ambiguous case-insensitive",
			projects: projects,
			input:    "Staging",
			wantUUID: "staging-uuid",
		},
		{
			name:     "ambiguous name error",
			projects: projects,
			input:    "STAGING",
			wantErr:  "multiple projects match",
		},
		{
			name:     "not found",
			projects: projects,
			input:    "Nonexistent",
			wantErr:  `project "Nonexistent" not found`,
		},
		{
			name:     "empty list not found",
			projects: nil,
			input:    "anything",
			wantErr:  "not found",
		},
		{
			name: "uuid-like string that is actually a name",
			projects: []api.ProjectSpec{
				{Name: "abc-123", UUID: "real-uuid"},
			},
			input:    "abc-123",
			wantUUID: "real-uuid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj, err := ResolveProject(tt.projects, tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if proj.UUID != tt.wantUUID {
				t.Errorf("got UUID %q, want %q", proj.UUID, tt.wantUUID)
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
