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
