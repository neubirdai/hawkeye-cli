package tui

import (
	"fmt"
	"strings"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"
	"hawkeye-cli/internal/service"

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
	{"/connections", "List/manage data source connections"},
	{"/discover", "Discover project resources"},
	{"/feedback", "Thumbs down feedback"},
	{"/help", "Show all commands"},
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

	// Streaming state
	chatPrinted  int               // how many chars of chat response we've already printed
	chatBuffer   string            // partial line buffer for chat response
	cotAccum     map[string]string // accumulated COT investigation text per ID
	cotPrinted   map[string]int    // how many chars printed per COT ID
	cotDescShown map[string]bool   // whether we've printed the COT header
	seenSources  map[string]bool
	seenProgress map[string]bool
	streamPrompt string // the prompt being streamed

	// Stream interleave protection — two-level gating:
	//
	// cotStepActive: true from cot_start (or first legacy COT event) to cot_end.
	//   Suppresses sources from leaking between the COT header and its content.
	//
	// cotTextActive: true ONLY after we've printed at least one line of
	//   investigation text. Suppresses progress from breaking mid-paragraph.
	cotStepActive   bool     // true while inside a COT step (header → end)
	cotTextActive   bool     // true once investigation text has been printed for current COT
	chatStreaming   bool     // true while chat response text is actively streaming
	activeCotID     string   // ID of the COT step we're currently displaying
	cotStepNum      int      // current COT step number (1-based)
	lastStatus      string   // latest progress text — shown in spinner line (like web UI status bar)
	pendingProgress []string // progress messages queued during active streaming, flushed on step end

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
		history:      make([]string, 0),
		historyIdx:   -1,
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
		var flushCmds []tea.Cmd
		if m.chatBuffer != "" {
			flushCmds = append(flushCmds, tea.Println("  "+m.chatBuffer))
			m.chatBuffer = ""
		}
		// Flush any remaining COT buffer
		if m.activeCotID != "" {
			buf := m.getCOTBuffer(m.activeCotID)
			if buf != "" && strings.TrimSpace(buf) != "" {
				flushCmds = append(flushCmds, tea.Println("    "+buf))
			}
		}
		// Flush any progress queued during the final streaming phase
		flushCmds = append(flushCmds, m.flushPendingProgress()...)
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

func (m model) View() string {
	if !m.ready {
		return ""
	}

	var s strings.Builder

	// Input or streaming indicator
	if m.mode == modeStreaming {
		status := "Investigating..."
		if m.lastStatus != "" {
			status = m.lastStatus
		}
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
	m.cotStepActive = false
	m.cotTextActive = false
	m.chatStreaming = false
	m.lastStatus = ""
	m.pendingProgress = nil
	m.activeCotID = ""
	m.cotStepNum = 0
}

// handleStreamChunk processes a streaming event and returns a tea.Println command.
func (m *model) handleStreamChunk(msg streamChunkMsg) tea.Cmd {
	switch msg.contentType {

	// ── Progress: always update spinner, queue during active streaming ───
	case "CONTENT_TYPE_PROGRESS_STATUS":
		display := service.ExtractProgressDisplay(msg.text)
		// Always update the spinner line — mirrors the web UI's status bar.
		m.lastStatus = display

		normKey := service.NormalizeProgress(display)
		if m.seenProgress[normKey] {
			return nil // already shown or queued
		}
		m.seenProgress[normKey] = true

		if m.cotTextActive || m.chatStreaming {
			// During active text streaming, queue for later to prevent
			// interleaving mid-paragraph. Will flush on step end.
			m.pendingProgress = append(m.pendingProgress, display)
			return nil
		}
		return tea.Println(statusStyle.Render("  ⟳ " + display))

	// ── Sources: suppress while inside a COT step (header → end) ────────
	// Sources between COT steps are fine, but within a step they'd break
	// the visual grouping of header + investigation text.
	case "CONTENT_TYPE_SOURCES":
		if m.cotStepActive || m.chatStreaming {
			return nil
		}
		label := parseSourceLabel(msg.text)
		if !m.seenSources[label] {
			m.seenSources[label] = true
			return tea.Println(sourceHeaderStyle.Render("  📎 ") + label)
		}

	// ── Chain of Thought ────────────────────────────────────────────────
	case "CONTENT_TYPE_CHAIN_OF_THOUGHT":
		return m.handleCOTChunk(msg)

	// ── Chat Response ───────────────────────────────────────────────────
	case "CONTENT_TYPE_CHAT_RESPONSE":
		var preCmds []tea.Cmd

		// Chat starts → finalize any active COT step and flush queued progress
		if m.cotStepActive || m.cotTextActive {
			m.finishCOTStreaming()
		}
		// Flush any progress queued during the COT phase (e.g. CheckPlan results)
		preCmds = append(preCmds, m.flushPendingProgress()...)

		m.chatStreaming = true

		newText := ""
		if msg.eventType == "chat_delta" {
			newText = msg.text
			m.chatPrinted += len(newText)
		} else {
			if len(msg.text) > m.chatPrinted {
				newText = msg.text[m.chatPrinted:]
				m.chatPrinted = len(msg.text)
			}
		}
		if newText == "" {
			if len(preCmds) > 0 {
				return tea.Sequence(preCmds...)
			}
			return nil
		}

		combined := m.chatBuffer + newText
		lines := strings.Split(combined, "\n")

		for i, line := range lines {
			if i < len(lines)-1 {
				preCmds = append(preCmds, tea.Println("  "+line))
			} else {
				m.chatBuffer = line
			}
		}

		if len(preCmds) > 0 {
			return tea.Sequence(preCmds...)
		}

	case "CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS":
		// Follow-ups arrive after chat response is done
		m.chatStreaming = false
		m.cotStepActive = false
		m.cotTextActive = false

		parts := strings.Split(msg.text, "\n")
		var printCmds []tea.Cmd
		printCmds = append(printCmds, tea.Println(""))
		printCmds = append(printCmds, tea.Println(followUpStyle.Render("  💡 Follow-up suggestions:")))
		for i, p := range parts {
			if strings.TrimSpace(p) != "" {
				printCmds = append(printCmds, tea.Println(followUpStyle.Render(fmt.Sprintf("     %d. %s", i+1, p))))
			}
		}
		return tea.Sequence(printCmds...)

	case "CONTENT_TYPE_SESSION_NAME":
		return tea.Println(sessionNameStyle.Render("  📛 " + msg.text))

	case "CONTENT_TYPE_ERROR_MESSAGE":
		// Suppressed: these are internal query retry/fix messages (e.g. SQL errors),
		// not meant for display. The web UI also ignores them.

	case "CONTENT_TYPE_EXECUTION_TIME":
		return tea.Println(dimStyle.Render("  ⏱  " + msg.text))
	}

	return nil
}

// handleCOTChunk handles chain-of-thought events with proper step tracking
// and trivial content filtering.
func (m *model) handleCOTChunk(msg streamChunkMsg) tea.Cmd {
	id, desc, explanation, investigation, _ := parseCOTFields(msg.text)
	cotID := id
	if cotID == "" {
		cotID = desc
	}
	if cotID == "" {
		cotID = "_default"
	}

	var printCmds []tea.Cmd

	// Detect new COT step: if the ID changed from what we're tracking,
	// finalize the previous step and start a new one.
	isNewStep := m.activeCotID != "" && cotID != m.activeCotID
	if isNewStep {
		// Flush the previous COT step's buffer
		flushCmds := m.flushCOTBuffer(m.activeCotID)
		printCmds = append(printCmds, flushCmds...)
		m.cotStepActive = false
		m.cotTextActive = false
		// Print any progress that was queued during the previous step
		printCmds = append(printCmds, m.flushPendingProgress()...)
		// Add a blank line separator between COT steps
		printCmds = append(printCmds, tea.Println(""))
	}

	// Track the active COT ID
	if m.activeCotID != cotID {
		m.activeCotID = cotID
	}

	// Helper: show header if not yet shown for this COT.
	showHeader := func() {
		if !m.cotDescShown[cotID] && desc != "" {
			m.cotDescShown[cotID] = true
			m.cotStepNum++
			// Do NOT reset seenProgress/seenSources here — that causes
			// duplicate progress messages across COT steps.
			printCmds = append(printCmds, tea.Println(cotHeaderStyle.Render(
				fmt.Sprintf("  🔍 %s", desc))))
			if explanation != "" {
				printCmds = append(printCmds, tea.Println(cotExplanationStyle.Render(
					"     ↳ "+explanation)))
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
		// Mark step as active — suppresses sources until cot_end.
		// Don't set cotTextActive yet — progress can still show before first delta.
		m.cotStepActive = true
		showHeader()

	case "cot_delta":
		showHeader()
		// Delta mode: investigation IS the new text to append.
		// Do NOT filter with isTrivialContent — deltas are short fragments
		// that are part of real content, not placeholders.
		if investigation != "" {
			m.cotTextActive = true
			printLines(investigation)
		}

	case "cot_end":
		// Flush remaining buffer for this COT
		flushCmds := m.flushCOTBuffer(cotID)
		printCmds = append(printCmds, flushCmds...)
		m.cotStepActive = false
		m.cotTextActive = false
		// Print any progress that was queued during this step
		printCmds = append(printCmds, m.flushPendingProgress()...)

	default:
		// Legacy non-delta: investigation contains full accumulated text.
		// stream.go now sends only the active (IN_PROGRESS) step.
		m.cotStepActive = true
		showHeader()

		// Filter only exact placeholder text in legacy mode (full text, not fragments)
		if investigation != "" && !service.IsTrivialContent(investigation) {
			prev := m.cotAccum[cotID]
			if len(investigation) > len(prev) {
				m.cotTextActive = true
				newText := investigation[len(prev):]
				m.cotAccum[cotID] = investigation
				printLines(newText)
			}
		}
	}

	if len(printCmds) > 0 {
		return tea.Sequence(printCmds...)
	}

	return nil
}

// finishCOTStreaming flushes the active COT step and marks streaming as done.
func (m *model) finishCOTStreaming() {
	if m.activeCotID != "" {
		buf := m.getCOTBuffer(m.activeCotID)
		if buf != "" && strings.TrimSpace(buf) != "" {
			m.setCOTBuffer(m.activeCotID, "")
		}
	}
	m.cotStepActive = false
	m.cotTextActive = false
}

// flushCOTBuffer flushes the line buffer for a given COT ID and returns
// any tea.Cmd needed to print remaining content.
func (m *model) flushCOTBuffer(cotID string) []tea.Cmd {
	var cmds []tea.Cmd
	buf := m.getCOTBuffer(cotID)
	if buf != "" && strings.TrimSpace(buf) != "" {
		cmds = append(cmds, tea.Println("    "+buf))
		m.setCOTBuffer(cotID, "")
	}
	return cmds
}

// flushPendingProgress prints any progress messages that were queued during
// active COT/chat streaming. Called on step transitions and stream end.
func (m *model) flushPendingProgress() []tea.Cmd {
	if len(m.pendingProgress) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, display := range m.pendingProgress {
		cmds = append(cmds, tea.Println(statusStyle.Render("  ⟳ "+display)))
	}
	m.pendingProgress = nil
	return cmds
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
