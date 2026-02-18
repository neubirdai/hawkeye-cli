package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// StreamDisplay handles clean terminal output for SSE streams.
// It deduplicates progress messages, parses source JSON, compresses
// chain-of-thought token streams, and strips HTML from responses.
type StreamDisplay struct {
	debug bool

	// Deduplication state
	lastProgress    string
	progressCount   int
	seenSourceIDs   map[string]bool
	sourcesPrinted  bool

	// Chain-of-thought: only keep the latest version per ID
	lastCOTID       string
	lastCOTContent  string
	cotPrinted      bool

	// Final answer accumulator
	FinalAnswer     string
	SessionUUID     string
	SessionName     string
}

func NewStreamDisplay(debug bool) *StreamDisplay {
	return &StreamDisplay{
		debug:         debug,
		seenSourceIDs: make(map[string]bool),
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

	switch ct {
	case "CONTENT_TYPE_PROGRESS_STATUS":
		d.handleProgress(parts)

	case "CONTENT_TYPE_SOURCES":
		d.handleSources(parts)

	case "CONTENT_TYPE_CHAIN_OF_THOUGHT":
		d.handleChainOfThought(parts)

	case "CONTENT_TYPE_CHAT_RESPONSE":
		d.handleChatResponse(parts)
	}

	if msg.EndTurn {
		d.flush()
	}
}

// flush prints any pending state at stream end.
func (d *StreamDisplay) flush() {
	// Print final chain-of-thought if we haven't yet
	if d.lastCOTContent != "" && !d.cotPrinted {
		d.printCOT()
	}
	// Clear progress line
	if d.progressCount > 0 {
		fmt.Println()
	}
}

// --- Progress ---

func (d *StreamDisplay) handleProgress(parts []string) {
	if len(parts) == 0 {
		return
	}
	text := parts[0]

	// Identical to last → just bump counter, overwrite line
	if text == d.lastProgress {
		d.progressCount++
		return
	}

	// New progress step
	d.lastProgress = text
	d.progressCount = 1

	// Print on a new line (carriage-return overwrite for rapid repeats isn't
	// worth the terminal-capability gymnastics in v1; just deduplicate.)
	fmt.Printf("  ⟳ %s\n", text)
}

// --- Sources ---

// sourceJSON is the shape of each element in the parts array for CONTENT_TYPE_SOURCES.
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
			// Not JSON — maybe plain text?
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
		return // all duplicates
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

// formatSourceLabel turns a sourceJSON into a readable one-liner.
func formatSourceLabel(s sourceJSON) string {
	cat := s.Category
	name := s.Title

	// Shorten long metric names: "metric_aws_prod.containerinsights_container_cpu_limit"
	// → "container_cpu_limit"
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	// Further cleanup: strip common prefixes
	name = strings.TrimPrefix(name, "containerinsights_")

	if cat != "" {
		return fmt.Sprintf("[%s] %s", cat, name)
	}
	return name
}

// --- Chain of Thought ---

// cotJSON is the shape of the investigation JSON inside CONTENT_TYPE_CHAIN_OF_THOUGHT parts.
type cotJSON struct {
	ID            string   `json:"id"`
	Category      string   `json:"category"`
	Description   string   `json:"description"`
	Explanation   string   `json:"explanation"`
	Investigation string   `json:"investigation"`
	Status        string   `json:"status"`
	Sources       []string `json:"sources_involved"`
}

func (d *StreamDisplay) handleChainOfThought(parts []string) {
	if len(parts) == 0 {
		return
	}

	// The server token-streams COT: it sends the SAME JSON object many times,
	// each time with a few more characters appended to the "investigation" field.
	// We only want to display the FINAL version, so we just keep overwriting.
	raw := parts[0]

	var cot cotJSON
	if err := json.Unmarshal([]byte(raw), &cot); err != nil {
		// Might be a partial or non-JSON string — store raw
		d.lastCOTContent = raw
		return
	}

	d.lastCOTID = cot.ID
	d.lastCOTContent = raw
	d.cotPrinted = false

	// If status changed to completed, print now
	if cot.Status == "completed" || cot.Status == "done" {
		d.printCOT()
	}
}

func (d *StreamDisplay) printCOT() {
	if d.lastCOTContent == "" {
		return
	}

	var cot cotJSON
	if err := json.Unmarshal([]byte(d.lastCOTContent), &cot); err != nil {
		// Non-JSON — just print as-is
		fmt.Printf("\n  🔍 Investigation:\n%s\n", d.lastCOTContent)
		d.cotPrinted = true
		return
	}

	fmt.Println()
	fmt.Println("  🔍 Investigation")
	if cot.Description != "" {
		fmt.Printf("     %s\n", cot.Description)
	}
	if cot.Explanation != "" {
		fmt.Printf("     %s\n", cot.Explanation)
	}
	fmt.Printf("     Status: %s\n", cot.Status)

	if cot.Investigation != "" {
		fmt.Println()
		// The investigation field contains markdown — print it directly.
		// Strip excessive leading whitespace per line for terminal readability.
		fmt.Println(cot.Investigation)
	}

	d.cotPrinted = true
}

// --- Chat Response ---

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func (d *StreamDisplay) handleChatResponse(parts []string) {
	if len(parts) == 0 {
		return
	}

	// Print any pending COT before the final answer
	if d.lastCOTContent != "" && !d.cotPrinted {
		d.printCOT()
	}

	text := strings.Join(parts, "\n")
	text = stripHTML(text)
	text = strings.TrimSpace(text)

	if text == "" {
		return
	}

	d.FinalAnswer = text

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  💬 Response")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println(text)
	fmt.Println()
}

func stripHTML(s string) string {
	// Replace <br/> and <br> with newlines
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	// Strip remaining HTML tags
	s = htmlTagRe.ReplaceAllString(s, "")
	return s
}
