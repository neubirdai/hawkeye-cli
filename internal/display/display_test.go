package display

import (
	"strings"
	"testing"
	"time"
)

func TestContentTypeLabel(t *testing.T) {
	knownTypes := []string{
		"CONTENT_TYPE_PROGRESS_STATUS",
		"CONTENT_TYPE_CHAT_RESPONSE",
		"CONTENT_TYPE_CHAIN_OF_THOUGHT",
		"CONTENT_TYPE_SOURCES",
		"CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS",
		"CONTENT_TYPE_VISUALIZATION",
		"CONTENT_TYPE_SESSION_NAME",
		"CONTENT_TYPE_ERROR_MESSAGE",
		"CONTENT_TYPE_DONE_INDICATOR",
		"CONTENT_TYPE_EXECUTION_TIME",
		"CONTENT_TYPE_ALTERNATE_QUESTIONS",
		"CONTENT_TYPE_MESSAGE",
	}

	for _, ct := range knownTypes {
		label := ContentTypeLabel(ct)
		if label == "" {
			t.Errorf("ContentTypeLabel(%q) returned empty string", ct)
		}
		// Known types should contain Reset (ANSI-colored)
		if !strings.Contains(label, Reset) {
			t.Errorf("ContentTypeLabel(%q) = %q, expected ANSI-colored output", ct, label)
		}
	}

	// Unknown type should return the type itself wrapped in Gray
	unknown := ContentTypeLabel("CONTENT_TYPE_UNKNOWN")
	if !strings.Contains(unknown, "CONTENT_TYPE_UNKNOWN") {
		t.Errorf("ContentTypeLabel(unknown) = %q, expected to contain the input type", unknown)
	}
	if !strings.Contains(unknown, Gray) {
		t.Errorf("ContentTypeLabel(unknown) = %q, expected Gray coloring", unknown)
	}
}

func TestCoTStatusLabel(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS", "In Progress"},
		{"CHAIN_OF_THOUGHT_STATUS_DONE", "Done"},
		{"CHAIN_OF_THOUGHT_STATUS_ERROR", "Error"},
		{"CHAIN_OF_THOUGHT_STATUS_CANCELLED", "Cancelled"},
		{"CHAIN_OF_THOUGHT_STATUS_PAUSED", "Paused"},
	}

	for _, tt := range tests {
		label := CoTStatusLabel(tt.input)
		if !strings.Contains(label, tt.contains) {
			t.Errorf("CoTStatusLabel(%q) = %q, expected to contain %q", tt.input, label, tt.contains)
		}
	}

	// Unknown status should return input as-is
	unknown := CoTStatusLabel("SOME_UNKNOWN_STATUS")
	if unknown != "SOME_UNKNOWN_STATUS" {
		t.Errorf("CoTStatusLabel(unknown) = %q, expected %q", unknown, "SOME_UNKNOWN_STATUS")
	}
}

func TestInvestigationStatusLabel(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"INVESTIGATION_STATUS_NOT_STARTED", "Not Started"},
		{"INVESTIGATION_STATUS_IN_PROGRESS", "In Progress"},
		{"INVESTIGATION_STATUS_INVESTIGATED", "Investigated"},
		{"INVESTIGATION_STATUS_COMPLETED", "Completed"},
		{"INVESTIGATION_STATUS_PAUSED", "Paused"},
		{"INVESTIGATION_STATUS_STOPPED", "Stopped"},
	}

	for _, tt := range tests {
		label := InvestigationStatusLabel(tt.input)
		if !strings.Contains(label, tt.contains) {
			t.Errorf("InvestigationStatusLabel(%q) = %q, expected to contain %q", tt.input, label, tt.contains)
		}
	}

	// Unknown returns input
	unknown := InvestigationStatusLabel("UNKNOWN")
	if unknown != "UNKNOWN" {
		t.Errorf("InvestigationStatusLabel(unknown) = %q, expected %q", unknown, "UNKNOWN")
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(string) bool
	}{
		{
			name:  "RFC3339",
			input: "2024-01-15T10:30:00Z",
			check: func(s string) bool {
				_, err := time.Parse("2006-01-02 15:04:05", s)
				return err == nil
			},
		},
		{
			name:  "RFC3339Nano",
			input: "2024-01-15T10:30:00.123456789Z",
			check: func(s string) bool {
				_, err := time.Parse("2006-01-02 15:04:05", s)
				return err == nil
			},
		},
		{
			name:  "invalid input",
			input: "not-a-date",
			check: func(s string) bool {
				return s == "not-a-date"
			},
		},
		{
			name:  "empty string",
			input: "",
			check: func(s string) bool {
				return s == ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTime(tt.input)
			if !tt.check(result) {
				t.Errorf("FormatTime(%q) = %q, unexpected result", tt.input, result)
			}
		})
	}
}
