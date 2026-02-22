package service

import (
	"testing"

	"hawkeye-cli/internal/api"
)

func TestBuildSessionFilters(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		from           string
		to             string
		search         string
		uninvestigated bool
		wantLen        int
		wantFirst      api.PaginationFilter
	}{
		{
			name:    "no filters",
			wantLen: 0,
		},
		{
			name:    "status filter",
			status:  "investigated",
			wantLen: 1,
			wantFirst: api.PaginationFilter{
				Key:      "investigation_status",
				Value:    "INVESTIGATION_STATUS_COMPLETED",
				Operator: "==",
			},
		},
		{
			name:    "from date",
			from:    "2025-01-01",
			wantLen: 1,
			wantFirst: api.PaginationFilter{
				Key:      "create_time",
				Value:    "2025-01-01",
				Operator: "gte",
			},
		},
		{
			name:    "to date",
			to:      "2025-12-31",
			wantLen: 1,
			wantFirst: api.PaginationFilter{
				Key:      "create_time",
				Value:    "2025-12-31",
				Operator: "lte",
			},
		},
		{
			name:    "search filter",
			search:  "API error",
			wantLen: 1,
			wantFirst: api.PaginationFilter{
				Key:      "incident_info.title",
				Value:    "API error",
				Operator: "in",
			},
		},
		{
			name:           "uninvestigated shorthand",
			uninvestigated: true,
			wantLen:        1,
			wantFirst: api.PaginationFilter{
				Key:      "investigation_status",
				Value:    "INVESTIGATION_STATUS_NOT_STARTED",
				Operator: "==",
			},
		},
		{
			name:    "multiple filters",
			status:  "in_progress",
			from:    "2025-01-01",
			to:      "2025-06-30",
			wantLen: 3,
			wantFirst: api.PaginationFilter{
				Key:      "investigation_status",
				Value:    "INVESTIGATION_STATUS_IN_PROGRESS",
				Operator: "==",
			},
		},
		{
			name:           "uninvestigated overrides status",
			status:         "investigated",
			uninvestigated: true,
			wantLen:        1,
			wantFirst: api.PaginationFilter{
				Key:      "investigation_status",
				Value:    "INVESTIGATION_STATUS_NOT_STARTED",
				Operator: "==",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSessionFilters(tt.status, tt.from, tt.to, tt.search, tt.uninvestigated)
			if len(got) != tt.wantLen {
				t.Fatalf("got %d filters, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen > 0 && got[0] != tt.wantFirst {
				t.Errorf("first filter = %+v, want %+v", got[0], tt.wantFirst)
			}
		})
	}
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"not_started", "INVESTIGATION_STATUS_NOT_STARTED"},
		{"in_progress", "INVESTIGATION_STATUS_IN_PROGRESS"},
		{"investigated", "INVESTIGATION_STATUS_COMPLETED"},
		{"completed", "INVESTIGATION_STATUS_COMPLETED"},
		{"INVESTIGATION_STATUS_CUSTOM", "INVESTIGATION_STATUS_CUSTOM"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeStatus(tt.input)
			if got != tt.want {
				t.Errorf("normalizeStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatQueries(t *testing.T) {
	t.Run("nil list", func(t *testing.T) {
		got := FormatQueries(nil)
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("with queries", func(t *testing.T) {
		queries := []api.QueryExecution{
			{ID: "q1", Query: "SELECT * FROM metrics", Source: "prometheus", Status: "COMPLETED"},
			{ID: "q2", Query: "search logs", Source: "elasticsearch", Status: "FAILED", ErrorMessage: "timeout"},
		}
		got := FormatQueries(queries)
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
		if got[0].Source != "prometheus" {
			t.Errorf("got[0].Source = %q, want %q", got[0].Source, "prometheus")
		}
		if got[1].ErrorMessage != "timeout" {
			t.Errorf("got[1].ErrorMessage = %q, want %q", got[1].ErrorMessage, "timeout")
		}
	})
}

func TestFormatSessionRow(t *testing.T) {
	tests := []struct {
		name     string
		input    api.SessionInfo
		wantName string
		wantIcon string
	}{
		{
			name:     "normal session",
			input:    api.SessionInfo{Name: "Test Session", SessionType: "SESSION_TYPE_CHAT"},
			wantName: "Test Session",
			wantIcon: "💬",
		},
		{
			name:     "unnamed session",
			input:    api.SessionInfo{Name: "", SessionType: "SESSION_TYPE_CHAT"},
			wantName: "(unnamed)",
			wantIcon: "💬",
		},
		{
			name:     "incident session",
			input:    api.SessionInfo{SessionType: "SESSION_TYPE_INCIDENT"},
			wantName: "(unnamed)",
			wantIcon: "🚨",
		},
		{
			name:     "pinned session",
			input:    api.SessionInfo{Name: "Pinned", Pinned: true},
			wantName: "Pinned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSessionRow(tt.input)
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if tt.wantIcon != "" && got.TypeIcon != tt.wantIcon {
				t.Errorf("TypeIcon = %q, want %q", got.TypeIcon, tt.wantIcon)
			}
			if tt.input.Pinned && !got.Pinned {
				t.Error("Pinned = false, want true")
			}
		})
	}
}
