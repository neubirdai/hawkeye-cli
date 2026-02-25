package service

import (
	"testing"
)

func TestBuildSessionURL(t *testing.T) {
	tests := []struct {
		name        string
		serverURL   string
		projectUUID string
		sessionUUID string
		want        string
	}{
		{
			name:        "simple URL",
			serverURL:   "https://app.neubird.ai",
			projectUUID: "proj-123",
			sessionUUID: "sess-456",
			want:        "https://app.neubird.ai/console/project/proj-123/session/sess-456?tab=results",
		},
		{
			name:        "URL with trailing slash",
			serverURL:   "https://app.neubird.ai/",
			projectUUID: "proj-123",
			sessionUUID: "sess-456",
			want:        "https://app.neubird.ai/console/project/proj-123/session/sess-456?tab=results",
		},
		{
			name:        "URL with /api suffix",
			serverURL:   "https://app.neubird.ai/api",
			projectUUID: "proj-123",
			sessionUUID: "sess-456",
			want:        "https://app.neubird.ai/console/project/proj-123/session/sess-456?tab=results",
		},
		{
			name:        "URL with /api/ suffix",
			serverURL:   "https://app.neubird.ai/api/",
			projectUUID: "proj-123",
			sessionUUID: "sess-456",
			want:        "https://app.neubird.ai/console/project/proj-123/session/sess-456?tab=results",
		},
		{
			name:        "localhost",
			serverURL:   "http://localhost:8080/api",
			projectUUID: "p1",
			sessionUUID: "s1",
			want:        "http://localhost:8080/console/project/p1/session/s1?tab=results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSessionURL(tt.serverURL, tt.projectUUID, tt.sessionUUID)
			if got != tt.want {
				t.Errorf("BuildSessionURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSessionURL(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		wantHost    string
		wantProject string
		wantSession string
		wantErr     bool
	}{
		{
			name:        "standard URL with tab",
			rawURL:      "https://hyphen.app.neubird.ai/console/project/proj-123/session/sess-456?tab=rca",
			wantHost:    "https://hyphen.app.neubird.ai",
			wantProject: "proj-123",
			wantSession: "sess-456",
		},
		{
			name:        "no query params",
			rawURL:      "https://hyphen.app.neubird.ai/console/project/proj-123/session/sess-456",
			wantHost:    "https://hyphen.app.neubird.ai",
			wantProject: "proj-123",
			wantSession: "sess-456",
		},
		{
			name:        "trailing slash",
			rawURL:      "https://hyphen.app.neubird.ai/console/project/proj-123/session/sess-456/",
			wantHost:    "https://hyphen.app.neubird.ai",
			wantProject: "proj-123",
			wantSession: "sess-456",
		},
		{
			name:        "localhost with port",
			rawURL:      "http://localhost:8080/console/project/p1/session/s1?tab=results",
			wantHost:    "http://localhost:8080",
			wantProject: "p1",
			wantSession: "s1",
		},
		{
			name:    "missing session segment",
			rawURL:  "https://app.neubird.ai/console/project/proj-123",
			wantErr: true,
		},
		{
			name:    "wrong path",
			rawURL:  "https://app.neubird.ai/dashboard/overview",
			wantErr: true,
		},
		{
			name:    "empty string",
			rawURL:  "",
			wantErr: true,
		},
		{
			name:    "no scheme",
			rawURL:  "app.neubird.ai/console/project/p/session/s",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, proj, sess, err := ParseSessionURL(tt.rawURL)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseSessionURL(%q) expected error, got host=%q proj=%q sess=%q", tt.rawURL, host, proj, sess)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSessionURL(%q) unexpected error: %v", tt.rawURL, err)
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if proj != tt.wantProject {
				t.Errorf("project = %q, want %q", proj, tt.wantProject)
			}
			if sess != tt.wantSession {
				t.Errorf("session = %q, want %q", sess, tt.wantSession)
			}
		})
	}
}

func TestParseSessionURL_RoundTrip(t *testing.T) {
	u := BuildSessionURL("https://app.neubird.ai/api", "proj-abc", "sess-xyz")
	_, proj, sess, err := ParseSessionURL(u)
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if proj != "proj-abc" {
		t.Errorf("project = %q, want %q", proj, "proj-abc")
	}
	if sess != "sess-xyz" {
		t.Errorf("session = %q, want %q", sess, "sess-xyz")
	}
}
