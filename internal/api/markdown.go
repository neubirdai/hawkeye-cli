package api

import (
	"fmt"
	"regexp"
	"strings"
)

// ANSI escape codes for terminal styling.
const (
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiDim       = "\033[2m"
	ansiItalic    = "\033[3m"
	ansiUnderline = "\033[4m"
	ansiCyan      = "\033[36m"
	ansiGreen     = "\033[32m"
	ansiYellow    = "\033[33m"
	ansiBlue      = "\033[34m"
	ansiMagenta   = "\033[35m"
	ansiWhite     = "\033[37m"
	ansiBoldCyan  = "\033[1;36m"
	ansiBoldGreen = "\033[1;32m"
)

// Regex patterns for inline markdown.
var (
	mdBoldRe = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdCodeRe = regexp.MustCompile("`([^`]+)`")
)

// mdPrinter handles streaming markdown colorization.
// It buffers incomplete lines so markdown tokens aren't split across chunks.
type mdPrinter struct {
	lineBuffer  string // partial line waiting for newline
	inCodeBlock bool   // inside a ``` fenced code block
}

// printMarkdown colorizes and prints a chunk of markdown text.
// Safe to call with partial text — buffers until line boundaries.
func (m *mdPrinter) printMarkdown(text string) {
	// Prepend any buffered partial line
	text = m.lineBuffer + text
	m.lineBuffer = ""

	endsWithNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")

	// Buffer the last partial line if text didn't end with newline
	if !endsWithNewline && len(lines) > 0 {
		m.lineBuffer = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	}

	for i, line := range lines {
		colored := m.colorizeLine(line)
		fmt.Print(colored)
		// Print newline for all lines except the very last if original didn't end with one
		if i < len(lines)-1 || endsWithNewline {
			fmt.Print("\n")
		}
	}
}

// flush prints any remaining buffered partial line.
func (m *mdPrinter) flush() {
	if m.lineBuffer != "" {
		fmt.Print(m.colorizeLine(m.lineBuffer))
		m.lineBuffer = ""
	}
}

// colorizeLine applies ANSI formatting to a single line of markdown.
func (m *mdPrinter) colorizeLine(line string) string {
	trimmed := strings.TrimSpace(line)

	// Fenced code blocks: toggle on ``` and dim everything inside
	if strings.HasPrefix(trimmed, "```") {
		m.inCodeBlock = !m.inCodeBlock
		return ansiDim + line + ansiReset
	}
	if m.inCodeBlock {
		return ansiDim + line + ansiReset
	}

	// Blank lines pass through
	if trimmed == "" {
		return line
	}

	// Headers: # through ####
	if strings.HasPrefix(trimmed, "# ") {
		content := strings.TrimPrefix(trimmed, "# ")
		return ansiBoldCyan + "━━ " + content + " ━━" + ansiReset
	}
	if strings.HasPrefix(trimmed, "## ") {
		content := strings.TrimPrefix(trimmed, "## ")
		return ansiBoldCyan + "── " + content + ansiReset
	}
	if strings.HasPrefix(trimmed, "### ") {
		content := strings.TrimPrefix(trimmed, "### ")
		return ansiBold + ansiCyan + content + ansiReset
	}
	if strings.HasPrefix(trimmed, "#### ") {
		content := strings.TrimPrefix(trimmed, "#### ")
		return ansiBold + content + ansiReset
	}
	if strings.HasPrefix(trimmed, "##### ") {
		content := strings.TrimPrefix(trimmed, "##### ")
		return ansiBold + content + ansiReset
	}

	// Horizontal rules
	if trimmed == "---" || trimmed == "***" || trimmed == "___" {
		return ansiDim + "────────────────────────────────────────────────" + ansiReset
	}

	// Table separator rows: | --- | --- |  — dim the whole line
	if strings.HasPrefix(trimmed, "|") && isTableSeparator(trimmed) {
		return ansiDim + line + ansiReset
	}

	// Table header/data rows: keep pipes for alignment, apply inline formatting to cells
	if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "|") {
		return colorizeTableRow(line)
	}

	// Blockquotes
	if strings.HasPrefix(trimmed, "> ") {
		content := strings.TrimPrefix(trimmed, "> ")
		return ansiDim + "  │ " + ansiReset + ansiItalic + colorizeInline(content) + ansiReset
	}

	// List items: color the bullet
	if strings.HasPrefix(trimmed, "- ") {
		indent := line[:len(line)-len(trimmed)]
		content := strings.TrimPrefix(trimmed, "- ")
		return indent + ansiCyan + "•" + ansiReset + " " + colorizeInline(content)
	}
	if strings.HasPrefix(trimmed, "* ") {
		indent := line[:len(line)-len(trimmed)]
		content := strings.TrimPrefix(trimmed, "* ")
		return indent + ansiCyan + "•" + ansiReset + " " + colorizeInline(content)
	}

	// Numbered list items: color the number
	if isNumberedListItem(trimmed) {
		indent := line[:len(line)-len(trimmed)]
		dotIdx := strings.Index(trimmed, ". ")
		num := trimmed[:dotIdx]
		content := trimmed[dotIdx+2:]
		return indent + ansiCyan + num + "." + ansiReset + " " + colorizeInline(content)
	}

	// Regular text: apply inline formatting
	return colorizeInline(line)
}

// colorizeInline handles **bold**, `code`, and *italic* within a line.
func colorizeInline(line string) string {
	// Bold: **text**
	line = mdBoldRe.ReplaceAllString(line, ansiBold+"$1"+ansiReset)
	// Inline code: `text`
	line = mdCodeRe.ReplaceAllString(line, ansiCyan+"$1"+ansiReset)
	return line
}

// colorizeTableRow applies inline formatting to cell contents while
// preserving original pipe characters and spacing for alignment.
func colorizeTableRow(line string) string {
	// Split on pipe, format cell contents, rejoin with original pipe char
	cells := strings.Split(line, "|")
	var out strings.Builder
	for i, cell := range cells {
		if i > 0 {
			out.WriteString(ansiDim + "|" + ansiReset)
		}
		out.WriteString(colorizeInline(cell))
	}
	return out.String()
}

// isTableSeparator checks if a line is a markdown table separator like | --- | --- |
func isTableSeparator(line string) bool {
	clean := strings.ReplaceAll(line, " ", "")
	clean = strings.ReplaceAll(clean, "|", "")
	clean = strings.ReplaceAll(clean, "-", "")
	clean = strings.ReplaceAll(clean, ":", "")
	return clean == ""
}

// isNumberedListItem checks if a line starts with "1. ", "2. ", etc.
func isNumberedListItem(line string) bool {
	for i, c := range line {
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '.' && i > 0 && i < len(line)-1 && line[i+1] == ' ' {
			return true
		}
		return false
	}
	return false
}
