package api

import (
	"testing"
)

func TestExtractProgressDescription(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Paren-wrapped → strip parens
		{"(Consulting logs and metrics)", "Consulting logs and metrics"},
		{"(Found 9 results)", "Found 9 results"},
		// FilterName (Description) → extract description
		{"SplitAnswer (Analyzing Telemetry)", "Analyzing Telemetry"},
		{"SQLExecute (Running queries)", "Running queries"},
		// Plain text → pass through
		{"Analyzing Telemetry", "Analyzing Telemetry"},
		{"Working", "Working"},
		// Edge: empty
		{"", ""},
		// Nested parens: extract innermost from " ("
		{"Filter (Step (inner))", "Step (inner)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractProgressDescription(tt.input)
			if got != tt.want {
				t.Errorf("extractProgressDescription(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeProgress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// "(Found N results)" normalization
		{"(Found 9 results)", "(Found N results)"},
		{"(Found 42 results)", "(Found N results)"},
		// "result streams" normalization
		{"(Analyzing 3 result streams)", "(Analyzing N result streams)"},
		// datasources normalization
		{"(Selected 5 datasources)", "(Selected N data sources)"},
		// Non-matching → pass through
		{"Analyzing Telemetry", "Analyzing Telemetry"},
		{"Working...", "Working..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeProgress(tt.input)
			if got != tt.want {
				t.Errorf("normalizeProgress(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsActivityOnly(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"(Found 9 results)", true},
		{"(Working...)", true},
		{"SplitAnswer (Analyzing)", false},
		{"Analyzing", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isActivityOnly(tt.input)
			if got != tt.want {
				t.Errorf("isActivityOnly(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatCOTCategory(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"CATEGORY_LOG_ANALYSIS", "Log analysis"},
		{"CATEGORY_METRIC_QUERY", "Metric query"},
		{"log_analysis", "Log analysis"},
		{"", ""},
		{"CATEGORY_A", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatCOTCategory(tt.input)
			if got != tt.want {
				t.Errorf("formatCOTCategory(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatCOTStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS", "⟳ In progress"},
		{"IN_PROGRESS", "⟳ In progress"},
		{"CHAIN_OF_THOUGHT_STATUS_DONE", "✓ Done"},
		{"DONE", "✓ Done"},
		{"CHAIN_OF_THOUGHT_STATUS_ERROR", "✗ Error"},
		{"ERROR", "✗ Error"},
		{"CHAIN_OF_THOUGHT_STATUS_CANCELLED", "⊘ Cancelled"},
		{"CANCELLED", "⊘ Cancelled"},
		{"SOME_OTHER_STATUS", "SOME_OTHER_STATUS"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatCOTStatus(tt.input)
			if got != tt.want {
				t.Errorf("formatCOTStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"br tag", "hello<br>world", "hello\nworld"},
		{"br self-closing", "hello<br/>world", "hello\nworld"},
		{"br with space", "hello<br />world", "hello\nworld"},
		{"bold tags", "<b>bold</b> text", "bold text"},
		{"italic tags", "<i>italic</i> text", "italic text"},
		{"paragraph tags", "<p>paragraph</p>", "paragraph"},
		{"mixed tags", "<b>bold</b> and <i>italic</i><br/>newline", "bold and italic\nnewline"},
		{"no tags", "plain text", "plain text"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
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
		{"in progress...", true},
		{"Investigating...", true},
		{"Analyzing...", true},
		{"short", true}, // < 20 chars
		{"This is a real investigation finding that has substantial content", false},
		{"A slightly longer text here!!", false}, // >= 20 chars
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isTrivialContent(tt.input)
			if got != tt.want {
				t.Errorf("isTrivialContent(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatSourceLabel(t *testing.T) {
	tests := []struct {
		name  string
		input sourceJSON
		want  string
	}{
		{
			name:  "with category and dotted name",
			input: sourceJSON{Category: "logs", Title: "cluster.containerinsights_pod_logs"},
			want:  "[logs] pod_logs",
		},
		{
			name:  "with category, no dot",
			input: sourceJSON{Category: "metrics", Title: "cpu_usage"},
			want:  "[metrics] cpu_usage",
		},
		{
			name:  "no category",
			input: sourceJSON{Title: "some.source_name"},
			want:  "source_name",
		},
		{
			name:  "containerinsights prefix strip",
			input: sourceJSON{Title: "db.containerinsights_network_bytes"},
			want:  "network_bytes",
		},
		{
			name:  "empty title",
			input: sourceJSON{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSourceLabel(tt.input)
			if got != tt.want {
				t.Errorf("formatSourceLabel(%+v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
