package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Spinner frames for activity indication
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StreamDisplay handles clean terminal output for SSE streams.
type StreamDisplay struct {
	debug bool

	// Progress tracking
	lastProgress string
	seenProgress map[string]bool
	spinnerIdx   int
	activityUp   bool

	// Source deduplication
	seenSourceIDs  map[string]bool
	sourcesPrinted bool

	// Chain-of-thought state.
	// Server sends ALL COT steps in the parts[] array of every COT event.
	// parts[0] = step 1, parts[1] = step 2, etc.  The CLI must find the
	// IN_PROGRESS step and stream its investigation text.
	cotRound       int    // step number (1-based)
	cotAccumulated string // full investigation text for current round
	cotPrintedLen  int    // how many chars we've printed
	cotHeaderUp    bool   // true once the step header is printed
	cotEverStarted bool   // true after first COT event
	currentCotID   string // ID of the COT step we're currently displaying

	// Rich metadata for current round
	cotDescription string
	cotExplanation string
	cotCategory    string
	cotStatus      string
	cotSources     []string

	lastContentType string

	// Chat response: delta-aware printing
	chatAccumulated string
	chatPrintedLen  int
	chatHeaderUp    bool

	// Final answer accumulator
	FinalAnswer string
	SessionUUID string

	// Markdown colorizer for streaming output
	md mdPrinter
}

func NewStreamDisplay(debug bool) *StreamDisplay {
	return &StreamDisplay{
		debug:         debug,
		seenSourceIDs: make(map[string]bool),
		seenProgress:  make(map[string]bool),
	}
}

// HandleEvent is the StreamCallback for ProcessPromptStream.
func (d *StreamDisplay) HandleEvent(resp *ProcessPromptResponse) {
	if resp.SessionUUID != "" {
		d.SessionUUID = resp.SessionUUID
	}

	msg := resp.Message
	if msg == nil || msg.Content == nil {
		return
	}

	ct := msg.Content.ContentType
	parts := msg.Content.Parts
	eventType := resp.EventType

	// When chat response starts, finalize any active COT
	if ct == "CONTENT_TYPE_CHAT_RESPONSE" && d.cotEverStarted && d.cotPrintedLen > 0 {
		d.endCOTRound()
	}

	// When content type changes FROM chat response, finish the chat block
	if d.chatHeaderUp && ct != "CONTENT_TYPE_CHAT_RESPONSE" {
		d.finishChatBlock()
	}

	// Clear spinner before non-progress content
	if ct != "CONTENT_TYPE_PROGRESS_STATUS" && d.activityUp {
		d.clearActivity()
	}

	// Ensure a newline after streamed COT/chat text before printing other content.
	// The investigation text streams via fmt.Print() and may not end with '\n'.
	if ct != "CONTENT_TYPE_CHAIN_OF_THOUGHT" && d.cotEverStarted && d.cotPrintedLen > 0 {
		if d.cotAccumulated != "" && !strings.HasSuffix(d.cotAccumulated, "\n") {
			fmt.Println()
		}
	}
	if ct != "CONTENT_TYPE_CHAT_RESPONSE" && d.chatAccumulated != "" && !strings.HasSuffix(d.chatAccumulated, "\n") {
		fmt.Println()
	}

	d.lastContentType = ct

	switch ct {
	case "CONTENT_TYPE_PROGRESS_STATUS":
		d.handleProgress(parts)

	case "CONTENT_TYPE_SOURCES":
		d.handleSources(parts)

	case "CONTENT_TYPE_CHAIN_OF_THOUGHT":
		d.handleChainOfThought(parts, eventType, msg.Metadata)

	case "CONTENT_TYPE_CHAT_RESPONSE":
		d.handleChatResponse(parts, msg.Metadata)

	case "CONTENT_TYPE_SESSION_NAME":
		if len(parts) > 0 {
			fmt.Printf("  📛 %s\n", parts[0])
		}

	case "CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS":
		fmt.Println()
		fmt.Println("  💡 Follow-up suggestions:")
		for i, p := range parts {
			fmt.Printf("     %d. %s\n", i+1, p)
		}

	case "CONTENT_TYPE_EXECUTION_TIME":
		if len(parts) > 0 {
			fmt.Printf("  ⏱  %s\n", parts[0])
		}

	case "CONTENT_TYPE_ERROR_MESSAGE":
		d.handleErrorMessage(parts)

	case "CONTENT_TYPE_ALTERNATE_QUESTIONS":
		if len(parts) > 0 {
			fmt.Println()
			fmt.Println("  ❓ Related questions:")
			for i, p := range parts {
				var q struct {
					Question string `json:"question"`
				}
				if err := json.Unmarshal([]byte(p), &q); err == nil && q.Question != "" {
					fmt.Printf("     %d. %s\n", i+1, q.Question)
				} else {
					fmt.Printf("     %d. %s\n", i+1, p)
				}
			}
		}

	case "CONTENT_TYPE_SHIFT_FOCUS_TO_SUMMARY":
		if d.cotEverStarted && d.cotPrintedLen > 0 {
			d.endCOTRound()
		}
	}

	if msg.EndTurn {
		d.flush()
	}
}

func (d *StreamDisplay) flush() {
	if d.activityUp {
		d.clearActivity()
	}
	d.md.flush() // flush any remaining buffered markdown
	if d.cotEverStarted && d.cotPrintedLen > 0 {
		d.endCOTRound()
	}
	if d.chatHeaderUp {
		d.finishChatBlock()
	}
}

// --- Progress ---

func (d *StreamDisplay) handleProgress(parts []string) {
	if len(parts) == 0 {
		return
	}
	text := parts[0]
	normKey := normalizeProgress(text)

	if d.seenProgress[normKey] {
		d.showActivity(text)
		return
	}

	d.seenProgress[normKey] = true

	if isActivityOnly(text) {
		d.showActivity(text)
		return
	}

	if d.activityUp {
		d.clearActivity()
	}

	d.lastProgress = text
	fmt.Printf("  ⟳ %s\n", text)
}

func (d *StreamDisplay) showActivity(text string) {
	frame := spinnerFrames[d.spinnerIdx%len(spinnerFrames)]
	d.spinnerIdx++
	display := text
	if len(display) > 70 {
		display = display[:67] + "..."
	}
	fmt.Printf("\r  %s %s%-20s", frame, display, "")
	d.activityUp = true
}

func (d *StreamDisplay) clearActivity() {
	if d.activityUp {
		fmt.Printf("\r%-80s\r", "")
		d.activityUp = false
	}
}

func isActivityOnly(text string) bool {
	return strings.HasPrefix(text, "(")
}

func normalizeProgress(text string) string {
	if strings.HasPrefix(text, "(Found ") && strings.HasSuffix(text, " results)") {
		return "(Found N results)"
	}
	if strings.Contains(text, "result streams") {
		return "(Analyzing N result streams)"
	}
	if strings.Contains(text, "datas") && strings.Contains(text, "ources") {
		return "(Selected N data sources)"
	}
	return text
}

// --- Error Messages ---

// errorJSON matches the structure the server sends in ERROR_MESSAGE events.
// Contains query SQL we do NOT want to display.
type errorJSON struct {
	Question string   `json:"question"`
	Query    []string `json:"query"`
	Error    string   `json:"error"`
	Status   string   `json:"status"`
}

// handleErrorMessage parses error events and shows only the question + error,
// NOT the raw SQL queries.
func (d *StreamDisplay) handleErrorMessage(parts []string) {
	for _, p := range parts {
		// Try to parse as structured error JSON
		var errObj errorJSON
		if err := json.Unmarshal([]byte(p), &errObj); err == nil && errObj.Question != "" {
			// Structured error — show question and error, suppress SQL
			fmt.Printf("  ✗ %s", errObj.Question)
			if errObj.Error != "" {
				fmt.Printf(" — %s", errObj.Error)
			}
			if errObj.Status != "" && errObj.Status != "error" {
				fmt.Printf(" (%s)", errObj.Status)
			}
			fmt.Println()
			continue
		}

		// Plain text error — show as-is, but skip if it looks like raw JSON with queries
		if strings.Contains(p, `"query"`) && strings.Contains(p, "SELECT ") {
			// Looks like unparsed query JSON — skip it
			continue
		}

		fmt.Printf("  ✗ %s\n", p)
	}
}

// --- Sources ---

type sourceJSON struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Title    string `json:"title"`
}

func (d *StreamDisplay) handleSources(parts []string) {
	var newSources []sourceJSON

	for _, raw := range parts {
		var s sourceJSON
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			if !d.seenSourceIDs[raw] {
				d.seenSourceIDs[raw] = true
				newSources = append(newSources, sourceJSON{Title: raw})
			}
			continue
		}
		key := s.ID
		if key == "" {
			key = s.Title
		}
		if d.seenSourceIDs[key] {
			continue
		}
		d.seenSourceIDs[key] = true
		newSources = append(newSources, s)
	}

	if len(newSources) == 0 {
		return
	}

	if !d.sourcesPrinted {
		fmt.Println()
		fmt.Println("  📎 Data Sources:")
		d.sourcesPrinted = true
	}

	for _, s := range newSources {
		label := formatSourceLabel(s)
		fmt.Printf("     • %s\n", label)
	}
}

func formatSourceLabel(s sourceJSON) string {
	cat := s.Category
	name := s.Title
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimPrefix(name, "containerinsights_")
	if cat != "" {
		return fmt.Sprintf("[%s] %s", cat, name)
	}
	return name
}

// --- Chain of Thought ---

type cotJSON struct {
	ID            string   `json:"id"`
	Category      string   `json:"category"`
	Description   string   `json:"description"`
	Explanation   string   `json:"explanation"`
	Investigation string   `json:"investigation"`
	Status        string   `json:"status"`
	Sources       []string `json:"sources_involved"`
}

// handleChainOfThought routes COT events.
// The server sends ALL COT steps in the parts[] array. We must find the
// active (IN_PROGRESS) step and stream its investigation text.
func (d *StreamDisplay) handleChainOfThought(parts []string, eventType string, meta *Metadata) {
	if len(parts) == 0 {
		return
	}

	isDelta := meta != nil && meta.IsDeltaTrue()

	// --- Delta-mode event types (cot_start / cot_delta / cot_end) ---
	// These still only carry one COT per event, so parts[0] is fine.
	switch eventType {
	case "cot_start":
		var cot cotJSON
		if err := json.Unmarshal([]byte(parts[0]), &cot); err != nil {
			return
		}
		d.startNewRound(cot)
		d.ensureHeader()
		return
	case "cot_delta":
		var cot cotJSON
		if err := json.Unmarshal([]byte(parts[0]), &cot); err != nil {
			return
		}
		d.onCOTDelta(cot)
		return
	case "cot_end":
		var cot cotJSON
		if err := json.Unmarshal([]byte(parts[0]), &cot); err != nil {
			return
		}
		d.updateCOTMetadata(cot)
		return
	}

	// --- Legacy mode (all events are evt=message) ---
	// Parse ALL parts to find the active step.
	var allCots []cotJSON
	for _, raw := range parts {
		var cot cotJSON
		if err := json.Unmarshal([]byte(raw), &cot); err != nil {
			continue
		}
		allCots = append(allCots, cot)
	}
	if len(allCots) == 0 {
		return
	}

	// Find the IN_PROGRESS step — that's the one actively being investigated.
	// Fall back to the last step if none is explicitly IN_PROGRESS.
	activeCot := allCots[len(allCots)-1]
	for _, cot := range allCots {
		if cot.Status == "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS" || cot.Status == "IN_PROGRESS" {
			activeCot = cot
			break
		}
	}

	if isDelta {
		d.onCOTDelta(activeCot)
	} else {
		d.onCOTFullText(activeCot)
	}
}

// startNewRound ends any previous round and initializes state for a new one.
func (d *StreamDisplay) startNewRound(cot cotJSON) {
	if d.cotEverStarted && d.cotPrintedLen > 0 {
		d.endCOTRound()
	}

	d.cotEverStarted = true
	d.cotRound++
	d.cotAccumulated = ""
	d.cotPrintedLen = 0
	d.cotHeaderUp = false
	d.cotDescription = ""
	d.cotExplanation = ""
	d.cotCategory = ""
	d.cotStatus = ""
	d.cotSources = nil
	d.currentCotID = cot.ID

	d.updateCOTMetadata(cot)
}

// updateCOTMetadata stores non-empty fields from a COT object.
func (d *StreamDisplay) updateCOTMetadata(cot cotJSON) {
	if cot.Description != "" {
		d.cotDescription = cot.Description
	}
	if cot.Explanation != "" {
		d.cotExplanation = cot.Explanation
	}
	if cot.Category != "" {
		d.cotCategory = cot.Category
	}
	if cot.Status != "" {
		d.cotStatus = cot.Status
	}
	if len(cot.Sources) > 0 {
		d.cotSources = cot.Sources
	}
}

// ensureHeader prints the step header if not already printed.
func (d *StreamDisplay) ensureHeader() {
	if d.cotHeaderUp {
		return
	}
	if d.activityUp {
		d.clearActivity()
	}

	fmt.Println()

	// Step number + category
	cat := formatCOTCategory(d.cotCategory)
	if cat != "" {
		fmt.Printf("  🔍 Investigation step %d  [%s]\n", d.cotRound, cat)
	} else {
		fmt.Printf("  🔍 Investigation step %d\n", d.cotRound)
	}

	// Explanation (primary — the short summary shown in UI sidebar)
	if d.cotExplanation != "" {
		fmt.Printf("     %s\n", d.cotExplanation)
	}

	// Description (secondary — the detailed scope shown in UI tooltip)
	if d.cotDescription != "" {
		if d.cotExplanation != "" {
			fmt.Printf("     ↳ %s\n", d.cotDescription)
		} else {
			fmt.Printf("     %s\n", d.cotDescription)
		}
	}

	fmt.Println()
	d.cotHeaderUp = true
}

// onCOTDelta handles incremental investigation text (for future server support).
func (d *StreamDisplay) onCOTDelta(cot cotJSON) {
	delta := cot.Investigation
	if delta == "" {
		return
	}

	if !d.cotEverStarted {
		d.startNewRound(cot)
	}

	d.updateCOTMetadata(cot)
	d.ensureHeader()

	if d.activityUp {
		d.clearActivity()
	}

	d.md.printMarkdown(delta)
	d.cotAccumulated += delta
	d.cotPrintedLen = len(d.cotAccumulated)
}

// onCOTFullText handles the legacy server behavior:
// Every COT event carries the full parts[] array. The active step's
// investigation field contains the FULL accumulated text. When the
// active step changes (different ID), we start a new round.
func (d *StreamDisplay) onCOTFullText(cot cotJSON) {
	fullText := cot.Investigation
	fullLen := len(fullText)

	// Detect new round — primary signal is the COT ID changing.
	isNewRound := false

	if d.cotEverStarted && cot.ID != "" && cot.ID != d.currentCotID {
		isNewRound = true
	}

	// Fallback heuristics for when IDs aren't available
	if !isNewRound && d.cotEverStarted && d.cotPrintedLen > 0 && fullLen > 0 && fullLen < d.cotPrintedLen {
		isNewRound = true
	}

	if isNewRound {
		d.startNewRound(cot)
	} else if !d.cotEverStarted {
		d.startNewRound(cot)
	} else {
		// Same round — update metadata (status, sources may update mid-round)
		d.updateCOTMetadata(cot)
	}

	// Print any new text beyond what we've already printed
	if fullText != "" && fullLen > d.cotPrintedLen {
		d.ensureHeader()

		if d.activityUp {
			d.clearActivity()
		}

		newText := fullText[d.cotPrintedLen:]
		d.md.printMarkdown(newText)
		d.cotAccumulated = fullText
		d.cotPrintedLen = fullLen
	}
}

// endCOTRound prints the footer and resets per-round state.
func (d *StreamDisplay) endCOTRound() {
	if d.cotPrintedLen > 0 {
		// Flush any buffered partial markdown line
		d.md.flush()

		// Ensure we're on a fresh line after streamed text
		if d.cotAccumulated != "" && !strings.HasSuffix(d.cotAccumulated, "\n") {
			fmt.Println()
		}

		// Step footer: status + source count
		statusLabel := formatCOTStatus(d.cotStatus)
		srcCount := len(d.cotSources)
		footer := []string{}
		if statusLabel != "" {
			footer = append(footer, statusLabel)
		}
		if srcCount > 0 {
			footer = append(footer, fmt.Sprintf("%d sources consulted", srcCount))
		}
		if len(footer) > 0 {
			fmt.Println()
			fmt.Printf("     %s\n", strings.Join(footer, " · "))
		}
		fmt.Println()
	}

	// Reset per-round state
	d.cotAccumulated = ""
	d.cotPrintedLen = 0
	d.cotHeaderUp = false
	d.cotDescription = ""
	d.cotExplanation = ""
	d.cotCategory = ""
	d.cotStatus = ""
	d.cotSources = nil
}

// formatCOTCategory returns a clean category label.
func formatCOTCategory(category string) string {
	if category == "" {
		return ""
	}
	cat := strings.TrimPrefix(category, "CATEGORY_")
	cat = strings.ReplaceAll(cat, "_", " ")
	cat = strings.ToLower(cat)
	if len(cat) > 0 {
		cat = strings.ToUpper(cat[:1]) + cat[1:]
	}
	return cat
}

// formatCOTStatus returns a display-friendly status label.
func formatCOTStatus(status string) string {
	switch status {
	case "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS", "IN_PROGRESS":
		return "⟳ In progress"
	case "CHAIN_OF_THOUGHT_STATUS_DONE", "DONE":
		return "✓ Done"
	case "CHAIN_OF_THOUGHT_STATUS_ERROR", "ERROR":
		return "✗ Error"
	case "CHAIN_OF_THOUGHT_STATUS_CANCELLED", "CANCELLED":
		return "⊘ Cancelled"
	default:
		if status != "" {
			return status
		}
		return ""
	}
}

// --- Chat Response ---

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func (d *StreamDisplay) handleChatResponse(parts []string, meta *Metadata) {
	if len(parts) == 0 {
		return
	}

	isDelta := meta != nil && meta.IsDeltaTrue()

	if isDelta {
		delta := parts[0]
		delta = stripHTML(delta)
		if delta == "" {
			return
		}
		if d.activityUp {
			d.clearActivity()
		}
		if !d.chatHeaderUp {
			d.printChatHeader()
		}
		d.md.printMarkdown(delta)
		d.chatAccumulated += delta
		d.chatPrintedLen = len(d.chatAccumulated)
		d.FinalAnswer = strings.TrimSpace(d.chatAccumulated)
	} else {
		text := strings.Join(parts, "\n")
		text = stripHTML(text)
		if text == "" {
			return
		}
		if len(text) > d.chatPrintedLen {
			newText := text[d.chatPrintedLen:]
			if d.activityUp {
				d.clearActivity()
			}
			if !d.chatHeaderUp {
				d.printChatHeader()
			}
			d.md.printMarkdown(newText)
			d.chatPrintedLen = len(text)
		}
		d.FinalAnswer = strings.TrimSpace(text)
	}
}

func (d *StreamDisplay) printChatHeader() {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  💬 Response")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	d.chatHeaderUp = true
}

func (d *StreamDisplay) finishChatBlock() {
	if !d.chatHeaderUp {
		return
	}
	// Flush any buffered partial markdown line
	d.md.flush()
	// Ensure we're on a fresh line after streamed text
	if d.chatAccumulated != "" && !strings.HasSuffix(d.chatAccumulated, "\n") {
		fmt.Println()
	}
	fmt.Println()
	d.chatHeaderUp = false
	d.chatPrintedLen = 0
	d.chatAccumulated = ""
}

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	return s
}
