package service

import (
	"strings"
)

// BuildSessionURL constructs the web UI URL for a session.
// Format: {baseURL}/console/project/{p}/session/{s}?tab=results
// Strips "/api" suffix if present in the server URL.
func BuildSessionURL(serverURL, projectUUID, sessionUUID string) string {
	base := strings.TrimRight(serverURL, "/")

	// Strip /api suffix to get the frontend URL
	base = strings.TrimSuffix(base, "/api")

	return base + "/console/project/" + projectUUID + "/session/" + sessionUUID + "?tab=results"
}
