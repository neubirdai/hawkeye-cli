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
	{"/help", "Show all commands"},
	{"/inspect", "View session details"},
	{"/login", "Login to a Hawkeye server"},
	{"/projects", "List available projects"},
	{"/prompts", "Browse investigation prompts"},
	{"/quit", "Exit Hawkeye"},
	{"/session", "Set active session"},
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
	client    *api.Client
	sessionID string
	version   string

	// Streaming state
	chatPrinted  int               // how many chars of chat response we've already printed
	chatBuffer   string            // partial line buffer for chat response
	cotAccum     map[string]string // accumulated COT investigation text per ID
	cotPrinted   map[string]int    // how many chars printed per COT ID
	cotDescShown map[string]bool   // whether we've printed the COT header
	seenSources  map[string]bool
	seenProgress map[string]bool
	streamPrompt string // the prompt being streamed

	// Login flow state
	loginURL  string
	loginUser string

	// UI state
	ready        bool
	cmdMenuIdx   int  // selected index in command menu (-1 = none)
	cmdMenuOpen  bool // whether the command menu is visible
	lastInputVal string // track input changes to reset menu index
	profile      string
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

	var client *api.Client
	if cfg != nil && cfg.Server != "" && cfg.Token != "" {
		client = api.NewClient(cfg)
	}

	return model{
		input:        ti,
		spinner:      sp,
		version:      version,
		profile:      profile,
		cfg:          cfg,
		client:       client,
		mode:         modeIdle,
		cotAccum:     make(map[string]string),
		cotPrinted:   make(map[string]int),
		cotDescShown: make(map[string]bool),
		seenSources:  make(map[string]bool),
		seenProgress: make(map[string]bool),
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
			if m.mode == modeIdle && m.cmdMenuOpen {
				matches := matchCommands(m.input.Value())
				if len(matches) > 0 {
					m.cmdMenuIdx--
					if m.cmdMenuIdx < 0 {
						m.cmdMenuIdx = len(matches) - 1
					}
					return m, nil
				}
			}

		case tea.KeyDown:
			if m.mode == modeIdle && m.cmdMenuOpen {
				matches := matchCommands(m.input.Value())
				if len(matches) > 0 {
					m.cmdMenuIdx++
					if m.cmdMenuIdx >= len(matches) {
						m.cmdMenuIdx = 0
					}
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
			tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Session: %s", truncateUUID(m.sessionID)))),
			beginStream(m.client, m.cfg.ProjectID, m.sessionID, m.streamPrompt),
		)
		return m, tea.Batch(cmds...)

	case streamChunkMsg:
		printCmd := m.handleStreamChunk(msg)
		if printCmd != nil {
			cmds = append(cmds, printCmd)
		}
		// Keep reading from the stream channel
		if activeStreamCh != nil {
			cmds = append(cmds, waitForStream(activeStreamCh))
		}
		return m, tea.Batch(cmds...)

	case streamDoneMsg:
		m.mode = modeIdle
		activeStreamCh = nil
		if msg.sessionID != "" {
			m.sessionID = msg.sessionID
		}
		// Flush any remaining chat buffer
		if m.chatBuffer != "" {
			cmds = append(cmds, tea.Println("  "+m.chatBuffer))
			m.chatBuffer = ""
		}
		cmds = append(cmds, tea.Sequence(
			tea.Println(""),
			tea.Println(successMsgStyle.Render("  ✓ Investigation complete")),
			tea.Println(dimStyle.Render(fmt.Sprintf("    Session: %s", m.sessionID))),
			tea.Println(""),
		))
		m.resetStreamState()
		return m, tea.Batch(cmds...)

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

func (m model) View() string {
	if !m.ready {
		return ""
	}

	var s strings.Builder

	// Input or streaming indicator
	if m.mode == modeStreaming {
		s.WriteString(m.spinner.View() + " Investigating...")
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
func matchCommands(prefix string) []slashCmd {
	prefix = strings.ToLower(prefix)
	// Just "/" with nothing else — show all
	if prefix == "/" {
		return slashCommands
	}
	var matches []slashCmd
	for _, c := range slashCommands {
		if strings.HasPrefix(c.name, prefix) {
			matches = append(matches, c)
		}
	}
	return matches
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func (m *model) resetStreamState() {
	m.chatPrinted = 0
	m.chatBuffer = ""
	m.cotAccum = make(map[string]string)
	m.cotPrinted = make(map[string]int)
	m.cotDescShown = make(map[string]bool)
	m.seenSources = make(map[string]bool)
	m.seenProgress = make(map[string]bool)
	m.streamPrompt = ""
}

// handleStreamChunk processes a streaming event and returns a tea.Println command.
func (m *model) handleStreamChunk(msg streamChunkMsg) tea.Cmd {
	switch msg.contentType {
	case "CONTENT_TYPE_PROGRESS_STATUS":
		display := extractProgressDisplay(msg.text)
		if !m.seenProgress[display] {
			m.seenProgress[display] = true
			return tea.Println(statusStyle.Render("  ⟳ " + display))
		}

	case "CONTENT_TYPE_SOURCES":
		label := parseSourceLabel(msg.text)
		if !m.seenSources[label] {
			m.seenSources[label] = true
			return tea.Println(sourceHeaderStyle.Render("  📎 ") + label)
		}

	case "CONTENT_TYPE_CHAIN_OF_THOUGHT":
		id, desc, explanation, investigation, _ := parseCOTFields(msg.text)
		cotID := id
		if cotID == "" {
			cotID = desc
		}
		if cotID == "" {
			cotID = "_default"
		}

		var printCmds []tea.Cmd

		// Helper: show header if not yet shown for this COT
		showHeader := func() {
			if !m.cotDescShown[cotID] && desc != "" {
				m.cotDescShown[cotID] = true
				printCmds = append(printCmds, tea.Println(cotHeaderStyle.Render("  🔍 "+desc)))
				if explanation != "" {
					printCmds = append(printCmds, tea.Println(dimStyle.Render("     ↳ "+explanation)))
				}
			}
		}

		// Helper: print text line-by-line with buffering for partial lines
		printLines := func(text string) {
			combined := m.getCOTBuffer(cotID) + text
			lines := strings.Split(combined, "\n")
			for i, line := range lines {
				if i < len(lines)-1 {
					if strings.TrimSpace(line) != "" {
						printCmds = append(printCmds, tea.Println("    "+line))
					}
				} else {
					m.setCOTBuffer(cotID, line)
				}
			}
		}

		switch msg.eventType {
		case "cot_start":
			showHeader()

		case "cot_delta":
			showHeader()
			// Delta mode: investigation IS the new text to append
			if investigation != "" {
				printLines(investigation)
			}

		case "cot_end":
			// Flush remaining buffer for this COT
			buf := m.getCOTBuffer(cotID)
			if buf != "" && strings.TrimSpace(buf) != "" {
				printCmds = append(printCmds, tea.Println("    "+buf))
				m.setCOTBuffer(cotID, "")
			}

		default:
			// Legacy non-delta: investigation contains full accumulated text
			showHeader()
			if investigation != "" {
				prev := m.cotAccum[cotID]
				if len(investigation) > len(prev) {
					newText := investigation[len(prev):]
					m.cotAccum[cotID] = investigation
					printLines(newText)
				}
			}
		}

		if len(printCmds) > 0 {
			return tea.Batch(printCmds...)
		}

	case "CONTENT_TYPE_CHAT_RESPONSE":
		// Print delta text line by line
		newText := ""
		if len(msg.text) > m.chatPrinted {
			newText = msg.text[m.chatPrinted:]
			m.chatPrinted = len(msg.text)
		}
		if newText == "" {
			return nil
		}

		combined := m.chatBuffer + newText
		lines := strings.Split(combined, "\n")
		var printCmds []tea.Cmd

		for i, line := range lines {
			if i < len(lines)-1 {
				// Complete line — print it
				printCmds = append(printCmds, tea.Println("  "+line))
			} else {
				// Partial last line — buffer it
				m.chatBuffer = line
			}
		}

		if len(printCmds) > 0 {
			return tea.Batch(printCmds...)
		}

	case "CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS":
		parts := strings.Split(msg.text, "\n")
		var printCmds []tea.Cmd
		printCmds = append(printCmds, tea.Println(""))
		printCmds = append(printCmds, tea.Println(followUpStyle.Render("  💡 Follow-up suggestions:")))
		for i, p := range parts {
			if strings.TrimSpace(p) != "" {
				printCmds = append(printCmds, tea.Println(followUpStyle.Render(fmt.Sprintf("     %d. %s", i+1, p))))
			}
		}
		return tea.Batch(printCmds...)

	case "CONTENT_TYPE_SESSION_NAME":
		return tea.Println(dimStyle.Render("  📛 " + msg.text))

	case "CONTENT_TYPE_ERROR_MESSAGE":
		return tea.Println(errorMsgStyle.Render("  ✗ " + msg.text))

	case "CONTENT_TYPE_EXECUTION_TIME":
		return tea.Println(dimStyle.Render("  ⏱  " + msg.text))
	}

	return nil
}

// COT line buffer per ID (stored in cotAccum with a special prefix)
func (m *model) getCOTBuffer(cotID string) string {
	key := "_buf_" + cotID
	return m.cotAccum[key]
}

func (m *model) setCOTBuffer(cotID string, buf string) {
	key := "_buf_" + cotID
	m.cotAccum[key] = buf
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

// extractProgressDisplay pulls out just the parenthetical description
// from progress text like "PromptGate (Preparing Telemetry Sources)".
func extractProgressDisplay(text string) string {
	if i := strings.Index(text, "("); i >= 0 {
		if j := strings.LastIndex(text, ")"); j > i {
			return text[i+1 : j]
		}
	}
	return text
}
