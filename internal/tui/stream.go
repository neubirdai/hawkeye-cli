package tui

import (
	"encoding/json"
	"strings"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/service"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Messages sent from stream goroutine to Bubble Tea ──────────────────────

type sessionCreatedMsg struct {
	sessionID string
}

type streamChunkMsg struct {
	contentType string
	eventType   string
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

func startInvestigation(client api.HawkeyeAPI, projectID, sessionID, prompt string) tea.Cmd {
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

func beginStream(client api.HawkeyeAPI, projectID, sessionID, prompt string) tea.Cmd {
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
				et := resp.EventType
				switch et {
				case "cot_start", "cot_delta", "cot_end":
					// Delta protocol: parts[0] is the active COT
					if len(parts) > 0 {
						ch <- streamChunkMsg{contentType: ct, eventType: et, text: parts[0], raw: resp}
					}
				default:
					// Legacy: server sends ALL COT steps in parts[].
					// Parse all parts, find the IN_PROGRESS step (the one
					// actively being investigated), and send only that one.
					// This mirrors what stream_display.go does.
					activePart := findActiveCOTPart(parts)
					if activePart != "" {
						ch <- streamChunkMsg{contentType: ct, eventType: et, text: activePart, raw: resp}
					}
				}

			case "CONTENT_TYPE_CHAT_RESPONSE":
				isDelta := resp.Message.Metadata != nil && resp.Message.Metadata.IsDeltaTrue()
				if isDelta {
					// Delta mode: parts[0] is the new fragment
					if len(parts) > 0 {
						text := service.StripHTML(parts[0])
						ch <- streamChunkMsg{contentType: ct, eventType: "chat_delta", text: text, raw: resp}
					}
				} else {
					// Full text mode: join all parts
					text := strings.Join(parts, "\n")
					text = service.StripHTML(text)
					ch <- streamChunkMsg{contentType: ct, eventType: "chat_full", text: text, raw: resp}
				}

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

// parseCOTFields extracts the investigation text from COT JSON.
func parseCOTFields(raw string) (id, description, explanation, investigation, status string) {
	var cot struct {
		ID            string `json:"id"`
		Description   string `json:"description"`
		Explanation   string `json:"explanation"`
		Investigation string `json:"investigation"`
		Status        string `json:"status"`
		CotStatus     string `json:"cot_status"`
	}
	if err := json.Unmarshal([]byte(raw), &cot); err != nil {
		return "", "", "", raw, ""
	}
	st := cot.CotStatus
	if st == "" {
		st = cot.Status
	}
	return cot.ID, cot.Description, cot.Explanation, cot.Investigation, st
}

// findActiveCOTPart parses all COT JSON parts from a legacy event and returns
// the raw JSON string for the IN_PROGRESS step. Falls back to the last step.
// This prevents the TUI from processing every completed step on every event.
func findActiveCOTPart(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	type cotInfo struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	bestIdx := len(parts) - 1 // fallback: last part
	for i, raw := range parts {
		var c cotInfo
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			continue
		}
		if c.Status == "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS" || c.Status == "IN_PROGRESS" {
			bestIdx = i
			break
		}
	}
	return parts[bestIdx]
}
