package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// StreamDisplay handles clean terminal output for SSE streams.
type StreamDisplay struct {
	debug bool

	// Progress deduplication — aggressive global dedup
	lastProgress string
	seenProgress map[string]bool

	// Source deduplication
	seenSourceIDs  map[string]bool
	sourcesPrinted bool

	// Chain-of-thought: track per-ID to avoid reprinting old investigations
	cotPrintedLen   map[string]int // COT ID → how many chars of investigation already printed
	cotHeaderPrinted map[string]bool // COT ID → whether we printed the header
	cotActive       bool
	activeCOTID     string
	lastContentType string

	// Final answer accumulator
	FinalAnswer string
	SessionUUID string
}

func NewStreamDisplay(debug bool) *StreamDisplay {
	return &StreamDisplay{
		debug:            debug,
		seenSourceIDs:    make(map[string]bool),
		seenProgress:     make(map[string]bool),
		cotPrintedLen:    make(map[string]int),
		cotHeaderPrinted: make(map[string]bool),
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

	// When content type changes FROM chain-of-thought, finalize the COT block
	if d.cotActive && ct != "CONTENT_TYPE_CHAIN_OF_THOUGHT" {
		d.finishCOTBlock()
	}

	d.lastContentType = ct

	switch ct {
	case "CONTENT_TYPE_PROGRESS_STATUS":
		d.handleProgress(parts)

	case "CONTENT_TYPE_SOURCES":
		d.handleSources(parts)

	case "CONTENT_TYPE_CHAIN_OF_THOUGHT":
		d.handleChainOfThought(parts)

	case "CONTENT_TYPE_CHAT_RESPONSE":
		d.handleChatResponse(parts)

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
		for _, p := range parts {
			fmt.Printf("  ✗ %s\n", p)
		}
	}

	if msg.EndTurn {
		d.flush()
	}
}

// flush prints any pending state at stream end.
func (d *StreamDisplay) flush() {
	if d.cotActive {
		d.finishCOTBlock()
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
		d.lastProgress = text
		return
	}

	d.seenProgress[normKey] = true
	d.lastProgress = text
	fmt.Printf("  ⟳ %s\n", text)
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

func (d *StreamDisplay) handleChainOfThought(parts []string) {
	if len(parts) == 0 {
		return
	}

	raw := parts[0]
	d.cotActive = true

	var cot cotJSON
	if err := json.Unmarshal([]byte(raw), &cot); err != nil {
		return
	}

	// Use COT ID to track per-investigation-step progress.
	// If no ID, use description as fallback key.
	cotID := cot.ID
	if cotID == "" {
		cotID = cot.Description
	}
	if cotID == "" {
		cotID = "_default"
	}
	d.activeCOTID = cotID

	// How much of this COT's investigation have we already printed?
	alreadyPrinted := d.cotPrintedLen[cotID]

	// Stream new investigation text as it arrives (delta printing)
	if cot.Investigation != "" && len(cot.Investigation) > alreadyPrinted {
		newText := cot.Investigation[alreadyPrinted:]

		if !d.cotHeaderPrinted[cotID] {
			// First chunk for this COT — print header
			fmt.Println()
			fmt.Println("  🔍 Investigation")
			if cot.Description != "" {
				fmt.Printf("     %s\n", cot.Description)
			}
			fmt.Println()
			d.cotHeaderPrinted[cotID] = true
		}

		fmt.Print(newText)
		d.cotPrintedLen[cotID] = len(cot.Investigation)
	}
}

func (d *StreamDisplay) finishCOTBlock() {
	if !d.cotActive {
		return
	}
	d.cotActive = false

	// Add trailing newlines if we printed any investigation text for this block
	if d.activeCOTID != "" && d.cotPrintedLen[d.activeCOTID] > 0 {
		fmt.Println()
		fmt.Println()
	}

	d.activeCOTID = ""
}

// --- Chat Response ---

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func (d *StreamDisplay) handleChatResponse(parts []string) {
	if len(parts) == 0 {
		return
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
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	return s
}
