package tui

import (
	"fmt"
	"strconv"
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
	OutputDivider                          // Horizontal rule between major sections (COT→COT, COT→chat)
	OutputTable                            // Fully formatted table (all rows buffered + rendered)
	OutputCodeFence                        // Code block fence (``` with optional language)
	OutputCodeLine                         // Line inside a code block
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
	lastCotID     string            // tracks last COT ID for chat transition divider
	cotAccum      map[string]string // accumulated investigation text per COT ID
	cotBuffers    map[string]string // partial line buffer per COT ID
	cotDescShown  map[string]bool
	cotStepNum    int
	cotStepActive bool
	cotTextActive bool
	cotDeltaMode  bool // true if using cot_start/cot_delta/cot_end protocol

	// Chat state
	chatPrinted   int
	chatBuffer    string
	chatStreaming bool

	// Table state — rows buffered until the table ends, then emitted as OutputTable
	tableBuffer []string

	// Code block state — tracks whether we're inside a fenced code block
	inCodeBlock bool

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
		return []OutputEvent{{Type: OutputExecTime, Text: formatExecTime(msg.text)}}
	default:
		return nil
	}
}

// Flush force-flushes all pending buffers. Called on stream done/cancel.
func (sp *StreamProcessor) Flush() []OutputEvent {
	var out []OutputEvent

	// Flush table buffer first (table may end at stream boundary)
	out = append(out, sp.flushTableBuffer()...)

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

	var out []OutputEvent

	// In legacy mode (no cot_start/cot_end), progress arriving means COT is done.
	// Flush the COT buffer immediately BEFORE checking for duplicates.
	// This ensures the last COT line is printed even if progress is a duplicate.
	if sp.cotTextActive && sp.activeCotID != "" && !sp.cotDeltaMode {
		flushed := sp.flushCOTBuffer(sp.activeCotID)
		if len(flushed) > 0 {
			out = append(out, flushed...)
			// Add blank line after COT to separate from progress
			out = append(out, OutputEvent{Type: OutputBlank})
		}
		sp.cotTextActive = false
		sp.cotStepActive = false
	}

	// If chat was streaming, progress arriving means chat is done.
	// Flush the chat buffer immediately BEFORE checking for duplicates.
	if sp.chatStreaming && sp.chatBuffer != "" && strings.TrimSpace(sp.chatBuffer) != "" {
		out = append(out, OutputEvent{Type: OutputChat, Text: sp.chatBuffer})
		sp.chatBuffer = ""
		sp.chatStreaming = false
	}

	// Now check for duplicate progress (after flushing COT/chat)
	normKey := normalizeProgress(display)
	if sp.seenProgress[normKey] {
		return out // may contain flushed COT/chat even if progress is duplicate
	}
	sp.seenProgress[normKey] = true

	// In delta mode, COT is still active — queue progress to prevent interleaving
	if sp.cotTextActive && sp.cotDeltaMode {
		sp.pendingProgress = append(sp.pendingProgress, display)
		return out
	}

	out = append(out, OutputEvent{Type: OutputProgress, Text: display})
	return out
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
	// Skip this for cot_end events that are for a different (stale) COT -
	// these are late arrivals for already-superseded steps.
	isNewStep := sp.activeCotID != "" && cotID != sp.activeCotID
	isStaleCotEnd := msg.eventType == "cot_end" && isNewStep
	if isNewStep && !isStaleCotEnd {
		out = append(out, sp.flushCOTBuffer(sp.activeCotID)...)
		sp.cotStepActive = false
		sp.cotTextActive = false
		out = append(out, sp.flushPendingProgress()...)
		out = append(out, OutputEvent{Type: OutputDivider})
	}

	// Don't update activeCotID for stale cot_end events
	if sp.activeCotID != cotID && !isStaleCotEnd {
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
	// Also buffers lines that look like incomplete list items (e.g., just "-" or "1.")
	// since the backend may replace them with full content.
	// Table rows (starting with "|") are routed to the shared tableBuffer.
	printLines := func(text string) {
		combined := sp.cotBuffers[cotID] + text
		lines := strings.Split(combined, "\n")
		for i, line := range lines {
			if i < len(lines)-1 {
				trimmed := strings.TrimSpace(line)
				// Skip empty lines
				if trimmed == "" {
					continue
				}
				// Buffer incomplete list markers - backend may replace them
				// Examples: "-", "- ", "1.", "1. ", "* ", etc.
				if isIncompleteListItem(trimmed) {
					// Keep this line and all remaining lines in buffer
					sp.cotBuffers[cotID] = strings.Join(lines[i:], "\n")
					return // stop processing, everything is buffered
				}
				// Route table rows to shared table buffer; flush on non-table line
				if strings.HasPrefix(trimmed, "|") && !sp.inCodeBlock {
					sp.tableBuffer = append(sp.tableBuffer, line)
				} else {
					out = append(out, sp.flushTableBuffer()...)
					// Check for code block fences
					if strings.HasPrefix(trimmed, "```") {
						sp.inCodeBlock = !sp.inCodeBlock
						out = append(out, OutputEvent{
							Type:  OutputCodeFence,
							Text:  line,
							CotID: cotID,
						})
					} else if sp.inCodeBlock {
						out = append(out, OutputEvent{
							Type:  OutputCodeLine,
							Text:  line,
							CotID: cotID,
						})
					} else {
						out = append(out, OutputEvent{
							Type:  OutputCOTText,
							Text:  line,
							CotID: cotID,
						})
					}
				}
			} else {
				sp.cotBuffers[cotID] = line
			}
		}
	}

	switch msg.eventType {
	case "cot_start":
		sp.cotDeltaMode = true // mark that we're using delta protocol
		sp.cotStepActive = true
		sp.chatStreaming = false // reset chat streaming when COT starts
		showHeader()
		// Process any initial investigation content (e.g., "No results")
		if investigation != "" && !isTrivialContent(investigation) {
			sp.cotTextActive = true
			printLines(investigation)
		}

	case "cot_delta":
		showHeader()
		if investigation != "" {
			sp.cotTextActive = true
			printLines(investigation)
		}

	case "cot_end":
		// Only flush and clear state if this is the currently active COT.
		// Don't add divider if we're ending an old COT that's no longer active
		// (which happens when backend sends cot_end for a previous step after
		// a new step has already started).
		if cotID == sp.activeCotID {
			out = append(out, sp.flushCOTBuffer(cotID)...)
			sp.cotStepActive = false
			sp.cotTextActive = false
			out = append(out, sp.flushPendingProgress()...)
		}
		// Always record that a COT ended (for chat transition divider)
		sp.lastCotID = cotID

	default:
		// Legacy non-delta: investigation contains full accumulated text.
		sp.cotStepActive = true
		showHeader()

		if investigation != "" && !isTrivialContent(investigation) {
			prev := sp.cotAccum[cotID]
			if len(investigation) > len(prev) {
				sp.cotTextActive = true
				newText := investigation[len(prev):]
				sp.cotAccum[cotID] = investigation
				printLines(newText)
			}
		}

		// Only flush when explicitly completed.
		// Don't flush just because no new text arrived - the backend may send
		// duplicate events while streaming character by character.
		if isCompleted {
			out = append(out, sp.flushCOTBuffer(cotID)...)
			sp.cotStepActive = false
			sp.cotTextActive = false
			out = append(out, sp.flushPendingProgress()...)
		}
	}

	return out
}

// ─── Chat Response ──────────────────────────────────────────────────────────

func (sp *StreamProcessor) handleChat(msg streamChunkMsg) []OutputEvent {
	var out []OutputEvent

	// Transition from COT to Chat: flush COT state and add a section divider.
	if sp.cotStepActive || sp.cotTextActive {
		if sp.activeCotID != "" {
			out = append(out, sp.flushCOTBuffer(sp.activeCotID)...)
		}
		sp.cotStepActive = false
		sp.cotTextActive = false
		out = append(out, OutputEvent{Type: OutputDivider})
		sp.lastCotID = "" // consumed
	} else if sp.lastCotID != "" && !sp.chatStreaming {
		// COT already ended via cot_end but chat hasn't started yet - add divider
		out = append(out, OutputEvent{Type: OutputDivider})
		sp.lastCotID = "" // consumed
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
			out = append(out, sp.routeChatLine(line)...)
		} else {
			sp.chatBuffer = line
		}
	}

	return out
}

// routeChatLine routes a completed chat line — buffering table rows or flushing
// the table when a non-table line is encountered. Also handles code block state.
func (sp *StreamProcessor) routeChatLine(line string) []OutputEvent {
	trimmed := strings.TrimSpace(line)

	// Table rows (but not inside code blocks)
	if strings.HasPrefix(trimmed, "|") && !sp.inCodeBlock {
		sp.tableBuffer = append(sp.tableBuffer, line)
		return nil
	}

	// Non-table line — flush any buffered table first
	var out []OutputEvent
	if len(sp.tableBuffer) > 0 {
		out = append(out, sp.flushTableBuffer()...)
	}

	// Check for code block fences
	if strings.HasPrefix(trimmed, "```") {
		sp.inCodeBlock = !sp.inCodeBlock
		out = append(out, OutputEvent{Type: OutputCodeFence, Text: line})
	} else if sp.inCodeBlock {
		out = append(out, OutputEvent{Type: OutputCodeLine, Text: line})
	} else {
		out = append(out, OutputEvent{Type: OutputChat, Text: line})
	}
	return out
}

// flushTableBuffer emits the accumulated table rows as a single OutputTable event.
func (sp *StreamProcessor) flushTableBuffer() []OutputEvent {
	if len(sp.tableBuffer) == 0 {
		return nil
	}
	rows := sp.tableBuffer
	sp.tableBuffer = nil
	return []OutputEvent{{Type: OutputTable, Text: strings.Join(rows, "\n")}}
}

// ─── Follow-up Suggestions ─────────────────────────────────────────────────

func (sp *StreamProcessor) handleFollowUp(msg streamChunkMsg) []OutputEvent {
	var out []OutputEvent

	// Flush table buffer before follow-ups.
	out = append(out, sp.flushTableBuffer()...)

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
	var out []OutputEvent
	buf := sp.cotBuffers[cotID]
	sp.cotBuffers[cotID] = ""

	if buf != "" && strings.TrimSpace(buf) != "" {
		trimmed := strings.TrimSpace(buf)
		if strings.HasPrefix(trimmed, "|") {
			// Last partial line is a table row — add it to the table buffer,
			// then fall through to flush the whole table below.
			sp.tableBuffer = append(sp.tableBuffer, buf)
		} else {
			// Non-table line: flush any pending table rows first.
			out = append(out, sp.flushTableBuffer()...)
			out = append(out, OutputEvent{
				Type:     OutputCOTText,
				Text:     buf,
				CotID:    cotID,
				Finished: true,
			})
		}
	}

	// Always flush any remaining table rows at the COT step boundary.
	out = append(out, sp.flushTableBuffer()...)
	return out
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

// isIncompleteListItem checks if a line looks like an incomplete list marker
// that the backend may replace with full content (e.g., "-" -> "- 400").
func isIncompleteListItem(trimmed string) bool {
	// Bullet list markers: "-", "- ", "*", "* "
	if trimmed == "-" || trimmed == "*" || trimmed == "- " || trimmed == "* " {
		return true
	}
	// Numbered list markers: "1.", "1. ", "2.", etc.
	if len(trimmed) <= 3 {
		for i, c := range trimmed {
			if i == len(trimmed)-1 {
				// Last char should be '.' or '. '
				if c == '.' {
					return true
				}
			} else if c < '0' || c > '9' {
				break
			}
		}
	}
	return false
}

// formatExecTime converts execution time from milliseconds to human-readable format.
// Input is expected to be a string of milliseconds (e.g., "301213").
// Output examples: "5m 1s", "2.3s", "150ms"
func formatExecTime(ms string) string {
	millis, err := strconv.ParseInt(strings.TrimSpace(ms), 10, 64)
	if err != nil {
		return ms // return as-is if not a number
	}

	if millis < 1000 {
		return fmt.Sprintf("%dms", millis)
	}

	seconds := millis / 1000
	remainingMs := millis % 1000

	if seconds < 60 {
		if remainingMs > 0 {
			return fmt.Sprintf("%d.%ds", seconds, remainingMs/100)
		}
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	remainingSec := seconds % 60

	if minutes < 60 {
		if remainingSec > 0 {
			return fmt.Sprintf("%dm %ds", minutes, remainingSec)
		}
		return fmt.Sprintf("%dm", minutes)
	}

	hours := minutes / 60
	remainingMin := minutes % 60
	if remainingMin > 0 {
		return fmt.Sprintf("%dh %dm", hours, remainingMin)
	}
	return fmt.Sprintf("%dh", hours)
}
