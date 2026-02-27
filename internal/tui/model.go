package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"
	"hawkeye-cli/internal/service"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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
	modeIncidentList // interactive incident selection list
	modeProjectSelect
	modeSessionSelect
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
	{"/incidents list", "Show open incidents (paginated)"},
	{"/incidents add", "Add an incident management connection"},
	{"/incidents test", "Test incident creation"},
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
	{"/open", "Open session from web URL"},
	{"/projects", "Select a project (interactive)"},
	{"/prompts", "Browse investigation prompts"},
	{"/queries", "Show investigation queries"},
	{"/quit", "Exit Hawkeye"},
	{"/report", "Show incident analytics"},
	{"/rerun", "Rerun an investigation"},
	{"/score", "Show RCA quality scores"},
	{"/session", "Pick or set active session"},
	{"/session-report", "Per-session report"},
	{"/set", "Set project or config"},
	{"/summary", "Get session summary"},
}

// ─── Model ──────────────────────────────────────────────────────────────────

type model struct {
	width  int
	height int

	// Bubble Tea components
	input      textarea.Model  // multiline input for prompts
	loginInput textinput.Model // single-line input for login flow
	spinner    spinner.Model

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

	// Project selection state
	projectList    []api.ProjectSpec
	projectListIdx int

	// Session selection state
	sessionList    []api.SessionInfo
	sessionListIdx int

	// UI state
	ready        bool
	cmdMenuIdx   int    // selected index in command menu (-1 = none)
	cmdMenuOpen  bool   // whether the command menu is visible
	lastInputVal string // track input changes to reset menu index
	profile      string

	// Command history
	history      []string
	historyIdx   int
	historySaved string

	resumeSessionID string

	// Incident list state (modeIncidentList)
	incidentList        []api.SessionInfo
	incidentListIdx     int
	incidentListPage    int
	incidentListHasMore bool
}

func initialModel(version, profile, resumeSessionID string) model {
	// Multiline textarea for prompts
	ta := textarea.New()
	ta.Placeholder = "Ask a question or type /help..."
	ta.Focus()
	ta.CharLimit = 4096
	ta.SetWidth(80)
	ta.SetHeight(1) // Start with single line, grows dynamically
	ta.ShowLineNumbers = false
	ta.Prompt = "❯ "
	ta.FocusedStyle.Prompt = promptSymbol
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle() // No highlight on cursor line
	ta.FocusedStyle.Base = lipgloss.NewStyle()       // No base styling
	ta.BlurredStyle = ta.FocusedStyle
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(colorOrange)

	// Single-line input for login flow
	li := textinput.New()
	li.Placeholder = ""
	li.CharLimit = 256
	li.Prompt = "❯ "
	li.PromptStyle = promptSymbol
	li.Cursor.Style = lipgloss.NewStyle().Foreground(colorOrange)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorOrange)

	cfg, _ := config.Load(profile)

	var client api.HawkeyeAPI
	if cfg != nil && cfg.Server != "" && cfg.Token != "" {
		client = api.NewClient(cfg)
	}

	return model{
		input:           ta,
		loginInput:      li,
		spinner:         sp,
		version:         version,
		profile:         profile,
		cfg:             cfg,
		client:          client,
		mode:            modeIdle,
		processor:       NewStreamProcessor(),
		history:         config.LoadHistory(profile),
		historyIdx:      -1,
		resumeSessionID: resumeSessionID,
	}
}

// ─── Init ───────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textarea.Blink,
		m.spinner.Tick,
	}
	// If we have a project ID but no project name, fetch it in the background
	if m.client != nil && m.cfg != nil && m.cfg.ProjectID != "" && m.cfg.ProjectName == "" {
		client := m.client
		projectID := m.cfg.ProjectID
		cmds = append(cmds, func() tea.Msg {
			resp, err := client.ListProjects()
			if err != nil {
				return nil
			}
			for _, p := range resp.Specs {
				if p.UUID == projectID {
					return projectNameFetchedMsg{name: p.Name}
				}
			}
			return nil
		})
	}
	if m.resumeSessionID != "" && m.client != nil {
		sessionUUID := m.resumeSessionID
		m.sessionID = sessionUUID
		m.resumeSessionID = ""
		client := m.client
		projectID := m.cfg.ProjectID
		cmds = append(cmds,
			tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Resuming session %s...", truncateUUID(sessionUUID)))),
			func() tea.Msg {
				resp, err := client.SessionInspect(projectID, sessionUUID)
				if err != nil {
					return inspectResultMsg{err: err}
				}
				return inspectResultMsg{resp: resp}
			},
		)
	}
	return tea.Batch(cmds...)
}

// projectNameFetchedMsg is sent when project name is fetched on startup
type projectNameFetchedMsg struct {
	name string
}

// ─── Update ─────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(m.width - 6)
		m.loginInput.Width = m.width - 6

		if !m.ready {
			m.ready = true
			// Print welcome header on first render
			welcome := renderWelcome(m.version, serverStr(m.cfg), projectNameStr(m.cfg), m.width)
			cmds = append(cmds, tea.Println(welcome))
		}

	case tea.KeyMsg:
		// ── Incident list navigation ──────────────────────────────────────
		if m.mode == modeIncidentList {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				m.mode = modeIdle
				return m, tea.Println(dimStyle.Render("  Incident list closed."))
			case tea.KeyUp:
				if m.incidentListIdx > 0 {
					m.incidentListIdx--
				}
				return m, nil
			case tea.KeyDown:
				if m.incidentListIdx < len(m.incidentList)-1 {
					m.incidentListIdx++
				}
				return m, nil
			case tea.KeyEnter:
				selected := m.incidentList[m.incidentListIdx]
				m.sessionID = selected.SessionUUID
				m.mode = modeIdle
				return m.cmdInspect([]string{selected.SessionUUID})
			case tea.KeyRunes:
				switch string(msg.Runes) {
				case "n":
					if m.incidentListHasMore {
						return m.cmdOpenIncidentsList([]string{fmt.Sprintf("%d", m.incidentListPage+1)})
					}
				case "p":
					if m.incidentListPage > 1 {
						return m.cmdOpenIncidentsList([]string{fmt.Sprintf("%d", m.incidentListPage-1)})
					}
				}
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			if m.mode == modeStreaming {
				m.mode = modeIdle
				activeStreamCh = nil
				m.resetStreamState()
				cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! Investigation cancelled.")))
				return m, tea.Batch(cmds...)
			}
			if m.mode == modeSessionSelect {
				m.mode = modeIdle
				m.sessionList = nil
				m.sessionListIdx = 0
				cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! Session selection cancelled.")))
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
				m.loginInput.SetValue("")
				m.loginInput.EchoMode = textinput.EchoNormal
				cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! Login cancelled.")))
				return m, tea.Batch(cmds...)
			}
			if m.mode == modeProjectSelect {
				m.mode = modeIdle
				m.projectList = nil
				m.projectListIdx = 0
				cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! Project selection cancelled.")))
				return m, tea.Batch(cmds...)
			}
			if m.mode == modeSessionSelect {
				m.mode = modeIdle
				m.sessionList = nil
				m.sessionListIdx = 0
				cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! Session selection cancelled.")))
				return m, tea.Batch(cmds...)
			}
			if m.cmdMenuOpen {
				m.cmdMenuOpen = false
				m.cmdMenuIdx = 0
				return m, nil
			}

		case tea.KeyUp:
			if m.mode == modeSessionSelect {
				if len(m.sessionList) > 0 {
					m.sessionListIdx--
					if m.sessionListIdx < 0 {
						m.sessionListIdx = len(m.sessionList) - 1
					}
				}
				return m, nil
			}
			if m.mode == modeProjectSelect {
				if len(m.projectList) > 0 {
					m.projectListIdx--
					if m.projectListIdx < 0 {
						m.projectListIdx = len(m.projectList) - 1
					}
				}
				return m, nil
			}
			// History navigation only when input is single-line (no newlines)
			if m.mode == modeIdle && len(m.history) > 0 && !strings.Contains(m.input.Value(), "\n") {
				m.cmdMenuOpen = false
				if m.historyIdx == -1 {
					m.historySaved = m.input.Value()
					m.historyIdx = len(m.history) - 1
				} else if m.historyIdx > 0 {
					m.historyIdx--
				}
				m.input.SetValue(m.history[m.historyIdx])
				return m, nil
			}

		case tea.KeyDown:
			if m.mode == modeSessionSelect {
				if len(m.sessionList) > 0 {
					m.sessionListIdx++
					if m.sessionListIdx >= len(m.sessionList) {
						m.sessionListIdx = 0
					}
				}
				return m, nil
			}
			if m.mode == modeProjectSelect {
				if len(m.projectList) > 0 {
					m.projectListIdx++
					if m.projectListIdx >= len(m.projectList) {
						m.projectListIdx = 0
					}
				}
				return m, nil
			}
			// History navigation only when input is single-line (no newlines)
			if m.mode == modeIdle && m.historyIdx != -1 && !strings.Contains(m.input.Value(), "\n") {
				m.historyIdx++
				if m.historyIdx >= len(m.history) {
					m.historyIdx = -1
					m.input.SetValue(m.historySaved)
					m.historySaved = ""
				} else {
					m.input.SetValue(m.history[m.historyIdx])
				}
				return m, nil
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
					m.cmdMenuOpen = false
					m.cmdMenuIdx = 0
				}
				return m, nil
			}

		case tea.KeyShiftTab:
			// Shift+Tab does nothing special
			return m, nil

		case tea.KeyCtrlJ:
			// Ctrl+J inserts newline (works in all terminals)
			if m.mode == modeIdle {
				m.insertNewline()
				return m, nil
			}

		case tea.KeyEnter:
			// In login modes, handle Enter via loginInput
			if m.mode == modeLoginURL || m.mode == modeLoginUser || m.mode == modeLoginPass {
				value := strings.TrimSpace(m.loginInput.Value())
				if value == "" {
					return m, nil
				}
				m.loginInput.SetValue("")
				switch m.mode {
				case modeLoginURL:
					return m.handleLoginURLSubmit(value)
				case modeLoginUser:
					return m.handleLoginUserSubmit(value)
				case modeLoginPass:
					return m.handleLoginPassSubmit(value)
				}
			}

			// Alt+Enter inserts newline instead of submitting
			if msg.Alt {
				m.insertNewline()
				return m, nil
			}

			if m.mode == modeSessionSelect && len(m.sessionList) > 0 {
				selected := m.sessionList[m.sessionListIdx]
				m.sessionID = selected.SessionUUID
				m.mode = modeIdle
				m.sessionList = nil
				m.sessionListIdx = 0
				name := selected.Name
				if name == "" {
					name = "(unnamed)"
				}
				return m, tea.Sequence(
					tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Session set to: %s", name))),
					tea.Println(dimStyle.Render(fmt.Sprintf("    %s", selected.SessionUUID))),
					tea.Println(dimStyle.Render("    Follow-up questions will continue in this session.")),
				)
			}

			// Project selection mode - select the highlighted project
			if m.mode == modeProjectSelect && len(m.projectList) > 0 {
				selected := m.projectList[m.projectListIdx]
				m.mode = modeIdle
				m.projectList = nil
				m.projectListIdx = 0
				return m.selectProject(selected)
			}

			// If command menu is open and an item is selected, pick it
			if m.mode == modeIdle && m.cmdMenuOpen && m.cmdMenuIdx >= 0 {
				matches := matchCommands(m.input.Value())
				if m.cmdMenuIdx < len(matches) {
					m.input.SetValue(matches[m.cmdMenuIdx].name + " ")
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
				if len(m.history) > 1000 {
					m.history = m.history[len(m.history)-1000:]
				}
				_ = config.SaveHistory(m.profile, m.history)
			}
			m.historyIdx = -1
			m.historySaved = ""

			m.input.SetValue("")
			m.input.SetHeight(1) // Reset to single line after submit
			m.cmdMenuOpen = false
			m.cmdMenuIdx = 0

			return m.dispatchInput(value)

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
		m.resetStreamState()

		// Check if this is a "project does not exist" error - offer to select a project
		errStr := msg.err.Error()
		if strings.Contains(errStr, "does not exist") || strings.Contains(errStr, "not found") {
			cmds = append(cmds,
				tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ %v", msg.err))),
				tea.Println(warnMsgStyle.Render("  ! Loading available projects...")),
			)
			// Auto-trigger project selection
			if m.client != nil {
				client := m.client
				cmds = append(cmds, func() tea.Msg {
					resp, err := client.ListProjects()
					if err != nil {
						return projectsLoadedMsg{err: err}
					}
					return projectsLoadedMsg{projects: service.FilterSystemProjects(resp.Specs)}
				})
			}
			return m, tea.Batch(cmds...)
		}

		cmds = append(cmds, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Stream error: %v", msg.err))))
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

	case openIncidentsLoadedMsg:
		return m.handleOpenIncidentsLoaded(msg)

	case setProjectResultMsg:
		return m.handleSetProjectResult(msg)

	case projectNameFetchedMsg:
		if msg.name != "" && m.cfg != nil {
			m.cfg.ProjectName = msg.name
			_ = m.cfg.Save() // Save silently, don't interrupt user
		}
		return m, nil
	}

	// Update sub-components
	var cmd tea.Cmd

	// Don't pass Enter key to textarea (we handle it for submit)
	// Only pass it if we want to insert a newline (Shift+Enter or Alt+Enter)
	shouldPassToInput := true
	if keyMsg, ok := msg.(tea.KeyMsg); ok && m.mode == modeIdle {
		if keyMsg.Type == tea.KeyEnter && !keyMsg.Alt {
			// Plain Enter = submit, don't pass to textarea
			shouldPassToInput = false
		}
	}

	if m.mode != modeStreaming && shouldPassToInput {
		// Use loginInput for login modes, otherwise use textarea
		if m.mode == modeLoginURL || m.mode == modeLoginUser || m.mode == modeLoginPass {
			m.loginInput, cmd = m.loginInput.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
		}
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

		// Dynamically adjust textarea height based on content
		m.updateInputHeight(newVal)
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

	if m.mode == modeIncidentList {
		s.WriteString(m.renderIncidentList())
		s.WriteString("\n")
		sepWidth := min(m.width, 80)
		if sepWidth < 20 {
			sepWidth = 20
		}
		s.WriteString(separatorStyle.Render(strings.Repeat("─", sepWidth)))
		s.WriteString("\n")
		return s.String()
	}

	if m.mode == modeStreaming {
		status := "Investigating..."
		if ps := m.processor.LastStatus(); ps != "" {
			status = ps
		}
		// Add blank lines to prevent spinner from overwriting last printed content
		s.WriteString("\n\n")
		s.WriteString(m.spinner.View() + " " + statusStyle.Render(status))
	} else if m.mode == modeProjectSelect {
		s.WriteString(m.renderProjectList())
	} else if m.mode == modeSessionSelect {
		s.WriteString(m.renderSessionList())
	} else if m.mode == modeLoginURL || m.mode == modeLoginUser || m.mode == modeLoginPass {
		s.WriteString(m.loginInput.View())
	} else {
		s.WriteString(m.input.View())
	}
	// Ensure we're on a new line after textarea content
	s.WriteString("\n")

	// Separator - add extra newline to ensure clean separation
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

// renderProjectList renders the interactive project selection list
func (m model) renderProjectList() string {
	var lines []string
	lines = append(lines, "")
	lines = append(lines, "  Select a project:")
	lines = append(lines, "")

	for i, p := range m.projectList {
		ready := successMsgStyle.Render("ready")
		if !p.Ready {
			ready = warnMsgStyle.Render("not ready")
		}

		if i == m.projectListIdx {
			// Highlighted row
			lines = append(lines, fmt.Sprintf("  %s %s  %s",
				cmdSelectedNameStyle.Render("▸"),
				cmdSelectedNameStyle.Render(p.Name),
				ready))
			lines = append(lines, fmt.Sprintf("    %s", dimStyle.Render(p.UUID)))
		} else {
			lines = append(lines, fmt.Sprintf("    %s  %s", p.Name, ready))
			lines = append(lines, fmt.Sprintf("    %s", dimStyle.Render(p.UUID)))
		}
	}
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

// renderSessionList renders the interactive session selection list
func (m model) renderSessionList() string {
	var lines []string
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Select a session (%d):", len(m.sessionList)))
	lines = append(lines, "")

	maxVisible := m.height - 6
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if m.sessionListIdx >= maxVisible {
		start = m.sessionListIdx - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.sessionList) {
		end = len(m.sessionList)
	}

	for i := start; i < end; i++ {
		s := m.sessionList[i]
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		if len(name) > 50 {
			name = name[:47] + "..."
		}
		status := formatSessionStatus(s.InvestigationStatus)
		ts := formatSessionTime(s.CreateTime)

		if i == m.sessionListIdx {
			lines = append(lines, fmt.Sprintf("  %s %s  %s  %s",
				cmdSelectedNameStyle.Render("▸"),
				cmdSelectedNameStyle.Render(name),
				status,
				dimStyle.Render(ts)))
		} else {
			lines = append(lines, fmt.Sprintf("    %s  %s  %s", name, status, dimStyle.Render(ts)))
		}
	}
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

func formatSessionStatus(s string) string {
	switch s {
	case "INVESTIGATION_STATUS_NOT_STARTED":
		return dimStyle.Render("[not started]")
	case "INVESTIGATION_STATUS_IN_PROGRESS":
		return statusStyle.Render("[in progress]")
	case "INVESTIGATION_STATUS_COMPLETED", "INVESTIGATION_STATUS_INVESTIGATED":
		return successMsgStyle.Render("[completed]")
	default:
		return ""
	}
}

func formatSessionTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return ts
		}
	}
	return t.Local().Format("Jan 02 15:04")
}

func sortSessionsNewestFirst(sessions []api.SessionInfo) {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreateTime > sessions[j].CreateTime
	})
}

// ─── Hint bar ───────────────────────────────────────────────────────────────

func (m model) renderHints() string {
	if m.mode == modeStreaming {
		return hintBarStyle.Render("  Esc cancel")
	}

	if m.mode == modeLoginURL || m.mode == modeLoginUser || m.mode == modeLoginPass {
		return hintBarStyle.Render("  Enter submit   Esc cancel")
	}

	if m.mode == modeProjectSelect || m.mode == modeSessionSelect {
		return hintBarStyle.Render("  ↑↓ navigate   Enter select   Esc cancel")
	}

	// Show vertical command menu when menu is open
	if m.cmdMenuOpen {
		val := m.input.Value()
		matches := matchCommands(val)
		if len(matches) > 0 {
			return m.renderCommandMenu(matches)
		}
	}

	// Show multiline hint when input contains newlines
	if strings.Contains(m.input.Value(), "\n") {
		return hintBarStyle.Render("  Enter send   Ctrl+J newline   ? help")
	}

	return hintBarStyle.Render("  Enter send   Ctrl+J newline   ? help")
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
// matchCommands returns the commands visible for the given prefix.
// Commands are shown at the same depth as the prefix: the number of spaces in
// the prefix (after the leading "/") determines which level is surfaced.
// This produces a progressive-disclosure menu — typing "/incidents " shows
// only "add" and "test", and typing "/incidents add " then reveals the three
// provider-level completions.
func matchCommands(prefix string) []slashCmd {
	prefix = strings.ToLower(prefix)
	if len(prefix) == 0 || prefix[0] != '/' {
		return nil
	}
	if prefix == "/" {
		var top []slashCmd
		for _, c := range slashCommands {
			if !strings.Contains(c.name[1:], " ") {
				top = append(top, c)
			}
		}
		return top
	}
	prefixDepth := strings.Count(prefix[1:], " ")
	var matches []slashCmd
	for _, c := range slashCommands {
		if !strings.HasPrefix(c.name, prefix) {
			continue
		}
		if strings.Count(c.name[1:], " ") != prefixDepth {
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

// insertNewline adds a newline to the input and adjusts the textarea height
func (m *model) insertNewline() {
	currentVal := m.input.Value()
	newVal := currentVal + "\n"
	m.input.SetValue(newVal)
	m.lastInputVal = newVal
	m.updateInputHeight(newVal)
}

// updateInputHeight calculates and sets the textarea height based on content
// It accounts for both explicit newlines and visual line wrapping
func (m *model) updateInputHeight(content string) {
	if content == "" {
		m.input.SetHeight(1)
		return
	}

	// Calculate visual lines needed
	// The textarea width is set to m.width - 6, but the actual editable area
	// is smaller due to the prompt. The prompt "❯ " takes ~3 characters.
	inputWidth := m.width - 10 // Conservative estimate for actual text area
	if inputWidth < 20 {
		inputWidth = 20
	}

	visualLines := 0
	for _, line := range strings.Split(content, "\n") {
		lineLen := len([]rune(line)) // Use rune count for proper Unicode handling
		if lineLen == 0 {
			visualLines++
		} else {
			// Each line may wrap multiple times
			// Add 1 for the line itself, plus additional lines for wrapping
			wrappedLines := (lineLen + inputWidth - 1) / inputWidth // Ceiling division
			visualLines += wrappedLines
		}
	}

	// Cap at 10 lines to avoid taking over the whole terminal
	if visualLines > 10 {
		visualLines = 10
	}
	if visualLines < 1 {
		visualLines = 1
	}

	m.input.SetHeight(visualLines)
}

// selectProject sets the selected project and saves to config
func (m model) selectProject(p api.ProjectSpec) (tea.Model, tea.Cmd) {
	m.cfg.ProjectID = p.UUID
	m.cfg.ProjectName = p.Name
	if err := m.cfg.Save(); err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to save config: %v", err)))
	}
	if m.cfg.Server != "" && m.cfg.Token != "" {
		m.client = api.NewClient(m.cfg)
	}
	return m, tea.Sequence(
		tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Project set to: %s", p.Name))),
		tea.Println(dimStyle.Render("    You can now start investigating!")),
	)
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
	case OutputCodeFence:
		return renderCodeFence(ev.Text)
	case OutputCodeLine:
		return renderCodeLine(ev.Text)
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

func projectNameStr(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	// Prefer project name, fall back to truncated UUID
	if cfg.ProjectName != "" {
		return cfg.ProjectName
	}
	if cfg.ProjectID != "" {
		return truncateUUID(cfg.ProjectID)
	}
	return ""
}

func truncateUUID(s string) string {
	if len(s) > 20 {
		return s[:8] + "..." + s[len(s)-4:]
	}
	return s
}
