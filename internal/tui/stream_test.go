package tui

import (
	"testing"
)

func TestParseSourceLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full JSON",
			input: `{"id":"src-1","category":"logs","title":"application.log"}`,
			want:  "[logs] log",
		},
		{
			name:  "no category",
			input: `{"id":"src-1","title":"metric_name"}`,
			want:  "metric_name",
		},
		{
			name:  "no title uses ID",
			input: `{"id":"src-1","category":"metrics"}`,
			want:  "[metrics] src-1",
		},
		{
			name:  "invalid JSON returns raw",
			input: "just a string",
			want:  "just a string",
		},
		{
			name:  "strips prefix after dot",
			input: `{"title":"namespace.pod.container.containerinsights_cpu","category":"k8s"}`,
			want:  "[k8s] cpu",
		},
		{
			name:  "strips containerinsights_ prefix",
			input: `{"title":"containerinsights_memory","category":"infra"}`,
			want:  "[infra] memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSourceLabel(tt.input)
			if got != tt.want {
				t.Errorf("parseSourceLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseCOTFields(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantID     string
		wantDesc   string
		wantExpl   string
		wantInv    string
		wantStatus string
	}{
		{
			name:       "full COT JSON",
			input:      `{"id":"cot-1","description":"Check logs","explanation":"Looking at error logs","investigation":"Found 3 errors","status":"IN_PROGRESS"}`,
			wantID:     "cot-1",
			wantDesc:   "Check logs",
			wantExpl:   "Looking at error logs",
			wantInv:    "Found 3 errors",
			wantStatus: "IN_PROGRESS",
		},
		{
			name:       "cot_status takes precedence",
			input:      `{"id":"cot-2","description":"Analyze","cot_status":"COMPLETED","status":"old_status"}`,
			wantID:     "cot-2",
			wantDesc:   "Analyze",
			wantStatus: "COMPLETED",
		},
		{
			name:       "falls back to status when cot_status empty",
			input:      `{"id":"cot-3","status":"IN_PROGRESS"}`,
			wantID:     "cot-3",
			wantStatus: "IN_PROGRESS",
		},
		{
			name:    "invalid JSON returns raw as investigation",
			input:   "raw text content",
			wantID:  "",
			wantInv: "raw text content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, desc, expl, inv, status := parseCOTFields(tt.input)
			if id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
			if desc != tt.wantDesc {
				t.Errorf("desc = %q, want %q", desc, tt.wantDesc)
			}
			if expl != tt.wantExpl {
				t.Errorf("expl = %q, want %q", expl, tt.wantExpl)
			}
			if inv != tt.wantInv {
				t.Errorf("inv = %q, want %q", inv, tt.wantInv)
			}
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
		})
	}
}

func TestFindActiveCOTPart(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{
			name:  "empty parts",
			parts: nil,
			want:  "",
		},
		{
			name:  "single part",
			parts: []string{`{"id":"1","status":"IN_PROGRESS"}`},
			want:  `{"id":"1","status":"IN_PROGRESS"}`,
		},
		{
			name: "picks IN_PROGRESS",
			parts: []string{
				`{"id":"1","status":"CHAIN_OF_THOUGHT_STATUS_COMPLETED"}`,
				`{"id":"2","status":"CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"}`,
				`{"id":"3","status":"CHAIN_OF_THOUGHT_STATUS_PENDING"}`,
			},
			want: `{"id":"2","status":"CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"}`,
		},
		{
			name: "picks short IN_PROGRESS status",
			parts: []string{
				`{"id":"1","status":"COMPLETED"}`,
				`{"id":"2","status":"IN_PROGRESS"}`,
			},
			want: `{"id":"2","status":"IN_PROGRESS"}`,
		},
		{
			name: "falls back to last if none in progress",
			parts: []string{
				`{"id":"1","status":"COMPLETED"}`,
				`{"id":"2","status":"COMPLETED"}`,
				`{"id":"3","status":"COMPLETED"}`,
			},
			want: `{"id":"3","status":"COMPLETED"}`,
		},
		{
			name: "handles unparseable parts",
			parts: []string{
				"invalid json",
				`{"id":"2","status":"IN_PROGRESS"}`,
			},
			want: `{"id":"2","status":"IN_PROGRESS"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findActiveCOTPart(tt.parts)
			if got != tt.want {
				t.Errorf("findActiveCOTPart() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchCommands(t *testing.T) {
	tests := []struct {
		prefix  string
		wantLen int
	}{
		{"/", len(slashCommands)},
		{"/h", 1}, // /help
		{"/s", 6}, // /score, /session, /session-report, /sessions, /set, /summary
		{"/q", 2}, // /queries, /quit
		{"/xyz", 0},
		{"/login", 1},
		{"/se", 4}, // /session, /session-report, /sessions, /set
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			got := matchCommands(tt.prefix)
			if len(got) != tt.wantLen {
				names := make([]string, len(got))
				for i, c := range got {
					names[i] = c.name
				}
				t.Errorf("matchCommands(%q) returned %d matches %v, want %d", tt.prefix, len(got), names, tt.wantLen)
			}
		})
	}
}

func TestTruncateUUID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short", "short"},
		{"12345678-1234-1234-1234-123456789012", "12345678...9012"},
		{"", ""},
		{"exactly-20-chars---!", "exactly-20-chars---!"},
		{"21-chars-long-string!", "21-chars...ing!"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateUUID(tt.input)
			if got != tt.want {
				t.Errorf("truncateUUID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
