package api

import "fmt"

// mdPrinter handles streaming text output.
// Pure passthrough — prints text exactly as received.
type mdPrinter struct {
	lineBuffer  string
	inCodeBlock bool
}

// printMarkdown prints text as-is.
func (m *mdPrinter) printMarkdown(text string) {
	fmt.Print(text)
}

// flush is a no-op in passthrough mode.
func (m *mdPrinter) flush() {}

// ANSI constants for terminal styling.
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
