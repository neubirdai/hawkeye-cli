package tui

import (
	"fmt"
	"strings"
	"testing"
)

// ─── renderTable ─────────────────────────────────────────────────────────────

func TestRenderTable_Basic(t *testing.T) {
	raw := "| Name | Value |\n|------|-------|\n| foo  | bar   |"
	out := renderTable(raw)

	for _, want := range []string{"┌", "├", "└", "Name", "Value", "foo", "bar"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderTable output missing %q\nOutput:\n%s", want, out)
		}
	}
}

func TestRenderTable_LastRowPresent(t *testing.T) {
	// Regression: last row must not be silently dropped
	raw := "| Col |\n|-----|\n| row1 |\n| row2 |\n| last row |"
	out := renderTable(raw)

	if !strings.Contains(out, "last row") {
		t.Errorf("last row missing from rendered table:\n%s", out)
	}
	if !strings.Contains(out, "row1") {
		t.Errorf("row1 missing from rendered table:\n%s", out)
	}
}

func TestRenderTable_SeparatorRowConvertedToBoxLine(t *testing.T) {
	raw := "| A | B |\n|---|---|\n| 1 | 2 |"
	out := renderTable(raw)

	// The markdown |---|---| row should become a ├─┼─┤ box-drawing line, not literal dashes
	if strings.Contains(out, "|---|") {
		t.Error("markdown separator row should be replaced with box-drawing line")
	}
	if !strings.Contains(out, "├") {
		t.Error("box-drawing mid separator ├ missing")
	}
}

func TestRenderTable_WideColumnCapped(t *testing.T) {
	// A 120-char cell must be capped and wrapped — table must still render
	longVal := strings.Repeat("x", 120)
	raw := fmt.Sprintf("| Short | Long |\n|-------|------|\n| v     | %s |", longVal)
	out := renderTable(raw)

	if !strings.Contains(out, "┌") {
		t.Error("wide table should still render with top border")
	}
	// Content must still appear (may be split across lines)
	if !strings.Contains(out, "x") {
		t.Error("cell content should be present in wrapped output")
	}
}

func TestRenderTable_Empty(t *testing.T) {
	if renderTable("") != "" {
		t.Error("empty input should return empty string")
	}
	if renderTable("   \n  \n") != "" {
		t.Error("whitespace-only input should return empty string")
	}
}

func TestRenderTable_IndentedRows(t *testing.T) {
	// COT rows arrive with 4-space indent; table should still parse correctly
	raw := "    | A | B |\n    |---|---|\n    | 1 | 2 |"
	out := renderTable(raw)

	if !strings.Contains(out, "A") || !strings.Contains(out, "1") {
		t.Errorf("indented table rows should render correctly:\n%s", out)
	}
}

func TestRenderTable_ExtraColumnsIgnored(t *testing.T) {
	// If a data row has more columns than the header, extra columns should be ignored
	// This prevents malformed rows from expanding the table width
	raw := "| Col1 | Col2 |\n|------|------|\n| a | b | extra | more |"
	out := renderTable(raw)

	// Header columns should be present
	if !strings.Contains(out, "Col1") || !strings.Contains(out, "Col2") {
		t.Errorf("header columns missing:\n%s", out)
	}
	// Data from first two columns should be present
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Errorf("data columns missing:\n%s", out)
	}
	// Extra columns should NOT create additional table structure
	// Count the number of │ characters in a data row line
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "a") && strings.Contains(line, "b") {
			// This is the data row - should have exactly 3 vertical bars (│ col1 │ col2 │)
			count := strings.Count(line, "│")
			if count != 3 {
				t.Errorf("data row should have 3 vertical bars for 2 columns, got %d: %s", count, line)
			}
		}
	}
}

func TestRenderTable_TopBorderMatchesColumns(t *testing.T) {
	// Verify that the top border has the correct number of column separators
	raw := "| Pattern | Frequency | Example |\n|---------|-----------|--------|\n| a | b | c |"
	out := renderTable(raw)

	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatal("no output")
	}

	topLine := lines[0]
	// Top line should have: ┌ + dashes + ┬ + dashes + ┬ + dashes + ┐
	// That's 2 ┬ for 3 columns
	midCount := strings.Count(topLine, "┬")
	if midCount != 2 {
		t.Errorf("top border should have 2 ┬ separators for 3 columns, got %d: %s", midCount, topLine)
	}

	// Should have exactly 1 ┐ at the end (before any trailing content)
	rightCount := strings.Count(topLine, "┐")
	if rightCount != 1 {
		t.Errorf("top border should have exactly 1 ┐, got %d: %s", rightCount, topLine)
	}

	// After ┐, there should only be ANSI reset codes, not more box-drawing characters
	idx := strings.Index(topLine, "┐")
	if idx >= 0 {
		afterCorner := topLine[idx+len("┐"):]
		// Remove ANSI codes
		afterCorner = strings.ReplaceAll(afterCorner, "\x1b[0m", "")
		afterCorner = strings.ReplaceAll(afterCorner, "\x1b[36m", "")
		if strings.Contains(afterCorner, "─") || strings.Contains(afterCorner, "┬") {
			t.Errorf("content after ┐ should not contain box-drawing chars: %q", afterCorner)
		}
	}
}

// ─── wrapCell ────────────────────────────────────────────────────────────────

func TestWrapCell_NoWrapNeeded(t *testing.T) {
	result := wrapCell("hello", 10)
	if len(result) != 1 || result[0] != "hello" {
		t.Errorf("expected single line, got %v", result)
	}
}

func TestWrapCell_ExactlyMaxWidth(t *testing.T) {
	result := wrapCell("hello", 5)
	if len(result) != 1 || result[0] != "hello" {
		t.Errorf("exact-width should not wrap, got %v", result)
	}
}

func TestWrapCell_WordBoundary(t *testing.T) {
	// "hello world foo" with max=11 should split at the space after "world"
	result := wrapCell("hello world foo", 11)
	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %v", result)
	}
	if result[0] != "hello world" {
		t.Errorf("first line = %q, want %q", result[0], "hello world")
	}
	if result[1] != "foo" {
		t.Errorf("second line = %q, want %q", result[1], "foo")
	}
}

func TestWrapCell_HardWrap(t *testing.T) {
	// No spaces — must hard-wrap at maxWidth
	result := wrapCell("abcdefghij", 5)
	if len(result) < 2 {
		t.Fatalf("expected at least 2 lines for hard wrap, got %v", result)
	}
	// All content must survive the wrap
	joined := strings.Join(result, "")
	if joined != "abcdefghij" {
		t.Errorf("hard-wrapped content = %q, want original text", joined)
	}
}

func TestWrapCell_ZeroWidth(t *testing.T) {
	// Zero/negative maxWidth should not panic
	result := wrapCell("text", 0)
	if len(result) != 1 || result[0] != "text" {
		t.Errorf("zero width should return text as-is, got %v", result)
	}
}

// ─── renderMarkdownText ───────────────────────────────────────────────────────

func TestRenderMarkdownText_Headers(t *testing.T) {
	cases := []struct {
		input       string
		wantText    string
	}{
		{"# Heading 1", "Heading 1"},
		{"## Heading 2", "Heading 2"},
		{"### Heading 3", "Heading 3"},
		{"#### H4", "H4"},
		{"##### H5", "H5"},
		{"###### H6", "H6"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			out := renderMarkdownText(tc.input)
			if !strings.Contains(out, tc.wantText) {
				t.Errorf("header text %q missing from output %q", tc.wantText, out)
			}
			if !strings.Contains(out, ansiHeading) {
				t.Errorf("header should apply ansiHeading escape, got %q", out)
			}
		})
	}
}

func TestRenderMarkdownText_BulletList(t *testing.T) {
	out := renderMarkdownText("- list item")
	if !strings.Contains(out, "•") {
		t.Errorf("bullet list should use • marker, got %q", out)
	}
	if !strings.Contains(out, "list item") {
		t.Errorf("list item text missing, got %q", out)
	}
}

func TestRenderMarkdownText_IndentedList(t *testing.T) {
	// Regression: indented lists (leading spaces) must still render as bullet points
	for _, input := range []string{"    - indented", "      * deep indent"} {
		out := renderMarkdownText(input)
		if !strings.Contains(out, "•") {
			t.Errorf("indented list %q should still render as •, got %q", input, out)
		}
	}
}

func TestRenderMarkdownText_NumberedList(t *testing.T) {
	out := renderMarkdownText("1. first item")
	if !strings.Contains(out, "first item") {
		t.Errorf("numbered list text missing, got %q", out)
	}
	if !strings.Contains(out, ansiAccent) {
		t.Errorf("numbered list number should use accent color, got %q", out)
	}
}

func TestRenderMarkdownText_HorizontalRule(t *testing.T) {
	out := renderMarkdownText("---")
	if !strings.Contains(out, "─") {
		t.Errorf("horizontal rule should render box-drawing chars, got %q", out)
	}
	if !strings.Contains(out, ansiAccent) {
		t.Errorf("horizontal rule should use accent color, got %q", out)
	}
}

func TestRenderMarkdownText_PlainText(t *testing.T) {
	out := renderMarkdownText("plain body text")
	if !strings.Contains(out, "plain body text") {
		t.Errorf("plain text should be preserved, got %q", out)
	}
	if !strings.Contains(out, ansiBody) {
		t.Errorf("plain text should use ansiBody color, got %q", out)
	}
}

// ─── renderInlineMarkdown ─────────────────────────────────────────────────────

func TestRenderInlineMarkdown_Bold(t *testing.T) {
	out := renderInlineMarkdown("**bold text**")
	if !strings.Contains(out, "bold text") {
		t.Errorf("bold text missing, got %q", out)
	}
	if !strings.Contains(out, ansiBold) {
		t.Errorf("bold should apply ansiBold escape, got %q", out)
	}
}

func TestRenderInlineMarkdown_InlineCode(t *testing.T) {
	out := renderInlineMarkdown("`code snippet`")
	if !strings.Contains(out, "code snippet") {
		t.Errorf("inline code text missing, got %q", out)
	}
	if !strings.Contains(out, ansiWarning) {
		t.Errorf("inline code should use warning (yellow) color, got %q", out)
	}
}

func TestRenderInlineMarkdown_Link(t *testing.T) {
	out := renderInlineMarkdown("[click here](https://example.com)")
	if !strings.Contains(out, "click here") {
		t.Errorf("link text missing, got %q", out)
	}
	if !strings.Contains(out, "https://example.com") {
		t.Errorf("link URL missing, got %q", out)
	}
	if !strings.Contains(out, ansiInfo) {
		t.Errorf("link should use info (cyan) color, got %q", out)
	}
}

func TestRenderInlineMarkdown_PlainPassthrough(t *testing.T) {
	out := renderInlineMarkdown("no markdown here")
	if out != "no markdown here" {
		t.Errorf("plain text should pass through unchanged, got %q", out)
	}
}

// ─── renderCodeFence / renderCodeLine ──────────────────────────────────────────

func TestRenderCodeFence_WithLanguage(t *testing.T) {
	out := renderCodeFence("```plaintext")
	if !strings.Contains(out, "plaintext") {
		t.Errorf("code fence should include language label, got %q", out)
	}
	if !strings.Contains(out, "┌") {
		t.Errorf("code fence should have opening border, got %q", out)
	}
	if !strings.Contains(out, ansiSuccess) {
		t.Errorf("code fence should use success (green) color, got %q", out)
	}
}

func TestRenderCodeFence_NoLanguage(t *testing.T) {
	out := renderCodeFence("```")
	if !strings.Contains(out, "─") {
		t.Errorf("plain code fence should have border, got %q", out)
	}
	if !strings.Contains(out, ansiSuccess) {
		t.Errorf("code fence should use success (green) color, got %q", out)
	}
}

func TestRenderCodeLine(t *testing.T) {
	out := renderCodeLine("  some code content")
	if !strings.Contains(out, "some code content") {
		t.Errorf("code line should preserve content, got %q", out)
	}
	if !strings.Contains(out, "│") {
		t.Errorf("code line should have vertical border, got %q", out)
	}
	if !strings.Contains(out, ansiSuccess) {
		t.Errorf("code line border should use success (green) color, got %q", out)
	}
}
