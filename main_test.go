package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{
			name:  "short text fits in width",
			text:  "hello world",
			width: 80,
			want:  []string{"hello world"},
		},
		{
			name:  "long text wraps",
			text:  "the quick brown fox jumps over the lazy dog",
			width: 20,
			want:  []string{"the quick brown fox", "jumps over the lazy", "dog"},
		},
		{
			name:  "preserves paragraphs",
			text:  "first paragraph\n\nsecond paragraph",
			width: 80,
			want:  []string{"first paragraph", "", "second paragraph"},
		},
		{
			name:  "empty string",
			text:  "",
			width: 80,
			want:  []string{""},
		},
		{
			name:  "single long word",
			text:  "superlongword",
			width: 5,
			want:  []string{"superlongword"},
		},
		{
			name:  "multiple newlines",
			text:  "a\nb\nc",
			width: 80,
			want:  []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.text, tt.width)
			if len(got) != len(tt.want) {
				t.Errorf("wrapText(%q, %d) returned %d lines, want %d\n  got:  %v\n  want: %v",
					tt.text, tt.width, len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("wrapText(%q, %d)[%d] = %q, want %q",
						tt.text, tt.width, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseGlobalFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantProfile string
		wantArgs    []string
	}{
		{
			name:        "no flags",
			args:        []string{"login", "url"},
			wantProfile: "",
			wantArgs:    []string{"login", "url"},
		},
		{
			name:        "profile before command",
			args:        []string{"--profile", "staging", "login"},
			wantProfile: "staging",
			wantArgs:    []string{"login"},
		},
		{
			name:        "profile after command",
			args:        []string{"config", "--profile", "prod"},
			wantProfile: "prod",
			wantArgs:    []string{"config"},
		},
		{
			name:        "profile with extra args",
			args:        []string{"--profile", "dev", "set", "server", "http://localhost"},
			wantProfile: "dev",
			wantArgs:    []string{"set", "server", "http://localhost"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activeProfile = ""
			got := parseGlobalFlags(tt.args)
			if activeProfile != tt.wantProfile {
				t.Errorf("activeProfile = %q, want %q", activeProfile, tt.wantProfile)
			}
			if len(got) != len(tt.wantArgs) {
				t.Errorf("remaining args = %v, want %v", got, tt.wantArgs)
				return
			}
			for i := range got {
				if got[i] != tt.wantArgs[i] {
					t.Errorf("arg[%d] = %q, want %q", i, got[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{
			name: "under max length",
			s:    "hello",
			max:  10,
			want: "hello",
		},
		{
			name: "at max length",
			s:    "hello",
			max:  5,
			want: "hello",
		},
		{
			name: "over max length",
			s:    "hello world this is long",
			max:  10,
			want: "hello w...",
		},
		{
			name: "empty string",
			s:    "",
			max:  10,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
			if len(got) > tt.max {
				t.Errorf("truncate(%q, %d) returned string of len %d, exceeds max", tt.s, tt.max, len(got))
			}
			if tt.max > 3 && len(tt.s) > tt.max && !strings.HasSuffix(got, "...") {
				t.Errorf("truncate(%q, %d) = %q, expected ... suffix for truncated string", tt.s, tt.max, got)
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	input := map[string]any{
		"name":  "test",
		"count": float64(42),
	}

	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}
	output := string(data)

	// Verify it round-trips
	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if decoded["name"] != "test" {
		t.Errorf("name = %v, want %q", decoded["name"], "test")
	}
	if decoded["count"] != float64(42) {
		t.Errorf("count = %v, want %v", decoded["count"], 42)
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		commit     string
		date       string
		wantPrefix string
		wantCommit bool
	}{
		{
			name:       "dev build",
			version:    "dev",
			commit:     "none",
			date:       "unknown",
			wantPrefix: "hawkeye dev",
			wantCommit: false,
		},
		{
			name:       "release build",
			version:    "v1.2.3",
			commit:     "abc1234",
			date:       "2026-02-25T10:00:00Z",
			wantPrefix: "hawkeye v1.2.3",
			wantCommit: true,
		},
		{
			name:       "dirty build from make",
			version:    "v0.1.0-49-gb3df7c4-dirty",
			commit:     "b3df7c4",
			date:       "2026-02-26T02:18:39Z",
			wantPrefix: "hawkeye v0.1.0-49-gb3df7c4-dirty",
			wantCommit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore globals
			origVersion, origCommit, origDate := version, commit, date
			defer func() { version, commit, date = origVersion, origCommit, origDate }()

			version = tt.version
			commit = tt.commit
			date = tt.date

			got := versionString()

			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("versionString() = %q, want prefix %q", got, tt.wantPrefix)
			}

			hasCommit := strings.Contains(got, "commit:")
			if hasCommit != tt.wantCommit {
				t.Errorf("versionString() commit present = %v, want %v\noutput: %q", hasCommit, tt.wantCommit, got)
			}

			if tt.wantCommit {
				if !strings.Contains(got, tt.commit) {
					t.Errorf("versionString() should contain commit %q, got %q", tt.commit, got)
				}
				if !strings.Contains(got, tt.date) {
					t.Errorf("versionString() should contain date %q, got %q", tt.date, got)
				}
			}
		})
	}
}

func TestVersionStringFormat(t *testing.T) {
	// Save and restore globals
	origVersion, origCommit, origDate := version, commit, date
	defer func() { version, commit, date = origVersion, origCommit, origDate }()

	version = "v1.0.0"
	commit = "abc123"
	date = "2026-01-01"

	got := versionString()
	lines := strings.Split(got, "\n")

	// First line should be "hawkeye <version>"
	if lines[0] != "hawkeye v1.0.0" {
		t.Errorf("first line = %q, want %q", lines[0], "hawkeye v1.0.0")
	}

	// Should have exactly 3 lines for release build
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(lines), got)
	}
}

func TestParseGlobalFlagsJSON(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantJSON bool
		wantArgs []string
	}{
		{
			name:     "short flag -j",
			args:     []string{"-j", "sessions"},
			wantJSON: true,
			wantArgs: []string{"sessions"},
		},
		{
			name:     "long flag --json",
			args:     []string{"--json", "config"},
			wantJSON: true,
			wantArgs: []string{"config"},
		},
		{
			name:     "json after command",
			args:     []string{"projects", "-j"},
			wantJSON: true,
			wantArgs: []string{"projects"},
		},
		{
			name:     "json with profile",
			args:     []string{"--profile", "staging", "-j", "sessions"},
			wantJSON: true,
			wantArgs: []string{"sessions"},
		},
		{
			name:     "no json flag",
			args:     []string{"sessions"},
			wantJSON: false,
			wantArgs: []string{"sessions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activeProfile = ""
			jsonOutput = false
			got := parseGlobalFlags(tt.args)
			if jsonOutput != tt.wantJSON {
				t.Errorf("jsonOutput = %v, want %v", jsonOutput, tt.wantJSON)
			}
			if len(got) != len(tt.wantArgs) {
				t.Errorf("remaining args = %v, want %v", got, tt.wantArgs)
				return
			}
			for i := range got {
				if got[i] != tt.wantArgs[i] {
					t.Errorf("arg[%d] = %q, want %q", i, got[i], tt.wantArgs[i])
				}
			}
		})
	}
}
