package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"
	"hawkeye-cli/internal/incidents"
	"hawkeye-cli/internal/service"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Input dispatcher ───────────────────────────────────────────────────────

func (m model) dispatchInput(input string) (tea.Model, tea.Cmd) {
	if input == "?" {
		return m.cmdHelp()
	}
	if strings.HasPrefix(input, "/") {
		return m.dispatchCommand(input)
	}
	// Default: treat as investigation prompt
	return m.cmdInvestigate(input)
}

func (m model) dispatchCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/help", "/h":
		return m.cmdHelp()
	case "/login":
		return m.cmdLogin(args)
	case "/projects":
		return m.cmdProjects(args)
	case "/inspect":
		return m.cmdInspect(args)
	case "/summary":
		return m.cmdSummary(args)
	case "/feedback", "/td":
		return m.cmdFeedback(args)
	case "/prompts":
		return m.cmdPrompts()
	case "/config":
		return m.cmdConfig()
	case "/set":
		return m.cmdSet(args)
	case "/clear":
		return m.cmdClear()
	case "/score":
		return m.cmdScore(args)
	case "/link":
		return m.cmdLink(args)
	case "/report":
		return m.cmdReport()
	case "/connections":
		return m.cmdConnections(args)
	case "/instructions":
		return m.cmdInstructions(args)
	case "/rerun":
		return m.cmdRerun(args)
	case "/investigate-alert":
		return m.cmdInvestigateAlert(args)
	case "/queries":
		return m.cmdQueries(args)
	case "/discover":
		return m.cmdDiscover()
	case "/session-report":
		return m.cmdSessionReport(args)
	case "/incidents":
		return m.cmdIncidents(args)
	case "/open":
		return m.cmdOpen(args)
	case "/session":
		return m.cmdSetSession(args)
	case "/quit", "/exit", "/q":
		return m, tea.Quit
	default:
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Unknown command: %s — type /help", cmd)))
	}
}

// ─── /help ──────────────────────────────────────────────────────────────────

func (m model) cmdHelp() (tea.Model, tea.Cmd) {
	pad := func(s string, w int) string {
		for len(s) < w {
			s += " "
		}
		return s
	}

	lines := []tea.Cmd{
		tea.Println(""),
		tea.Println(dimStyle.Render("  Shortcuts:")),
		tea.Println(""),
		tea.Println("  " + pad(hintKeyStyle.Render("/login <url>"), 30) + dimStyle.Render("Login to a Hawkeye server")),
		tea.Println("  " + pad(hintKeyStyle.Render("/projects"), 30) + dimStyle.Render("List available projects")),
		tea.Println("  " + pad(hintKeyStyle.Render("/session [uuid]"), 30) + dimStyle.Render("Pick or set active session")),
		tea.Println("  " + pad(hintKeyStyle.Render("/inspect <uuid>"), 30) + dimStyle.Render("View session details")),
		tea.Println("  " + pad(hintKeyStyle.Render("/summary <uuid>"), 30) + dimStyle.Render("Get session summary")),
		tea.Println("  " + pad(hintKeyStyle.Render("/score <uuid>"), 30) + dimStyle.Render("Show RCA quality scores")),
		tea.Println("  " + pad(hintKeyStyle.Render("/link <uuid>"), 30) + dimStyle.Render("Get web UI URL for session")),
		tea.Println("  " + pad(hintKeyStyle.Render("/open <url>"), 30) + dimStyle.Render("Open session from web URL")),
		tea.Println("  " + pad(hintKeyStyle.Render("/report"), 30) + dimStyle.Render("Show incident analytics")),
		tea.Println("  " + pad(hintKeyStyle.Render("/connections"), 30) + dimStyle.Render("Manage data source connections")),
		tea.Println("  " + pad(hintKeyStyle.Render("/incidents"), 30) + dimStyle.Render("Add incident tool connections (add)")),
		tea.Println("  " + pad(hintKeyStyle.Render("/instructions"), 30) + dimStyle.Render("Manage project instructions")),
		tea.Println("  " + pad(hintKeyStyle.Render("/investigate-alert <id>"), 30) + dimStyle.Render("Investigate an alert")),
		tea.Println("  " + pad(hintKeyStyle.Render("/queries [uuid]"), 30) + dimStyle.Render("Show investigation queries")),
		tea.Println("  " + pad(hintKeyStyle.Render("/rerun [uuid]"), 30) + dimStyle.Render("Rerun an investigation")),
		tea.Println("  " + pad(hintKeyStyle.Render("/discover"), 30) + dimStyle.Render("Discover project resources")),
		tea.Println("  " + pad(hintKeyStyle.Render("/session-report [uuid]"), 30) + dimStyle.Render("Per-session time-saved report")),
		tea.Println("  " + pad(hintKeyStyle.Render("/prompts"), 30) + dimStyle.Render("Browse investigation prompts")),
		tea.Println("  " + pad(hintKeyStyle.Render("/set project <uuid>"), 30) + dimStyle.Render("Set the active project")),
		tea.Println("  " + pad(hintKeyStyle.Render("/config"), 30) + dimStyle.Render("Show current configuration")),
		tea.Println("  " + pad(hintKeyStyle.Render("/clear"), 30) + dimStyle.Render("Clear the screen")),
		tea.Println("  " + pad(hintKeyStyle.Render("/quit"), 30) + dimStyle.Render("Exit Hawkeye")),
		tea.Println(""),
		tea.Println(dimStyle.Render("  Or just type a question to start investigating!")),
		tea.Println(""),
	}
	return m, tea.Sequence(lines...)
}

// ─── /login ─────────────────────────────────────────────────────────────────

func (m model) cmdLogin(args []string) (tea.Model, tea.Cmd) {
	m.loginInput.Focus()
	if len(args) > 0 {
		m.loginURL = args[0]
		m.mode = modeLoginUser
		m.loginInput.Placeholder = "Username / Email..."
		m.loginInput.SetValue("")
		return m, tea.Println(dimStyle.Render(fmt.Sprintf("  Logging in to %s", m.loginURL)))
	}

	m.mode = modeLoginURL
	m.loginInput.Placeholder = "Server URL (e.g. https://myenv.app.neubird.ai/)..."
	m.loginInput.SetValue("")
	return m, tea.Println(dimStyle.Render("  Enter the Hawkeye server URL:"))
}

func (m model) handleLoginURLSubmit(value string) (tea.Model, tea.Cmd) {
	m.loginURL = value
	m.mode = modeLoginUser
	m.loginInput.Placeholder = "Username / Email..."
	m.loginInput.SetValue("")
	return m, tea.Sequence(
		tea.Println(dimStyle.Render(fmt.Sprintf("  Server: %s", value))),
		tea.Println(dimStyle.Render("  Enter your username/email:")),
	)
}

func (m model) handleLoginUserSubmit(value string) (tea.Model, tea.Cmd) {
	m.loginUser = value
	m.mode = modeLoginPass
	m.loginInput.Placeholder = "Password..."
	m.loginInput.SetValue("")
	m.loginInput.EchoCharacter = '•'
	m.loginInput.EchoMode = textinput.EchoPassword
	return m, tea.Sequence(
		tea.Println(dimStyle.Render(fmt.Sprintf("  User: %s", value))),
		tea.Println(dimStyle.Render("  Enter your password:")),
	)
}

type loginResultMsg struct {
	cfg *config.Config
	err error
}

func (m model) handleLoginPassSubmit(value string) (tea.Model, tea.Cmd) {
	password := value
	m.loginInput.EchoMode = textinput.EchoNormal
	m.loginInput.SetValue("")
	m.loginInput.Placeholder = "Authenticating..."

	serverURL := m.loginURL
	username := m.loginUser
	profile := m.profile

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Authenticating...")),
		func() tea.Msg {
			backendURL := api.NormalizeBackendURL(serverURL)
			client := api.NewClientWithServer(backendURL)

			loginResp, err := client.Login(username, password)
			if err != nil {
				return loginResultMsg{err: fmt.Errorf("authentication failed: %w", err)}
			}

			cfg, err := config.Load(profile)
			if err != nil {
				return loginResultMsg{err: err}
			}

			cfg.Server = backendURL
			// Frontend URL should not have /api suffix
			frontendURL := strings.TrimRight(serverURL, "/")
			if idx := strings.Index(frontendURL, "/api"); idx > 0 {
				frontendURL = frontendURL[:idx]
			}
			cfg.FrontendURL = frontendURL
			cfg.Username = username
			cfg.Token = loginResp.AccessToken

			// Auto-fetch org UUID
			authedClient := api.NewClient(cfg)
			userInfo, userErr := authedClient.FetchUserInfo()
			if userErr == nil && userInfo != nil && userInfo.OrgUUID != "" {
				cfg.OrgUUID = userInfo.OrgUUID
			}

			if err := cfg.Save(); err != nil {
				return loginResultMsg{err: err}
			}

			return loginResultMsg{cfg: cfg}
		},
	)
}

func (m model) handleLoginResult(msg loginResultMsg) (tea.Model, tea.Cmd) {
	m.mode = modeIdle
	m.input.Placeholder = "Ask a question or type /help..."

	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ %v", msg.err)))
	}

	m.cfg = msg.cfg
	m.client = api.NewClient(m.cfg)

	var cmds []tea.Cmd
	cmds = append(cmds,
		tea.Println(successMsgStyle.Render("  ✓ Logged in successfully!")),
		tea.Println(dimStyle.Render(fmt.Sprintf("    Server: %s", m.cfg.Server))),
		tea.Println(dimStyle.Render(fmt.Sprintf("    User: %s", m.cfg.Username))),
	)
	if m.cfg.OrgUUID != "" {
		cmds = append(cmds, tea.Println(dimStyle.Render(fmt.Sprintf("    Org: %s", m.cfg.OrgUUID))))
	}
	if m.cfg.ProjectID == "" {
		cmds = append(cmds, tea.Println(dimStyle.Render("    Next: type /projects to select a project")))
	}
	cmds = append(cmds, tea.Println(""))

	m.loginURL = ""
	m.loginUser = ""
	return m, tea.Sequence(cmds...)
}

// ─── /config ────────────────────────────────────────────────────────────────

func (m model) cmdConfig() (tea.Model, tea.Cmd) {
	if m.cfg == nil {
		return m, tea.Println(warnMsgStyle.Render("  ! No configuration found. Run /login first."))
	}

	val := func(s string) string {
		if s == "" {
			return dimStyle.Render("(not set)")
		}
		return s
	}
	token := dimStyle.Render("(not set)")
	if m.cfg.Token != "" {
		end := 12
		if len(m.cfg.Token) < end {
			end = len(m.cfg.Token)
		}
		token = m.cfg.Token[:end] + "..."
	}

	return m, tea.Sequence(
		tea.Println(""),
		tea.Println(dimStyle.Render("  Configuration:")),
		tea.Println(fmt.Sprintf("    Profile:      %s", config.ProfileName(m.profile))),
		tea.Println(fmt.Sprintf("    Server:       %s", val(m.cfg.Server))),
		tea.Println(fmt.Sprintf("    User:         %s", val(m.cfg.Username))),
		tea.Println(fmt.Sprintf("    Project:      %s", val(m.cfg.ProjectID))),
		tea.Println(fmt.Sprintf("    Organization: %s", val(m.cfg.OrgUUID))),
		tea.Println(fmt.Sprintf("    Token:        %s", token)),
		tea.Println(""),
	)
}

type sessionsLoadedMsg struct {
	sessions []api.SessionInfo
	err      error
}

func (m model) handleSessionsLoaded(msg sessionsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to load sessions: %v", msg.err)))
	}

	if len(msg.sessions) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No sessions found."))
	}

	sortSessionsNewestFirst(msg.sessions)
	m.mode = modeSessionSelect
	m.sessionList = msg.sessions
	m.sessionListIdx = 0
	return m, nil
}

// ─── /incidents list ────────────────────────────────────────────────────────

const openIncidentsPageSize = 20

type openIncidentsLoadedMsg struct {
	sessions []api.SessionInfo
	page     int
	hasMore  bool
	err      error
}

func (m model) cmdOpenIncidentsList(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Run /projects first."))
	}

	page := 1
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			page = n
		}
	}

	client := m.client
	projectID := m.cfg.ProjectID
	start := (page - 1) * openIncidentsPageSize

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Loading open incidents (page %d)...", page))),
		func() tea.Msg {
			filters := []api.PaginationFilter{
				{Key: "session_type", Value: "SESSION_TYPE_INCIDENT", Operator: "=="},
			}
			resp, err := client.SessionList(projectID, start, openIncidentsPageSize, filters)
			if err != nil {
				return openIncidentsLoadedMsg{err: err, page: page}
			}
			hasMore := len(resp.Sessions) == openIncidentsPageSize
			return openIncidentsLoadedMsg{sessions: resp.Sessions, page: page, hasMore: hasMore}
		},
	)
}

func (m model) handleOpenIncidentsLoaded(msg openIncidentsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.mode = modeIdle
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to load incidents: %v", msg.err)))
	}

	if len(msg.sessions) == 0 {
		m.mode = modeIdle
		if msg.page == 1 {
			return m, tea.Println(warnMsgStyle.Render("  ! No open incidents found."))
		}
		return m, tea.Println(warnMsgStyle.Render("  ! No more incidents."))
	}

	m.incidentList = msg.sessions
	m.incidentListIdx = 0
	m.incidentListPage = msg.page
	m.incidentListHasMore = msg.hasMore
	m.mode = modeIncidentList
	return m, nil
}

func formatInvestigationStatus(s string) string {
	switch s {
	case "INVESTIGATION_STATUS_NOT_STARTED":
		return dimStyle.Render("[not started]")
	case "INVESTIGATION_STATUS_IN_PROGRESS":
		return statusStyle.Render("[in progress]")
	case "INVESTIGATION_STATUS_INVESTIGATED":
		return successMsgStyle.Render("[investigated]")
	case "INVESTIGATION_STATUS_COMPLETED":
		return successMsgStyle.Render("[completed]")
	default:
		if s == "" {
			return ""
		}
		return dimStyle.Render("[" + s + "]")
	}
}

// ─── Incident list renderer ──────────────────────────────────────────────────

func (m model) renderIncidentList() string {
	var b strings.Builder
	b.WriteString("\n")
	header := fmt.Sprintf("  🚨 Open Incidents — page %d (%d shown)", m.incidentListPage, len(m.incidentList))
	b.WriteString(dimStyle.Render(header) + "\n\n")

	// Cap visible rows to avoid overflowing short terminals (reserve ~6 lines for header/footer)
	maxVisible := m.height - 6
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if m.incidentListIdx >= maxVisible {
		start = m.incidentListIdx - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.incidentList) {
		end = len(m.incidentList)
	}

	for i := start; i < end; i++ {
		s := m.incidentList[i]
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		if len(name) > 50 {
			name = name[:47] + "..."
		}
		status := formatInvestigationStatus(s.InvestigationStatus)
		padded := fmt.Sprintf("%-52s %s", name, status)
		if i == m.incidentListIdx {
			b.WriteString("  " + incidentRowSelectedStyle.Render("🦜 "+padded) + "\n")
		} else {
			b.WriteString("  " + incidentRowStyle.Render("   "+padded) + "\n")
		}
	}

	b.WriteString("\n")
	hints := "  ↑↓ navigate  Enter select"
	if m.incidentListHasMore {
		hints += "  n next"
	}
	if m.incidentListPage > 1 {
		hints += "  p prev"
	}
	hints += "  Esc cancel"
	b.WriteString(hintBarStyle.Render(hints))
	return b.String()
}

// ─── /inspect ───────────────────────────────────────────────────────────────

type inspectResultMsg struct {
	resp *api.SessionInspectResponse
	err  error
}

func (m model) cmdInspect(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if len(args) == 0 {
		if m.sessionID != "" {
			args = []string{m.sessionID}
		} else {
			return m, tea.Println(warnMsgStyle.Render("  ! Usage: /inspect <session-uuid>"))
		}
	}

	sessionUUID := args[0]
	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Inspecting %s...", truncateUUID(sessionUUID)))),
		func() tea.Msg {
			resp, err := client.SessionInspect(projectID, sessionUUID)
			if err != nil {
				return inspectResultMsg{err: err}
			}
			return inspectResultMsg{resp: resp}
		},
	)
}

func (m model) handleInspectResult(msg inspectResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Inspect failed: %v", msg.err)))
	}

	resp := msg.resp
	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""))

	if resp.SessionInfo != nil {
		s := resp.SessionInfo
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		cmds = append(cmds,
			tea.Println(fmt.Sprintf("  Session: %s", name)),
			tea.Println(dimStyle.Render(fmt.Sprintf("    UUID: %s", s.SessionUUID))),
			tea.Println(dimStyle.Render(fmt.Sprintf("    Created: %s  Type: %s", s.CreateTime, s.SessionType))),
		)
	}

	if len(resp.PromptCycle) == 0 {
		cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! No prompt cycles found.")))
		return m, tea.Sequence(cmds...)
	}

	for i, pc := range resp.PromptCycle {
		cmds = append(cmds,
			tea.Println(""),
			tea.Println(dimStyle.Render(fmt.Sprintf("  ── Prompt Cycle %d ──", i+1))),
		)

		if pc.Request != nil && len(pc.Request.Messages) > 0 {
			for _, msg := range pc.Request.Messages {
				if msg.Content != nil && len(msg.Content.Parts) > 0 {
					cmds = append(cmds, tea.Println(userPromptStyle.Render("  ❯ "+strings.Join(msg.Content.Parts, " "))))
				}
			}
		}

		if len(pc.ChainOfThoughts) > 0 {
			for _, cot := range pc.ChainOfThoughts {
				cat := cot.Category
				if cat == "" {
					cat = "analysis"
				}
				if cot.Description != "" {
					cmds = append(cmds, tea.Println(cotHeaderStyle.Render(fmt.Sprintf("  🔍 [%s] %s", cat, cot.Description))))
				}
			}
		}

		if len(pc.Sources) > 0 {
			cmds = append(cmds, tea.Println(sourceHeaderStyle.Render("  📎 Sources:")))
			for _, src := range pc.Sources {
				name := src.Title
				if name == "" {
					name = src.ID
				}
				cmds = append(cmds, tea.Println(dimStyle.Render("     • "+name)))
			}
		}

		if pc.FinalAnswer != "" {
			rendered := renderMarkdownBlock(pc.FinalAnswer)
			cmds = append(cmds, tea.Println(""))
			for _, line := range strings.Split(rendered, "\n") {
				cmds = append(cmds, tea.Println("  "+line))
			}
		}

		if len(pc.FollowUpSuggestions) > 0 {
			cmds = append(cmds, tea.Println(followUpStyle.Render("  💡 Follow-ups:")))
			for j, s := range pc.FollowUpSuggestions {
				cmds = append(cmds, tea.Println(followUpStyle.Render(fmt.Sprintf("     %d. %s", j+1, s))))
			}
		}
	}

	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /summary ───────────────────────────────────────────────────────────────

type summaryResultMsg struct {
	resp *api.GetSessionSummaryResponse
	err  error
}

func (m model) cmdSummary(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if len(args) == 0 {
		if m.sessionID != "" {
			args = []string{m.sessionID}
		} else {
			return m, tea.Println(warnMsgStyle.Render("  ! Usage: /summary <session-uuid>"))
		}
	}

	sessionUUID := args[0]
	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Loading summary for %s...", truncateUUID(sessionUUID)))),
		func() tea.Msg {
			resp, err := client.GetSessionSummary(projectID, sessionUUID)
			if err != nil {
				return summaryResultMsg{err: err}
			}
			return summaryResultMsg{resp: resp}
		},
	)
}

func (m model) handleSummaryResult(msg summaryResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Summary failed: %v", msg.err)))
	}

	resp := msg.resp
	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""))

	if resp.SessionInfo != nil {
		name := resp.SessionInfo.Name
		if name == "" {
			name = "Session Summary"
		}
		cmds = append(cmds, tea.Println(fmt.Sprintf("  Summary: %s", name)))
	}

	if resp.SessionSummary == nil {
		cmds = append(cmds, tea.Println(warnMsgStyle.Render("  ! No summary available yet.")))
		return m, tea.Sequence(cmds...)
	}

	summary := resp.SessionSummary

	if summary.ShortSummary != nil {
		if summary.ShortSummary.Question != "" {
			cmds = append(cmds, tea.Println(dimStyle.Render(fmt.Sprintf("  Question: %s", summary.ShortSummary.Question))))
		}
		if summary.ShortSummary.Analysis != "" {
			cmds = append(cmds, tea.Println(dimStyle.Render(fmt.Sprintf("  Quick Analysis: %s", summary.ShortSummary.Analysis))))
		}
	}

	if summary.Analysis != "" {
		rendered := renderMarkdownBlock(summary.Analysis)
		cmds = append(cmds, tea.Println(""))
		for _, line := range strings.Split(rendered, "\n") {
			cmds = append(cmds, tea.Println("  "+line))
		}
	}

	if len(summary.ActionItems) > 0 {
		cmds = append(cmds, tea.Println(""), tea.Println("  🎯 Action Items:"))
		for i, item := range summary.ActionItems {
			cmds = append(cmds, tea.Println(fmt.Sprintf("     %d. %s", i+1, item)))
		}
	}

	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /feedback (/td) ────────────────────────────────────────────────────────

type feedbackResultMsg struct {
	err error
}

func (m model) cmdFeedback(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}

	sessionUUID := ""
	reason := "Thumbs down from CLI"

	for i := 0; i < len(args); i++ {
		if (args[i] == "-r" || args[i] == "--reason") && i+1 < len(args) {
			i++
			reason = args[i]
		} else if sessionUUID == "" {
			sessionUUID = args[i]
		}
	}

	if sessionUUID == "" {
		sessionUUID = m.sessionID
	}
	if sessionUUID == "" {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /feedback [session-uuid] [-r reason]"))
	}

	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Submitting feedback for %s...", truncateUUID(sessionUUID)))),
		func() tea.Msg {
			resp, err := client.SessionInspect(projectID, sessionUUID)
			if err != nil {
				return feedbackResultMsg{err: err}
			}
			if len(resp.PromptCycle) == 0 {
				return feedbackResultMsg{err: fmt.Errorf("no prompt cycles found")}
			}
			last := resp.PromptCycle[len(resp.PromptCycle)-1]
			items := []api.RatingItemID{{ItemType: "ITEM_TYPE_PROMPT_CYCLE", ItemID: last.ID}}
			err = client.PutRating(projectID, sessionUUID, items, "RATING_THUMBS_DOWN", reason)
			return feedbackResultMsg{err: err}
		},
	)
}

func (m model) handleFeedbackResult(msg feedbackResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Feedback failed: %v", msg.err)))
	}
	return m, tea.Println(statusStyle.Render("  ✓ Thumbs down submitted"))
}

// ─── /prompts ───────────────────────────────────────────────────────────────

type promptsLoadedMsg struct {
	items []api.InitialPrompt
	err   error
}

func (m model) cmdPrompts() (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Run /projects first."))
	}

	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Loading prompts...")),
		func() tea.Msg {
			resp, err := client.PromptLibrary(projectID)
			if err != nil {
				return promptsLoadedMsg{err: err}
			}
			return promptsLoadedMsg{items: resp.Items}
		},
	)
}

func (m model) handlePromptsLoaded(msg promptsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to load prompts: %v", msg.err)))
	}

	if len(msg.items) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No prompts found."))
	}

	var cmds []tea.Cmd
	cmds = append(cmds,
		tea.Println(""),
		tea.Println(dimStyle.Render(fmt.Sprintf("  Prompt Library (%d):", len(msg.items)))),
		tea.Println(""),
	)

	for i, p := range msg.items {
		label := p.Oneliner
		if label == "" {
			label = p.Prompt
			if len(label) > 80 {
				label = label[:77] + "..."
			}
		}
		cmds = append(cmds, tea.Println(fmt.Sprintf("  %d. %s", i+1, label)))
	}

	cmds = append(cmds,
		tea.Println(""),
		tea.Println(dimStyle.Render("  Tip: Copy a prompt and paste it to investigate")),
		tea.Println(""),
	)

	return m, tea.Sequence(cmds...)
}

// ─── /projects ──────────────────────────────────────────────────────────────

type projectsLoadedMsg struct {
	projects []api.ProjectSpec
	err      error
}

func (m model) cmdProjects(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}

	// Subcommand dispatch
	if len(args) > 0 {
		switch args[0] {
		case "info":
			return m.cmdProjectInfo(args[1:])
		case "create":
			return m.cmdProjectCreate(args[1:])
		case "delete":
			return m.cmdProjectDelete(args[1:])
		}
	}

	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Loading projects...")),
		func() tea.Msg {
			resp, err := client.ListProjects()
			if err != nil {
				return projectsLoadedMsg{err: err}
			}
			return projectsLoadedMsg{projects: service.FilterSystemProjects(resp.Specs)}
		},
	)
}

func (m model) handleProjectsLoaded(msg projectsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to load projects: %v", msg.err)))
	}

	if len(msg.projects) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No projects found."))
	}

	// Find current project index to pre-select it
	selectedIdx := 0
	for i, p := range msg.projects {
		if p.UUID == m.cfg.ProjectID {
			selectedIdx = i
			break
		}
	}

	// Enter interactive project selection mode
	m.mode = modeProjectSelect
	m.projectList = msg.projects
	m.projectListIdx = selectedIdx

	return m, nil
}

// ─── /projects info ──────────────────────────────────────────────────────────

type projectInfoMsg struct {
	detail service.ProjectDetailDisplay
	err    error
}

func (m model) cmdProjectInfo(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /projects info <uuid>"))
	}
	projectUUID := args[0]
	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Loading project %s...", truncateUUID(projectUUID)))),
		func() tea.Msg {
			resp, err := client.GetProject(projectUUID)
			if err != nil {
				return projectInfoMsg{err: err}
			}
			return projectInfoMsg{detail: service.FormatProjectDetail(resp.Spec)}
		},
	)
}

func (m model) handleProjectInfo(msg projectInfoMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}

	p := msg.detail
	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""))
	cmds = append(cmds, tea.Println(fmt.Sprintf("  Project: %s", p.Name)))
	cmds = append(cmds, tea.Println(dimStyle.Render(fmt.Sprintf("    UUID: %s", p.UUID))))
	if p.Description != "" {
		cmds = append(cmds, tea.Println(dimStyle.Render(fmt.Sprintf("    Description: %s", p.Description))))
	}
	ready := successMsgStyle.Render("ready")
	if !p.Ready {
		ready = warnMsgStyle.Render("not ready")
	}
	cmds = append(cmds, tea.Println(fmt.Sprintf("    Status: %s", ready)))
	if p.CreateTime != "" {
		cmds = append(cmds, tea.Println(dimStyle.Render(fmt.Sprintf("    Created: %s", p.CreateTime))))
	}
	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /projects create ───────────────────────────────────────────────────────

type projectCreateMsg struct {
	spec *api.ProjectDetail
	err  error
}

func (m model) cmdProjectCreate(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /projects create <name>"))
	}
	name := strings.Join(args, " ")
	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Creating project '%s'...", name))),
		func() tea.Msg {
			resp, err := client.CreateProject(name, "")
			if err != nil {
				return projectCreateMsg{err: err}
			}
			return projectCreateMsg{spec: resp.Spec}
		},
	)
}

func (m model) handleProjectCreate(msg projectCreateMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}
	if msg.spec != nil {
		return m, tea.Sequence(
			tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Project created: %s", msg.spec.Name))),
			tea.Println(dimStyle.Render(fmt.Sprintf("    UUID: %s", msg.spec.UUID))),
			tea.Println(dimStyle.Render("    Use /set project <uuid> to activate")),
			tea.Println(""),
		)
	}
	return m, tea.Println(successMsgStyle.Render("  ✓ Project created"))
}

// ─── /projects delete ───────────────────────────────────────────────────────

type projectDeleteMsg struct {
	uuid string
	err  error
}

func (m model) cmdProjectDelete(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /projects delete <uuid>"))
	}
	projectUUID := args[0]
	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Deleting project %s...", truncateUUID(projectUUID)))),
		func() tea.Msg {
			err := client.DeleteProject(projectUUID)
			return projectDeleteMsg{uuid: projectUUID, err: err}
		},
	)
}

func (m model) handleProjectDelete(msg projectDeleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Delete failed: %v", msg.err)))
	}
	return m, tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Project %s deleted", truncateUUID(msg.uuid))))
}

// ─── /set ───────────────────────────────────────────────────────────────────

// setProjectResultMsg is returned after looking up project name
type setProjectResultMsg struct {
	projectID   string
	projectName string
	err         error
}

func (m model) cmdSet(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Sequence(
			tea.Println(""),
			tea.Println(dimStyle.Render("  Usage: /set project [uuid-or-name]")),
			tea.Println(dimStyle.Render("  Or use /projects for interactive selection")),
			tea.Println(""),
		)
	}

	key := strings.ToLower(args[0])

	switch key {
	case "project":
		if m.cfg == nil {
			return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
		}
		if m.client == nil {
			return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
		}

		// If no value provided, show interactive selector
		if len(args) < 2 {
			return m.cmdProjects(nil)
		}

		value := args[1]
		client := m.client
		return m, tea.Sequence(
			tea.Println(statusStyle.Render("  ⟳ Looking up project...")),
			func() tea.Msg {
				resp, err := client.ListProjects()
				if err != nil {
					return setProjectResultMsg{err: err}
				}
				projects := service.FilterSystemProjects(resp.Specs)
				// Try to find by UUID or name
				for _, p := range projects {
					if p.UUID == value || strings.EqualFold(p.Name, value) {
						return setProjectResultMsg{projectID: p.UUID, projectName: p.Name}
					}
				}
				// Not found - return error instead of using invalid value
				return setProjectResultMsg{err: fmt.Errorf("project %q not found", value)}
			},
		)

	default:
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Unknown key: %s (valid: project)", key)))
	}
}

func (m model) handleSetProjectResult(msg setProjectResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to set project: %v", msg.err)))
	}

	m.cfg.ProjectID = msg.projectID
	m.cfg.ProjectName = msg.projectName
	if err := m.cfg.Save(); err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to save config: %v", err)))
	}
	if m.cfg.Server != "" && m.cfg.Token != "" {
		m.client = api.NewClient(m.cfg)
	}

	return m, tea.Sequence(
		tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Project set to: %s", msg.projectName))),
		tea.Println(dimStyle.Render("    You can now start investigating!")),
	)
}

// ─── /session ───────────────────────────────────────────────────────────────

func (m model) cmdSetSession(args []string) (tea.Model, tea.Cmd) {
	if len(args) > 0 {
		m.sessionID = args[0]
		return m, tea.Sequence(
			tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Session set to: %s", m.sessionID))),
			tea.Println(dimStyle.Render("    Follow-up questions will continue in this session.")),
		)
	}

	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Run /projects first."))
	}

	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Loading sessions...")),
		func() tea.Msg {
			filters := []api.PaginationFilter{{
				Key:      "session_type",
				Value:    "SESSION_TYPE_CHAT",
				Operator: "==",
			}}
			resp, err := client.SessionList(projectID, 0, 20, filters)
			if err != nil {
				return sessionsLoadedMsg{err: err}
			}
			return sessionsLoadedMsg{sessions: resp.Sessions}
		},
	)
}

// ─── /score ─────────────────────────────────────────────────────────────────

type scoreResultMsg struct {
	scores service.ScoreDisplay
	err    error
}

func (m model) cmdScore(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if len(args) == 0 {
		if m.sessionID != "" {
			args = []string{m.sessionID}
		} else {
			return m, tea.Println(warnMsgStyle.Render("  ! Usage: /score <session-uuid>"))
		}
	}

	sessionUUID := args[0]
	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Loading scores for %s...", truncateUUID(sessionUUID)))),
		func() tea.Msg {
			resp, err := client.GetSessionSummary(projectID, sessionUUID)
			if err != nil {
				return scoreResultMsg{err: err}
			}
			return scoreResultMsg{scores: service.ExtractScores(resp)}
		},
	)
}

func (m model) handleScoreResult(msg scoreResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Score failed: %v", msg.err)))
	}
	if !msg.scores.HasScores {
		return m, tea.Println(warnMsgStyle.Render("  ! No RCA scores available for this session."))
	}

	s := msg.scores
	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""))
	cmds = append(cmds, tea.Println(dimStyle.Render("  RCA Quality Scores:")))

	if s.ScoredBy != "" {
		cmds = append(cmds, tea.Println(dimStyle.Render(fmt.Sprintf("    Scored by: %s", s.ScoredBy))))
	}

	cmds = append(cmds, tea.Println(fmt.Sprintf("    📊 Accuracy:     %.1f/100", s.Accuracy.Score)))
	if s.Accuracy.Summary != "" {
		cmds = append(cmds, tea.Println(dimStyle.Render("       "+s.Accuracy.Summary)))
	}
	cmds = append(cmds, tea.Println(fmt.Sprintf("    📊 Completeness: %.1f/100", s.Completeness.Score)))
	if s.Completeness.Summary != "" {
		cmds = append(cmds, tea.Println(dimStyle.Render("       "+s.Completeness.Summary)))
	}

	if len(s.Qualitative.Strengths) > 0 {
		cmds = append(cmds, tea.Println(successMsgStyle.Render("    ✅ Strengths:")))
		for _, str := range s.Qualitative.Strengths {
			cmds = append(cmds, tea.Println("      • "+str))
		}
	}
	if len(s.Qualitative.Improvements) > 0 {
		cmds = append(cmds, tea.Println(warnMsgStyle.Render("    💡 Improvements:")))
		for _, imp := range s.Qualitative.Improvements {
			cmds = append(cmds, tea.Println("      • "+imp))
		}
	}

	if s.TimeSaved != nil {
		cmds = append(cmds, tea.Println(fmt.Sprintf("    ⏱  Time saved: %.0f min (%.0f → %.0f)",
			s.TimeSaved.TimeSavedMinutes,
			s.TimeSaved.StandardInvestigationMin,
			s.TimeSaved.HawkeyeInvestigationMin)))
	}

	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /link ──────────────────────────────────────────────────────────────────

func (m model) cmdLink(args []string) (tea.Model, tea.Cmd) {
	if m.cfg == nil || m.cfg.Server == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Run /projects first."))
	}

	sessionUUID := ""
	if len(args) > 0 {
		sessionUUID = args[0]
	} else if m.sessionID != "" {
		sessionUUID = m.sessionID
	} else {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /link <session-uuid>"))
	}

	url := service.BuildSessionURL(m.cfg.Server, m.cfg.ProjectID, sessionUUID)
	return m, tea.Sequence(
		tea.Println(""),
		tea.Println("  "+url),
		tea.Println(""),
	)
}

// ─── /open ──────────────────────────────────────────────────────────────────

func (m model) cmdOpen(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /open <url>"))
	}

	_, projectUUID, sessionUUID, err := service.ParseSessionURL(args[0])
	if err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Invalid URL: %v", err)))
	}

	m.sessionID = sessionUUID
	if m.cfg != nil {
		m.cfg.ProjectID = projectUUID
	}
	return m.cmdInspect([]string{sessionUUID})
}

// ─── /report ────────────────────────────────────────────────────────────────

type reportResultMsg struct {
	report service.ReportDisplay
	err    error
}

func (m model) cmdReport() (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}

	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Loading incident report...")),
		func() tea.Msg {
			resp, err := client.GetIncidentReport()
			if err != nil {
				return reportResultMsg{err: err}
			}
			return reportResultMsg{report: service.FormatReport(resp)}
		},
	)
}

func (m model) handleReportResult(msg reportResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Report failed: %v", msg.err)))
	}

	r := msg.report
	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""))
	cmds = append(cmds, tea.Println(dimStyle.Render("  Incident Analytics Report:")))

	if r.Period != "" {
		cmds = append(cmds, tea.Println(dimStyle.Render("    Period: "+r.Period)))
	}

	cmds = append(cmds,
		tea.Println(fmt.Sprintf("    Total incidents:      %d", r.TotalIncidents)),
		tea.Println(fmt.Sprintf("    Total investigations: %d", r.TotalInvestigations)),
		tea.Println(fmt.Sprintf("    Avg time saved:       %s", r.AvgTimeSavedMinutes)),
		tea.Println(fmt.Sprintf("    Avg MTTR:             %s", r.AvgMTTR)),
		tea.Println(fmt.Sprintf("    Noise reduction:      %s", r.NoiseReduction)),
		tea.Println(fmt.Sprintf("    Total time saved:     %s", r.TotalTimeSavedHours)),
	)

	if len(r.IncidentTypes) > 0 {
		cmds = append(cmds, tea.Println(""))
		cmds = append(cmds, tea.Println(dimStyle.Render("    By type:")))
		for _, it := range r.IncidentTypes {
			cmds = append(cmds, tea.Println(fmt.Sprintf("      %s", it.Type)))
			for _, pr := range it.Priorities {
				cmds = append(cmds, tea.Println(fmt.Sprintf("        [%s]  incidents: %-5d  investigated: %-3d  grouped: %-6s  saved: %s",
					pr.Priority, pr.TotalIncidents, pr.Investigated, pr.PercentGrouped, pr.AvgTimeSaved)))
			}
		}
	}

	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /connections ───────────────────────────────────────────────────────────

type connectionsResultMsg struct {
	connections []service.ConnectionDisplay
	err         error
}

type resourcesResultMsg struct {
	resources []service.ResourceDisplay
	connUUID  string
	err       error
}

func (m model) cmdConnections(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Run /projects first."))
	}

	// Subcommand dispatch
	if len(args) > 0 {
		switch args[0] {
		case "list":
			client := m.client
			projectID := m.cfg.ProjectID
			return m, tea.Sequence(
				tea.Println(statusStyle.Render("  ⟳ Loading connections...")),
				func() tea.Msg {
					resp, err := client.ListConnections(projectID)
					if err != nil {
						return connectionsResultMsg{err: err}
					}
					var conns []service.ConnectionDisplay
					for _, spec := range resp.Specs {
						conns = append(conns, service.FormatConnection(spec))
					}
					return connectionsResultMsg{connections: conns}
				},
			)
		case "resources":
			if len(args) < 2 {
				return m, tea.Println(warnMsgStyle.Render("  ! Usage: /connections resources <connection-uuid>"))
			}
			connUUID := args[1]
			client := m.client
			return m, tea.Sequence(
				tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Loading resources for %s...", truncateUUID(connUUID)))),
				func() tea.Msg {
					resp, err := client.ListConnectionResources(connUUID, 100)
					if err != nil {
						return resourcesResultMsg{err: err}
					}
					return resourcesResultMsg{
						resources: service.FormatResources(resp.Specs),
						connUUID:  connUUID,
					}
				},
			)
		case "types":
			return m.cmdConnectionTypes()
		case "info":
			return m.cmdConnectionInfo(args[1:])
		case "add":
			return m.cmdConnectionAdd(args[1:])
		case "remove":
			return m.cmdConnectionRemove(args[1:])
		}
	}

	// No subcommand: list connections by default
	client := m.client
	projectID := m.cfg.ProjectID
	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Loading connections...")),
		func() tea.Msg {
			resp, err := client.ListConnections(projectID)
			if err != nil {
				return connectionsResultMsg{err: err}
			}
			var conns []service.ConnectionDisplay
			for _, spec := range resp.Specs {
				conns = append(conns, service.FormatConnection(spec))
			}
			return connectionsResultMsg{connections: conns}
		},
	)
}

func (m model) cmdConnectionTypes() (tea.Model, tea.Cmd) {
	types := service.GetConnectionTypes()
	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""), tea.Println(dimStyle.Render(fmt.Sprintf("  Connection Types (%d):", len(types)))), tea.Println(""))
	for _, ct := range types {
		cmds = append(cmds, tea.Println(fmt.Sprintf("  • %-15s %s", ct.Type, dimStyle.Render(ct.Description))))
	}
	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

type connInfoMsg struct {
	detail service.ConnectionDetailDisplay
	err    error
}

func (m model) cmdConnectionInfo(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /connections info <uuid>"))
	}
	connUUID := args[0]
	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Loading connection %s...", truncateUUID(connUUID)))),
		func() tea.Msg {
			resp, err := client.GetConnectionInfo(connUUID)
			if err != nil {
				return connInfoMsg{err: err}
			}
			return connInfoMsg{detail: service.FormatConnectionDetail(resp.Spec)}
		},
	)
}

func (m model) handleConnInfo(msg connInfoMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}
	c := msg.detail
	return m, tea.Sequence(
		tea.Println(""),
		tea.Println(fmt.Sprintf("  Connection: %s", c.Name)),
		tea.Println(dimStyle.Render(fmt.Sprintf("    UUID: %s  Type: %s", c.UUID, c.Type))),
		tea.Println(dimStyle.Render(fmt.Sprintf("    Sync: %s  Training: %s", c.SyncState, c.TrainingState))),
		tea.Println(""),
	)
}

type connAddMsg struct {
	connUUID string
	err      error
}

func (m model) cmdConnectionAdd(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /connections add <uuid>"))
	}
	connUUID := args[0]
	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Adding connection %s...", truncateUUID(connUUID)))),
		func() tea.Msg {
			err := client.AddConnectionToProject(projectID, connUUID)
			return connAddMsg{connUUID: connUUID, err: err}
		},
	)
}

func (m model) handleConnAdd(msg connAddMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}
	return m, tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Connection %s added to project", truncateUUID(msg.connUUID))))
}

type connRemoveMsg struct {
	connUUID string
	err      error
}

func (m model) cmdConnectionRemove(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /connections remove <uuid>"))
	}
	connUUID := args[0]
	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Removing connection %s...", truncateUUID(connUUID)))),
		func() tea.Msg {
			err := client.RemoveConnectionFromProject(projectID, connUUID)
			return connRemoveMsg{connUUID: connUUID, err: err}
		},
	)
}

func (m model) handleConnRemove(msg connRemoveMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}
	return m, tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Connection %s removed from project", truncateUUID(msg.connUUID))))
}

func (m model) handleConnectionsResult(msg connectionsResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Connections failed: %v", msg.err)))
	}

	if len(msg.connections) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No connections found."))
	}

	var cmds []tea.Cmd
	cmds = append(cmds,
		tea.Println(""),
		tea.Println(dimStyle.Render(fmt.Sprintf("  Connections (%d):", len(msg.connections)))),
		tea.Println(""),
	)

	for _, c := range msg.connections {
		syncIcon := "🔄"
		if c.SyncState == "SYNCED" || c.SyncState == "SYNC_STATE_SYNCED" {
			syncIcon = "✅"
		}
		cmds = append(cmds,
			tea.Println(fmt.Sprintf("  %s %s  (%s)", syncIcon, c.Name, c.Type)),
			tea.Println(dimStyle.Render(fmt.Sprintf("    %s  sync: %s  training: %s", c.UUID, c.SyncState, c.TrainingState))),
		)
	}

	cmds = append(cmds,
		tea.Println(""),
		tea.Println(dimStyle.Render("  Tip: /connections resources <uuid> to list resources")),
		tea.Println(""),
	)

	return m, tea.Sequence(cmds...)
}

func (m model) handleResourcesResult(msg resourcesResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Resources failed: %v", msg.err)))
	}

	if len(msg.resources) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No resources found."))
	}

	var cmds []tea.Cmd
	cmds = append(cmds,
		tea.Println(""),
		tea.Println(dimStyle.Render(fmt.Sprintf("  Resources for %s (%d):", truncateUUID(msg.connUUID), len(msg.resources)))),
		tea.Println(""),
	)

	for _, r := range msg.resources {
		cmds = append(cmds, tea.Println(fmt.Sprintf("  • %-30s  %s", r.Name, dimStyle.Render(r.TelemetryType))))
	}

	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /instructions ──────────────────────────────────────────────────────────

type instructionsLoadedMsg struct {
	instructions []service.InstructionDisplay
	err          error
}

func (m model) cmdInstructions(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Run /projects first."))
	}

	// Subcommand dispatch
	if len(args) > 0 {
		switch args[0] {
		case "create":
			return m.cmdInstructionCreate(args[1:])
		case "enable":
			return m.cmdInstructionToggle(args[1:], true)
		case "disable":
			return m.cmdInstructionToggle(args[1:], false)
		case "delete":
			return m.cmdInstructionDelete(args[1:])
		}
	}

	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Loading instructions...")),
		func() tea.Msg {
			resp, err := client.ListInstructions(projectID)
			if err != nil {
				return instructionsLoadedMsg{err: err}
			}
			return instructionsLoadedMsg{instructions: service.FormatInstructions(resp.Instructions)}
		},
	)
}

func (m model) handleInstructionsLoaded(msg instructionsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}

	if len(msg.instructions) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No instructions found."))
	}

	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""), tea.Println(dimStyle.Render(fmt.Sprintf("  Instructions (%d):", len(msg.instructions)))), tea.Println(""))

	for _, instr := range msg.instructions {
		status := successMsgStyle.Render("enabled")
		if !instr.Enabled {
			status = dimStyle.Render("disabled")
		}
		cmds = append(cmds,
			tea.Println(fmt.Sprintf("  %s  [%s]  %s", instr.Name, instr.Type, status)),
			tea.Println(dimStyle.Render(fmt.Sprintf("    %s", instr.UUID))),
		)
	}
	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

type instructionCreateMsg struct {
	spec *api.InstructionSpec
	err  error
}

func (m model) cmdInstructionCreate(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /instructions create <name>"))
	}
	name := strings.Join(args, " ")
	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Creating instruction '%s'...", name))),
		func() tea.Msg {
			resp, err := client.CreateInstruction(projectID, name, "system", "")
			if err != nil {
				return instructionCreateMsg{err: err}
			}
			return instructionCreateMsg{spec: resp.Instruction}
		},
	)
}

func (m model) handleInstructionCreate(msg instructionCreateMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}
	if msg.spec != nil {
		return m, tea.Sequence(
			tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Instruction created: %s", msg.spec.Name))),
			tea.Println(dimStyle.Render(fmt.Sprintf("    UUID: %s", msg.spec.UUID))),
		)
	}
	return m, tea.Println(successMsgStyle.Render("  ✓ Instruction created"))
}

type instructionToggleMsg struct {
	uuid    string
	enabled bool
	err     error
}

func (m model) cmdInstructionToggle(args []string, enable bool) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		action := "enable"
		if !enable {
			action = "disable"
		}
		return m, tea.Println(warnMsgStyle.Render(fmt.Sprintf("  ! Usage: /instructions %s <uuid>", action)))
	}
	instrUUID := args[0]
	client := m.client

	action := "Enabling"
	if !enable {
		action = "Disabling"
	}

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ %s %s...", action, truncateUUID(instrUUID)))),
		func() tea.Msg {
			err := client.UpdateInstructionStatus(instrUUID, enable)
			return instructionToggleMsg{uuid: instrUUID, enabled: enable, err: err}
		},
	)
}

func (m model) handleInstructionToggle(msg instructionToggleMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}
	action := "enabled"
	if !msg.enabled {
		action = "disabled"
	}
	return m, tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Instruction %s %s", truncateUUID(msg.uuid), action)))
}

type instructionDeleteMsg struct {
	uuid string
	err  error
}

func (m model) cmdInstructionDelete(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /instructions delete <uuid>"))
	}
	instrUUID := args[0]
	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Deleting instruction %s...", truncateUUID(instrUUID)))),
		func() tea.Msg {
			err := client.DeleteInstruction(instrUUID)
			return instructionDeleteMsg{uuid: instrUUID, err: err}
		},
	)
}

func (m model) handleInstructionDelete(msg instructionDeleteMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed: %v", msg.err)))
	}
	return m, tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Instruction %s deleted", truncateUUID(msg.uuid))))
}

// ─── /rerun ─────────────────────────────────────────────────────────────────

type rerunResultMsg struct {
	sessionUUID string
	err         error
}

func (m model) cmdRerun(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}

	sessionUUID := ""
	if len(args) > 0 {
		sessionUUID = args[0]
	} else if m.sessionID != "" {
		sessionUUID = m.sessionID
	} else {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /rerun [session-uuid]"))
	}

	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Rerunning session %s...", truncateUUID(sessionUUID)))),
		func() tea.Msg {
			resp, err := client.RerunSession(sessionUUID)
			if err != nil {
				return rerunResultMsg{err: err}
			}
			return rerunResultMsg{sessionUUID: resp.SessionUUID}
		},
	)
}

func (m model) handleRerunResult(msg rerunResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Rerun failed: %v", msg.err)))
	}
	return m, tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Rerun started (session: %s)", truncateUUID(msg.sessionUUID))))
}

// ─── /investigate-alert ──────────────────────────────────────────────────────

func (m model) cmdInvestigateAlert(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg == nil || m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Use /set project <uuid>"))
	}
	if len(args) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /investigate-alert <alert-id>"))
	}

	alertID := args[0]
	m.mode = modeStreaming
	m.resetStreamState()
	m.streamPrompt = fmt.Sprintf("Investigate alert %s", alertID)

	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(""),
		tea.Println(userPromptStyle.Render("  ❯ Investigate alert: "+alertID)),
		tea.Println(""),
		tea.Println(statusStyle.Render("  ⟳ Creating session from alert...")),
		func() tea.Msg {
			sessResp, err := client.CreateSessionFromAlert(projectID, alertID)
			if err != nil {
				return streamErrMsg{err: fmt.Errorf("creating alert session: %w", err)}
			}
			return sessionCreatedMsg{sessionID: sessResp.SessionUUID}
		},
	)
}

// ─── /queries ───────────────────────────────────────────────────────────────

type queriesResultMsg struct {
	queries []service.QueryDisplay
	err     error
}

func (m model) cmdQueries(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}

	sessionUUID := ""
	if len(args) > 0 {
		sessionUUID = args[0]
	} else if m.sessionID != "" {
		sessionUUID = m.sessionID
	} else {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /queries [session-uuid]"))
	}

	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Loading queries for %s...", truncateUUID(sessionUUID)))),
		func() tea.Msg {
			resp, err := client.GetInvestigationQueries(m.cfg.ProjectID, sessionUUID)
			if err != nil {
				return queriesResultMsg{err: err}
			}
			return queriesResultMsg{queries: service.FormatQueries(resp.Queries)}
		},
	)
}

func (m model) handleQueriesResult(msg queriesResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Queries failed: %v", msg.err)))
	}

	if len(msg.queries) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No queries found."))
	}

	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""), tea.Println(dimStyle.Render(fmt.Sprintf("  Queries (%d):", len(msg.queries)))), tea.Println(""))

	for i, q := range msg.queries {
		statusIcon := "✅"
		if q.Status == "FAILED" || q.Status == "ERROR" {
			statusIcon = "❌"
		}
		cmds = append(cmds, tea.Println(fmt.Sprintf("  %s Query %d  (%s)", statusIcon, i+1, q.Source)))
		if q.Query != "" {
			query := q.Query
			if len(query) > 80 {
				query = query[:77] + "..."
			}
			cmds = append(cmds, tea.Println(dimStyle.Render("    "+query)))
		}
	}

	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /discover ──────────────────────────────────────────────────────────────

type discoverResultMsg struct {
	resources []service.DiscoveredResource
	err       error
}

func (m model) cmdDiscover() (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Run /projects first."))
	}

	client := m.client
	projectID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Discovering project resources...")),
		func() tea.Msg {
			resp, err := client.DiscoverProjectResources(projectID, "", "")
			if err != nil {
				return discoverResultMsg{err: err}
			}
			return discoverResultMsg{resources: service.FormatDiscoveredResources(resp.Resources)}
		},
	)
}

func (m model) handleDiscoverResult(msg discoverResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Discovery failed: %v", msg.err)))
	}

	if len(msg.resources) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No resources discovered."))
	}

	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""), tea.Println(dimStyle.Render(fmt.Sprintf("  Discovered Resources (%d):", len(msg.resources)))), tea.Println(""))

	for _, r := range msg.resources {
		cmds = append(cmds, tea.Println(fmt.Sprintf("  • %-30s %s", r.Name, dimStyle.Render(r.TelemetryType))))
	}

	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /session-report ────────────────────────────────────────────────────────

type sessionReportMsg struct {
	items []api.SessionReportItem
	err   error
}

func (m model) cmdSessionReport(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}

	sessionUUID := ""
	if len(args) > 0 {
		sessionUUID = args[0]
	} else if m.sessionID != "" {
		sessionUUID = m.sessionID
	} else {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /session-report [session-uuid]"))
	}

	client := m.client
	projectUUID := m.cfg.ProjectID

	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Loading report for %s...", truncateUUID(sessionUUID)))),
		func() tea.Msg {
			items, err := client.GetSessionReport(projectUUID, []string{sessionUUID})
			if err != nil {
				return sessionReportMsg{err: err}
			}
			return sessionReportMsg{items: items}
		},
	)
}

func (m model) handleSessionReport(msg sessionReportMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Report failed: %v", msg.err)))
	}

	var cmds []tea.Cmd
	cmds = append(cmds, tea.Println(""), tea.Println(dimStyle.Render("  Session Report:")))

	for _, item := range msg.items {
		if item.Summary != "" {
			cmds = append(cmds, tea.Println(dimStyle.Render("    Summary: "+item.Summary)))
		}
		if item.TimeSaved > 0 {
			cmds = append(cmds, tea.Println(fmt.Sprintf("    ⏱  Time saved: %d min", item.TimeSaved/60)))
		}
	}

	if len(msg.items) == 0 {
		cmds = append(cmds, tea.Println(dimStyle.Render("    No report data available.")))
	}

	cmds = append(cmds, tea.Println(""))
	return m, tea.Sequence(cmds...)
}

// ─── /incidents (add connections: PagerDuty, FireHydrant, incident.io) ──────

type addConnectionResultMsg struct {
	label string // human-readable connection type, e.g. "PagerDuty"
	name  string
	uuid  string
	err   error
}

func parseAddConnectionArgs(args []string) (name, apiKey string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				i++
				name = args[i]
			}
		case "--api-key":
			if i+1 < len(args) {
				i++
				apiKey = args[i]
			}
		}
	}
	return
}

func (m model) cmdConnectionAddPagerDuty(args []string) (tea.Model, tea.Cmd) {
	name, apiKey := parseAddConnectionArgs(args)
	if name == "" || apiKey == "" {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /incidents add pagerduty --name <name> --api-key <key>"))
	}
	client := m.client
	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Adding PagerDuty connection %q...", name))),
		func() tea.Msg {
			resp, err := client.AddConnection(&api.AddConnectionRequest{
				Connection: api.AddConnectionInput{
					Name:           name,
					ConnectionType: "CONNECTION_TYPE_PAGERDUTY",
					PagerdutyConnectionInfo: &api.PagerdutyConnectionInfo{
						ApiAccessKey: apiKey,
					},
				},
			})
			if err != nil {
				return addConnectionResultMsg{err: err}
			}
			if resp.Response.ErrorMessage != "" {
				return addConnectionResultMsg{err: fmt.Errorf("%s", resp.Response.ErrorMessage)}
			}
			return addConnectionResultMsg{label: "PagerDuty", name: name, uuid: resp.Response.UUID}
		},
	)
}

func (m model) cmdConnectionAddFirehydrant(args []string) (tea.Model, tea.Cmd) {
	name, apiKey := parseAddConnectionArgs(args)
	if name == "" || apiKey == "" {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /incidents add firehydrant --name <name> --api-key <key>"))
	}
	client := m.client
	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Adding FireHydrant connection %q...", name))),
		func() tea.Msg {
			resp, err := client.AddConnection(&api.AddConnectionRequest{
				Connection: api.AddConnectionInput{
					Name:           name,
					ConnectionType: "CONNECTION_TYPE_FIREHYDRANT",
					FirehydrantConnectionInfo: &api.FirehydrantConnectionInfo{
						ApiKey: apiKey,
					},
				},
			})
			if err != nil {
				return addConnectionResultMsg{err: err}
			}
			if resp.Response.ErrorMessage != "" {
				return addConnectionResultMsg{err: fmt.Errorf("%s", resp.Response.ErrorMessage)}
			}
			return addConnectionResultMsg{label: "FireHydrant", name: name, uuid: resp.Response.UUID}
		},
	)
}

func (m model) cmdConnectionAddIncidentio(args []string) (tea.Model, tea.Cmd) {
	name, apiKey := parseAddConnectionArgs(args)
	if name == "" || apiKey == "" {
		return m, tea.Println(warnMsgStyle.Render("  ! Usage: /incidents add incidentio --name <name> --api-key <key>"))
	}
	client := m.client
	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Adding incident.io connection %q...", name))),
		func() tea.Msg {
			resp, err := client.AddConnection(&api.AddConnectionRequest{
				Connection: api.AddConnectionInput{
					Name:           name,
					ConnectionType: "CONNECTION_TYPE_INCIDENTIO",
					IncidentioConnectionInfo: &api.IncidentioConnectionInfo{
						ApiKey: apiKey,
					},
				},
			})
			if err != nil {
				return addConnectionResultMsg{err: err}
			}
			if resp.Response.ErrorMessage != "" {
				return addConnectionResultMsg{err: fmt.Errorf("%s", resp.Response.ErrorMessage)}
			}
			return addConnectionResultMsg{label: "incident.io", name: name, uuid: resp.Response.UUID}
		},
	)
}

type incidentTestResultMsg struct {
	providerType string
	created      []incidents.CreatedIncident
	err          error
}

// parseIncidentTestArgs extracts --api-key, --routing-key, --file, --run-level from args.
// Defaults: file = ~/.hawkeye/test_config, run-level = 1.
func parseIncidentTestArgs(args []string) (apiKey, routingKey, filename string, runLevel int) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--api-key":
			if i+1 < len(args) {
				i++
				apiKey = args[i]
			}
		case "--routing-key":
			if i+1 < len(args) {
				i++
				routingKey = args[i]
			}
		case "--file":
			if i+1 < len(args) {
				i++
				filename = args[i]
			}
		case "--run-level":
			if i+1 < len(args) {
				i++
				n, err := strconv.Atoi(args[i])
				if err == nil {
					runLevel = n
				}
			}
		}
	}
	if filename == "" {
		home, _ := os.UserHomeDir()
		candidate := home + "/.hawkeye/test_config"
		if _, statErr := os.Stat(candidate); statErr == nil {
			filename = candidate
		}
	}
	if runLevel == 0 {
		runLevel = 1
	}
	return
}

func (m model) cmdIncidentsTest(providerType string, args []string) (tea.Model, tea.Cmd) {
	apiKey, routingKey, filename, runLevel := parseIncidentTestArgs(args)
	if apiKey == "" && routingKey == "" {
		return m, tea.Println(warnMsgStyle.Render("  ! --api-key is required (use --routing-key for PagerDuty Events API)"))
	}
	creds := incidents.Creds{
		ApiKey:     apiKey,
		RoutingKey: routingKey,
	}
	input := incidents.IncidentInput{Count: runLevel}
	return m, tea.Sequence(
		tea.Println(statusStyle.Render(fmt.Sprintf("  ⟳ Running incident test via %s (run-level %d)...", providerType, runLevel))),
		func() tea.Msg {
			created, err := incidents.RunTest(providerType, creds, filename, input)
			return incidentTestResultMsg{providerType: providerType, created: created, err: err}
		},
	)
}

func (m model) handleIncidentTestResult(msg incidentTestResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Incident test failed: %v", msg.err)))
	}
	cmds := []tea.Cmd{
		tea.Println(""),
		tea.Println(dimStyle.Render(fmt.Sprintf("  Created %d incident(s) via %s:", len(msg.created), msg.providerType))),
		tea.Println(""),
	}
	for _, inc := range msg.created {
		cmds = append(cmds,
			tea.Println(fmt.Sprintf("  %-12s %s", dimStyle.Render("source:"), inc.SourceID)),
			tea.Println(fmt.Sprintf("  %-12s %s", dimStyle.Render("remote:"), inc.RemoteID)),
			tea.Println(fmt.Sprintf("  %-12s %s", dimStyle.Render("title:"), inc.Title)),
		)
		if inc.URL != "" {
			cmds = append(cmds, tea.Println(fmt.Sprintf("  %-12s %s", dimStyle.Render("url:"), inc.URL)))
		}
		cmds = append(cmds, tea.Println(""))
	}
	return m, tea.Sequence(cmds...)
}

func (m model) handleAddConnectionResult(msg addConnectionResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Add connection failed: %v", msg.err)))
	}
	return m, tea.Sequence(
		tea.Println(""),
		tea.Println(dimStyle.Render(fmt.Sprintf("  %s connection added:", msg.label))),
		tea.Println(""),
		tea.Println(fmt.Sprintf("  %-12s %s", dimStyle.Render("name:"), msg.name)),
		tea.Println(fmt.Sprintf("  %-12s %s", dimStyle.Render("uuid:"), msg.uuid)),
		tea.Println(""),
	)
}

func (m model) cmdIncidents(args []string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}
	if m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Run /projects first."))
	}

	pad := func(s string, w int) string {
		for len(s) < w {
			s += " "
		}
		return s
	}

	if len(args) == 0 {
		return m, tea.Sequence(
			tea.Println(""),
			tea.Println(dimStyle.Render("  /incidents subcommands:")),
			tea.Println(""),
			tea.Println("  "+pad(hintKeyStyle.Render("list"), 30)+dimStyle.Render("Show open incidents (paginated)")),
			tea.Println("  "+pad(hintKeyStyle.Render("add"), 30)+dimStyle.Render("Add an incident management connection")),
			tea.Println("  "+pad(hintKeyStyle.Render("test"), 30)+dimStyle.Render("Test incident creation")),
			tea.Println(""),
		)
	}

	if args[0] == "list" {
		return m.cmdOpenIncidentsList(args[1:])
	}

	if args[0] == "add" {
		if len(args) < 2 {
			return m, tea.Sequence(
				tea.Println(""),
				tea.Println(dimStyle.Render("  /incidents add <type>:")),
				tea.Println(""),
				tea.Println("  "+pad(hintKeyStyle.Render("add pagerduty"), 30)+dimStyle.Render("Add a PagerDuty connection (--name, --api-key)")),
				tea.Println("  "+pad(hintKeyStyle.Render("add firehydrant"), 30)+dimStyle.Render("Add a FireHydrant connection (--name, --api-key)")),
				tea.Println("  "+pad(hintKeyStyle.Render("add incidentio"), 30)+dimStyle.Render("Add an incident.io connection (--name, --api-key)")),
				tea.Println(""),
			)
		}
		switch args[1] {
		case "pagerduty":
			return m.cmdConnectionAddPagerDuty(args[2:])
		case "firehydrant":
			return m.cmdConnectionAddFirehydrant(args[2:])
		case "incidentio":
			return m.cmdConnectionAddIncidentio(args[2:])
		default:
			return m, tea.Println(warnMsgStyle.Render(fmt.Sprintf("  ! Unknown type %q. Types: pagerduty, firehydrant, incidentio", args[1])))
		}
	}

	if args[0] == "test" {
		if len(args) < 2 {
			return m, tea.Sequence(
				tea.Println(""),
				tea.Println(dimStyle.Render("  /incidents test <type>:")),
				tea.Println(""),
				tea.Println("  "+pad(hintKeyStyle.Render("test pagerduty"), 30)+dimStyle.Render("Test PagerDuty incidents (--api-key or --routing-key; --file, --run-level optional)")),
				tea.Println("  "+pad(hintKeyStyle.Render("test firehydrant"), 30)+dimStyle.Render("Test FireHydrant incidents (--api-key; --file, --run-level optional)")),
				tea.Println("  "+pad(hintKeyStyle.Render("test incidentio"), 30)+dimStyle.Render("Test incident.io incidents (--api-key; --file, --run-level optional)")),
				tea.Println(""),
			)
		}
		switch args[1] {
		case "pagerduty", "firehydrant", "incidentio":
			return m.cmdIncidentsTest(args[1], args[2:])
		default:
			return m, tea.Println(warnMsgStyle.Render(fmt.Sprintf("  ! Unknown type %q. Types: pagerduty, firehydrant, incidentio", args[1])))
		}
	}

	return m, tea.Println(warnMsgStyle.Render(fmt.Sprintf("  ! Unknown subcommand %q — try /incidents add | test", args[0])))
}

// ─── /clear ─────────────────────────────────────────────────────────────────

func (m model) cmdClear() (tea.Model, tea.Cmd) {
	return m, tea.ClearScreen
}

// ─── Investigate ────────────────────────────────────────────────────────────

func (m model) cmdInvestigate(prompt string) (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Type /login to get started."))
	}
	if m.cfg == nil || m.cfg.ProjectID == "" {
		return m, tea.Println(errorMsgStyle.Render("  ✗ No project set. Use: hawkeye set project <uuid>"))
	}

	m.mode = modeStreaming
	m.resetStreamState()
	m.streamPrompt = prompt

	return m, tea.Sequence(
		tea.Println(""),
		tea.Println(userPromptStyle.Render("  ❯ "+prompt)),
		tea.Println(""),
		tea.Println(statusStyle.Render("  ⟳ Starting investigation...")),
		startInvestigation(m.client, m.cfg.ProjectID, m.sessionID, prompt),
	)
}
