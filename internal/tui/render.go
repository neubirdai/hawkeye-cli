package tui

import (
	"fmt"
	"strings"
)

// ─── Welcome Screen ─────────────────────────────────────────────────────────

func renderWelcome(version, server, projectName string, width int) string {
	titleLine := logoTitleStyle.Render("Hawkeye CLI") + " " + versionStyle.Render("v"+version)

	var infoLine string
	if server == "" {
		infoLine = welcomeHintStyle.Render("Type /login <url> to get started")
	} else {
		serverDisplay := server
		if len(serverDisplay) > 40 {
			serverDisplay = serverDisplay[:37] + "..."
		}
		projectDisplay := dimStyle.Render("not set")
		if projectName != "" {
			projectDisplay = projectName
			if len(projectDisplay) > 36 {
				projectDisplay = projectDisplay[:33] + "..."
			}
		}
		infoLine = welcomeInfoLabel.Render(fmt.Sprintf("%s · %s", serverDisplay, projectDisplay))
	}

	bird := renderBirdASCIIArt()
	return fmt.Sprintf("\n%s\n\n%s\n%s\n", bird, titleLine, infoLine)
}

const hawkASCIIArt = `
             **************
          ********************
       ******             *******
      *****                  *****
    ****                       *****
   ****        *******++++++++++++++++++++++
  ****       *************++++++++++++++++++++
  ***       *****     *****+++++++++++++++++++++
 ****      ****  *****  ****++++++++++++++++++++++
 ****     ***** ******** ****++++++++++++++++++++++
 ****    ****** ******** ****++++++++++++++++++++++
 ****    ******  *****  ****+++++++++++++++++++++++
  ***   ******  **    ******+++++++++++++++++++++++
  ****  ****  **************      ****
   ****     *****************   *****
    ***** **************************
      ****************************
       *************************
          ********************
              ************
`

func renderBirdASCIIArt() string {
	lines := strings.Split(hawkASCIIArt, "\n")
	lines = trimEmptyEdgeLines(lines)

	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := countLeadingSpaces(line)
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	for i, line := range lines {
		line = strings.TrimRight(line, " ")
		if minIndent > 0 && len(line) >= minIndent {
			line = line[minIndent:]
		}
		lines[i] = colorizeBirdLine(line)
	}

	return strings.Join(lines, "\n")
}

func trimEmptyEdgeLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

func countLeadingSpaces(s string) int {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}

func colorizeBirdLine(line string) string {
	const (
		stylePlain = iota
		styleBody
		styleBeak
	)

	styleFor := func(r rune) int {
		switch r {
		case '*', '%', '@':
			return styleBody
		case '+', '#':
			return styleBeak
		default:
			return stylePlain
		}
	}

	render := func(style int, s string) string {
		switch style {
		case styleBody:
			return logoBodyStyle.Render(s)
		case styleBeak:
			return logoBeakStyle.Render(s)
		default:
			return s
		}
	}

	var out strings.Builder
	var run strings.Builder
	currentStyle := stylePlain
	first := true

	flush := func() {
		if run.Len() == 0 {
			return
		}
		out.WriteString(render(currentStyle, run.String()))
		run.Reset()
	}

	for _, r := range line {
		nextStyle := styleFor(r)
		if first {
			currentStyle = nextStyle
			first = false
		} else if nextStyle != currentStyle {
			flush()
			currentStyle = nextStyle
		}
		run.WriteRune(r)
	}

	flush()
	return out.String()
}

// ─── Markdown (lightweight in-house renderer) ──────────────────────────────
//
// Fast line-by-line markdown rendering without external dependencies.
// Supports: headers, bold, italic, code blocks, inline code, lists,
// blockquotes, links, horizontal rules.

// ANSI escape codes for styling
// See docs/tui-color-format-guide.md for the full color & format specification.

// Dark mode
const (
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiItalic    = "\033[3m"
	ansiUnderline = "\033[4m"

	ansiHeading = "\033[1;97m"     // bold bright white — all header levels (distinct from **bold**)
	ansiInfo    = "\033[38;5;39m"  // cyan 39    — links, session, icons
	ansiWarning = "\033[38;5;220m" // yellow 220 — inline code
	ansiSuccess = "\033[38;5;78m"  // green 78   — ✓, code borders (softer than neon 46)
	ansiAccent  = "\033[38;5;73m"  // teal 73    — │, ──, numbered dots
	ansiBody    = "\033[38;5;252m" // light 252  — body text (readable content, not metadata)
)

// Light mode — swap these in when bg is light
// const (
// 	ansiInfo    = "\033[38;5;25m"  // blue 25
// 	ansiWarning = "\033[38;5;136m" // amber 136
// 	ansiSuccess = "\033[38;5;28m"  // green 28
// 	ansiAccent  = "\033[38;5;30m"  // teal 30
// 	ansiBody    = "\033[38;5;239m" // dark mid 239 — readable on light bg
// 	// ansiHeading: use \033[1;30m (bold + near-black) on light bg
// )

// mdState tracks state across lines (e.g., inside code block)
type mdState struct {
	inCodeBlock bool
}

// renderMarkdownLine renders a single line of markdown to styled terminal output.
// See docs/tui-color-format-guide.md for rendering rules.
func renderMarkdownLine(line string, state *mdState) string {
	trimmed := strings.TrimSpace(line)

	// Code block fences — borders in green (success)
	if strings.HasPrefix(trimmed, "```") {
		if !state.inCodeBlock {
			state.inCodeBlock = true
			lang := strings.TrimSpace(trimmed[3:])
			if lang != "" {
				return fmt.Sprintf("%s┌─ %s ─%s", ansiSuccess, lang, ansiReset)
			}
			return fmt.Sprintf("%s┌──%s", ansiSuccess, ansiReset)
		}
		state.inCodeBlock = false
		return fmt.Sprintf("%s└──%s", ansiSuccess, ansiReset)
	}

	// Inside code block — green border, body color text
	if state.inCodeBlock {
		return fmt.Sprintf("%s│%s %s%s%s", ansiSuccess, ansiReset, ansiBody, line, ansiReset)
	}

	// Headers — bold only, no color (all levels)
	if strings.HasPrefix(trimmed, "###### ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[7:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "##### ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[6:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "#### ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[5:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "### ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[4:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "## ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[3:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "# ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[2:], ansiReset)
	}

	// Horizontal rules — teal (accent)
	if trimmed == "---" || trimmed == "***" || trimmed == "___" {
		return fmt.Sprintf("%s────────────────────────────────────────%s", ansiAccent, ansiReset)
	}

	// Blockquotes — teal pipe, body color text
	if strings.HasPrefix(trimmed, "> ") {
		return fmt.Sprintf("%s│%s %s%s%s", ansiAccent, ansiReset, ansiBody, renderInlineMarkdown(trimmed[2:]), ansiReset)
	}

	// Preserve indentation for lists
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	pad := strings.Repeat(" ", indent)

	// Bullet lists — body color
	if strings.HasPrefix(trimmed, "- ") {
		return fmt.Sprintf("%s%s• %s%s", pad, ansiBody, renderInlineMarkdown(trimmed[2:]), ansiReset)
	}
	if strings.HasPrefix(trimmed, "* ") {
		return fmt.Sprintf("%s%s• %s%s", pad, ansiBody, renderInlineMarkdown(trimmed[2:]), ansiReset)
	}

	// Numbered lists — teal number dot, body color text
	if dotIdx := strings.Index(trimmed, ". "); dotIdx > 0 && dotIdx <= 3 {
		num := trimmed[:dotIdx]
		allDigit := true
		for _, c := range num {
			if c < '0' || c > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			return fmt.Sprintf("%s%s%s.%s %s%s%s", pad, ansiAccent, num, ansiReset, ansiBody, renderInlineMarkdown(trimmed[dotIdx+2:]), ansiReset)
		}
	}

	// Regular text — body color
	return fmt.Sprintf("%s%s%s", ansiBody, renderInlineMarkdown(line), ansiReset)
}

// renderMarkdownText renders a single line with list/header detection + inline formatting.
// This is a stateless version for streaming (doesn't track code block state).
// See docs/tui-color-format-guide.md for rendering rules.
func renderMarkdownText(line string) string {
	trimmed := strings.TrimSpace(line)

	// Headers — bold only, no color (all levels)
	if strings.HasPrefix(trimmed, "###### ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[7:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "##### ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[6:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "#### ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[5:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "### ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[4:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "## ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[3:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "# ") {
		return fmt.Sprintf("%s%s%s", ansiHeading, trimmed[2:], ansiReset)
	}

	// Horizontal rules — teal (accent)
	if trimmed == "---" || trimmed == "***" || trimmed == "___" {
		return fmt.Sprintf("%s────────────────────────────────────────%s", ansiAccent, ansiReset)
	}

	// Blockquotes — teal pipe, body color text
	if strings.HasPrefix(trimmed, "> ") {
		return fmt.Sprintf("%s│%s %s%s%s", ansiAccent, ansiReset, ansiBody, renderInlineMarkdown(trimmed[2:]), ansiReset)
	}

	// Preserve original indentation
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	pad := strings.Repeat(" ", indent)

	// Bullet lists — body color
	if strings.HasPrefix(trimmed, "- ") {
		return fmt.Sprintf("%s%s• %s%s", pad, ansiBody, renderInlineMarkdown(trimmed[2:]), ansiReset)
	}
	if strings.HasPrefix(trimmed, "* ") {
		return fmt.Sprintf("%s%s• %s%s", pad, ansiBody, renderInlineMarkdown(trimmed[2:]), ansiReset)
	}

	// Numbered lists — teal number dot, body color text
	if dotIdx := strings.Index(trimmed, ". "); dotIdx > 0 && dotIdx <= 3 {
		num := trimmed[:dotIdx]
		allDigit := true
		for _, c := range num {
			if c < '0' || c > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			return fmt.Sprintf("%s%s%s.%s %s%s%s", pad, ansiAccent, num, ansiReset, ansiBody, renderInlineMarkdown(trimmed[dotIdx+2:]), ansiReset)
		}
	}

	// Regular text — body color
	return fmt.Sprintf("%s%s%s", ansiBody, renderInlineMarkdown(line), ansiReset)
}

// renderInlineMarkdown handles inline formatting: **bold**, *italic*, `code`, [links](url)
// See docs/tui-color-format-guide.md for rendering rules.
func renderInlineMarkdown(text string) string {
	var out strings.Builder
	i := 0
	for i < len(text) {
		// Bold: **text**
		if i+3 < len(text) && text[i] == '*' && text[i+1] == '*' {
			end := strings.Index(text[i+2:], "**")
			if end > 0 {
				out.WriteString(ansiBold)
				out.WriteString(renderInlineMarkdown(text[i+2 : i+2+end]))
				out.WriteString(ansiReset)
				i += 4 + end
				continue
			}
		}

		// Italic: *text*
		if text[i] == '*' && (i == 0 || text[i-1] == ' ') {
			end := strings.IndexByte(text[i+1:], '*')
			if end > 0 {
				out.WriteString(ansiItalic)
				out.WriteString(text[i+1 : i+1+end])
				out.WriteString(ansiReset)
				i += 2 + end
				continue
			}
		}

		// Inline code: `code` — yellow (warning)
		if text[i] == '`' {
			end := strings.IndexByte(text[i+1:], '`')
			if end >= 0 {
				out.WriteString(ansiWarning)
				out.WriteString(text[i+1 : i+1+end])
				out.WriteString(ansiReset)
				i += 2 + end
				continue
			}
		}

		// Links: [text](url) — underline + cyan (info) on text, URL in cyan
		if text[i] == '[' {
			cb := strings.IndexByte(text[i:], ']')
			if cb > 1 && i+cb+1 < len(text) && text[i+cb+1] == '(' {
				cp := strings.IndexByte(text[i+cb+1:], ')')
				if cp > 0 {
					linkText := text[i+1 : i+cb]
					url := text[i+cb+2 : i+cb+1+cp]
					out.WriteString(ansiUnderline)
					out.WriteString(ansiInfo)
					out.WriteString(linkText)
					out.WriteString(ansiReset)
					out.WriteString(ansiInfo)
					out.WriteString(" (")
					out.WriteString(url)
					out.WriteString(")")
					out.WriteString(ansiReset)
					i += cb + 1 + cp + 1
					continue
				}
			}
		}

		out.WriteByte(text[i])
		i++
	}
	return out.String()
}

// renderMarkdownBlock renders a full markdown block (multiple lines)
func renderMarkdownBlock(content string) string {
	lines := strings.Split(content, "\n")
	state := &mdState{}
	var result []string
	for _, line := range lines {
		result = append(result, renderMarkdownLine(line, state))
	}
	return strings.Join(result, "\n")
}

// renderTable formats a markdown pipe table into a styled terminal table.
// It parses all rows, computes column widths, caps them to fit the terminal,
// and wraps cell content across multiple lines when needed.
// See docs/tui-color-format-guide.md for rendering rules.
func renderTable(raw string) string {
	rawLines := strings.Split(raw, "\n")

	// Parse rows into cells, stripping leading indentation and outer pipes.
	type row struct {
		cells []string
		isSep bool // separator row (|---|)
	}
	var rows []row
	indent := ""

	for _, line := range rawLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Capture indent from first row
		if indent == "" {
			indent = strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " \t")))
		}
		trimmed := strings.TrimSpace(line)
		// Strip outer pipes
		trimmed = strings.Trim(trimmed, "|")
		parts := strings.Split(trimmed, "|")
		cells := make([]string, len(parts))
		isSep := true
		for i, p := range parts {
			cells[i] = strings.TrimSpace(p)
			// A separator row has cells containing only dashes/colons
			if len(cells[i]) > 0 {
				for _, c := range cells[i] {
					if c != '-' && c != ':' && c != ' ' {
						isSep = false
					}
				}
			}
		}
		rows = append(rows, row{cells: cells, isSep: isSep})
	}

	if len(rows) == 0 {
		return ""
	}

	// Compute natural column widths across all non-separator rows.
	numCols := 0
	for _, r := range rows {
		if len(r.cells) > numCols {
			numCols = len(r.cells)
		}
	}
	widths := make([]int, numCols)
	for _, r := range rows {
		if r.isSep {
			continue
		}
		for i, cell := range r.cells {
			if i < numCols && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Cap total table width to fit a reasonable terminal.
	// Overhead per column: 1 border + 2 padding spaces = 3; plus 1 for the final border.
	const maxTableWidth = 100 // content area (excluding table's own indent)
	const minColWidth = 8     // never shrink a column below this
	overhead := 3*numCols + 1
	available := maxTableWidth - overhead
	if available < numCols*minColWidth {
		available = numCols * minColWidth
	}

	totalNat := 0
	for _, w := range widths {
		totalNat += w
	}

	if totalNat > available {
		// Binary-search for the smallest column cap that keeps total ≤ available.
		maxNat := 0
		for _, w := range widths {
			if w > maxNat {
				maxNat = w
			}
		}
		lo, hi := minColWidth, maxNat
		colCap := maxNat
		for lo <= hi {
			mid := (lo + hi) / 2
			total := 0
			for _, w := range widths {
				total += min(w, mid)
			}
			if total <= available {
				colCap = mid
				hi = mid - 1
			} else {
				lo = mid + 1
			}
		}
		for i, w := range widths {
			if w > colCap {
				widths[i] = colCap
			}
		}
	}

	// Build separator line using box-drawing characters.
	buildSepLine := func(left, mid, right, fill string) string {
		var sb strings.Builder
		sb.WriteString(ansiAccent)
		sb.WriteString(left)
		for i, w := range widths {
			sb.WriteString(strings.Repeat(fill, w+2))
			if i < len(widths)-1 {
				sb.WriteString(mid)
			}
		}
		sb.WriteString(right)
		sb.WriteString(ansiReset)
		return sb.String()
	}

	topLine := buildSepLine("┌", "┬", "┐", "─")
	midLine := buildSepLine("├", "┼", "┤", "─")
	botLine := buildSepLine("└", "┴", "┘", "─")

	var out strings.Builder
	out.WriteString(indent + topLine + "\n")

	for rowIdx, r := range rows {
		if r.isSep {
			// Replace markdown separator row with box-drawing mid-line
			out.WriteString(indent + midLine + "\n")
			continue
		}

		// If there's no markdown separator row, insert mid-line after the first row
		if rowIdx == 1 {
			hasSep := false
			for _, prev := range rows[:rowIdx] {
				if prev.isSep {
					hasSep = true
				}
			}
			if !hasSep {
				out.WriteString(indent + midLine + "\n")
			}
		}

		// Wrap each cell to its capped column width, producing sub-lines.
		cellLines := make([][]string, numCols)
		maxSubLines := 1
		for i := 0; i < numCols; i++ {
			cell := ""
			if i < len(r.cells) {
				cell = r.cells[i]
			}
			cellLines[i] = wrapCell(cell, widths[i])
			if len(cellLines[i]) > maxSubLines {
				maxSubLines = len(cellLines[i])
			}
		}

		// Render each sub-line of the row.
		for lineIdx := 0; lineIdx < maxSubLines; lineIdx++ {
			out.WriteString(ansiAccent + indent + "│" + ansiReset)
			for i := 0; i < numCols; i++ {
				cell := ""
				if lineIdx < len(cellLines[i]) {
					cell = cellLines[i][lineIdx]
				}
				var rendered string
				if rowIdx == 0 {
					// Header row: bold
					rendered = ansiBold + cell + ansiReset
				} else {
					rendered = ansiBody + cell + ansiReset
				}
				padding := strings.Repeat(" ", widths[i]-len(cell))
				out.WriteString(" " + rendered + padding + " ")
				out.WriteString(ansiAccent + "│" + ansiReset)
			}
			out.WriteString("\n")
		}
	}

	out.WriteString(indent + botLine)
	return out.String()
}

// wrapCell splits text into lines of at most maxWidth characters.
// It tries to break at word boundaries (spaces); falls back to hard wrap
// when no good break point is found in the second half of the line.
func wrapCell(text string, maxWidth int) []string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return []string{text}
	}
	var lines []string
	for len(text) > maxWidth {
		split := maxWidth
		// Walk back from maxWidth looking for a space to break at.
		for split > maxWidth/2 && text[split] != ' ' {
			split--
		}
		if split <= maxWidth/2 {
			// No good break point — hard wrap at maxWidth.
			split = maxWidth
		}
		lines = append(lines, text[:split])
		text = strings.TrimLeft(text[split:], " ")
	}
	if len(text) > 0 {
		lines = append(lines, text)
	}
	return lines
}

func indentText(text, prefix string) string {
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
