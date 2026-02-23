package tui

import "github.com/charmbracelet/lipgloss"

// ─── Adaptive Colors ─────────────────────────────────────────────────────────
//
// We detect the terminal background at init time and pick a color scheme
// that contrasts well. Dark backgrounds get brighter colors; light backgrounds
// get darker colors.

var hasDarkBg = lipgloss.HasDarkBackground()

// adaptiveColor returns the appropriate color based on terminal background.
func adaptiveColor(dark, light lipgloss.Color) lipgloss.Color {
	if hasDarkBg {
		return dark
	}
	return light
}

// ─── Color Palette ───────────────────────────────────────────────────────────

var (
	// Primary accent — orange (works on both, slightly adjusted)
	colorOrange = adaptiveColor(
		lipgloss.Color("#F28C28"), // warm orange for dark bg
		lipgloss.Color("#D2691E"), // darker orange for light bg
	)

	// Greens — 78/28: matches ansiSuccess in render.go
	colorGreen = adaptiveColor(
		lipgloss.Color("78"), // green 78 for dark bg — softer than neon 46
		lipgloss.Color("28"), // green 28 for light bg
	)

	// Yellows
	colorYellow = adaptiveColor(
		lipgloss.Color("220"), // bright yellow for dark bg
		lipgloss.Color("136"), // darker gold for light bg
	)

	// Reds
	colorRed = adaptiveColor(
		lipgloss.Color("196"), // bright red for dark bg
		lipgloss.Color("160"), // darker red for light bg
	)

	// Blues
	colorBlue = adaptiveColor(
		lipgloss.Color("111"), // bright blue for dark bg
		lipgloss.Color("26"),  // darker blue for light bg
	)

	// Cyans — unified to one family
	// 39/25: info cyan — links, session names, interactive elements
	colorCyan = adaptiveColor(
		lipgloss.Color("39"), // cyan 39 for dark bg — matches ansiInfo in render.go
		lipgloss.Color("25"), // blue 25 for light bg
	)
	// 73/30: accent teal — COT, structural chrome (softer than cyan)
	colorLightCyan = adaptiveColor(
		lipgloss.Color("73"), // teal 73 for dark bg — matches ansiAccent in render.go
		lipgloss.Color("30"), // teal 30 for light bg
	)

	// Grays — these need the most adjustment
	colorGray = adaptiveColor(
		lipgloss.Color("250"), // light gray for dark bg (visible!)
		lipgloss.Color("240"), // darker gray for light bg
	)
	colorDimGray = adaptiveColor(
		lipgloss.Color("240"), // gray 240 for dark bg — visible but dimmer than body (252)
		lipgloss.Color("245"), // mid gray for light bg — dimmer than body text
	)

	// White/Black swap for text
	colorWhite = adaptiveColor(
		lipgloss.Color("255"), // white for dark bg
		lipgloss.Color("232"), // near-black for light bg
	)
)

// ─── Welcome ────────────────────────────────────────────────────────────────

var logoBodyStyle = lipgloss.NewStyle().
	Foreground(colorGray)

var logoBeakStyle = lipgloss.NewStyle().
	Foreground(colorOrange)

var logoTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(colorOrange)

var versionStyle = lipgloss.NewStyle().
	Foreground(colorGray)

var welcomeHintStyle = lipgloss.NewStyle().
	Foreground(colorGray).
	Italic(true)

var welcomeInfoLabel = lipgloss.NewStyle().
	Foreground(colorGray)

// ─── Input / Prompt ─────────────────────────────────────────────────────────

var promptSymbol = lipgloss.NewStyle().
	Foreground(colorOrange).
	Bold(true)

// ─── Hint Bar ───────────────────────────────────────────────────────────────

var hintBarStyle = lipgloss.NewStyle().
	Foreground(colorGray)

var hintKeyStyle = lipgloss.NewStyle().
	Foreground(colorGray).
	Bold(true)

// Command menu styles
var cmdNameStyle = lipgloss.NewStyle().
	Foreground(colorOrange)

var cmdDescStyle = lipgloss.NewStyle().
	Foreground(colorGray)

// Selected/highlighted command in the menu
var cmdSelectedNameStyle = lipgloss.NewStyle().
	Foreground(colorOrange).
	Bold(true).
	Reverse(true)

var cmdSelectedDescStyle = lipgloss.NewStyle().
	Foreground(colorWhite).
	Bold(true)

// ─── Output Styles ──────────────────────────────────────────────────────────

var successMsgStyle = lipgloss.NewStyle().
	Foreground(colorGreen)

var errorMsgStyle = lipgloss.NewStyle().
	Foreground(colorRed)

var warnMsgStyle = lipgloss.NewStyle().
	Foreground(colorYellow)

var statusStyle = lipgloss.NewStyle().
	Foreground(colorYellow)

var userPromptStyle = lipgloss.NewStyle().
	Foreground(colorOrange).
	Bold(true)

var sourceHeaderStyle = lipgloss.NewStyle().
	Foreground(colorBlue).
	Bold(true)

var cotHeaderStyle = lipgloss.NewStyle().
	Foreground(colorCyan). // info cyan — investigation step marker
	Bold(true)

// COT explanation text (↳ line) — visible on both backgrounds
var cotExplanationStyle = lipgloss.NewStyle().
	Foreground(colorLightCyan)

// Session name style — visible and distinct
var sessionNameStyle = lipgloss.NewStyle().
	Foreground(colorCyan)

var followUpStyle = lipgloss.NewStyle().
	Foreground(colorOrange)

var dimStyle = lipgloss.NewStyle().
	Foreground(colorGray)

var separatorStyle = lipgloss.NewStyle().
	Foreground(colorDimGray)
