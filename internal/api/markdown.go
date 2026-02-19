package api

import (
	"fmt"
	"regexp"
	"strings"
)

var htmlTagRe2 = regexp.MustCompile(`<[^>]+>`)

func cleanHTML(s string) string {
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	return htmlTagRe2.ReplaceAllString(s, "")
}

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
	ansiGray      = "\033[90m"
)

type mdPrinter struct {
	buf    string
	inCode bool
}

func (m *mdPrinter) printMarkdown(text string) {
	m.buf += cleanHTML(text)
	for {
		idx := strings.IndexByte(m.buf, '\n')
		if idx < 0 {
			break
		}
		line := m.buf[:idx]
		m.buf = m.buf[idx+1:]
		fmt.Println(m.renderLine(line))
	}
}

func (m *mdPrinter) flush() {
	if m.buf == "" {
		return
	}
	fmt.Print(m.renderLine(m.buf))
	m.buf = ""
}

func (m *mdPrinter) renderLine(line string) string {
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "```") {
		if !m.inCode {
			m.inCode = true
			lang := strings.TrimSpace(trimmed[3:])
			if lang != "" {
				return fmt.Sprintf("  %s┌─ %s ─%s", ansiDim, lang, ansiReset)
			}
			return fmt.Sprintf("  %s┌──%s", ansiDim, ansiReset)
		}
		m.inCode = false
		return fmt.Sprintf("  %s└──%s", ansiDim, ansiReset)
	}

	if m.inCode {
		return fmt.Sprintf("  %s│%s %s", ansiDim, ansiReset, line)
	}

	if strings.HasPrefix(trimmed, "#### ") {
		return fmt.Sprintf("  %s%s%s", ansiBold, trimmed[5:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "### ") {
		return fmt.Sprintf("  %s%s%s", ansiBold, trimmed[4:], ansiReset)
	}
	if strings.HasPrefix(trimmed, "## ") {
		return fmt.Sprintf("\n  %s%s%s%s", ansiBoldCyan, trimmed[3:], ansiReset, "")
	}
	if strings.HasPrefix(trimmed, "# ") {
		return fmt.Sprintf("\n  %s%s%s%s", ansiBoldCyan, trimmed[2:], ansiReset, "")
	}

	if trimmed == "---" || trimmed == "***" || trimmed == "___" {
		return fmt.Sprintf("  %s────────────────────────────────────────%s", ansiDim, ansiReset)
	}

	if strings.HasPrefix(trimmed, "> ") {
		return fmt.Sprintf("  %s│%s %s", ansiDim, ansiReset, renderInline(trimmed[2:]))
	}

	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	pad := strings.Repeat(" ", indent)

	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return fmt.Sprintf("%s  • %s", pad, renderInline(trimmed[2:]))
	}

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
			return fmt.Sprintf("%s  %s. %s", pad, num, renderInline(trimmed[dotIdx+2:]))
		}
	}

	return renderInline(line)
}

func renderInline(text string) string {
	var out strings.Builder
	i := 0
	for i < len(text) {
		if i+3 < len(text) && text[i] == '*' && text[i+1] == '*' {
			end := strings.Index(text[i+2:], "**")
			if end > 0 {
				out.WriteString(ansiBold)
				out.WriteString(renderInline(text[i+2 : i+2+end]))
				out.WriteString(ansiReset)
				i += 4 + end
				continue
			}
		}

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

		if text[i] == '`' {
			end := strings.IndexByte(text[i+1:], '`')
			if end >= 0 {
				out.WriteString(ansiDim)
				out.WriteString(text[i+1 : i+1+end])
				out.WriteString(ansiReset)
				i += 2 + end
				continue
			}
		}

		if text[i] == '[' {
			cb := strings.IndexByte(text[i:], ']')
			if cb > 1 && i+cb+1 < len(text) && text[i+cb+1] == '(' {
				cp := strings.IndexByte(text[i+cb+1:], ')')
				if cp > 0 {
					linkText := text[i+1 : i+cb]
					url := text[i+cb+2 : i+cb+1+cp]
					out.WriteString(ansiUnderline)
					out.WriteString(linkText)
					out.WriteString(ansiReset)
					out.WriteString(ansiDim)
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

func RenderMarkdown(text string) string {
	var m mdPrinter
	var lines []string
	for _, line := range strings.Split(cleanHTML(text), "\n") {
		lines = append(lines, m.renderLine(line))
	}
	return strings.Join(lines, "\n")
}
