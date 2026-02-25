package service

import (
	"fmt"
	"net/url"
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

func ParseSessionURL(rawURL string) (host, projectUUID, sessionUUID string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", "", fmt.Errorf("invalid URL: missing scheme or host")
	}

	host = u.Scheme + "://" + u.Host
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")

	for i := 0; i+4 < len(segments); i++ {
		if segments[i] == "console" && segments[i+1] == "project" && segments[i+3] == "session" {
			projectUUID = segments[i+2]
			sessionUUID = segments[i+4]
			if projectUUID == "" || sessionUUID == "" {
				return "", "", "", fmt.Errorf("URL path has empty project or session ID")
			}
			return host, projectUUID, sessionUUID, nil
		}
	}
	return "", "", "", fmt.Errorf("URL path does not match /console/project/{id}/session/{id}")
}
