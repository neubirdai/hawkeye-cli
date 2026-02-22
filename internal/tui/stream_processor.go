package tui

import (
	"strings"
)

// ─── Output types ───────────────────────────────────────────────────────────

// OutputType identifies the kind of stream output event.
type OutputType int

const (
	OutputProgress       OutputType = iota // Progress status message
	OutputCOTHeader                        // COT step header (description)
	OutputCOTExplanation                   // COT step explanation
	OutputCOTText                          // COT investigation text line
	OutputChat                             // Chat response text line
	OutputFollowUpHeader                   // "Follow-up suggestions:" header
	OutputFollowUpItem                     // Individual follow-up suggestion
	OutputSource                           // Data source label
	OutputSessionName                      // Session name
	OutputExecTime                         // Execution time
	OutputBlank                            // Blank separator line
)

// OutputEvent is a structured event emitted by the StreamProcessor.
// The consumer (model.go) decides how to render or skip each type.
type OutputEvent struct {
	Type     OutputType
	Text     string // raw content (markdown, plain text, etc.)
	CotID    string // which COT step this belongs to (COT events)
	CotDesc  string // COT step description (COTHeader only)
	Finished bool   // true when this block/step is complete
	Index    int    // 1-based index for numbered items (follow-ups)
}

// ─── StreamProcessor ────────────────────────────────────────────────────────

// StreamProcessor manages all stream buffering, gating, and block transitions.
// It accepts streamChunkMsg events and emits structured OutputEvents.
// It has no dependency on Bubble Tea.
type StreamProcessor struct {
	// COT state
	activeCotID   string
	cotAccum      map[string]string // accumulated investigation text per COT ID
	cotBuffers    map[string]string // partial line buffer per COT ID
	cotDescShown  map[string]bool
	cotStepNum    int
	cotStepActive bool
	cotTextActive bool

	// Chat state
	chatPrinted   int
	chatBuffer    string
	chatStreaming bool

	// Gating
	seenSources     map[string]bool
	seenProgress    map[string]bool
	pendingProgress []string

	// Status
	lastStatus string
}

// NewStreamProcessor creates a fresh processor.
func NewStreamProcessor() *StreamProcessor {
	return &StreamProcessor{
		cotAccum:     make(map[string]string),
		cotBuffers:   make(map[string]string),
		cotDescShown: make(map[string]bool),
		seenSources:  make(map[string]bool),
		seenProgress: make(map[string]bool),
	}
}

// LastStatus returns the latest progress text for the spinner.
func (sp *StreamProcessor) LastStatus() string {
	return sp.lastStatus
}

// Process handles a single stream chunk and returns output events.
func (sp *StreamProcessor) Process(msg streamChunkMsg) []OutputEvent {
	switch msg.contentType {
	case "CONTENT_TYPE_PROGRESS_STATUS":
		return sp.handleProgress(msg)
	case "CONTENT_TYPE_SOURCES":
		return sp.handleSource(msg)
	case "CONTENT_TYPE_CHAIN_OF_THOUGHT":
		return sp.handleCOT(msg)
	case "CONTENT_TYPE_CHAT_RESPONSE":
		return sp.handleChat(msg)
	case "CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS":
		return sp.handleFollowUp(msg)
	case "CONTENT_TYPE_SESSION_NAME":
		return []OutputEvent{{Type: OutputSessionName, Text: msg.text}}
	case "CONTENT_TYPE_ERROR_MESSAGE":
		// Suppressed: internal query retry/fix messages (e.g. SQL errors),
		// not meant for display. The web UI also ignores them.
		return nil
	case "CONTENT_TYPE_EXECUTION_TIME":
		return []OutputEvent{{Type: OutputExecTime, Text: msg.text}}
	default:
		return nil
	}
}

// Flush force-flushes all pending buffers. Called on stream done/cancel.
func (sp *StreamProcessor) Flush() []OutputEvent {
	var out []OutputEvent

	// Flush chat buffer
	if sp.chatBuffer != "" && strings.TrimSpace(sp.chatBuffer) != "" {
		out = append(out, OutputEvent{Type: OutputChat, Text: sp.chatBuffer})
		sp.chatBuffer = ""
	}

	// Flush active COT buffer
	if sp.activeCotID != "" {
		out = append(out, sp.flushCOTBuffer(sp.activeCotID)...)
	}

	// Flush pending progress
	out = append(out, sp.flushPendingProgress()...)

	sp.cotStepActive = false
	sp.cotTextActive = false
	sp.chatStreaming = false

	return out
}

// ─── Progress ───────────────────────────────────────────────────────────────

func (sp *StreamProcessor) handleProgress(msg streamChunkMsg) []OutputEvent {
	display := extractProgressDisplay(msg.text)
	sp.lastStatus = display

	normKey := normalizeProgress(display)
	if sp.seenProgress[normKey] {
		return nil
	}
	sp.seenProgress[normKey] = true

	if sp.cotTextActive || sp.chatStreaming {
		// Queue for later to prevent interleaving mid-paragraph.
		sp.pendingProgress = append(sp.pendingProgress, display)
		return nil
	}

	return []OutputEvent{{Type: OutputProgress, Text: display}}
}

// ─── Sources ────────────────────────────────────────────────────────────────

func (sp *StreamProcessor) handleSource(msg streamChunkMsg) []OutputEvent {
	if sp.cotStepActive || sp.chatStreaming {
		return nil
	}

	label := parseSourceLabel(msg.text)
	if sp.seenSources[label] {
		return nil
	}
	sp.seenSources[label] = true

	return []OutputEvent{{Type: OutputSource, Text: label}}
}

// ─── Chain of Thought ───────────────────────────────────────────────────────

func (sp *StreamProcessor) handleCOT(msg streamChunkMsg) []OutputEvent {
	id, desc, explanation, investigation, status := parseCOTFields(msg.text)
	cotID := id
	if cotID == "" {
		cotID = desc
	}
	if cotID == "" {
		cotID = "_default"
	}

	statusUpper := strings.ToUpper(status)
	isCompleted := strings.Contains(statusUpper, "COMPLETED")

	var out []OutputEvent

	// Detect new COT step: flush the previous step.
	isNewStep := sp.activeCotID != "" && cotID != sp.activeCotID
	if isNewStep {
		out = append(out, sp.flushCOTBuffer(sp.activeCotID)...)
		sp.cotStepActive = false
		sp.cotTextActive = false
		out = append(out, sp.flushPendingProgress()...)
		out = append(out, OutputEvent{Type: OutputBlank})
	}

	if sp.activeCotID != cotID {
		sp.activeCotID = cotID
	}

	// Helper: show header if not yet shown for this COT.
	showHeader := func() {
		if !sp.cotDescShown[cotID] && desc != "" {
			sp.cotDescShown[cotID] = true
			sp.cotStepNum++
			out = append(out, OutputEvent{
				Type:    OutputCOTHeader,
				Text:    desc,
				CotID:   cotID,
				CotDesc: desc,
			})
			if explanation != "" {
				out = append(out, OutputEvent{
					Type:  OutputCOTExplanation,
					Text:  explanation,
					CotID: cotID,
				})
			}
		}
	}

	// Helper: print complete lines, buffer the last partial line.
	printLines := func(text string) {
		combined := sp.cotBuffers[cotID] + text
		lines := strings.Split(combined, "\n")
		for i, line := range lines {
			if i < len(lines)-1 {
				if strings.TrimSpace(line) != "" {
					out = append(out, OutputEvent{
						Type:  OutputCOTText,
						Text:  line,
						CotID: cotID,
					})
				}
			} else {
				sp.cotBuffers[cotID] = line
			}
		}
	}

	switch msg.eventType {
	case "cot_start":
		sp.cotStepActive = true
		showHeader()

	case "cot_delta":
		showHeader()
		if investigation != "" {
			sp.cotTextActive = true
			printLines(investigation)
		}

	case "cot_end":
		out = append(out, sp.flushCOTBuffer(cotID)...)
		sp.cotStepActive = false
		sp.cotTextActive = false
		out = append(out, sp.flushPendingProgress()...)

	default:
		// Legacy non-delta: investigation contains full accumulated text.
		sp.cotStepActive = true
		showHeader()

		hasNewText := false
		if investigation != "" && !isTrivialContent(investigation) {
			prev := sp.cotAccum[cotID]
			if len(investigation) > len(prev) {
				hasNewText = true
				sp.cotTextActive = true
				newText := investigation[len(prev):]
				sp.cotAccum[cotID] = investigation
				printLines(newText)
			}
		}

		// Flush when completed or when no new text but buffer has content.
		shouldFlush := isCompleted || (!hasNewText && sp.cotBuffers[cotID] != "")
		if shouldFlush {
			out = append(out, sp.flushCOTBuffer(cotID)...)
			if isCompleted {
				sp.cotStepActive = false
				sp.cotTextActive = false
				out = append(out, sp.flushPendingProgress()...)
			}
		}
	}

	return out
}

// ─── Chat Response ──────────────────────────────────────────────────────────

func (sp *StreamProcessor) handleChat(msg streamChunkMsg) []OutputEvent {
	var out []OutputEvent

	// Transition from COT to Chat: flush COT state.
	if sp.cotStepActive || sp.cotTextActive {
		if sp.activeCotID != "" {
			out = append(out, sp.flushCOTBuffer(sp.activeCotID)...)
		}
		sp.cotStepActive = false
		sp.cotTextActive = false
	}
	out = append(out, sp.flushPendingProgress()...)

	sp.chatStreaming = true

	newText := ""
	if msg.eventType == "chat_delta" {
		newText = msg.text
		sp.chatPrinted += len(newText)
	} else {
		if len(msg.text) > sp.chatPrinted {
			newText = msg.text[sp.chatPrinted:]
			sp.chatPrinted = len(msg.text)
		}
	}

	if newText == "" {
		return out // may still have flush events from transition
	}

	combined := sp.chatBuffer + newText
	lines := strings.Split(combined, "\n")
	for i, line := range lines {
		if i < len(lines)-1 {
			out = append(out, OutputEvent{Type: OutputChat, Text: line})
		} else {
			sp.chatBuffer = line
		}
	}

	return out
}

// ─── Follow-up Suggestions ─────────────────────────────────────────────────

func (sp *StreamProcessor) handleFollowUp(msg streamChunkMsg) []OutputEvent {
	var out []OutputEvent

	// Flush chat buffer before follow-ups.
	if sp.chatBuffer != "" && strings.TrimSpace(sp.chatBuffer) != "" {
		out = append(out, OutputEvent{Type: OutputChat, Text: sp.chatBuffer})
		sp.chatBuffer = ""
	}
	sp.chatStreaming = false
	sp.cotStepActive = false
	sp.cotTextActive = false

	parts := strings.Split(msg.text, "\n")
	out = append(out, OutputEvent{Type: OutputBlank})
	out = append(out, OutputEvent{Type: OutputFollowUpHeader, Text: "Follow-up suggestions:"})
	idx := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			idx++
			out = append(out, OutputEvent{
				Type:  OutputFollowUpItem,
				Text:  p,
				Index: idx,
			})
		}
	}

	return out
}

// ─── Internal helpers ───────────────────────────────────────────────────────

func (sp *StreamProcessor) flushCOTBuffer(cotID string) []OutputEvent {
	buf := sp.cotBuffers[cotID]
	if buf != "" && strings.TrimSpace(buf) != "" {
		sp.cotBuffers[cotID] = ""
		return []OutputEvent{{
			Type:     OutputCOTText,
			Text:     buf,
			CotID:    cotID,
			Finished: true,
		}}
	}
	sp.cotBuffers[cotID] = ""
	return nil
}

func (sp *StreamProcessor) flushPendingProgress() []OutputEvent {
	if len(sp.pendingProgress) == 0 {
		return nil
	}
	var out []OutputEvent
	for _, display := range sp.pendingProgress {
		out = append(out, OutputEvent{Type: OutputProgress, Text: display})
	}
	sp.pendingProgress = nil
	return out
}

// ─── Shared helpers (used by processor) ─────────────────────────────────────

// isTrivialContent checks if the text is just a placeholder/status message
// that shouldn't be displayed as investigation content.
func isTrivialContent(text string) bool {
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

// normalizeProgress deduplicates progress messages that differ only in counts.
func normalizeProgress(display string) string {
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

// extractProgressDisplay pulls out just the parenthetical description
// from progress text like "PromptGate (Preparing Telemetry Sources)".
func extractProgressDisplay(text string) string {
	if i := strings.Index(text, "("); i >= 0 {
		if j := strings.LastIndex(text, ")"); j > i {
			return text[i+1 : j]
		}
	}
	return text
}
