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

	// Greens
	colorGreen = adaptiveColor(
		lipgloss.Color("78"), // bright green for dark bg
		lipgloss.Color("28"), // darker green for light bg
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

	// Magentas
	colorMagenta = adaptiveColor(
		lipgloss.Color("213"), // bright magenta for dark bg
		lipgloss.Color("127"), // darker magenta for light bg
	)

	// Blues
	colorBlue = adaptiveColor(
		lipgloss.Color("111"), // bright blue for dark bg
		lipgloss.Color("26"),  // darker blue for light bg
	)

	// Cyans
	colorCyan = adaptiveColor(
		lipgloss.Color("86"), // bright cyan for dark bg
		lipgloss.Color("30"), // darker cyan for light bg
	)
	colorLightCyan = adaptiveColor(
		lipgloss.Color("123"), // very bright cyan for dark bg
		lipgloss.Color("37"),  // teal for light bg
	)

	// Grays — these need the most adjustment
	colorGray = adaptiveColor(
		lipgloss.Color("250"), // light gray for dark bg (visible!)
		lipgloss.Color("240"), // darker gray for light bg
	)
	colorDimGray = adaptiveColor(
		lipgloss.Color("245"), // medium gray for dark bg
		lipgloss.Color("250"), // lighter for light bg (less contrast needed)
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
	Foreground(colorMagenta).
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
