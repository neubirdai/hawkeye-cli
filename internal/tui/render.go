package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
)

// ─── Welcome Screen ─────────────────────────────────────────────────────────

func renderWelcome(version, server, project, org string, width int) string {
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
		if project != "" {
			projectDisplay = project
			if len(projectDisplay) > 36 {
				projectDisplay = projectDisplay[:8] + "..." + projectDisplay[len(projectDisplay)-4:]
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

// ─── Markdown ───────────────────────────────────────────────────────────────

func renderMarkdown(content string, width int) string {
	if width < 40 {
		width = 40
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return indentText(content, "  ")
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		return indentText(content, "  ")
	}

	return strings.TrimRight(rendered, "\n")
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
