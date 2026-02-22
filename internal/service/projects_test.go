package service

import (
	"testing"

	"hawkeye-cli/internal/api"
)

func TestFormatProjectDetail(t *testing.T) {
	tests := []struct {
		name     string
		input    *api.ProjectDetail
		wantName string
		wantUUID string
	}{
		{
			name:     "nil input",
			input:    nil,
			wantName: "",
			wantUUID: "",
		},
		{
			name:     "with all fields",
			input:    &api.ProjectDetail{UUID: "p1", Name: "Prod", Description: "Production env", Ready: true, CreateTime: "2025-01-01"},
			wantName: "Prod",
			wantUUID: "p1",
		},
		{
			name:     "unnamed project",
			input:    &api.ProjectDetail{UUID: "p2"},
			wantName: "(unnamed)",
			wantUUID: "p2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatProjectDetail(tt.input)
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.UUID != tt.wantUUID {
				t.Errorf("UUID = %q, want %q", got.UUID, tt.wantUUID)
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
