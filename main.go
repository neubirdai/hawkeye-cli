package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"
	"hawkeye-cli/internal/display"
	"hawkeye-cli/internal/service"
	"hawkeye-cli/internal/tui"
)

var version = "0.1.0"

var activeProfile string
var jsonOutput bool

func main() {
	args := os.Args[1:]

	// Parse global flags first (--profile)
	args = parseGlobalFlags(args)

	// No args → launch interactive mode (default)
	if len(args) == 0 {
		if jsonOutput {
			display.Error("--json is not supported in interactive mode")
			os.Exit(1)
		}
		if err := tui.Run(version, activeProfile); err != nil {
			display.Error(err.Error())
			os.Exit(1)
		}
		return
	}

	// Explicit -i flag also launches interactive mode
	if args[0] == "-i" || args[0] == "--interactive" || args[0] == "interactive" {
		if jsonOutput {
			display.Error("--json is not supported in interactive mode")
			os.Exit(1)
		}
		if err := tui.Run(version, activeProfile); err != nil {
			display.Error(err.Error())
			os.Exit(1)
		}
		return
	}

	var err error

	switch args[0] {
	case "login":
		err = cmdLogin(args[1:])
	case "set":
		err = cmdSet(args[1:])
	case "config":
		err = cmdConfig()
	case "investigate", "ask":
		err = cmdInvestigate(args[1:])
	case "sessions":
		err = cmdSessions(args[1:])
	case "inspect":
		err = cmdInspect(args[1:])
	case "summary":
		err = cmdSummary(args[1:])
	case "feedback", "td":
		err = cmdFeedback(args[1:])
	case "prompts":
		err = cmdPrompts()
	case "projects":
		err = cmdProjects()
	case "score":
		err = cmdScore(args[1:])
	case "link":
		err = cmdLink(args[1:])
	case "report":
		err = cmdReport()
	case "connections":
		err = cmdConnections(args[1:])
	case "profiles":
		err = cmdProfiles()
	case "help", "--help", "-h":
		printUsage()
	case "version", "--version", "-v":
		fmt.Printf("hawkeye %s\n", version)
	default:
		display.Error(fmt.Sprintf("Unknown command: %s", args[0]))
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		display.Error(err.Error())
		os.Exit(1)
	}
}

// ─── login ───────────────────────────────────────────────────────────────────

func cmdLogin(args []string) error {
	var username, password string
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-u", "--username":
			if i+1 < len(args) {
				i++
				username = args[i]
			} else {
				return fmt.Errorf("--username requires a value")
			}
		case "-p", "--password":
			if i+1 < len(args) {
				i++
				password = args[i]
			} else {
				return fmt.Errorf("--password requires a value")
			}
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		fmt.Println("Usage: hawkeye login <url> -u <username> -p <password>")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  hawkeye login https://myenv.app.neubird.ai/ -u user@company.com -p pass")
		fmt.Println("  hawkeye login http://localhost:3000 -u admin@company.com -p mypassword")
		return nil
	}

	frontendURL := positional[0]

	if username == "" {
		fmt.Print("Username/Email: ")
		fmt.Scanln(&username)
	}
	if password == "" {
		fmt.Print("Password: ")
		fmt.Scanln(&password)
	}

	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	fmt.Println()
	display.Spinner("Resolving backend from " + frontendURL + " ...")

	serverURL, err := api.ResolveBackendURL(frontendURL)
	if err != nil {
		display.ClearLine()
		display.Warn(fmt.Sprintf("Could not resolve backend from frontend: %v", err))
		display.Info("Fallback:", "using URL directly as backend")
		serverURL = strings.TrimRight(frontendURL, "/")
	} else {
		display.ClearLine()
		display.Success(fmt.Sprintf("Resolved backend: %s", serverURL))
	}

	display.Spinner("Authenticating...")

	client := api.NewClientWithServer(serverURL)
	loginResp, err := client.Login(username, password)
	if err != nil {
		display.ClearLine()
		return fmt.Errorf("authentication failed: %w", err)
	}

	display.ClearLine()
	display.Success("Authenticated successfully")

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}

	cfg.Server = serverURL
	cfg.FrontendURL = strings.TrimRight(frontendURL, "/")
	cfg.Username = username
	cfg.Token = loginResp.AccessToken

	// Auto-fetch organization UUID from user profile
	authedClient := api.NewClient(cfg)
	userInfo, userErr := authedClient.FetchUserInfo()
	if userErr != nil {
		display.Warn(fmt.Sprintf("Could not auto-detect organization: %v", userErr))
		display.Warn("You can set it manually: hawkeye set org <uuid>")
	} else if userInfo != nil && userInfo.OrgUUID != "" {
		cfg.OrgUUID = userInfo.OrgUUID
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	display.Info("Server:", serverURL)
	display.Info("User:", username)
	if cfg.OrgUUID != "" {
		display.Info("Organization:", cfg.OrgUUID)
	}

	pf := ""
	if activeProfile != "" {
		pf = " --profile " + activeProfile
	}

	fmt.Println()
	if cfg.ProjectID == "" {
		fmt.Printf("  %sNext:%s Run %shawkeye%s set project <uuid>%s to set your project.\n\n",
			display.Dim, display.Reset, display.Cyan, pf, display.Reset)
	} else {
		fmt.Printf("  %sReady!%s Project is already set to %s.\n",
			display.Dim, display.Reset, cfg.ProjectID)
		fmt.Printf("  %sNext:%s Run %shawkeye%s investigate \"<question>\"%s to start.\n\n",
			display.Dim, display.Reset, display.Cyan, pf, display.Reset)
	}

	return nil
}

// ─── set ────────────────────────────────────────────────────────────────────

func cmdSet(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: hawkeye set <key> <value>")
		fmt.Println()
		fmt.Println("Keys:")
		fmt.Println("  server   Hawkeye server URL  (e.g. http://server:8080)")
		fmt.Println("  project  Active project UUID")
		fmt.Println("  token    JWT authentication token")
		fmt.Println("  org      Organization UUID")
		return nil
	}

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}

	key, value := args[0], args[1]

	switch key {
	case "server":
		cfg.Server = value
	case "project":
		cfg.ProjectID = value
	case "token":
		cfg.Token = value
	case "org":
		cfg.OrgUUID = value
	default:
		return fmt.Errorf("unknown config key: %s (valid: server, project, token, org)", key)
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	display.Success(fmt.Sprintf("%s set to %s", key, value))
	return nil
}

// ─── config ─────────────────────────────────────────────────────────────────

func cmdConfig() error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(map[string]string{
			"profile":      config.ProfileName(activeProfile),
			"server":       cfg.Server,
			"username":     cfg.Username,
			"project":      cfg.ProjectID,
			"org":          cfg.OrgUUID,
			"last_session": cfg.LastSession,
		})
	}

	display.Header("Hawkeye CLI Configuration")

	display.Info("Profile:", config.ProfileName(activeProfile))

	server := cfg.Server
	if server == "" {
		server = display.Dim + "(not set)" + display.Reset
	}
	display.Info("Server:", server)

	username := cfg.Username
	if username == "" {
		username = display.Dim + "(not set)" + display.Reset
	}
	display.Info("User:", username)

	project := cfg.ProjectID
	if project == "" {
		project = display.Dim + "(not set)" + display.Reset
	}
	display.Info("Project:", project)

	org := cfg.OrgUUID
	if org == "" {
		org = display.Dim + "(not set)" + display.Reset
	}
	display.Info("Organization:", org)

	token := display.Dim + "(not set)" + display.Reset
	if cfg.Token != "" {
		end := 12
		if len(cfg.Token) < end {
			end = len(cfg.Token)
		}
		token = cfg.Token[:end] + "..."
	}
	display.Info("Token:", token)

	session := cfg.LastSession
	if session == "" {
		session = display.Dim + "(none)" + display.Reset
	}
	display.Info("Last Session:", session)
	fmt.Println()

	return nil
}

// ─── investigate ────────────────────────────────────────────────────────────

func cmdInvestigate(args []string) error {
	var sessionUUID string
	var debugMode bool
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-s", "--session":
			if i+1 < len(args) {
				i++
				sessionUUID = args[i]
			} else {
				return fmt.Errorf("--session requires a value")
			}
		case "--debug":
			debugMode = true
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		fmt.Println("Usage: hawkeye investigate <question> [--session <uuid>]")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println(`  hawkeye investigate "Why is the API returning 500 errors?"`)
		fmt.Println(`  hawkeye investigate "Check database latency" --session <uuid>`)
		return nil
	}
	prompt := strings.Join(positional, " ")

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	client := api.NewClient(cfg)
	client.SetDebug(debugMode)

	// Create session if needed
	if sessionUUID == "" {
		fmt.Println()
		display.Spinner("Creating new investigation session...")
		sessResp, err := client.NewSession(cfg.ProjectID)
		if err != nil {
			display.ClearLine()
			return fmt.Errorf("creating session: %w", err)
		}
		sessionUUID = sessResp.SessionUUID
		display.ClearLine()
		display.Success(fmt.Sprintf("Session created: %s", sessionUUID))
	} else {
		fmt.Println()
		display.Success(fmt.Sprintf("Continuing session: %s", sessionUUID))
	}

	cfg.LastSession = sessionUUID
	_ = cfg.Save()

	fmt.Printf("\n %s── 🦅 Hawkeye Investigation ──────────────────────────────────────────────%s\n", display.Dim, display.Reset)
	fmt.Println()
	fmt.Printf("    %sPrompt:%s   %s\n", display.Dim, display.Reset, prompt)
	fmt.Printf("    %sSession:%s  %s\n", display.Dim, display.Reset, sessionUUID)
	if consoleURL := cfg.ConsoleSessionURL(sessionUUID); consoleURL != "" {
		fmt.Printf("    %sConsole:%s  %s\n", display.Dim, display.Reset, consoleURL)
	}
	fmt.Println()
	fmt.Printf(" %s──────────────────────────────────────────────────────────────────────────%s\n", display.Dim, display.Reset)

	// Use the StreamDisplay handler — it deduplicates progress messages,
	// compresses chain-of-thought token streams, parses source JSON,
	// and strips HTML from chat responses.
	streamDisplay := api.NewStreamDisplay(debugMode)

	err = client.ProcessPromptStream(cfg.ProjectID, sessionUUID, prompt, streamDisplay.HandleEvent)

	fmt.Println()
	fmt.Printf(" %s──────────────────────────────────────────────────────────────────────────%s\n", display.Dim, display.Reset)

	if err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	display.Success("Investigation complete")
	fmt.Printf("\n  %sTip:%s Run %shawkeye inspect %s%s to review the full session.\n",
		display.Dim, display.Reset, display.Cyan, sessionUUID, display.Reset)
	fmt.Printf("  %sTip:%s Run %shawkeye summary %s%s for an executive summary.\n\n",
		display.Dim, display.Reset, display.Cyan, sessionUUID, display.Reset)

	return nil
}

// ─── sessions ───────────────────────────────────────────────────────────────

func cmdSessions(args []string) error {
	limit := 20
	var status, from, to, search string
	var uninvestigated bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--limit":
			if i+1 < len(args) {
				i++
				n, err := strconv.Atoi(args[i])
				if err != nil {
					return fmt.Errorf("invalid limit: %s", args[i])
				}
				limit = n
			}
		case "--status":
			if i+1 < len(args) {
				i++
				status = args[i]
			}
		case "--from":
			if i+1 < len(args) {
				i++
				from = args[i]
			}
		case "--to":
			if i+1 < len(args) {
				i++
				to = args[i]
			}
		case "--search":
			if i+1 < len(args) {
				i++
				search = args[i]
			}
		case "--uninvestigated":
			uninvestigated = true
		}
	}

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	client := api.NewClient(cfg)

	filters := service.BuildSessionFilters(status, from, to, search, uninvestigated)
	resp, err := client.SessionList(cfg.ProjectID, limit, filters)
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Sessions)
	}

	display.Header(fmt.Sprintf("Sessions (%d)", len(resp.Sessions)))

	if len(resp.Sessions) == 0 {
		display.Warn("No sessions found.")
		return nil
	}

	for _, s := range resp.Sessions {
		name := s.Name
		if name == "" {
			name = display.Dim + "(unnamed)" + display.Reset
		}

		pinned := ""
		if s.Pinned {
			pinned = " 📌"
		}

		typeIcon := "💬"
		if s.SessionType == "SESSION_TYPE_INCIDENT" {
			typeIcon = "🚨"
		}

		created := display.FormatTime(s.CreateTime)
		status := display.InvestigationStatusLabel(s.InvestigationStatus)

		fmt.Printf("\n  %s %s%s%s%s\n", typeIcon, display.Bold, name, display.Reset, pinned)
		fmt.Printf("    %sID:%s      %s\n", display.Dim, display.Reset, s.SessionUUID)
		fmt.Printf("    %sCreated:%s %s\n", display.Dim, display.Reset, created)
		fmt.Printf("    %sStatus:%s  %s\n", display.Dim, display.Reset, status)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("  %sTip:%s Run %shawkeye inspect <session-uuid>%s to see details.\n\n",
		display.Dim, display.Reset, display.Cyan, display.Reset)

	return nil
}

// ─── inspect ────────────────────────────────────────────────────────────────

func cmdInspect(args []string) error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	sessionUUID := ""
	if len(args) > 0 {
		sessionUUID = args[0]
	} else if cfg.LastSession != "" {
		sessionUUID = cfg.LastSession
	} else {
		fmt.Println("Usage: hawkeye inspect [session-uuid]")
		return nil
	}

	client := api.NewClient(cfg)

	resp, err := client.SessionInspect(cfg.ProjectID, sessionUUID)
	if err != nil {
		return fmt.Errorf("inspecting session: %w", err)
	}

	if jsonOutput {
		return printJSON(resp)
	}

	if resp.SessionInfo != nil {
		s := resp.SessionInfo
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		display.Header(fmt.Sprintf("Session: %s", name))
		display.Info("UUID:", s.SessionUUID)
		display.Info("Created:", display.FormatTime(s.CreateTime))
		display.Info("Updated:", display.FormatTime(s.LastUpdate))
		display.Info("Type:", s.SessionType)
		display.Info("Investigation:", display.InvestigationStatusLabel(s.InvestigationStatus))
	}

	if len(resp.PromptCycle) == 0 {
		fmt.Println()
		display.Warn("No prompt cycles found.")
		return nil
	}

	for i, pc := range resp.PromptCycle {
		fmt.Println()
		display.SubHeader(fmt.Sprintf("── Prompt Cycle %d ──", i+1))

		if pc.Request != nil && len(pc.Request.Messages) > 0 {
			for _, msg := range pc.Request.Messages {
				if msg.Content != nil && len(msg.Content.Parts) > 0 {
					fmt.Printf("  %s❯%s %s\n", display.Cyan, display.Reset,
						strings.Join(msg.Content.Parts, " "))
				}
			}
		}

		if pc.Status != "" {
			fmt.Printf("  %sStatus:%s %s\n", display.Dim, display.Reset, pc.Status)
		}

		// Chain of Thoughts
		if len(pc.ChainOfThoughts) > 0 {
			fmt.Printf("\n  %s🧠 Chain of Thought:%s\n", display.Magenta, display.Reset)
			for _, cot := range pc.ChainOfThoughts {
				status := display.CoTStatusLabel(cot.CotStatus)
				if cot.CotStatus == "" {
					status = display.CoTStatusLabel(cot.Status)
				}
				category := cot.Category
				if category == "" {
					category = "analysis"
				}

				fmt.Printf("    %s[%s]%s %s\n", display.Bold, category, display.Reset, status)

				if cot.Description != "" {
					for _, line := range strings.Split(api.RenderMarkdown(cot.Description), "\n") {
						fmt.Printf("      %s\n", line)
					}
				}

				if cot.Investigation != "" {
					fmt.Printf("      %sInvestigation:%s\n", display.Dim, display.Reset)
					for _, line := range strings.Split(api.RenderMarkdown(cot.Investigation), "\n") {
						fmt.Printf("        %s\n", line)
					}
				}

				if len(cot.Sources) > 0 {
					fmt.Printf("      %sSources:%s %s\n", display.Dim, display.Reset,
						strings.Join(cot.Sources, ", "))
				}

				if cot.ProcessingTime != "" && cot.ProcessingTime != "0" {
					fmt.Printf("      %sTime:%s %sms\n", display.Dim, display.Reset, cot.ProcessingTime)
				}
			}
		}

		// Sources
		if len(pc.Sources) > 0 {
			fmt.Printf("\n  %s📎 Sources:%s\n", display.Blue, display.Reset)
			for _, src := range pc.Sources {
				title := src.Title
				if title == "" {
					title = src.ID
				}
				cat := ""
				if src.Category != "" {
					cat = fmt.Sprintf(" %s(%s)%s", display.Dim, src.Category, display.Reset)
				}
				fmt.Printf("    • %s%s\n", title, cat)
				if src.Description != "" {
					fmt.Printf("      %s%s%s\n", display.Gray, truncate(src.Description, 100), display.Reset)
				}
			}
		}

		// Final Answer
		if pc.FinalAnswer != "" {
			fmt.Printf("\n  %s💬 Answer:%s\n", display.Green, display.Reset)
			for _, line := range strings.Split(api.RenderMarkdown(pc.FinalAnswer), "\n") {
				fmt.Printf("    %s\n", line)
			}
		}

		// Follow-ups
		if len(pc.FollowUpSuggestions) > 0 {
			fmt.Printf("\n  %s💡 Follow-up suggestions:%s\n", display.Cyan, display.Reset)
			for j, s := range pc.FollowUpSuggestions {
				fmt.Printf("    %d. %s\n", j+1, s)
			}
		}
	}

	fmt.Println()
	return nil
}

// ─── summary ────────────────────────────────────────────────────────────────

func cmdSummary(args []string) error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	sessionUUID := ""
	if len(args) > 0 {
		sessionUUID = args[0]
	} else if cfg.LastSession != "" {
		sessionUUID = cfg.LastSession
	} else {
		fmt.Println("Usage: hawkeye summary [session-uuid]")
		return nil
	}

	client := api.NewClient(cfg)

	resp, err := client.GetSessionSummary(cfg.ProjectID, sessionUUID)
	if err != nil {
		return fmt.Errorf("getting summary: %w", err)
	}

	if jsonOutput {
		return printJSON(resp)
	}

	if resp.SessionInfo != nil {
		name := resp.SessionInfo.Name
		if name == "" {
			name = sessionUUID
		}
		display.Header(fmt.Sprintf("Summary: %s", name))
	} else {
		display.Header("Session Summary")
	}

	if resp.SessionSummary == nil {
		display.Warn("No summary available yet.")
		return nil
	}

	summary := resp.SessionSummary

	if summary.ShortSummary != nil {
		if summary.ShortSummary.Question != "" {
			fmt.Printf("\n  %sQuestion:%s\n", display.Dim, display.Reset)
			for _, line := range strings.Split(api.RenderMarkdown(summary.ShortSummary.Question), "\n") {
				fmt.Printf("    %s\n", line)
			}
		}
		if summary.ShortSummary.Analysis != "" {
			fmt.Printf("\n  %sQuick Analysis:%s\n", display.Dim, display.Reset)
			for _, line := range strings.Split(api.RenderMarkdown(summary.ShortSummary.Analysis), "\n") {
				fmt.Printf("    %s\n", line)
			}
		}
	}

	if summary.Analysis != "" {
		fmt.Printf("\n  %s📋 Full Analysis:%s\n", display.Green, display.Reset)
		for _, line := range strings.Split(api.RenderMarkdown(summary.Analysis), "\n") {
			fmt.Printf("    %s\n", line)
		}
	}

	if len(summary.ActionItems) > 0 {
		fmt.Printf("\n  %s🎯 Action Items:%s\n", display.Yellow, display.Reset)
		for i, item := range summary.ActionItems {
			fmt.Printf("    %d. %s\n", i+1, item)
		}
	}

	fmt.Println()
	return nil
}

// ─── feedback ───────────────────────────────────────────────────────────────

func cmdFeedback(args []string) error {
	var reason string
	var debugMode bool
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-r", "--reason":
			if i+1 < len(args) {
				i++
				reason = args[i]
			} else {
				return fmt.Errorf("--reason requires a value")
			}
		case "--debug":
			debugMode = true
		default:
			positional = append(positional, args[i])
		}
	}

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	sessionUUID := ""
	if len(positional) > 0 {
		sessionUUID = positional[0]
	} else if cfg.LastSession != "" {
		sessionUUID = cfg.LastSession
	} else {
		fmt.Println("Usage: hawkeye feedback|td [session-uuid] [-r reason]")
		return nil
	}

	client := api.NewClient(cfg)
	client.SetDebug(debugMode)

	resp, err := client.SessionInspect(cfg.ProjectID, sessionUUID)
	if err != nil {
		return fmt.Errorf("inspecting session: %w", err)
	}

	if len(resp.PromptCycle) == 0 {
		return fmt.Errorf("no prompt cycles found in session %s", sessionUUID)
	}

	last := resp.PromptCycle[len(resp.PromptCycle)-1]
	items := []api.RatingItemID{{ItemType: "ITEM_TYPE_PROMPT_CYCLE", ItemID: last.ID}}

	if reason == "" {
		reason = "Thumbs down from CLI"
	}

	if err := client.PutRating(cfg.ProjectID, sessionUUID, items, "RATING_THUMBS_DOWN", reason); err != nil {
		return fmt.Errorf("submitting feedback: %w", err)
	}

	display.Success(fmt.Sprintf("Thumbs down submitted for session %s", sessionUUID))
	return nil
}

// ─── prompts ────────────────────────────────────────────────────────────────

func cmdPrompts() error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	client := api.NewClient(cfg)

	resp, err := client.PromptLibrary(cfg.ProjectID)
	if err != nil {
		return fmt.Errorf("getting prompt library: %w", err)
	}

	if jsonOutput {
		return printJSON(resp)
	}

	display.Header(fmt.Sprintf("Prompt Library (%d prompts)", len(resp.Items)))

	if len(resp.Items) == 0 {
		display.Warn("No prompts found in the library.")
		return nil
	}

	for i, p := range resp.Items {
		label := p.Oneliner
		if label == "" {
			label = truncate(p.Prompt, 80)
		}
		fmt.Printf("  %s%d.%s %s\n", display.Cyan, i+1, display.Reset, label)
		if p.Oneliner != "" && p.Prompt != "" && p.Prompt != p.Oneliner {
			fmt.Printf("     %s%s%s\n", display.Gray, truncate(p.Prompt, 90), display.Reset)
		}
	}

	fmt.Printf("\n  %sTip:%s Copy a prompt and run %shawkeye investigate \"<prompt>\"%s\n\n",
		display.Dim, display.Reset, display.Cyan, display.Reset)

	return nil
}

// ─── projects ───────────────────────────────────────────────────────────────

func cmdProjects() error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	client := api.NewClient(cfg)

	resp, err := client.ListProjects()
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	projects := service.FilterSystemProjects(resp.Specs)

	if jsonOutput {
		return printJSON(projects)
	}

	display.Header(fmt.Sprintf("Projects (%d)", len(projects)))

	if len(projects) == 0 {
		display.Warn("No projects found.")
		return nil
	}

	for _, p := range projects {
		ready := display.Green + "ready" + display.Reset
		if !p.Ready {
			ready = display.Yellow + "not ready" + display.Reset
		}
		fmt.Printf("  ⏺ %s%-20s%s %s%s%s  %s[%s]%s\n", display.Bold, p.Name, display.Reset, display.Dim, p.UUID, display.Reset, display.Dim, ready, display.Reset)
	}

	fmt.Println()
	fmt.Printf("  %sTip:%s Run %shawkeye set project <uuid>%s to select a project.\n\n",
		display.Dim, display.Reset, display.Cyan, display.Reset)

	return nil
}

// ─── score ──────────────────────────────────────────────────────────────────

func cmdScore(args []string) error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	sessionUUID := ""
	if len(args) > 0 {
		sessionUUID = args[0]
	} else if cfg.LastSession != "" {
		sessionUUID = cfg.LastSession
	} else {
		fmt.Println("Usage: hawkeye score [session-uuid]")
		return nil
	}

	client := api.NewClient(cfg)

	resp, err := client.GetSessionSummary(cfg.ProjectID, sessionUUID)
	if err != nil {
		return fmt.Errorf("getting summary: %w", err)
	}

	if jsonOutput {
		return printJSON(resp)
	}

	scores := service.ExtractScores(resp)
	if !scores.HasScores {
		display.Warn("No RCA scores available for this session.")
		return nil
	}

	display.Header("RCA Quality Scores")

	if scores.ScoredBy != "" {
		display.Info("Scored by:", scores.ScoredBy)
	}

	fmt.Println()
	fmt.Printf("  %s📊 Accuracy:%s     %.1f/100\n", display.Cyan, display.Reset, scores.Accuracy.Score)
	if scores.Accuracy.Summary != "" {
		fmt.Printf("     %s%s%s\n", display.Gray, scores.Accuracy.Summary, display.Reset)
	}

	fmt.Printf("  %s📊 Completeness:%s %.1f/100\n", display.Cyan, display.Reset, scores.Completeness.Score)
	if scores.Completeness.Summary != "" {
		fmt.Printf("     %s%s%s\n", display.Gray, scores.Completeness.Summary, display.Reset)
	}

	if len(scores.Qualitative.Strengths) > 0 {
		fmt.Printf("\n  %s✅ Strengths:%s\n", display.Green, display.Reset)
		for _, s := range scores.Qualitative.Strengths {
			fmt.Printf("    • %s\n", s)
		}
	}

	if len(scores.Qualitative.Improvements) > 0 {
		fmt.Printf("\n  %s💡 Improvements:%s\n", display.Yellow, display.Reset)
		for _, s := range scores.Qualitative.Improvements {
			fmt.Printf("    • %s\n", s)
		}
	}

	if scores.TimeSaved != nil {
		fmt.Printf("\n  %s⏱  Time Saved:%s\n", display.Blue, display.Reset)
		fmt.Printf("    Standard investigation: %.0f min\n", scores.TimeSaved.StandardInvestigationMin)
		fmt.Printf("    Hawkeye investigation:  %.0f min\n", scores.TimeSaved.HawkeyeInvestigationMin)
		fmt.Printf("    %sTime saved:%s            %.0f min\n",
			display.Bold, display.Reset, scores.TimeSaved.TimeSavedMinutes)
	}

	fmt.Println()
	return nil
}

// ─── link ───────────────────────────────────────────────────────────────────

func cmdLink(args []string) error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	sessionUUID := ""
	if len(args) > 0 {
		sessionUUID = args[0]
	} else if cfg.LastSession != "" {
		sessionUUID = cfg.LastSession
	} else {
		fmt.Println("Usage: hawkeye link [session-uuid]")
		return nil
	}

	url := service.BuildSessionURL(cfg.Server, cfg.ProjectID, sessionUUID)

	if jsonOutput {
		return printJSON(map[string]string{"url": url})
	}

	fmt.Println(url)
	return nil
}

// ─── report ─────────────────────────────────────────────────────────────────

func cmdReport() error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	client := api.NewClient(cfg)

	resp, err := client.GetIncidentReport()
	if err != nil {
		return fmt.Errorf("getting incident report: %w", err)
	}

	if jsonOutput {
		return printJSON(resp)
	}

	report := service.FormatReport(resp)

	display.Header("Incident Analytics Report")

	if report.Period != "" {
		display.Info("Period:", report.Period)
	}

	fmt.Println()
	fmt.Printf("  %s📈 Overview%s\n", display.Cyan, display.Reset)
	fmt.Printf("    Total incidents:      %d\n", report.TotalIncidents)
	fmt.Printf("    Total investigations: %d\n", report.TotalInvestigations)
	fmt.Printf("    Avg time saved:       %s\n", report.AvgTimeSavedMinutes)
	fmt.Printf("    Avg MTTR:             %s\n", report.AvgMTTR)
	fmt.Printf("    Noise reduction:      %s\n", report.NoiseReduction)
	fmt.Printf("    Total time saved:     %s\n", report.TotalTimeSavedHours)

	if len(report.IncidentTypes) > 0 {
		fmt.Printf("\n  %s📋 By Incident Type%s\n", display.Cyan, display.Reset)
		for _, it := range report.IncidentTypes {
			fmt.Printf("    %s%-20s%s  count: %-5d  saved: %-10s  noise: %s\n",
				display.Bold, it.Type, display.Reset, it.Count, it.AvgTimeSaved, it.NoiseReduction)
		}
	}

	fmt.Println()
	return nil
}

// ─── connections ────────────────────────────────────────────────────────────

func cmdConnections(args []string) error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	// Subcommand: "resources <conn-uuid>"
	if len(args) > 0 && args[0] == "resources" {
		if len(args) < 2 {
			fmt.Println("Usage: hawkeye connections resources <connection-uuid>")
			return nil
		}
		return cmdConnectionResources(cfg, args[1])
	}

	client := api.NewClient(cfg)

	resp, err := client.ListConnections(cfg.ProjectID)
	if err != nil {
		return fmt.Errorf("listing connections: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Specs)
	}

	display.Header(fmt.Sprintf("Connections (%d)", len(resp.Specs)))

	if len(resp.Specs) == 0 {
		display.Warn("No connections found.")
		return nil
	}

	for _, spec := range resp.Specs {
		c := service.FormatConnection(spec)
		syncIcon := "🔄"
		if c.SyncState == "SYNCED" || c.SyncState == "SYNC_STATE_SYNCED" {
			syncIcon = "✅"
		}
		fmt.Printf("\n  %s %s%s%s  %s(%s)%s\n", syncIcon, display.Bold, c.Name, display.Reset,
			display.Dim, c.Type, display.Reset)
		fmt.Printf("    %sUUID:%s  %s\n", display.Dim, display.Reset, c.UUID)
		fmt.Printf("    %sSync:%s  %s   %sTraining:%s %s\n",
			display.Dim, display.Reset, c.SyncState,
			display.Dim, display.Reset, c.TrainingState)
	}

	fmt.Println()
	fmt.Printf("  %sTip:%s Run %shawkeye connections resources <uuid>%s to list resources.\n\n",
		display.Dim, display.Reset, display.Cyan, display.Reset)

	return nil
}

func cmdConnectionResources(cfg *config.Config, connUUID string) error {
	client := api.NewClient(cfg)

	resp, err := client.ListConnectionResources(connUUID, 100)
	if err != nil {
		return fmt.Errorf("listing resources: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Specs)
	}

	resources := service.FormatResources(resp.Specs)

	display.Header(fmt.Sprintf("Resources for %s (%d)", connUUID, len(resources)))

	if len(resources) == 0 {
		display.Warn("No resources found.")
		return nil
	}

	for _, r := range resources {
		fmt.Printf("  • %s%-30s%s  %s%s%s\n",
			display.Bold, r.Name, display.Reset,
			display.Dim, r.TelemetryType, display.Reset)
	}

	fmt.Println()
	return nil
}

// ─── profiles ───────────────────────────────────────────────────────────────

func cmdProfiles() error {
	profiles, err := config.ListProfiles()
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(map[string]any{
			"profiles": profiles,
			"active":   config.ProfileName(activeProfile),
		})
	}

	display.Header(fmt.Sprintf("Profiles (%d)", len(profiles)))

	if len(profiles) == 0 {
		display.Warn("No profiles found.")
		return nil
	}

	for _, p := range profiles {
		marker := " "
		if p == config.ProfileName(activeProfile) {
			marker = display.Green + "●" + display.Reset
		}
		fmt.Printf("  %s %s\n", marker, p)
	}
	fmt.Println()

	return nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON marshal: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func wrapText(text string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		words := strings.Fields(paragraph)
		current := ""
		for _, word := range words {
			if current == "" {
				current = word
			} else if len(current)+1+len(word) <= width {
				current += " " + word
			} else {
				lines = append(lines, current)
				current = word
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}

func parseGlobalFlags(args []string) []string {
	var remaining []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			if i+1 < len(args) {
				i++
				activeProfile = args[i]
			}
		case "-j", "--json":
			jsonOutput = true
		default:
			remaining = append(remaining, args[i])
		}
	}
	return remaining
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// ─── usage ──────────────────────────────────────────────────────────────────

func printUsage() {
	fmt.Printf(`%sHawkeye CLI%s — Neubird AI SRE Platform (v%s)

%sUsage:%s
  hawkeye                                            Launch interactive mode (default)
  hawkeye [--profile <name>] [-j] <command> [args]   Run a specific command

%sGlobal Options:%s
  --profile <name>            Use a named config profile (default: unnamed)
  -j, --json                  Output results as JSON (for scripting/piping)

%sGetting Started:%s
  login <url> -u <user> -p <pass>  Authenticate (URL = frontend address)
  projects                         List available projects
  set project <uuid>               Set the active project UUID
  config                           Show current configuration

%sSettings:%s
  set server <url>          Override the server URL
  set project <uuid>        Set the active project UUID
  set token <jwt>           Manually set the auth token
  set org <uuid>            Set the organization UUID

%sInvestigation:%s
  investigate|ask "<question>"  Run an AI-powered investigation (streams output)
    -s, --session <uuid>    Continue in an existing session
  link [session-uuid]       Get web UI URL for a session

%sSessions:%s
  sessions                  List recent investigation sessions
    -n, --limit <count>     Number of sessions to list (default: 20)
    --status <status>       Filter by status (not_started, in_progress, investigated)
    --from <date>           Filter sessions created after date
    --to <date>             Filter sessions created before date
    --search <text>         Search sessions by title
    --uninvestigated        Shorthand for --status not_started
  inspect [session-uuid]    View session details (defaults to last session)
  summary [session-uuid]    Get executive summary (defaults to last session)
  feedback|td [session-uuid]  Thumbs down feedback (defaults to last session)
    -r, --reason <text>     Reason for negative feedback

%sAnalysis:%s
  score [session-uuid]      Show RCA quality scores
  report                    Show org-wide incident analytics

%sConnections:%s
  connections [list]                  List data source connections
  connections resources <conn-uuid>   List resources for a connection

%sLibrary:%s
  prompts                   Browse available investigation prompts

%sProfiles:%s
  profiles                    List all config profiles

%sExamples:%s
  hawkeye                                            # Start interactive mode
  hawkeye login https://myenv.app.neubird.ai/ -u admin@company.com -p secret
  hawkeye set project 66520f61-6a43-48ac-8286-a7e7cf9755c5
  hawkeye investigate "Why is the API returning 500 errors?"
  hawkeye investigate "Check DB connections" -s <session-uuid>
  hawkeye sessions --uninvestigated
  hawkeye sessions --status investigated --from 2025-01-01
  hawkeye score <session-uuid>
  hawkeye link <session-uuid>
  hawkeye report
  hawkeye connections
  hawkeye connections resources <conn-uuid>
  hawkeye inspect <session-uuid>
  hawkeye --profile staging login https://myenv.app.neubird.ai/ -u user -p pass

`, display.Bold, display.Reset, version,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset)
}
