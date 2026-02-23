package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestClaudeSkillContent(t *testing.T) {
	content := claudeSkillContent

	// Must start with YAML frontmatter
	if !strings.HasPrefix(content, "---\n") {
		t.Error("skill content must start with YAML frontmatter (---)")
	}

	// Must have required frontmatter fields
	for _, field := range []string{"name: hawkeye", "description:", "allowed-tools:"} {
		if !strings.Contains(content, field) {
			t.Errorf("skill content missing frontmatter field: %s", field)
		}
	}

	// Must restrict to hawkeye commands only
	if !strings.Contains(content, "Bash(hawkeye *)") {
		t.Error("skill content must restrict allowed-tools to Bash(hawkeye *)")
	}

	// Must include dynamic help injection
	if !strings.Contains(content, "`!`hawkeye help`") {
		t.Error("skill content must include dynamic help injection: `!`hawkeye help`")
	}

	// Must include key workflow sections
	for _, section := range []string{
		"Investigate an Incident",
		"Review Sessions",
		"Uninvestigated Alerts",
		"Analytics",
		"Manage Connections",
		"Configure Instructions",
	} {
		if !strings.Contains(content, section) {
			t.Errorf("skill content missing workflow section: %s", section)
		}
	}

	// Must include --json tip
	if !strings.Contains(content, "--json") {
		t.Error("skill content must mention --json flag")
	}

	// Sanity check: content should be substantial (at least 50 lines)
	lines := strings.Count(content, "\n")
	if lines < 50 {
		t.Errorf("skill content only has %d lines, expected at least 50", lines)
	}
}

func TestCmdSetupClaude(t *testing.T) {
	// Use a temp dir as HOME to isolate from real ~/.claude
	home := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(home, ".claude", "skills", "hawkeye")
	skillFile := filepath.Join(skillDir, "SKILL.md")

	// Test 1: Fresh install
	if err := cmdSetupClaude(nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("skill file not created: %v", err)
	}
	if string(data) != claudeSkillContent {
		t.Error("skill file content does not match claudeSkillContent")
	}

	// Verify permissions
	info, _ := os.Stat(skillFile)
	if perm := info.Mode().Perm(); perm != 0644 {
		t.Errorf("skill file permissions = %o, want 0644", perm)
	}
	dirInfo, _ := os.Stat(skillDir)
	if perm := dirInfo.Mode().Perm(); perm != 0755 {
		t.Errorf("skill dir permissions = %o, want 0755", perm)
	}

	// Test 2: Update (overwrite)
	if err := cmdSetupClaude(nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	data2, _ := os.ReadFile(skillFile)
	if string(data2) != claudeSkillContent {
		t.Error("skill file content changed after update")
	}

	// Test 3: Uninstall
	if err := cmdSetupClaude([]string{"--uninstall"}); err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("skill directory should be removed after uninstall")
	}

	// Test 4: Uninstall when not installed (no-op)
	if err := cmdSetupClaude([]string{"--uninstall"}); err != nil {
		t.Fatalf("uninstall-when-not-installed failed: %v", err)
	}
}
