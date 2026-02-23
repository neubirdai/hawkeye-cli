package tui

import (
	"fmt"
	"strings"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── App mode ───────────────────────────────────────────────────────────────

type appMode int

const (
	modeIdle appMode = iota
	modeStreaming
	modeLoginURL
	modeLoginUser
	modeLoginPass
)

// ─── Slash command registry ─────────────────────────────────────────────────

type slashCmd struct {
	name string
	desc string
}

var slashCommands = []slashCmd{
	{"/clear", "Clear the screen"},
	{"/config", "Show current configuration"},
	{"/connections", "Manage data source connections"},
	{"/connections list", "List data source connections"},
	{"/connections resources", "List resources for a connection"},
	{"/discover", "Discover project resources"},
	{"/feedback", "Thumbs down feedback"},
	{"/help", "Show all commands"},
	{"/incidents", "Add incident tool connections"},
	{"/incidents add pagerduty", "Add a PagerDuty connection (--name, --api-key)"},
	{"/incidents add firehydrant", "Add a FireHydrant connection (--name, --api-key)"},
	{"/incidents add incidentio", "Add an incident.io connection (--name, --api-key)"},
	{"/incidents test pagerduty", "Test PagerDuty incidents (--api-key, --routing-key, --file, --run-level)"},
	{"/incidents test firehydrant", "Test FireHydrant incidents (--api-key, --file, --run-level)"},
	{"/incidents test incidentio", "Test incident.io incidents (--api-key, --file, --run-level)"},
	{"/inspect", "View session details"},
	{"/instructions", "Manage project instructions"},
	{"/investigate-alert", "Investigate an alert"},
	{"/link", "Get web UI URL for session"},
	{"/login", "Login to a Hawkeye server"},
	{"/projects", "List/manage projects"},
	{"/prompts", "Browse investigation prompts"},
	{"/queries", "Show investigation queries"},
	{"/quit", "Exit Hawkeye"},
	{"/report", "Show incident analytics"},
	{"/rerun", "Rerun an investigation"},
	{"/score", "Show RCA quality scores"},
	{"/session", "Set active session"},
	{"/session-report", "Per-session report"},
	{"/sessions", "List recent sessions"},
	{"/set", "Set project or config"},
	{"/summary", "Get session summary"},
}

// ─── Model ──────────────────────────────────────────────────────────────────

type model struct {
	width  int
	height int

	// Bubble Tea components
	input   textinput.Model
	spinner spinner.Model

	// App state
	mode      appMode
	cfg       *config.Config
	client    api.HawkeyeAPI
	sessionID string
	version   string

	// Stream processor (manages all buffering, gating, block transitions)
	processor    *StreamProcessor
	streamPrompt string // the prompt being streamed

	// Login flow state
	loginURL  string
	loginUser string

	// UI state
	ready        bool
	cmdMenuIdx   int    // selected index in command menu (-1 = none)
	cmdMenuOpen  bool   // whether the command menu is visible
	lastInputVal string // track input changes to reset menu index
	profile      string

	// Command history
	history      []string // stored command history
	historyIdx   int      // current position in history (-1 = not browsing)
	historySaved string   // saved input value when entering history mode
}

func initialModel(version, profile string) model {
	ti := textinput.New()
	ti.Placeholder = "Ask a question or type /help..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Prompt = "❯ "
	ti.PromptStyle = promptSymbol
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorOrange)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorOrange)

	cfg, _ := config.Load(profile)

	var client api.HawkeyeAPI
	if cfg != nil && cfg.Server != "" && cfg.Token != "" {
		client = api.NewClient(cfg)
	}

	return model{
		input:      ti,
		spinner:    sp,
		version:    version,
		profile:    profile,
		cfg:        cfg,
		client:     client,
		mode:       modeIdle,
		processor:  NewStreamProcessor(),
		history:    make([]string, 0),
		historyIdx: -1,
	}
}

// ─── Init ───────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

// ─── Update ─────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = m.width - 6

		if !m.ready {
			m.ready = true
			// Print welcome header on first render
			welcome := renderWelcome(m.version, serverStr(m.cfg), projectStr(m.cfg), orgStr(m.cfg), m.width)
			cmds = append(cmds, tea.Println(welcome))
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.mode == modeStreaming {
				m.mode = modeIdle
				activeStreamCh = nil
				m.resetStreamState()
				cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! Investigation cancelled.")))
				return m, tea.Batch(cmds...)
			}
			return m, tea.Quit

		case tea.KeyEsc:
			if m.mode == modeStreaming {
				m.mode = modeIdle
				activeStreamCh = nil
				m.resetStreamState()
				cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! Investigation cancelled.")))
				return m, tea.Batch(cmds...)
			}
			if m.mode == modeLoginURL || m.mode == modeLoginUser || m.mode == modeLoginPass {
				m.mode = modeIdle
				m.input.Placeholder = "Ask a question or type /help..."
				m.input.SetValue("")
				m.input.EchoMode = textinput.EchoNormal
				cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! Login cancelled.")))
				return m, tea.Batch(cmds...)
			}
			if m.cmdMenuOpen {
				m.cmdMenuOpen = false
				m.cmdMenuIdx = 0
				return m, nil
			}

		case tea.KeyUp:
			if m.mode == modeIdle {
				if m.cmdMenuOpen {
					matches := matchCommands(m.input.Value())
					if len(matches) > 0 {
						m.cmdMenuIdx--
						if m.cmdMenuIdx < 0 {
							m.cmdMenuIdx = len(matches) - 1
						}
						return m, nil
					}
				} else if len(m.history) > 0 {
					// Navigate command history
					if m.historyIdx == -1 {
						// Entering history mode - save current input
						m.historySaved = m.input.Value()
						m.historyIdx = len(m.history) - 1
					} else {
						// Move up in history
						m.historyIdx--
						if m.historyIdx < 0 {
							m.historyIdx = 0
						}
					}
					m.input.SetValue(m.history[m.historyIdx])
					m.input.CursorEnd()
					return m, nil
				}
			}

		case tea.KeyDown:
			if m.mode == modeIdle {
				if m.cmdMenuOpen {
					matches := matchCommands(m.input.Value())
					if len(matches) > 0 {
						m.cmdMenuIdx++
						if m.cmdMenuIdx >= len(matches) {
							m.cmdMenuIdx = 0
						}
						return m, nil
					}
				} else if m.historyIdx != -1 {
					// Navigate down in history
					m.historyIdx++
					if m.historyIdx >= len(m.history) {
						// Exit history mode - restore saved input
						m.historyIdx = -1
						m.input.SetValue(m.historySaved)
						m.historySaved = ""
					} else {
						m.input.SetValue(m.history[m.historyIdx])
					}
					m.input.CursorEnd()
					return m, nil
				}
			}

		case tea.KeyTab:
			if m.mode == modeIdle && m.cmdMenuOpen {
				matches := matchCommands(m.input.Value())
				if len(matches) > 0 {
					idx := m.cmdMenuIdx
					if idx < 0 || idx >= len(matches) {
						idx = 0
					}
					m.input.SetValue(matches[idx].name + " ")
					m.input.CursorEnd()
					m.cmdMenuOpen = false
					m.cmdMenuIdx = 0
				}
				return m, nil
			}

		case tea.KeyEnter:
			// If command menu is open and an item is selected, pick it
			if m.mode == modeIdle && m.cmdMenuOpen && m.cmdMenuIdx >= 0 {
				matches := matchCommands(m.input.Value())
				if m.cmdMenuIdx < len(matches) {
					m.input.SetValue(matches[m.cmdMenuIdx].name + " ")
					m.input.CursorEnd()
					m.cmdMenuOpen = false
					m.cmdMenuIdx = 0
					return m, nil
				}
			}

			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				return m, nil
			}

			// Add to history (avoid duplicates if same as last command)
			if len(m.history) == 0 || m.history[len(m.history)-1] != value {
				m.history = append(m.history, value)
				// Limit history size to 1000 entries
				if len(m.history) > 1000 {
					m.history = m.history[len(m.history)-1000:]
				}
			}
			m.historyIdx = -1
			m.historySaved = ""

			m.input.SetValue("")
			m.cmdMenuOpen = false
			m.cmdMenuIdx = 0

			switch m.mode {
			case modeLoginURL:
				return m.handleLoginURLSubmit(value)
			case modeLoginUser:
				return m.handleLoginUserSubmit(value)
			case modeLoginPass:
				return m.handleLoginPassSubmit(value)
			default:
				return m.dispatchInput(value)
			}

		}

	// ── Stream messages ───────────────────────────────────────────────
	case sessionCreatedMsg:
		m.sessionID = msg.sessionID
		cmds = append(cmds,
			tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Session: %s", m.sessionID))),
		)
		if consoleURL := m.cfg.ConsoleSessionURL(m.sessionID); consoleURL != "" {
			cmds = append(cmds, tea.Println(dimStyle.Render(fmt.Sprintf("    🔗 %s", consoleURL))))
		}
		cmds = append(cmds,
			beginStream(m.client, m.cfg.ProjectID, m.sessionID, m.streamPrompt),
		)
		return m, tea.Batch(cmds...)

	case streamChunkMsg:
		printCmd := m.handleStreamChunk(msg)
		if printCmd != nil {
			cmds = append(cmds, printCmd)
		}
		// Keep reading from the stream channel.
		// IMPORTANT: use tea.Sequence (not tea.Batch) so that all print
		// commands are sent to the main loop BEFORE we read the next chunk.
		// With tea.Batch, waitForStream resolves immediately from the
		// buffered channel, sending a new streamChunkMsg that races ahead
		// of queued prints — causing output to stall then dump in bursts.
		if activeStreamCh != nil {
			cmds = append(cmds, waitForStream(activeStreamCh))
		}
		return m, tea.Sequence(cmds...)

	case streamDoneMsg:
		m.mode = modeIdle
		activeStreamCh = nil
		if msg.sessionID != "" {
			m.sessionID = msg.sessionID
		}
		// Flush any remaining buffers via the processor
		var flushCmds []tea.Cmd
		for _, ev := range m.processor.Flush() {
			flushCmds = append(flushCmds, tea.Println(renderOutputEvent(ev)))
		}
		flushCmds = append(flushCmds,
			tea.Println(""),
			tea.Println(successMsgStyle.Render("  ✓ Investigation complete")),
			tea.Println(dimStyle.Render(fmt.Sprintf("    Session: %s", m.sessionID))),
			tea.Println(""),
		)
		m.resetStreamState()
		return m, tea.Batch(append(cmds, tea.Sequence(flushCmds...))...)

	case streamErrMsg:
		m.mode = modeIdle
		activeStreamCh = nil
		cmds = append(cmds, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Stream error: %v", msg.err))))
		m.resetStreamState()
		return m, tea.Batch(cmds...)

	// ── Login result ──────────────────────────────────────────────────
	case loginResultMsg:
		return m.handleLoginResult(msg)

	// ── Async results ─────────────────────────────────────────────────
	case sessionsLoadedMsg:
		return m.handleSessionsLoaded(msg)

	case promptsLoadedMsg:
		return m.handlePromptsLoaded(msg)

	case projectsLoadedMsg:
		return m.handleProjectsLoaded(msg)

	case inspectResultMsg:
		return m.handleInspectResult(msg)

	case summaryResultMsg:
		return m.handleSummaryResult(msg)

	case feedbackResultMsg:
		return m.handleFeedbackResult(msg)

	case scoreResultMsg:
		return m.handleScoreResult(msg)

	case reportResultMsg:
		return m.handleReportResult(msg)

	case connectionsResultMsg:
		return m.handleConnectionsResult(msg)

	case resourcesResultMsg:
		return m.handleResourcesResult(msg)

	case projectInfoMsg:
		return m.handleProjectInfo(msg)

	case projectCreateMsg:
		return m.handleProjectCreate(msg)

	case projectDeleteMsg:
		return m.handleProjectDelete(msg)

	case connInfoMsg:
		return m.handleConnInfo(msg)

	case connAddMsg:
		return m.handleConnAdd(msg)

	case connRemoveMsg:
		return m.handleConnRemove(msg)

	case instructionsLoadedMsg:
		return m.handleInstructionsLoaded(msg)

	case instructionCreateMsg:
		return m.handleInstructionCreate(msg)

	case instructionToggleMsg:
		return m.handleInstructionToggle(msg)

	case instructionDeleteMsg:
		return m.handleInstructionDelete(msg)

	case rerunResultMsg:
		return m.handleRerunResult(msg)

	case queriesResultMsg:
		return m.handleQueriesResult(msg)

	case discoverResultMsg:
		return m.handleDiscoverResult(msg)

	case sessionReportMsg:
		return m.handleSessionReport(msg)

	case addConnectionResultMsg:
		return m.handleAddConnectionResult(msg)

	case incidentTestResultMsg:
		return m.handleIncidentTestResult(msg)
	}

	// Update sub-components
	var cmd tea.Cmd

	if m.mode != modeStreaming {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	// Track input changes to open/close command menu and reset selection
	newVal := m.input.Value()
	if newVal != m.lastInputVal {
		m.lastInputVal = newVal
		// Exit history mode when user types (manually edits input)
		if m.historyIdx != -1 {
			// If input doesn't match current history item, exit history mode
			if m.historyIdx < len(m.history) && m.history[m.historyIdx] != newVal {
				m.historyIdx = -1
				m.historySaved = ""
			}
		}
		if strings.HasPrefix(newVal, "/") {
			m.cmdMenuOpen = true
			m.cmdMenuIdx = 0
		} else {
			m.cmdMenuOpen = false
			m.cmdMenuIdx = 0
		}
	}

	return m, tea.Batch(cmds...)
}

// ─── View ───────────────────────────────────────────────────────────────────
//
// Inline mode: View() only shows the input prompt + hints.
// All output is printed above via tea.Println.
// During streaming, only the spinner + status is shown (single line).
// This prevents terminal rendering artifacts where spinner text bleeds
// into the permanent scrollback output.

func (m model) View() string {
	if !m.ready {
		return ""
	}

	var s strings.Builder

	if m.mode == modeStreaming {
		status := "Investigating..."
		if ps := m.processor.LastStatus(); ps != "" {
			status = ps
		}
		// Add blank lines to prevent spinner from overwriting last printed content
		s.WriteString("\n\n")
		s.WriteString(m.spinner.View() + " " + statusStyle.Render(status))
	} else {
		s.WriteString(m.input.View())
	}
	s.WriteString("\n")

	// Separator
	sepWidth := min(m.width, 80)
	if sepWidth < 20 {
		sepWidth = 20
	}
	s.WriteString(separatorStyle.Render(strings.Repeat("─", sepWidth)))
	s.WriteString("\n")

	// Hint bar
	s.WriteString(m.renderHints())

	return s.String()
}

// ─── Hint bar ───────────────────────────────────────────────────────────────

func (m model) renderHints() string {
	if m.mode == modeStreaming {
		return hintBarStyle.Render("  Esc cancel")
	}

	if m.mode == modeLoginURL || m.mode == modeLoginUser || m.mode == modeLoginPass {
		return hintBarStyle.Render("  Enter submit   Esc cancel")
	}

	// Show vertical command menu when menu is open
	if m.cmdMenuOpen {
		val := m.input.Value()
		matches := matchCommands(val)
		if len(matches) > 0 {
			return m.renderCommandMenu(matches)
		}
	}

	return hintBarStyle.Render("  ? for help")
}

// renderCommandMenu renders a vertical list of matching commands, Claude Code style.
func (m model) renderCommandMenu(matches []slashCmd) string {
	// Find the longest command name for alignment
	maxLen := 0
	for _, c := range matches {
		if len(c.name) > maxLen {
			maxLen = len(c.name)
		}
	}

	var lines []string
	for i, c := range matches {
		padded := c.name
		for len(padded) < maxLen {
			padded += " "
		}

		var line string
		if i == m.cmdMenuIdx {
			// Highlighted row
			line = "  " + cmdSelectedNameStyle.Render(padded) + "  " + cmdSelectedDescStyle.Render(c.desc)
		} else {
			line = "  " + cmdNameStyle.Render(padded) + "  " + cmdDescStyle.Render(c.desc)
		}
		lines = append(lines, line)
	}

	// Navigation hint at the bottom
	lines = append(lines, hintBarStyle.Render("  ↑↓ navigate  Tab/Enter select"))

	return strings.Join(lines, "\n")
}

// matchCommands returns all slash commands matching a prefix.
// Top-level commands (no spaces) are shown while the user is still typing the
// base command. Subcommands (contain a space) only appear once the user has
// typed a space after the base command.
func matchCommands(prefix string) []slashCmd {
	prefix = strings.ToLower(prefix)
	// Just "/" — show only top-level commands
	if prefix == "/" {
		var top []slashCmd
		for _, c := range slashCommands {
			if !strings.Contains(c.name[1:], " ") {
				top = append(top, c)
			}
		}
		return top
	}
	prefixHasSpace := strings.Contains(prefix[1:], " ")
	var matches []slashCmd
	for _, c := range slashCommands {
		if !strings.HasPrefix(c.name, prefix) {
			continue
		}
		nameHasSpace := strings.Contains(c.name[1:], " ")
		// Only surface subcommands when the user has already typed a space
		if nameHasSpace && !prefixHasSpace {
			continue
		}
		// Only surface top-level commands when the user hasn't typed a space
		if !nameHasSpace && prefixHasSpace {
			continue
		}
		matches = append(matches, c)
	}
	return matches
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func (m *model) resetStreamState() {
	m.processor = NewStreamProcessor()
	m.streamPrompt = ""
}

// handleStreamChunk processes a streaming event via the StreamProcessor
// and converts structured OutputEvents into styled tea.Println commands.
func (m *model) handleStreamChunk(msg streamChunkMsg) tea.Cmd {
	events := m.processor.Process(msg)
	if len(events) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, ev := range events {
		// Skip progress events - they're shown in the spinner only
		if ev.Type == OutputProgress {
			continue
		}
		rendered := renderOutputEvent(ev)
		cmds = append(cmds, tea.Println(rendered))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Sequence(cmds...)
}

// renderOutputEvent converts a structured OutputEvent into a styled string.
// This is the single rendering point — change styles or hide blocks here.
func renderOutputEvent(ev OutputEvent) string {
	switch ev.Type {
	case OutputProgress:
		// Progress is shown in the spinner (View), not printed to scrollback.
		// This keeps the output clean - only meaningful content is permanent.
		return ""
	case OutputCOTHeader:
		return cotHeaderStyle.Render(fmt.Sprintf("  🔍 %s", ev.Text))
	case OutputCOTExplanation:
		return cotExplanationStyle.Render("     ↳ " + ev.Text)
	case OutputCOTText:
		return "    " + renderMarkdownText(ev.Text)
	case OutputChat:
		return "  " + renderMarkdownText(ev.Text)
	case OutputFollowUpHeader:
		return followUpStyle.Render("  💡 " + ev.Text)
	case OutputFollowUpItem:
		return followUpStyle.Render(fmt.Sprintf("     %d. %s", ev.Index, ev.Text))
	case OutputSource:
		return sourceHeaderStyle.Render("  📎 ") + ev.Text
	case OutputSessionName:
		return sessionNameStyle.Render("  📛 " + ev.Text)
	case OutputExecTime:
		return dimStyle.Render("  ⏱  " + ev.Text)
	case OutputTable:
		return renderTable(ev.Text)
	case OutputBlank:
		return ""
	case OutputDivider:
		return fmt.Sprintf("  %s%s%s", ansiAccent, strings.Repeat("─", 44), ansiReset)
	default:
		return ev.Text
	}
}

func serverStr(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.Server
}

func projectStr(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.ProjectID
}

func orgStr(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.OrgUUID
}

func truncateUUID(s string) string {
	if len(s) > 20 {
		return s[:8] + "..." + s[len(s)-4:]
	}
	return s
}
