package main

import (
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
