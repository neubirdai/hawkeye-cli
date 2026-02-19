package tui

import (
	"encoding/json"
	"regexp"
	"strings"

	"hawkeye-cli/internal/api"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Messages sent from stream goroutine to Bubble Tea ──────────────────────

type sessionCreatedMsg struct {
	sessionID string
}

type streamChunkMsg struct {
	contentType string
	text        string
	raw         *api.ProcessPromptResponse
}

type streamDoneMsg struct {
	sessionID string
}

type streamErrMsg struct {
	err error
}

// ─── Stream command ─────────────────────────────────────────────────────────
//
// Launches the investigation in a goroutine, streams results through a
// channel, and returns a tea.Cmd that keeps reading from that channel
// until the stream ends.

func startInvestigation(client *api.Client, projectID, sessionID, prompt string) tea.Cmd {
	return func() tea.Msg {
		// Create session if none provided
		if sessionID == "" {
			resp, err := client.NewSession(projectID)
			if err != nil {
				return streamErrMsg{err: err}
			}
			sessionID = resp.SessionUUID
		}
		// Return the session ID first, then start streaming
		return sessionCreatedMsg{sessionID: sessionID}
	}
}

// streamChannel is stored in the model so we can keep reading from it.
// We use a simple pattern: each waitForStream call reads one message
// and returns it. The model's Update dispatches another waitForStream
// after each chunk.

var activeStreamCh chan tea.Msg

func beginStream(client *api.Client, projectID, sessionID, prompt string) tea.Cmd {
	ch := make(chan tea.Msg, 64)
	activeStreamCh = ch

	go func() {
		defer close(ch)

		err := client.ProcessPromptStream(projectID, sessionID, prompt, func(resp *api.ProcessPromptResponse) {
			if resp.Message == nil || resp.Message.Content == nil {
				return
			}

			ct := resp.Message.Content.ContentType
			parts := resp.Message.Content.Parts

			switch ct {
			case "CONTENT_TYPE_PROGRESS_STATUS":
				if len(parts) > 0 {
					ch <- streamChunkMsg{contentType: ct, text: parts[0], raw: resp}
				}

			case "CONTENT_TYPE_SOURCES":
				for _, raw := range parts {
					ch <- streamChunkMsg{contentType: ct, text: raw, raw: resp}
				}

			case "CONTENT_TYPE_CHAIN_OF_THOUGHT":
				if len(parts) > 0 {
					ch <- streamChunkMsg{contentType: ct, text: parts[0], raw: resp}
				}

			case "CONTENT_TYPE_CHAT_RESPONSE":
				text := strings.Join(parts, "\n")
				text = stripHTML(text)
				ch <- streamChunkMsg{contentType: ct, text: text, raw: resp}

			case "CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS":
				ch <- streamChunkMsg{contentType: ct, text: strings.Join(parts, "\n"), raw: resp}

			case "CONTENT_TYPE_SESSION_NAME":
				if len(parts) > 0 {
					ch <- streamChunkMsg{contentType: ct, text: parts[0], raw: resp}
				}

			case "CONTENT_TYPE_ERROR_MESSAGE":
				if len(parts) > 0 {
					ch <- streamChunkMsg{contentType: ct, text: strings.Join(parts, "\n"), raw: resp}
				}

			case "CONTENT_TYPE_EXECUTION_TIME":
				if len(parts) > 0 {
					ch <- streamChunkMsg{contentType: ct, text: parts[0], raw: resp}
				}

			default:
				if len(parts) > 0 {
					ch <- streamChunkMsg{contentType: ct, text: parts[0], raw: resp}
				}
			}

			if resp.Message.EndTurn {
				ch <- streamDoneMsg{sessionID: sessionID}
			}
		})

		if err != nil {
			ch <- streamErrMsg{err: err}
		}
	}()

	return waitForStream(ch)
}

// waitForStream reads the next message from the channel.
func waitForStream(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		return msg
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	return s
}

// parseSourceLabel extracts a display label from a source JSON string.
func parseSourceLabel(raw string) string {
	var s struct {
		ID       string `json:"id"`
		Category string `json:"category"`
		Title    string `json:"title"`
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return raw
	}
	name := s.Title
	if name == "" {
		name = s.ID
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimPrefix(name, "containerinsights_")
	if s.Category != "" {
		return "[" + s.Category + "] " + name
	}
	return name
}

// parseCOTInvestigation extracts the investigation text from COT JSON.
func parseCOTFields(raw string) (description, investigation, status string) {
	var cot struct {
		Description   string `json:"description"`
		Investigation string `json:"investigation"`
		Status        string `json:"status"`
		CotStatus     string `json:"cot_status"`
	}
	if err := json.Unmarshal([]byte(raw), &cot); err != nil {
		return "", raw, ""
	}
	st := cot.CotStatus
	if st == "" {
		st = cot.Status
	}
	return cot.Description, cot.Investigation, st
}
