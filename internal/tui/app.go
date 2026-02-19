package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Run launches the interactive TUI mode (inline, like Claude Code).
func Run(version, profile string) error {
	m := initialModel(version, profile)

	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
