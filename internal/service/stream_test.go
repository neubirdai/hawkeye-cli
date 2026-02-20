package service

import (
	"testing"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no HTML", "plain text", "plain text"},
		{"br tag", "line1<br/>line2", "line1\nline2"},
		{"br without slash", "line1<br>line2", "line1\nline2"},
		{"br with space", "line1<br />line2", "line1\nline2"},
		{"p tags", "<p>text</p>", "text"},
		{"mixed", "<b>bold</b> and <i>italic</i>", "bold and italic"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.input)
			if got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsTrivialContent(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"   ", true},
		{"In progress...", true},
		{"investigating...", true},
		{"ANALYZING...", true},
		{"thinking...", true},
		{"Actual investigation content here", false},
		{"Found 5 errors", false},
		{"Short", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsTrivialContent(tt.input)
			if got != tt.want {
				t.Errorf("IsTrivialContent(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeProgress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Found 2 results", "Found N results"},
		{"Found 100 results", "Found N results"},
		{"Analyzing 3 result streams", "Analyzing N result streams"},
		{"Selected 5 datasources", "Selected N data sources"},
		{"Preparing sources", "Preparing sources"},
		{"unique message", "unique message"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeProgress(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeProgress(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractProgressDisplay(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PromptGate (Preparing Telemetry Sources)", "Preparing Telemetry Sources"},
		{"Step (Found 5 results)", "Found 5 results"},
		{"No parens here", "No parens here"},
		{"(Only parens)", "Only parens"},
		{"Multiple (first) and (second)", "first) and (second"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractProgressDisplay(tt.input)
			if got != tt.want {
				t.Errorf("ExtractProgressDisplay(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
