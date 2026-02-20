package display

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"
)

func Header(text string) {
	fmt.Printf("\n%s%s%s\n", Bold+Cyan, text, Reset)
	fmt.Println(strings.Repeat("─", min(len(text)+4, 80)))
}

func SubHeader(text string) {
	fmt.Printf("%s%s%s\n", Bold+White, text, Reset)
}

func Success(text string) {
	fmt.Printf("%s✓%s %s\n", Green, Reset, text)
}

func Error(text string) {
	fmt.Fprintf(os.Stderr, "%s✗%s %s\n", Red, Reset, text)
}

func Warn(text string) {
	fmt.Printf("%s!%s %s\n", Yellow, Reset, text)
}

func Info(label, value string) {
	fmt.Printf("  %s%-20s%s %s\n", Dim, label, Reset, value)
}

func Spinner(text string) {
	fmt.Printf("\r%s⟳%s %s", Yellow, Reset, text)
}

func ClearLine() {
	fmt.Print("\r\033[K")
}

// Content type display for streaming
func ContentTypeLabel(ct string) string {
	labels := map[string]string{
		"CONTENT_TYPE_PROGRESS_STATUS":       Yellow + "⟳ Progress" + Reset,
		"CONTENT_TYPE_CHAT_RESPONSE":         Green + "💬 Response" + Reset,
		"CONTENT_TYPE_CHAIN_OF_THOUGHT":      Magenta + "🧠 Thinking" + Reset,
		"CONTENT_TYPE_SOURCES":               Blue + "📎 Sources" + Reset,
		"CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS": Cyan + "💡 Suggestions" + Reset,
		"CONTENT_TYPE_VISUALIZATION":         Cyan + "📊 Visualization" + Reset,
		"CONTENT_TYPE_SESSION_NAME":          Gray + "📋 Session" + Reset,
		"CONTENT_TYPE_ERROR_MESSAGE":         Red + "❌ Error" + Reset,
		"CONTENT_TYPE_DONE_INDICATOR":        Green + "✓ Done" + Reset,
		"CONTENT_TYPE_EXECUTION_TIME":        Gray + "⏱  Time" + Reset,
		"CONTENT_TYPE_ALTERNATE_QUESTIONS":   Cyan + "❓ Alternatives" + Reset,
		"CONTENT_TYPE_MESSAGE":               White + "📨 Message" + Reset,
	}
	if label, ok := labels[ct]; ok {
		return label
	}
	return Gray + ct + Reset
}

// Chain of thought status display
func CoTStatusLabel(status string) string {
	labels := map[string]string{
		"CHAIN_OF_THOUGHT_STATUS_IN_PROGRESS": Yellow + "⟳ In Progress" + Reset,
		"CHAIN_OF_THOUGHT_STATUS_DONE":        Green + "✓ Done" + Reset,
		"CHAIN_OF_THOUGHT_STATUS_ERROR":       Red + "✗ Error" + Reset,
		"CHAIN_OF_THOUGHT_STATUS_CANCELLED":   Gray + "⊘ Cancelled" + Reset,
		"CHAIN_OF_THOUGHT_STATUS_PAUSED":      Yellow + "⏸ Paused" + Reset,
	}
	if label, ok := labels[status]; ok {
		return label
	}
	return status
}

func InvestigationStatusLabel(status string) string {
	labels := map[string]string{
		"INVESTIGATION_STATUS_NOT_STARTED":  Gray + "Not Started" + Reset,
		"INVESTIGATION_STATUS_IN_PROGRESS":  Yellow + "In Progress" + Reset,
		"INVESTIGATION_STATUS_INVESTIGATED": Blue + "Investigated" + Reset,
		"INVESTIGATION_STATUS_COMPLETED":    Green + "Completed" + Reset,
		"INVESTIGATION_STATUS_PAUSED":       Yellow + "Paused" + Reset,
		"INVESTIGATION_STATUS_STOPPED":      Red + "Stopped" + Reset,
	}
	if label, ok := labels[status]; ok {
		return label
	}
	return status
}

func FormatTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return ts
		}
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
