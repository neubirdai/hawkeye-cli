package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
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

	// Background spinner: keeps animating during dead air
	spinnerMu        sync.Mutex
	activityText     string        // text currently shown on the spinner line
	stopSpinner      chan struct{} // closed to stop the background ticker
	spinnerRunning   bool          // true when background goroutine is alive
	contentStreaming bool          // true while COT/chat text is being printed — spinner pauses

	// Source deduplication
	seenSourceIDs  map[string]bool
	sourcesPrinted bool

	// Error deduplication
	seenErrors map[string]bool

	// Chain-of-thought state.
	// Server sends ALL COT steps in the parts[] array of every COT event.
	// parts[0] = step 1, parts[1] = step 2, etc.  The CLI must find the
	// IN_PROGRESS step and stream its investigation text.
	cotRound         int    // step number (1-based)
	cotAccumulated   string // full investigation text for current round
	cotPrintedLen    int    // how many chars we've printed
	cotHeaderUp      bool   // true once the step header is printed
	cotEverStarted   bool   // true after first COT event
	currentCotID     string // ID of the COT step we're currently displaying
	cotSeparatorDone bool   // true once the separator line after COT content is printed

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
	FinalAnswer         string
	SessionUUID         string
	FollowUpSuggestions []string

	// Markdown colorizer for streaming output
	md mdPrinter
}

func NewStreamDisplay(debug bool) *StreamDisplay {
	return &StreamDisplay{
		debug:         debug,
		seenSourceIDs: make(map[string]bool),
		seenProgress:  make(map[string]bool),
		seenErrors:    make(map[string]bool),
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
		// Print a separator line once after COT content ends
		if !d.cotSeparatorDone {
			fmt.Println()
			fmt.Printf(" %s──────────────────────────────────────────────────────────────────────────%s\n", ansiDim, ansiReset)
			d.cotSeparatorDone = true
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
			fmt.Println()
			fmt.Printf(" %s━━ 📛 %s ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", ansiDim, parts[0], ansiReset)
		}

	case "CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS":
		d.FollowUpSuggestions = nil
		fmt.Println()
		fmt.Println("  💡 Follow-up suggestions:")
		for i, p := range parts {
			fmt.Printf("     %d. %s\n", i+1, p)
			d.FollowUpSuggestions = append(d.FollowUpSuggestions, p)
		}

	case "CONTENT_TYPE_EXECUTION_TIME":
		if len(parts) > 0 {
			fmt.Printf("  ⏱  %s\n", parts[0])
		}

	case "CONTENT_TYPE_ERROR_MESSAGE":
		// The web UI does nothing with ERROR_MESSAGE events (no-op).
		// These are internal query retry/fix messages not meant for display.
		// Do nothing.

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

// Stop cleans up the background spinner goroutine.
// Call this when the stream is done.
func (d *StreamDisplay) Stop() {
	d.clearActivity()
}

// Reset prepares the display for a new prompt in the same session.
// Keeps SessionUUID and debug, resets everything else.
func (d *StreamDisplay) Reset() {
	d.Stop()
	d.lastProgress = ""
	d.seenProgress = make(map[string]bool)
	d.spinnerIdx = 0
	d.seenSourceIDs = make(map[string]bool)
	d.sourcesPrinted = false
	d.seenErrors = make(map[string]bool)
	d.cotRound = 0
	d.cotAccumulated = ""
	d.cotPrintedLen = 0
	d.cotHeaderUp = false
	d.cotEverStarted = false
	d.currentCotID = ""
	d.cotSeparatorDone = false
	d.cotDescription = ""
	d.cotExplanation = ""
	d.cotCategory = ""
	d.cotStatus = ""
	d.cotSources = nil
	d.lastContentType = ""
	d.chatAccumulated = ""
	d.chatPrintedLen = 0
	d.chatHeaderUp = false
	d.FinalAnswer = ""
	d.FollowUpSuggestions = nil
	d.md = mdPrinter{}
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
	display := extractProgressDescription(text)
	fmt.Printf("  %s●%s %s\n", ansiDim, ansiReset, display)

	// Start a spinner below the permanent line so there's
	// visible animation during dead air between events.
	d.showActivity(text)
}

func (d *StreamDisplay) showActivity(text string) {
	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	d.activityText = extractProgressDescription(text)
	d.renderSpinnerFrame()
	d.activityUp = true

	// Start background ticker if not already running
	if !d.spinnerRunning {
		d.stopSpinner = make(chan struct{})
		d.spinnerRunning = true
		go d.runSpinnerLoop(d.stopSpinner)
	}
}

func (d *StreamDisplay) clearActivity() {
	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if !d.activityUp {
		return
	}

	// Stop the background ticker
	if d.spinnerRunning {
		close(d.stopSpinner)
		d.spinnerRunning = false
	}

	fmt.Printf("\r\033[2K")
	d.activityUp = false
	d.activityText = ""
}

// renderSpinnerFrame writes one frame to the terminal. Caller must hold spinnerMu.
func (d *StreamDisplay) renderSpinnerFrame() {
	frame := spinnerFrames[d.spinnerIdx%len(spinnerFrames)]
	d.spinnerIdx++
	display := d.activityText
	if len(display) > 70 {
		display = display[:67] + "..."
	}
	fmt.Printf("\r  %s %s%-20s", frame, display, "")
}

// runSpinnerLoop animates the spinner in the background every 120ms.
func (d *StreamDisplay) runSpinnerLoop(stop chan struct{}) {
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			d.spinnerMu.Lock()
			if d.activityUp && !d.contentStreaming {
				d.renderSpinnerFrame()
			}
			d.spinnerMu.Unlock()
		}
	}
}

func isActivityOnly(text string) bool {
	return strings.HasPrefix(text, "(")
}

// extractProgressDescription extracts only the meaningful description
// from progress messages, stripping internal filter/stage names.
// "SplitAnswer (Analyzing Telemetry)" → "Analyzing Telemetry"
// "(Found 9 results)"                → "Found 9 results"
// "Analyzing Telemetry"              → "Analyzing Telemetry"
func extractProgressDescription(text string) string {
	// Messages like "(Consulting logs and metrics)" — strip parens
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		return text[1 : len(text)-1]
	}
	// Messages like "FilterName (Description)" — extract description
	if idx := strings.Index(text, " ("); idx >= 0 {
		rest := text[idx+2:]
		if end := strings.LastIndex(rest, ")"); end >= 0 {
			return rest[:end]
		}
	}
	return text
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

	// Group sources by category
	groups := make(map[string][]string)
	var order []string
	for _, s := range newSources {
		cat := strings.ToLower(s.Category)
		if cat == "" {
			cat = "other"
		}
		title := s.Title
		if title == "" {
			title = s.ID
		}
		if _, exists := groups[cat]; !exists {
			order = append(order, cat)
		}
		groups[cat] = append(groups[cat], title)
	}

	totalCount := 0
	for _, titles := range groups {
		totalCount += len(titles)
	}

	if !d.sourcesPrinted {
		fmt.Println()
		fmt.Printf("  📎 %sSources (%d):%s\n", ansiDim, totalCount, ansiReset)
		d.sourcesPrinted = true
	}

	for _, cat := range order {
		titles := groups[cat]
		fmt.Printf("     %s%-8s%s %s\n", ansiDim, cat, ansiReset, strings.Join(titles, " · "))
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
// If the previous round was a placeholder (trivial content like "In progress..."),
// we reuse the step number instead of incrementing.
func (d *StreamDisplay) startNewRound(cot cotJSON) {
	if d.cotEverStarted {
		prevWasTrivial := !d.cotHeaderUp && (d.cotPrintedLen == 0 || isTrivialContent(d.cotAccumulated))

		if d.cotPrintedLen > 0 && !prevWasTrivial {
			d.endCOTRound()
		} else if d.cotPrintedLen > 0 {
			// Trivial round with printed content — silently clear without footer
			d.silentClearRound()
		}

		// Only bump step number if the previous round had real content
		if !prevWasTrivial {
			d.cotRound++
		}
		// else: reuse the current step number
	} else {
		// Very first round
		d.cotRound++
	}

	d.cotEverStarted = true
	d.cotAccumulated = ""
	d.cotPrintedLen = 0
	d.cotHeaderUp = false
	d.cotDescription = ""
	d.cotExplanation = ""
	d.cotCategory = ""
	d.cotStatus = ""
	d.cotSources = nil
	d.currentCotID = cot.ID
	d.cotSeparatorDone = false

	d.updateCOTMetadata(cot)
}

// isTrivialContent checks if the accumulated text is just a placeholder
// that shouldn't count as a real investigation step.
func isTrivialContent(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	return lower == "in progress..." ||
		lower == "investigating..." ||
		lower == "analyzing..." ||
		len(trimmed) < 20
}

// silentClearRound clears the current round's output without printing a footer.
func (d *StreamDisplay) silentClearRound() {
	d.cotAccumulated = ""
	d.cotPrintedLen = 0
	d.cotHeaderUp = false
	d.cotDescription = ""
	d.cotExplanation = ""
	d.cotCategory = ""
	d.cotStatus = ""
	d.cotSources = nil
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

	// Step number + category — left-anchored line
	cat := formatCOTCategory(d.cotCategory)
	if cat != "" {
		fmt.Printf(" %s%s── Step %d · %s ────────────────────────────────────────────────────────────%s\n",
			ansiBold, ansiBlue, d.cotRound, cat, ansiReset)
	} else {
		fmt.Printf(" %s%s── Step %d ──────────────────────────────────────────────────────────────────%s\n",
			ansiBold, ansiBlue, d.cotRound, ansiReset)
	}

	// Explanation (primary — the short summary shown in UI sidebar)
	if d.cotExplanation != "" {
		fmt.Printf("    %s\n", d.cotExplanation)
	}

	// Description (secondary — the detailed scope shown in UI tooltip)
	if d.cotDescription != "" {
		if d.cotExplanation != "" {
			fmt.Printf("    %s↳ %s%s\n", ansiDim, d.cotDescription, ansiReset)
		} else {
			fmt.Printf("    %s\n", d.cotDescription)
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

	// Pause background spinner while we print content
	d.spinnerMu.Lock()
	d.contentStreaming = true
	d.spinnerMu.Unlock()

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
	isNewRound := d.cotEverStarted && cot.ID != "" && cot.ID != d.currentCotID

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

	// Show the step header as soon as we have a description/explanation,
	// even if the investigation text is still trivial/empty.
	if d.cotDescription != "" || d.cotExplanation != "" {
		d.ensureHeader()
	}

	// Don't print trivial/placeholder content ("In progress...", etc).
	// Just update metadata silently — real content will print when it arrives.
	if isTrivialContent(fullText) {
		return
	}

	// Print any new text beyond what we've already printed
	if fullText != "" && fullLen > d.cotPrintedLen {
		d.ensureHeader()

		// Pause background spinner while we print content
		d.spinnerMu.Lock()
		d.contentStreaming = true
		d.spinnerMu.Unlock()

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

		// Step footer: source count with checkmark
		srcCount := len(d.cotSources)
		if srcCount > 0 {
			fmt.Println()
			fmt.Printf("     %s✓ %d sources consulted%s\n", ansiDim, srcCount, ansiReset)
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

	// Resume spinner
	d.spinnerMu.Lock()
	d.contentStreaming = false
	d.spinnerMu.Unlock()
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

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	return s
}

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
		// Pause background spinner while we print content
		d.spinnerMu.Lock()
		d.contentStreaming = true
		d.spinnerMu.Unlock()
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
			// Pause background spinner while we print content
			d.spinnerMu.Lock()
			d.contentStreaming = true
			d.spinnerMu.Unlock()
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
	fmt.Printf(" %s━━ 💬 Response ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", ansiDim, ansiReset)
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

	// Resume spinner
	d.spinnerMu.Lock()
	d.contentStreaming = false
	d.spinnerMu.Unlock()
}
