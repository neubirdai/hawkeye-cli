package service

import (
	"regexp"
	"strings"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// StripHTML converts HTML breaks to newlines and removes all HTML tags.
func StripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	return s
}

// IsTrivialContent checks if the text is just a placeholder/status message
// that shouldn't be displayed as investigation content.
// Only filters exact placeholder strings the server sends before real content.
func IsTrivialContent(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	return lower == "in progress..." ||
		lower == "investigating..." ||
		lower == "analyzing..." ||
		lower == "thinking..."
}

// NormalizeProgress deduplicates progress messages that differ only in counts.
// Operates on display text (after ExtractProgressDisplay), e.g.:
// "Found 2 results" and "Found 3 results" -> same key.
func NormalizeProgress(display string) string {
	if strings.HasPrefix(display, "Found ") && strings.HasSuffix(display, " results") {
		return "Found N results"
	}
	if strings.Contains(display, "result streams") {
		return "Analyzing N result streams"
	}
	if strings.Contains(display, "datas") && strings.Contains(display, "ources") {
		return "Selected N data sources"
	}
	return display
}

// ExtractProgressDisplay pulls out just the parenthetical description
// from progress text like "PromptGate (Preparing Telemetry Sources)".
func ExtractProgressDisplay(text string) string {
	if i := strings.Index(text, "("); i >= 0 {
		if j := strings.LastIndex(text, ")"); j > i {
			return text[i+1 : j]
		}
	}
	return text
}
