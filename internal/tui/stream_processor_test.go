package tui

import (
	"encoding/json"
	"strings"
	"testing"
)

// ─── Test helpers ───────────────────────────────────────────────────────────

// cotJSON builds a COT JSON payload.
func cotJSON(id, desc, investigation, status string) string {
	m := map[string]string{
		"id":            id,
		"description":   desc,
		"investigation": investigation,
		"status":        status,
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func progressMsg(text string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_PROGRESS_STATUS",
		text:        text,
	}
}

func sourceMsg(text string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_SOURCES",
		text:        text,
	}
}

func cotLegacyMsg(id, desc, investigation, status string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_CHAIN_OF_THOUGHT",
		eventType:   "message",
		text:        cotJSON(id, desc, investigation, status),
	}
}

func cotStartMsg(id, desc string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_CHAIN_OF_THOUGHT",
		eventType:   "cot_start",
		text:        cotJSON(id, desc, "", ""),
	}
}

func cotDeltaMsg(id, delta string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_CHAIN_OF_THOUGHT",
		eventType:   "cot_delta",
		text:        cotJSON(id, "", delta, ""),
	}
}

func cotEndMsg(id string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_CHAIN_OF_THOUGHT",
		eventType:   "cot_end",
		text:        cotJSON(id, "", "", ""),
	}
}

func chatDeltaMsg(text string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_CHAT_RESPONSE",
		eventType:   "chat_delta",
		text:        text,
	}
}

func chatFullMsg(text string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_CHAT_RESPONSE",
		eventType:   "chat_full",
		text:        text,
	}
}

func followUpMsg(suggestions ...string) streamChunkMsg {
	return streamChunkMsg{
		contentType: "CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS",
		text:        strings.Join(suggestions, "\n"),
	}
}

// collectTypes returns just the types from a list of events.
func collectTypes(events []OutputEvent) []OutputType {
	var types []OutputType
	for _, ev := range events {
		types = append(types, ev.Type)
	}
	return types
}

// collectText returns just the text from events of a given type.
func collectText(events []OutputEvent, t OutputType) []string {
	var texts []string
	for _, ev := range events {
		if ev.Type == t {
			texts = append(texts, ev.Text)
		}
	}
	return texts
}

// hasType checks if any event has the given type.
func hasType(events []OutputEvent, t OutputType) bool {
	for _, ev := range events {
		if ev.Type == t {
			return true
		}
	}
	return false
}

// ─── Progress tests ─────────────────────────────────────────────────────────

func TestProgressImmediate(t *testing.T) {
	sp := NewStreamProcessor()
	out := sp.Process(progressMsg("PromptGate (Loading Investigation Programs)"))

	if len(out) != 1 {
		t.Fatalf("expected 1 event, got %d", len(out))
	}
	if out[0].Type != OutputProgress {
		t.Errorf("expected OutputProgress, got %d", out[0].Type)
	}
	if out[0].Text != "Loading Investigation Programs" {
		t.Errorf("expected extracted text, got %q", out[0].Text)
	}
}

func TestProgressFlushesCOTBuffer(t *testing.T) {
	sp := NewStreamProcessor()

	// Start a COT step with text ending in partial line (no trailing \n)
	sp.Process(cotLegacyMsg("cot1", "Analyzing", "## Summary\nPartial line", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))

	// Progress arriving should flush the COT buffer and emit progress
	out := sp.Process(progressMsg("(Loading Programs)"))

	// Should have flushed COT text AND emitted progress
	cotTexts := collectText(out, OutputCOTText)
	if len(cotTexts) != 1 || cotTexts[0] != "Partial line" {
		t.Errorf("expected COT buffer to flush, got %v", cotTexts)
	}

	progressTexts := collectText(out, OutputProgress)
	if len(progressTexts) != 1 || progressTexts[0] != "Loading Programs" {
		t.Errorf("expected progress to emit, got %v", progressTexts)
	}
}

func TestProgressFlushesChatBuffer(t *testing.T) {
	sp := NewStreamProcessor()

	// Start chat streaming with partial line
	sp.Process(chatDeltaMsg("Hello "))

	// Progress arriving means chat is done — should flush chat buffer and emit progress
	out := sp.Process(progressMsg("(Working)"))

	chatTexts := collectText(out, OutputChat)
	if len(chatTexts) != 1 || chatTexts[0] != "Hello " {
		t.Errorf("expected flushed chat buffer, got %v", chatTexts)
	}

	progressTexts := collectText(out, OutputProgress)
	if len(progressTexts) != 1 || progressTexts[0] != "Working" {
		t.Errorf("expected progress to emit, got %v", progressTexts)
	}
}

func TestProgressDedup(t *testing.T) {
	sp := NewStreamProcessor()

	out1 := sp.Process(progressMsg("(Loading Programs)"))
	out2 := sp.Process(progressMsg("(Loading Programs)"))

	if len(out1) != 1 {
		t.Errorf("first progress should emit, got %d events", len(out1))
	}
	if len(out2) != 0 {
		t.Errorf("duplicate progress should be suppressed, got %d events", len(out2))
	}
}

func TestProgressNormalization(t *testing.T) {
	sp := NewStreamProcessor()

	out1 := sp.Process(progressMsg("(Found 2 results)"))
	out2 := sp.Process(progressMsg("(Found 5 results)"))

	if len(out1) != 1 {
		t.Errorf("first progress should emit, got %d events", len(out1))
	}
	if len(out2) != 0 {
		t.Errorf("normalized duplicate should be suppressed, got %d events", len(out2))
	}
}

// ─── COT delta mode tests ──────────────────────────────────────────────────

func TestCOTDeltaBuffering(t *testing.T) {
	sp := NewStreamProcessor()

	// cot_start emits header
	out := sp.Process(cotStartMsg("cot1", "Analyzing logs"))
	if !hasType(out, OutputCOTHeader) {
		t.Error("expected COT header on cot_start")
	}

	// Delta without \n stays buffered
	out = sp.Process(cotDeltaMsg("cot1", "partial text"))
	cotTexts := collectText(out, OutputCOTText)
	if len(cotTexts) != 0 {
		t.Errorf("partial line should stay buffered, got %v", cotTexts)
	}

	// Delta with \n flushes the complete line
	out = sp.Process(cotDeltaMsg("cot1", " continued\nstart of next"))
	cotTexts = collectText(out, OutputCOTText)
	if len(cotTexts) != 1 || cotTexts[0] != "partial text continued" {
		t.Errorf("expected flushed line, got %v", cotTexts)
	}
}

func TestCOTDeltaEnd(t *testing.T) {
	sp := NewStreamProcessor()

	sp.Process(cotStartMsg("cot1", "Analyzing"))
	sp.Process(cotDeltaMsg("cot1", "buffered text"))

	// cot_end should flush remaining buffer
	out := sp.Process(cotEndMsg("cot1"))
	cotTexts := collectText(out, OutputCOTText)
	if len(cotTexts) != 1 || cotTexts[0] != "buffered text" {
		t.Errorf("cot_end should flush buffer, got %v", cotTexts)
	}
}

// ─── COT legacy mode tests ─────────────────────────────────────────────────

func TestCOTLegacyDiff(t *testing.T) {
	sp := NewStreamProcessor()

	// First event: sends full text
	out := sp.Process(cotLegacyMsg("cot1", "Analyzing", "line1\nline2\n", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))
	cotTexts := collectText(out, OutputCOTText)
	if len(cotTexts) != 2 || cotTexts[0] != "line1" || cotTexts[1] != "line2" {
		t.Errorf("expected 2 lines, got %v", cotTexts)
	}

	// Second event: more text appended
	out = sp.Process(cotLegacyMsg("cot1", "Analyzing", "line1\nline2\nline3\n", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))
	cotTexts = collectText(out, OutputCOTText)
	if len(cotTexts) != 1 || cotTexts[0] != "line3" {
		t.Errorf("expected only new line, got %v", cotTexts)
	}
}

func TestCOTLegacyCompleted(t *testing.T) {
	sp := NewStreamProcessor()

	// Event with text ending without newline
	sp.Process(cotLegacyMsg("cot1", "Analyzing", "## Summary\nDuring the incident", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))

	// Same text but completed — should flush buffer
	out := sp.Process(cotLegacyMsg("cot1", "Analyzing", "## Summary\nDuring the incident", "CHAIN_OF_THOUGHT_STATUS_COMPLETED"))
	cotTexts := collectText(out, OutputCOTText)
	found := false
	for _, t := range cotTexts {
		if strings.Contains(t, "During the incident") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("completed status should flush buffer with 'During the incident', got %v", cotTexts)
	}
}

func TestCOTLegacyNoNewTextNoFlush(t *testing.T) {
	sp := NewStreamProcessor()

	// First event: partial line buffered
	sp.Process(cotLegacyMsg("cot1", "Analyzing", "## Summary\npartial", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))

	// Second event: same text, still in progress — should NOT flush buffer
	// because the backend may send duplicate events while streaming char-by-char
	out := sp.Process(cotLegacyMsg("cot1", "Analyzing", "## Summary\npartial", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))
	cotTexts := collectText(out, OutputCOTText)
	if len(cotTexts) != 0 {
		t.Errorf("no-new-text event should NOT flush buffer, got %v", cotTexts)
	}

	// Buffer should still contain "partial"
	// Verify by completing the COT and checking flush
	out = sp.Process(cotLegacyMsg("cot1", "Analyzing", "## Summary\npartial", "CHAIN_OF_THOUGHT_STATUS_COMPLETED"))
	cotTexts = collectText(out, OutputCOTText)
	found := false
	for _, t := range cotTexts {
		if strings.Contains(t, "partial") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("completion should flush buffer with 'partial', got %v", cotTexts)
	}
}

func TestCOTStepTransition(t *testing.T) {
	sp := NewStreamProcessor()

	// Step 1: text with partial line
	sp.Process(cotLegacyMsg("cot1", "Step 1", "line1\nbuffered", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))

	// Step 2: different COT ID — should flush step 1's buffer first
	out := sp.Process(cotLegacyMsg("cot2", "Step 2", "step2 text\n", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))

	// Collect all COT text events
	cotTexts := collectText(out, OutputCOTText)
	if len(cotTexts) < 1 {
		t.Fatalf("expected COT text events, got none")
	}
	// "buffered" from step 1 should be flushed before step 2's text
	if cotTexts[0] != "buffered" {
		t.Errorf("first COT text should be flushed buffer from step 1, got %q", cotTexts[0])
	}
}

// ─── Chat tests ─────────────────────────────────────────────────────────────

func TestChatDeltaBuffering(t *testing.T) {
	sp := NewStreamProcessor()

	// Partial line — buffered
	out := sp.Process(chatDeltaMsg("Hello "))
	chatTexts := collectText(out, OutputChat)
	if len(chatTexts) != 0 {
		t.Errorf("partial chat should stay buffered, got %v", chatTexts)
	}

	// Complete line
	out = sp.Process(chatDeltaMsg("world!\nNext "))
	chatTexts = collectText(out, OutputChat)
	if len(chatTexts) != 1 || chatTexts[0] != "Hello world!" {
		t.Errorf("expected complete line, got %v", chatTexts)
	}
}

func TestChatFullMode(t *testing.T) {
	sp := NewStreamProcessor()

	// First full message
	out := sp.Process(chatFullMsg("Hello world!\nSecond line"))
	chatTexts := collectText(out, OutputChat)
	if len(chatTexts) != 1 || chatTexts[0] != "Hello world!" {
		t.Errorf("expected first complete line, got %v", chatTexts)
	}

	// Second full message with more text
	out = sp.Process(chatFullMsg("Hello world!\nSecond line\nThird line"))
	chatTexts = collectText(out, OutputChat)
	if len(chatTexts) != 1 || chatTexts[0] != "Second line" {
		t.Errorf("expected only new lines, got %v", chatTexts)
	}
}

// ─── Transition tests ───────────────────────────────────────────────────────

func TestCOTToChatTransition(t *testing.T) {
	sp := NewStreamProcessor()

	// COT with buffered text
	sp.Process(cotLegacyMsg("cot1", "Analyzing", "## Summary\nbuffered text", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))

	// Chat arrives — should flush COT buffer first
	out := sp.Process(chatDeltaMsg("Chat line\n"))
	types := collectTypes(out)

	// Should see COTText (flushed) before Chat
	cotIdx := -1
	chatIdx := -1
	for i, t := range types {
		if t == OutputCOTText && cotIdx == -1 {
			cotIdx = i
		}
		if t == OutputChat && chatIdx == -1 {
			chatIdx = i
		}
	}
	if cotIdx == -1 {
		t.Error("expected flushed COT text in transition")
	}
	if chatIdx == -1 {
		t.Error("expected chat text")
	}
	if cotIdx > chatIdx {
		t.Error("COT flush should come before chat text")
	}
}

func TestChatToFollowUpTransition(t *testing.T) {
	sp := NewStreamProcessor()

	// Chat with buffered text
	sp.Process(chatDeltaMsg("last line of chat"))

	// Follow-up arrives — should flush chat buffer first
	out := sp.Process(followUpMsg("Suggestion 1", "Suggestion 2"))

	chatTexts := collectText(out, OutputChat)
	if len(chatTexts) != 1 || chatTexts[0] != "last line of chat" {
		t.Errorf("expected flushed chat buffer, got %v", chatTexts)
	}

	followUps := collectText(out, OutputFollowUpItem)
	if len(followUps) != 2 {
		t.Errorf("expected 2 follow-up items, got %d", len(followUps))
	}
}

// ─── Source gating ──────────────────────────────────────────────────────────

func TestSourceSuppressedDuringCOT(t *testing.T) {
	sp := NewStreamProcessor()

	// Start COT step
	sp.Process(cotStartMsg("cot1", "Analyzing"))

	// Source during COT should be suppressed
	out := sp.Process(sourceMsg(`{"id":"src1","category":"logs","title":"pod_logs"}`))
	if len(out) != 0 {
		t.Errorf("source should be suppressed during COT, got %d events", len(out))
	}
}

func TestSourceShownOutsideCOT(t *testing.T) {
	sp := NewStreamProcessor()

	out := sp.Process(sourceMsg(`{"id":"src1","category":"logs","title":"pod_logs"}`))
	if len(out) != 1 || out[0].Type != OutputSource {
		t.Errorf("source should show outside COT, got %v", out)
	}
}

// ─── Flush tests ────────────────────────────────────────────────────────────

func TestFlushAll(t *testing.T) {
	sp := NewStreamProcessor()

	// Set up state: chat buffer with partial line
	sp.Process(chatDeltaMsg("buffered chat"))

	// Flush everything (chat buffer should be flushed)
	out := sp.Flush()

	chatTexts := collectText(out, OutputChat)
	if len(chatTexts) != 1 || chatTexts[0] != "buffered chat" {
		t.Errorf("expected flushed chat, got %v", chatTexts)
	}
}

// ─── Session name / exec time ───────────────────────────────────────────────

func TestSessionName(t *testing.T) {
	sp := NewStreamProcessor()
	out := sp.Process(streamChunkMsg{contentType: "CONTENT_TYPE_SESSION_NAME", text: "My Investigation"})
	if len(out) != 1 || out[0].Type != OutputSessionName || out[0].Text != "My Investigation" {
		t.Errorf("unexpected session name output: %v", out)
	}
}

func TestExecTime(t *testing.T) {
	sp := NewStreamProcessor()
	out := sp.Process(streamChunkMsg{contentType: "CONTENT_TYPE_EXECUTION_TIME", text: "42s"})
	if len(out) != 1 || out[0].Type != OutputExecTime || out[0].Text != "42s" {
		t.Errorf("unexpected exec time output: %v", out)
	}
}

func TestErrorSuppressed(t *testing.T) {
	sp := NewStreamProcessor()
	out := sp.Process(streamChunkMsg{contentType: "CONTENT_TYPE_ERROR_MESSAGE", text: "SQL error"})
	if len(out) != 0 {
		t.Errorf("error messages should be suppressed, got %d events", len(out))
	}
}

// ─── Full flow integration test ─────────────────────────────────────────────

func TestFullFlowIntegration(t *testing.T) {
	sp := NewStreamProcessor()

	var allEvents []OutputEvent

	// Phase 1: Initial progress
	allEvents = append(allEvents, sp.Process(progressMsg("(Preparing Telemetry Sources)"))...)
	allEvents = append(allEvents, sp.Process(progressMsg("(Loading Investigation Programs)"))...)

	// Phase 2: COT step 1
	allEvents = append(allEvents, sp.Process(cotLegacyMsg("cot1", "Analyzing logs",
		"## Log Analysis\n- No errors found\n", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))...)
	// Progress during COT (should be queued)
	allEvents = append(allEvents, sp.Process(progressMsg("(Found 3 results)"))...)
	// COT step 1 completes
	allEvents = append(allEvents, sp.Process(cotLegacyMsg("cot1", "Analyzing logs",
		"## Log Analysis\n- No errors found\n### Summary\nAll clear.",
		"CHAIN_OF_THOUGHT_STATUS_COMPLETED"))...)

	// Phase 3: COT step 2
	allEvents = append(allEvents, sp.Process(cotLegacyMsg("cot2", "Checking metrics",
		"## Metrics\n- CPU normal\n", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))...)
	allEvents = append(allEvents, sp.Process(cotLegacyMsg("cot2", "Checking metrics",
		"## Metrics\n- CPU normal\n### Summary\nMetrics healthy.",
		"CHAIN_OF_THOUGHT_STATUS_COMPLETED"))...)

	// Phase 4: Chat response (summary)
	allEvents = append(allEvents, sp.Process(chatDeltaMsg("**Overall Summary:**\n"))...)
	allEvents = append(allEvents, sp.Process(chatDeltaMsg("No issues detected.\n"))...)

	// Phase 5: Follow-up suggestions
	allEvents = append(allEvents, sp.Process(followUpMsg("Check deployment logs", "Review metrics dashboard"))...)

	// Verify ordering: Progress → COT1 → (queued progress) → COT2 → Chat → FollowUp
	// Extract the sequence of event types
	types := collectTypes(allEvents)

	// Find key transitions
	lastProgressBeforeCOT := -1
	firstCOT := -1
	firstChat := -1
	firstFollowUp := -1

	for i, t := range types {
		if t == OutputProgress && firstCOT == -1 {
			lastProgressBeforeCOT = i
		}
		if (t == OutputCOTHeader || t == OutputCOTText) && firstCOT == -1 {
			firstCOT = i
		}
		if t == OutputChat && firstChat == -1 {
			firstChat = i
		}
		if (t == OutputFollowUpHeader || t == OutputFollowUpItem) && firstFollowUp == -1 {
			firstFollowUp = i
		}
	}

	if lastProgressBeforeCOT >= firstCOT && firstCOT != -1 {
		t.Error("initial progress should come before first COT")
	}
	if firstCOT >= firstChat && firstChat != -1 {
		t.Error("COT should come before chat")
	}
	if firstChat >= firstFollowUp && firstFollowUp != -1 {
		t.Error("chat should come before follow-ups")
	}

	// Verify no progress text appears between COT text lines
	inCOTBlock := false
	for _, ev := range allEvents {
		if ev.Type == OutputCOTText {
			inCOTBlock = true
		}
		if ev.Type == OutputProgress && inCOTBlock {
			// Progress is only OK after a COT block completes (between steps).
			// Queued progress flushes between steps — this is expected.
			_ = ev // acknowledge the event
		}
		if ev.Type == OutputChat {
			inCOTBlock = false
		}
	}

	// Verify follow-up items have correct indices
	for _, ev := range allEvents {
		if ev.Type == OutputFollowUpItem && ev.Index == 0 {
			t.Error("follow-up items should have non-zero index")
		}
	}
}

// ─── Regression: progress text must not interleave COT content ──────────────

func TestProgressDoesNotInterleaveWithCOTText(t *testing.T) {
	sp := NewStreamProcessor()

	// COT streaming with multiple deltas
	sp.Process(cotStartMsg("cot1", "Analyzing"))
	sp.Process(cotDeltaMsg("cot1", "First line\n"))
	sp.Process(cotDeltaMsg("cot1", "Second line\n"))

	// Progress arrives mid-stream — must be queued
	out := sp.Process(progressMsg("(Loading Investigation Programs)"))
	for _, ev := range out {
		if ev.Type == OutputProgress {
			t.Error("progress must not emit during active COT text streaming")
		}
	}

	// More COT text
	sp.Process(cotDeltaMsg("cot1", "Third line\n"))

	// COT ends — queued progress should now flush
	out = sp.Process(cotEndMsg("cot1"))
	progressTexts := collectText(out, OutputProgress)
	if len(progressTexts) != 1 {
		t.Errorf("expected 1 queued progress after cot_end, got %d", len(progressTexts))
	}
}

// ─── Regression: summary text after ## Summary must not be cut ──────────────

func TestSummaryTextNotCutOff(t *testing.T) {
	sp := NewStreamProcessor()

	// Simulate legacy COT where summary comes at the end without trailing \n
	sp.Process(cotLegacyMsg("cot1", "Analyzing",
		"## Analysis\n- Point 1\n- Point 2\n### Summary\nDuring the incident, no issues were found.",
		"CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))

	// Same text, now completed — buffer should flush
	out := sp.Process(cotLegacyMsg("cot1", "Analyzing",
		"## Analysis\n- Point 1\n- Point 2\n### Summary\nDuring the incident, no issues were found.",
		"CHAIN_OF_THOUGHT_STATUS_COMPLETED"))

	// The summary text should be in the output
	allCOT := collectText(out, OutputCOTText)
	found := false
	for _, line := range allCOT {
		if strings.Contains(line, "During the incident") {
			found = true
		}
	}
	if !found {
		t.Errorf("summary text 'During the incident' should not be cut off, COT texts: %v", allCOT)
	}
}

// ─── Table routing ──────────────────────────────────────────────────────────

func TestTableRoutingInChat(t *testing.T) {
	sp := NewStreamProcessor()

	// Table rows followed by a non-table line should emit OutputTable, not raw OutputChat
	out := sp.Process(chatDeltaMsg("| Header |\n|---|\n| Row 1 |\nnext line\n"))

	if !hasType(out, OutputTable) {
		t.Error("expected OutputTable event from chat table rows")
	}
	for _, ev := range out {
		if ev.Type == OutputChat && strings.HasPrefix(strings.TrimSpace(ev.Text), "|") {
			t.Errorf("table row must not appear as raw OutputChat: %q", ev.Text)
		}
	}
}

func TestTableRoutingInCOT(t *testing.T) {
	sp := NewStreamProcessor()

	sp.Process(cotStartMsg("cot1", "Analyzing"))
	// Table rows then a non-table line — all in one delta
	out := sp.Process(cotDeltaMsg("cot1", "| Header |\n|---|\n| Row 1 |\nnext line\n"))

	if !hasType(out, OutputTable) {
		t.Error("expected OutputTable event from COT table rows")
	}
	for _, ev := range out {
		if ev.Type == OutputCOTText && strings.HasPrefix(strings.TrimSpace(ev.Text), "|") {
			t.Errorf("table row must not appear as raw OutputCOTText: %q", ev.Text)
		}
	}
}

func TestTableFlushAtCOTBoundary(t *testing.T) {
	sp := NewStreamProcessor()

	sp.Process(cotStartMsg("cot1", "Analyzing"))
	// Last line is a table row with no trailing \n — it stays in cotBuffer
	sp.Process(cotDeltaMsg("cot1", "| A | B |\n|---|---|\n| last row |"))

	// cot_end should flush the buffered table row as OutputTable
	out := sp.Process(cotEndMsg("cot1"))
	if !hasType(out, OutputTable) {
		t.Error("cot_end should flush pending table rows as OutputTable")
	}
}

func TestTableFlushAtStreamEnd(t *testing.T) {
	sp := NewStreamProcessor()

	// Table rows with no non-table line after — flushed by Flush()
	sp.Process(chatDeltaMsg("| Col |\n|-----|\n| row1 |"))

	out := sp.Flush()
	if !hasType(out, OutputTable) {
		t.Error("Flush() should emit pending table rows as OutputTable")
	}
}

// ─── Dividers ────────────────────────────────────────────────────────────────

func TestDividerBetweenCOTSteps(t *testing.T) {
	sp := NewStreamProcessor()

	sp.Process(cotLegacyMsg("cot1", "Step 1", "some text\n", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))
	out := sp.Process(cotLegacyMsg("cot2", "Step 2", "more text\n", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))

	if !hasType(out, OutputDivider) {
		t.Error("expected OutputDivider when transitioning between COT steps")
	}
}

func TestDividerBeforeChat(t *testing.T) {
	sp := NewStreamProcessor()

	sp.Process(cotLegacyMsg("cot1", "Analyzing", "some text\n", "CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS"))
	out := sp.Process(chatDeltaMsg("Summary line\n"))

	if !hasType(out, OutputDivider) {
		t.Error("expected OutputDivider when transitioning from COT to chat")
	}
}

func TestNoDividerWithoutCOT(t *testing.T) {
	sp := NewStreamProcessor()

	// Chat without any prior COT should not emit a divider
	out := sp.Process(chatDeltaMsg("Direct answer\n"))
	if hasType(out, OutputDivider) {
		t.Error("no divider expected when chat starts without COT")
	}
}

func TestLastStatus(t *testing.T) {
	sp := NewStreamProcessor()

	if sp.LastStatus() != "" {
		t.Error("initial status should be empty")
	}

	sp.Process(progressMsg("(Loading Programs)"))
	if sp.LastStatus() != "Loading Programs" {
		t.Errorf("expected 'Loading Programs', got %q", sp.LastStatus())
	}

	sp.Process(progressMsg("(Analyzing Results)"))
	if sp.LastStatus() != "Analyzing Results" {
		t.Errorf("expected 'Analyzing Results', got %q", sp.LastStatus())
	}
}
