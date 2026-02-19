package tui

import (
	"fmt"
	"strings"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"

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
		return m.cmdProjects()
	case "/sessions":
		return m.cmdSessions()
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
		tea.Println("  " + pad(hintKeyStyle.Render("/sessions"), 30) + dimStyle.Render("List recent sessions")),
		tea.Println("  " + pad(hintKeyStyle.Render("/inspect <uuid>"), 30) + dimStyle.Render("View session details")),
		tea.Println("  " + pad(hintKeyStyle.Render("/summary <uuid>"), 30) + dimStyle.Render("Get session summary")),
		tea.Println("  " + pad(hintKeyStyle.Render("/prompts"), 30) + dimStyle.Render("Browse investigation prompts")),
		tea.Println("  " + pad(hintKeyStyle.Render("/set project <uuid>"), 30) + dimStyle.Render("Set the active project")),
		tea.Println("  " + pad(hintKeyStyle.Render("/session <uuid>"), 30) + dimStyle.Render("Set active session for follow-ups")),
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
	if len(args) > 0 {
		m.loginURL = args[0]
		m.mode = modeLoginUser
		m.input.Placeholder = "Username / Email..."
		m.input.SetValue("")
		return m, tea.Println(dimStyle.Render(fmt.Sprintf("  Logging in to %s", m.loginURL)))
	}

	m.mode = modeLoginURL
	m.input.Placeholder = "Server URL (e.g. https://littlebird.app.neubird.ai/)..."
	m.input.SetValue("")
	return m, tea.Println(dimStyle.Render("  Enter the Hawkeye server URL:"))
}

func (m model) handleLoginURLSubmit(value string) (tea.Model, tea.Cmd) {
	m.loginURL = value
	m.mode = modeLoginUser
	m.input.Placeholder = "Username / Email..."
	m.input.SetValue("")
	return m, tea.Sequence(
		tea.Println(dimStyle.Render(fmt.Sprintf("  Server: %s", value))),
		tea.Println(dimStyle.Render("  Enter your username/email:")),
	)
}

func (m model) handleLoginUserSubmit(value string) (tea.Model, tea.Cmd) {
	m.loginUser = value
	m.mode = modeLoginPass
	m.input.Placeholder = "Password..."
	m.input.SetValue("")
	m.input.EchoCharacter = '•'
	m.input.EchoMode = textinput.EchoPassword
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
	m.input.EchoMode = textinput.EchoNormal
	m.input.SetValue("")
	m.input.Placeholder = "Authenticating..."

	serverURL := m.loginURL
	username := m.loginUser
	profile := m.profile

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Authenticating...")),
		func() tea.Msg {
			client := api.NewClientWithServer(serverURL)

			backendURL, err := api.ResolveBackendURL(serverURL)
			if err != nil {
				backendURL = strings.TrimRight(serverURL, "/")
			} else {
				client = api.NewClientWithServer(backendURL)
			}

			loginResp, err := client.Login(username, password)
			if err != nil {
				return loginResultMsg{err: fmt.Errorf("authentication failed: %w", err)}
			}

			cfg, err := config.Load(profile)
			if err != nil {
				return loginResultMsg{err: err}
			}

			cfg.Server = backendURL
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

// ─── /sessions ──────────────────────────────────────────────────────────────

type sessionsLoadedMsg struct {
	sessions []api.SessionInfo
	err      error
}

func (m model) cmdSessions() (tea.Model, tea.Cmd) {
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
			resp, err := client.SessionList(projectID, 15)
			if err != nil {
				return sessionsLoadedMsg{err: err}
			}
			return sessionsLoadedMsg{sessions: resp.Sessions}
		},
	)
}

func (m model) handleSessionsLoaded(msg sessionsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to load sessions: %v", msg.err)))
	}

	if len(msg.sessions) == 0 {
		return m, tea.Println(warnMsgStyle.Render("  ! No sessions found."))
	}

	var cmds []tea.Cmd
	cmds = append(cmds,
		tea.Println(""),
		tea.Println(dimStyle.Render(fmt.Sprintf("  Sessions (%d):", len(msg.sessions)))),
		tea.Println(""),
	)

	for _, s := range msg.sessions {
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		typeIcon := "💬"
		if s.SessionType == "SESSION_TYPE_INCIDENT" {
			typeIcon = "🚨"
		}
		pinned := ""
		if s.Pinned {
			pinned = " 📌"
		}

		cmds = append(cmds,
			tea.Println(fmt.Sprintf("  %s %s%s", typeIcon, name, pinned)),
			tea.Println(dimStyle.Render(fmt.Sprintf("    %s  %s", s.SessionUUID, s.CreateTime))),
		)
	}

	cmds = append(cmds,
		tea.Println(""),
		tea.Println(dimStyle.Render("  Tip: /inspect <uuid> to view · /session <uuid> to continue")),
		tea.Println(""),
	)

	return m, tea.Sequence(cmds...)
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
			rendered := renderMarkdown(pc.FinalAnswer, 76)
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
		rendered := renderMarkdown(summary.Analysis, 76)
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

func (m model) cmdProjects() (tea.Model, tea.Cmd) {
	if m.client == nil {
		return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
	}

	client := m.client

	return m, tea.Sequence(
		tea.Println(statusStyle.Render("  ⟳ Loading projects...")),
		func() tea.Msg {
			resp, err := client.ListProjects()
			if err != nil {
				return projectsLoadedMsg{err: err}
			}
			var projects []api.ProjectSpec
			for _, p := range resp.Specs {
				if !strings.Contains(p.Name, "SystemGlobalProject") {
					projects = append(projects, p)
				}
			}
			return projectsLoadedMsg{projects: projects}
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

	var cmds []tea.Cmd
	cmds = append(cmds,
		tea.Println(""),
		tea.Println(fmt.Sprintf("  Projects (%d):", len(msg.projects))),
		tea.Println(""),
	)

	for _, p := range msg.projects {
		ready := successMsgStyle.Render("ready")
		if !p.Ready {
			ready = warnMsgStyle.Render("not ready")
		}
		cmds = append(cmds,
			tea.Println(fmt.Sprintf("  ⏺ %s  %s", p.Name, ready)),
			tea.Println(fmt.Sprintf("    %s", p.UUID)),
		)
	}

	cmds = append(cmds,
		tea.Println(""),
		tea.Println(dimStyle.Render("  Use /set project <uuid> to select a project")),
		tea.Println(""),
	)

	return m, tea.Sequence(cmds...)
}

// ─── /set ───────────────────────────────────────────────────────────────────

func (m model) cmdSet(args []string) (tea.Model, tea.Cmd) {
	if len(args) < 2 {
		return m, tea.Sequence(
			tea.Println(""),
			tea.Println(dimStyle.Render("  Usage: /set project <uuid>")),
			tea.Println(""),
		)
	}

	key := strings.ToLower(args[0])
	value := args[1]

	switch key {
	case "project":
		if m.cfg == nil {
			return m, tea.Println(errorMsgStyle.Render("  ✗ Not logged in. Run /login first."))
		}
		m.cfg.ProjectID = value
		if err := m.cfg.Save(); err != nil {
			return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Failed to save config: %v", err)))
		}
		if m.cfg.Server != "" && m.cfg.Token != "" {
			m.client = api.NewClient(m.cfg)
		}
		return m, tea.Sequence(
			tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Project set to: %s", value))),
			tea.Println(dimStyle.Render("    You can now start investigating!")),
		)

	default:
		return m, tea.Println(errorMsgStyle.Render(fmt.Sprintf("  ✗ Unknown key: %s (valid: project)", key)))
	}
}

// ─── /session ───────────────────────────────────────────────────────────────

func (m model) cmdSetSession(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		if m.sessionID != "" {
			return m, tea.Println(dimStyle.Render(fmt.Sprintf("  Active session: %s", m.sessionID)))
		}
		return m, tea.Println(dimStyle.Render("  No active session. Start an investigation or use /session <uuid>."))
	}

	m.sessionID = args[0]
	return m, tea.Sequence(
		tea.Println(successMsgStyle.Render(fmt.Sprintf("  ✓ Session set to: %s", m.sessionID))),
		tea.Println(dimStyle.Render("    Follow-up questions will continue in this session.")),
	)
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
