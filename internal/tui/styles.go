package tui

import "github.com/charmbracelet/lipgloss"

// ─── Colors ─────────────────────────────────────────────────────────────────

var (
	colorOrange  = lipgloss.Color("#F28C28") // warm comfortable orange — primary accent
	colorGreen   = lipgloss.Color("78")
	colorYellow  = lipgloss.Color("220")
	colorRed     = lipgloss.Color("196")
	colorMagenta = lipgloss.Color("213")
	colorBlue    = lipgloss.Color("111")
	colorGray    = lipgloss.Color("242")
	colorDimGray = lipgloss.Color("238")
	colorWhite   = lipgloss.Color("255")
)

// ─── Welcome ────────────────────────────────────────────────────────────────

var logoBodyStyle = lipgloss.NewStyle().
	Foreground(colorGray)

var logoBeakStyle = lipgloss.NewStyle().
	Foreground(colorOrange)

var logoEyeStyle = lipgloss.NewStyle().
	Foreground(colorWhite).
	Bold(true)

var logoTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(colorWhite)

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

var dimHintStyle = lipgloss.NewStyle().
	Foreground(colorDimGray)

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

var followUpStyle = lipgloss.NewStyle().
	Foreground(colorOrange)

var dimStyle = lipgloss.NewStyle().
	Foreground(colorGray)

var separatorStyle = lipgloss.NewStyle().
	Foreground(colorDimGray)
