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
		err = cmdProjects(args[1:])
	case "score":
		err = cmdScore(args[1:])
	case "link":
		err = cmdLink(args[1:])
	case "report":
		err = cmdReport()
	case "connections":
		err = cmdConnections(args[1:])
	case "investigate-alert":
		err = cmdInvestigateAlert(args[1:])
	case "queries":
		err = cmdQueries(args[1:])
	case "discover":
		err = cmdDiscover(args[1:])
	case "resource-types":
		err = cmdResourceTypes(args[1:])
	case "session-report":
		err = cmdSessionReport(args[1:])
	case "instructions":
		err = cmdInstructions(args[1:])
	case "rerun":
		err = cmdRerun(args[1:])
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

func cmdProjects(args []string) error {
	// Subcommand dispatch
	if len(args) > 0 {
		switch args[0] {
		case "info":
			return cmdProjectInfo(args[1:])
		case "create":
			return cmdProjectCreate(args[1:])
		case "update":
			return cmdProjectUpdate(args[1:])
		case "delete":
			return cmdProjectDelete(args[1:])
		}
	}

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

func cmdProjectInfo(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye projects info <uuid>")
		return nil
	}

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	client := api.NewClient(cfg)
	resp, err := client.GetProject(args[0])
	if err != nil {
		return fmt.Errorf("getting project: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Spec)
	}

	p := service.FormatProjectDetail(resp.Spec)
	display.Header(fmt.Sprintf("Project: %s", p.Name))
	display.Info("UUID:", p.UUID)
	if p.Description != "" {
		display.Info("Description:", p.Description)
	}
	ready := display.Green + "ready" + display.Reset
	if !p.Ready {
		ready = display.Yellow + "not ready" + display.Reset
	}
	display.Info("Status:", ready)
	if p.CreateTime != "" {
		display.Info("Created:", p.CreateTime)
	}
	if p.UpdateTime != "" {
		display.Info("Updated:", p.UpdateTime)
	}
	fmt.Println()
	return nil
}

func cmdProjectCreate(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye projects create <name> [--description <text>]")
		return nil
	}

	var description string
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--description", "-d":
			if i+1 < len(args) {
				i++
				description = args[i]
			} else {
				return fmt.Errorf("--description requires a value")
			}
		default:
			positional = append(positional, args[i])
		}
	}

	name := strings.Join(positional, " ")
	if name == "" {
		fmt.Println("Usage: hawkeye projects create <name> [--description <text>]")
		return nil
	}

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	client := api.NewClient(cfg)
	resp, err := client.CreateProject(name, description)
	if err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Spec)
	}

	if resp.Spec != nil {
		display.Success(fmt.Sprintf("Project created: %s (%s)", resp.Spec.Name, resp.Spec.UUID))
	} else {
		display.Success("Project created")
	}
	return nil
}

func cmdProjectUpdate(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye projects update <uuid> [--name <name>] [--description <text>]")
		return nil
	}

	projectUUID := args[0]
	var name, description string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 < len(args) {
				i++
				name = args[i]
			} else {
				return fmt.Errorf("--name requires a value")
			}
		case "--description", "-d":
			if i+1 < len(args) {
				i++
				description = args[i]
			} else {
				return fmt.Errorf("--description requires a value")
			}
		}
	}

	if name == "" && description == "" {
		fmt.Println("Usage: hawkeye projects update <uuid> [--name <name>] [--description <text>]")
		return nil
	}

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	client := api.NewClient(cfg)
	resp, err := client.UpdateProject(projectUUID, name, description)
	if err != nil {
		return fmt.Errorf("updating project: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Spec)
	}

	display.Success(fmt.Sprintf("Project %s updated", projectUUID))
	return nil
}

func cmdProjectDelete(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye projects delete <uuid> [--confirm]")
		return nil
	}

	projectUUID := args[0]
	confirmed := false
	for _, a := range args[1:] {
		if a == "--confirm" || a == "-y" {
			confirmed = true
		}
	}

	if !confirmed {
		fmt.Printf("Delete project %s? This cannot be undone. Use --confirm to proceed.\n", projectUUID)
		return nil
	}

	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	client := api.NewClient(cfg)
	if err := client.DeleteProject(projectUUID); err != nil {
		return fmt.Errorf("deleting project: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]string{"deleted": projectUUID})
	}

	display.Success(fmt.Sprintf("Project %s deleted", projectUUID))
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
			fmt.Printf("    %s%s%s\n", display.Bold, it.Type, display.Reset)
			for _, pr := range it.Priorities {
				fmt.Printf("      [%s]  incidents: %-5d  investigated: %-3d  grouped: %-6s  saved: %s\n",
					pr.Priority, pr.TotalIncidents, pr.Investigated, pr.PercentGrouped, pr.AvgTimeSaved)
			}
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

	// Subcommand dispatch
	if len(args) > 0 {
		switch args[0] {
		case "resources":
			if err := cfg.ValidateProject(); err != nil {
				return err
			}
			if len(args) < 2 {
				fmt.Println("Usage: hawkeye connections resources <connection-uuid>")
				return nil
			}
			return cmdConnectionResources(cfg, args[1])
		case "types":
			return cmdConnectionTypes()
		case "info":
			if err := cfg.Validate(); err != nil {
				return err
			}
			if len(args) < 2 {
				fmt.Println("Usage: hawkeye connections info <connection-uuid>")
				return nil
			}
			return cmdConnectionInfo(cfg, args[1])
		case "create":
			if err := cfg.Validate(); err != nil {
				return err
			}
			return cmdConnectionCreate(cfg, args[1:])
		case "sync":
			if err := cfg.Validate(); err != nil {
				return err
			}
			return cmdConnectionSync(cfg, args[1:])
		case "add":
			if err := cfg.ValidateProject(); err != nil {
				return err
			}
			return cmdConnectionAdd(cfg, args[1:])
		case "remove":
			if err := cfg.ValidateProject(); err != nil {
				return err
			}
			return cmdConnectionRemove(cfg, args[1:])
		case "project":
			if err := cfg.ValidateProject(); err != nil {
				return err
			}
			return cmdConnectionProject(cfg, args[1:])
		}
	}

	if err := cfg.ValidateProject(); err != nil {
		return err
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

func cmdConnectionTypes() error {
	types := service.GetConnectionTypes()

	if jsonOutput {
		return printJSON(types)
	}

	display.Header(fmt.Sprintf("Supported Connection Types (%d)", len(types)))
	for _, ct := range types {
		fmt.Printf("  • %s%-15s%s %s%s%s\n", display.Bold, ct.Type, display.Reset,
			display.Dim, ct.Description, display.Reset)
	}
	fmt.Println()
	return nil
}

func cmdConnectionInfo(cfg *config.Config, connUUID string) error {
	client := api.NewClient(cfg)
	resp, err := client.GetConnectionInfo(connUUID)
	if err != nil {
		return fmt.Errorf("getting connection info: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Spec)
	}

	c := service.FormatConnectionDetail(resp.Spec)
	display.Header(fmt.Sprintf("Connection: %s", c.Name))
	display.Info("UUID:", c.UUID)
	display.Info("Type:", c.Type)
	display.Info("Sync:", c.SyncState)
	display.Info("Training:", c.TrainingState)
	if c.CreateTime != "" {
		display.Info("Created:", c.CreateTime)
	}
	fmt.Println()
	return nil
}

func cmdConnectionCreate(cfg *config.Config, args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: hawkeye connections create <type> <name> [--key value ...]")
		fmt.Println()
		fmt.Println("Run 'hawkeye connections types' to see supported types.")
		return nil
	}

	connType := args[0]
	connName := args[1]
	connConfig := make(map[string]string)

	for i := 2; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") && i+1 < len(args) {
			key := strings.TrimPrefix(args[i], "--")
			i++
			connConfig[key] = args[i]
		}
	}

	client := api.NewClient(cfg)
	resp, err := client.CreateConnection(connName, connType, connConfig)
	if err != nil {
		return fmt.Errorf("creating connection: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Spec)
	}

	if resp.Spec != nil {
		display.Success(fmt.Sprintf("Connection created: %s (%s)", resp.Spec.Name, resp.Spec.UUID))
	} else {
		display.Success("Connection created")
	}
	return nil
}

func cmdConnectionSync(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye connections sync <connection-uuid> [--timeout 300]")
		return nil
	}

	connUUID := args[0]
	timeout := 300

	for i := 1; i < len(args); i++ {
		if args[i] == "--timeout" && i+1 < len(args) {
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return fmt.Errorf("invalid timeout: %s", args[i])
			}
			timeout = n
		}
	}

	display.Spinner(fmt.Sprintf("Waiting for connection %s to sync (timeout: %ds)...", connUUID, timeout))

	client := api.NewClient(cfg)
	resp, err := client.WaitForConnectionSync(connUUID, timeout)
	display.ClearLine()

	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Spec)
	}

	display.Success(fmt.Sprintf("Connection %s synced", connUUID))
	return nil
}

func cmdConnectionAdd(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye connections add <connection-uuid> [--project <uuid>]")
		return nil
	}

	connUUID := args[0]
	projectUUID := cfg.ProjectID

	for i := 1; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			i++
			projectUUID = args[i]
		}
	}

	client := api.NewClient(cfg)
	if err := client.AddConnectionToProject(projectUUID, connUUID); err != nil {
		return fmt.Errorf("adding connection to project: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]string{"added": connUUID, "project": projectUUID})
	}

	display.Success(fmt.Sprintf("Connection %s added to project %s", connUUID, projectUUID))
	return nil
}

func cmdConnectionRemove(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye connections remove <connection-uuid> [--project <uuid>] [--confirm]")
		return nil
	}

	connUUID := args[0]
	projectUUID := cfg.ProjectID
	confirmed := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 < len(args) {
				i++
				projectUUID = args[i]
			}
		case "--confirm", "-y":
			confirmed = true
		}
	}

	if !confirmed {
		fmt.Printf("Remove connection %s from project %s? Use --confirm to proceed.\n", connUUID, projectUUID)
		return nil
	}

	client := api.NewClient(cfg)
	if err := client.RemoveConnectionFromProject(projectUUID, connUUID); err != nil {
		return fmt.Errorf("removing connection from project: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]string{"removed": connUUID, "project": projectUUID})
	}

	display.Success(fmt.Sprintf("Connection %s removed from project %s", connUUID, projectUUID))
	return nil
}

func cmdConnectionProject(cfg *config.Config, args []string) error {
	projectUUID := cfg.ProjectID

	for i := 0; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			i++
			projectUUID = args[i]
		}
	}

	client := api.NewClient(cfg)
	resp, err := client.ListProjectConnections(projectUUID)
	if err != nil {
		return fmt.Errorf("listing project connections: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Specs)
	}

	display.Header(fmt.Sprintf("Connections in project %s (%d)", projectUUID, len(resp.Specs)))

	if len(resp.Specs) == 0 {
		display.Warn("No connections found in this project.")
		return nil
	}

	for _, spec := range resp.Specs {
		c := service.FormatConnection(spec)
		fmt.Printf("  • %s%-20s%s  %s(%s)%s  %s\n",
			display.Bold, c.Name, display.Reset,
			display.Dim, c.Type, display.Reset, c.UUID)
	}
	fmt.Println()
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

// ─── discover ───────────────────────────────────────────────────────────────

func cmdDiscover(args []string) error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	var telemetryType, connectionType string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--telemetry-type":
			if i+1 < len(args) {
				i++
				telemetryType = args[i]
			}
		case "--connection-type":
			if i+1 < len(args) {
				i++
				connectionType = args[i]
			}
		case "--project":
			if i+1 < len(args) {
				i++
				cfg.ProjectID = args[i]
			}
		}
	}

	client := api.NewClient(cfg)
	resp, err := client.DiscoverProjectResources(cfg.ProjectID, telemetryType, connectionType)
	if err != nil {
		return fmt.Errorf("discovering resources: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Resources)
	}

	resources := service.FormatDiscoveredResources(resp.Resources)
	display.Header(fmt.Sprintf("Discovered Resources (%d)", len(resources)))

	if len(resources) == 0 {
		display.Warn("No resources found.")
		return nil
	}

	for _, r := range resources {
		fmt.Printf("  • %s%-30s%s  %s%s%s  %s(%s)%s\n",
			display.Bold, r.Name, display.Reset,
			display.Dim, r.TelemetryType, display.Reset,
			display.Dim, r.ConnectionUUID, display.Reset)
	}

	fmt.Println()
	return nil
}

// ─── resource-types ─────────────────────────────────────────────────────────

func cmdResourceTypes(args []string) error {
	var connectionType, telemetryType string

	if len(args) >= 1 {
		connectionType = args[0]
	}
	if len(args) >= 2 {
		telemetryType = args[1]
	}

	types := service.GetResourceTypes(connectionType, telemetryType)

	if jsonOutput {
		return printJSON(types)
	}

	display.Header(fmt.Sprintf("Resource Types (%d)", len(types)))

	if len(types) == 0 {
		display.Warn("No resource types found for the given parameters.")
		return nil
	}

	for _, rt := range types {
		fmt.Printf("  • %s%-25s%s %s%s%s\n",
			display.Bold, rt.Type, display.Reset,
			display.Dim, rt.Description, display.Reset)
	}

	fmt.Println()
	return nil
}

// ─── session-report ─────────────────────────────────────────────────────────

func cmdSessionReport(args []string) error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	if len(args) == 0 {
		if cfg.LastSession != "" {
			args = []string{cfg.LastSession}
		} else {
			fmt.Println("Usage: hawkeye session-report <session-uuid> [<uuid>...]")
			return nil
		}
	}

	client := api.NewClient(cfg)

	items, err := client.GetSessionReport(cfg.ProjectID, args)
	if err != nil {
		return fmt.Errorf("getting session report: %w", err)
	}

	if jsonOutput {
		return printJSON(items)
	}

	display.Header(fmt.Sprintf("Session Reports (%d)", len(items)))

	for _, item := range items {
		fmt.Printf("\n  %s%s%s\n", display.Bold, item.Prompt, display.Reset)
		if item.Summary != "" {
			fmt.Printf("  %sSummary:%s %s\n", display.Dim, display.Reset, truncate(item.Summary, 120))
		}
		if item.TimeSaved > 0 {
			fmt.Printf("  %sTime saved:%s %d min\n", display.Blue+display.Bold, display.Reset, item.TimeSaved/60)
		}
		if item.SessionLink != "" {
			fmt.Printf("  %sLink:%s %s\n", display.Dim, display.Reset, item.SessionLink)
		}
	}

	fmt.Println()
	return nil
}

// ─── investigate-alert ──────────────────────────────────────────────────────

func cmdInvestigateAlert(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye investigate-alert <alert-id> [--project <uuid>]")
		return nil
	}

	alertID := args[0]
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	projectUUID := cfg.ProjectID
	for i := 1; i < len(args); i++ {
		if args[i] == "--project" && i+1 < len(args) {
			i++
			projectUUID = args[i]
		}
	}

	client := api.NewClient(cfg)

	fmt.Println()
	display.Spinner("Creating session from alert...")
	sessResp, err := client.CreateSessionFromAlert(projectUUID, alertID)
	if err != nil {
		display.ClearLine()
		return fmt.Errorf("creating session from alert: %w", err)
	}
	display.ClearLine()

	sessionUUID := sessResp.SessionUUID
	display.Success(fmt.Sprintf("Session created from alert: %s", sessionUUID))

	cfg.LastSession = sessionUUID
	_ = cfg.Save()

	// Auto-send a prompt to start the investigation
	prompt := fmt.Sprintf("Investigate alert %s", alertID)
	streamDisplay := api.NewStreamDisplay(false)
	err = client.ProcessPromptStream(projectUUID, sessionUUID, prompt, streamDisplay.HandleEvent)

	fmt.Println()
	if err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	display.Success("Investigation complete")
	return nil
}

// ─── queries ────────────────────────────────────────────────────────────────

func cmdQueries(args []string) error {
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
		fmt.Println("Usage: hawkeye queries [session-uuid]")
		return nil
	}

	client := api.NewClient(cfg)
	resp, err := client.GetInvestigationQueries(cfg.ProjectID, sessionUUID)
	if err != nil {
		return fmt.Errorf("getting queries: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Queries)
	}

	queries := service.FormatQueries(resp.Queries)
	display.Header(fmt.Sprintf("Investigation Queries (%d)", len(queries)))

	if len(queries) == 0 {
		display.Warn("No queries found.")
		return nil
	}

	for i, q := range queries {
		statusIcon := "✅"
		switch q.Status {
		case "FAILED", "ERROR":
			statusIcon = "❌"
		case "RUNNING", "IN_PROGRESS":
			statusIcon = "🔄"
		}

		fmt.Printf("\n  %s %sQuery %d%s  %s(%s)%s\n", statusIcon, display.Bold, i+1, display.Reset,
			display.Dim, q.Source, display.Reset)
		if q.Query != "" {
			fmt.Printf("    %s%s%s\n", display.Gray, truncate(q.Query, 100), display.Reset)
		}
		if q.ExecutionTime != "" {
			fmt.Printf("    %sTime:%s %s", display.Dim, display.Reset, q.ExecutionTime)
		}
		if q.ResultCount > 0 {
			fmt.Printf("  %sResults:%s %d", display.Dim, display.Reset, q.ResultCount)
		}
		if q.ExecutionTime != "" || q.ResultCount > 0 {
			fmt.Println()
		}
		if q.ErrorMessage != "" {
			fmt.Printf("    %sError:%s %s\n", display.Red, display.Reset, q.ErrorMessage)
		}
	}

	fmt.Println()
	return nil
}

// ─── instructions ───────────────────────────────────────────────────────────

func cmdInstructions(args []string) error {
	cfg, err := config.Load(activeProfile)
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	// Subcommand dispatch
	if len(args) > 0 {
		switch args[0] {
		case "create":
			return cmdInstructionCreate(cfg, args[1:])
		case "enable":
			return cmdInstructionToggle(cfg, args[1:], true)
		case "disable":
			return cmdInstructionToggle(cfg, args[1:], false)
		case "delete":
			return cmdInstructionDelete(cfg, args[1:])
		case "validate":
			return cmdInstructionValidate(cfg, args[1:])
		case "apply":
			return cmdInstructionApply(cfg, args[1:])
		case "info":
			// info falls through to list with filter
			if len(args) < 2 {
				fmt.Println("Usage: hawkeye instructions info <uuid>")
				return nil
			}
			return cmdInstructionInfo(cfg, args[1])
		}
	}

	// Default: list instructions
	client := api.NewClient(cfg)
	resp, err := client.ListInstructions(cfg.ProjectID)
	if err != nil {
		return fmt.Errorf("listing instructions: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Instructions)
	}

	instructions := service.FormatInstructions(resp.Instructions)
	display.Header(fmt.Sprintf("Instructions (%d)", len(instructions)))

	if len(instructions) == 0 {
		display.Warn("No instructions found.")
		return nil
	}

	for _, instr := range instructions {
		status := display.Green + "enabled" + display.Reset
		if !instr.Enabled {
			status = display.Dim + "disabled" + display.Reset
		}
		fmt.Printf("\n  %s%s%s  %s[%s]%s  %s\n",
			display.Bold, instr.Name, display.Reset,
			display.Dim, instr.Type, display.Reset, status)
		fmt.Printf("    %sUUID:%s %s\n", display.Dim, display.Reset, instr.UUID)
		if instr.Content != "" {
			fmt.Printf("    %s%s%s\n", display.Gray, truncate(instr.Content, 80), display.Reset)
		}
	}

	fmt.Println()
	return nil
}

func cmdInstructionInfo(cfg *config.Config, instrUUID string) error {
	client := api.NewClient(cfg)
	resp, err := client.ListInstructions(cfg.ProjectID)
	if err != nil {
		return fmt.Errorf("listing instructions: %w", err)
	}

	for _, s := range resp.Instructions {
		if s.UUID == instrUUID {
			if jsonOutput {
				return printJSON(s)
			}
			instr := service.FormatInstruction(s)
			display.Header(fmt.Sprintf("Instruction: %s", instr.Name))
			display.Info("UUID:", instr.UUID)
			display.Info("Type:", instr.Type)
			status := "enabled"
			if !instr.Enabled {
				status = "disabled"
			}
			display.Info("Status:", status)
			if instr.Content != "" {
				fmt.Printf("\n  %sContent:%s\n", display.Dim, display.Reset)
				fmt.Printf("  %s\n", instr.Content)
			}
			fmt.Println()
			return nil
		}
	}

	return fmt.Errorf("instruction %s not found", instrUUID)
}

func cmdInstructionCreate(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye instructions create <name> --type <filter|system|grouping|rca> --content <text>")
		return nil
	}

	var instrType, content string
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--type", "-t":
			if i+1 < len(args) {
				i++
				instrType = args[i]
			} else {
				return fmt.Errorf("--type requires a value")
			}
		case "--content", "-c":
			if i+1 < len(args) {
				i++
				content = args[i]
			} else {
				return fmt.Errorf("--content requires a value")
			}
		default:
			positional = append(positional, args[i])
		}
	}

	name := strings.Join(positional, " ")
	if name == "" || instrType == "" || content == "" {
		fmt.Println("Usage: hawkeye instructions create <name> --type <filter|system|grouping|rca> --content <text>")
		return nil
	}

	if !service.ValidInstructionType(instrType) {
		return fmt.Errorf("invalid instruction type: %s (valid: filter, system, grouping, rca)", instrType)
	}

	client := api.NewClient(cfg)
	resp, err := client.CreateInstruction(cfg.ProjectID, name, instrType, content)
	if err != nil {
		return fmt.Errorf("creating instruction: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Instruction)
	}

	if resp.Instruction != nil {
		display.Success(fmt.Sprintf("Instruction created: %s (%s)", resp.Instruction.Name, resp.Instruction.UUID))
	} else {
		display.Success("Instruction created")
	}
	return nil
}

func cmdInstructionToggle(cfg *config.Config, args []string, enable bool) error {
	if len(args) == 0 {
		action := "enable"
		if !enable {
			action = "disable"
		}
		fmt.Printf("Usage: hawkeye instructions %s <uuid>\n", action)
		return nil
	}

	client := api.NewClient(cfg)
	if err := client.UpdateInstructionStatus(args[0], enable); err != nil {
		return fmt.Errorf("updating instruction: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]any{"uuid": args[0], "enabled": enable})
	}

	action := "enabled"
	if !enable {
		action = "disabled"
	}
	display.Success(fmt.Sprintf("Instruction %s %s", args[0], action))
	return nil
}

func cmdInstructionDelete(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye instructions delete <uuid> [--confirm]")
		return nil
	}

	instrUUID := args[0]
	confirmed := false
	for _, a := range args[1:] {
		if a == "--confirm" || a == "-y" {
			confirmed = true
		}
	}

	if !confirmed {
		fmt.Printf("Delete instruction %s? Use --confirm to proceed.\n", instrUUID)
		return nil
	}

	client := api.NewClient(cfg)
	if err := client.DeleteInstruction(instrUUID); err != nil {
		return fmt.Errorf("deleting instruction: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]string{"deleted": instrUUID})
	}

	display.Success(fmt.Sprintf("Instruction %s deleted", instrUUID))
	return nil
}

func cmdInstructionValidate(cfg *config.Config, args []string) error {
	var instrType, content string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--type", "-t":
			if i+1 < len(args) {
				i++
				instrType = args[i]
			}
		case "--content", "-c":
			if i+1 < len(args) {
				i++
				content = args[i]
			}
		}
	}

	if instrType == "" || content == "" {
		fmt.Println("Usage: hawkeye instructions validate --type <type> --content <text>")
		return nil
	}

	client := api.NewClient(cfg)
	resp, err := client.ValidateInstruction(instrType, content)
	if err != nil {
		return fmt.Errorf("validating instruction: %w", err)
	}

	if jsonOutput {
		return printJSON(resp)
	}

	if resp.Instruction != nil {
		display.Success("Instruction content is valid")
		display.Info("Name:", resp.Instruction.Name)
		display.Info("Type:", resp.Instruction.Type)
		display.Info("Content:", resp.Instruction.Content)
	} else {
		display.Error("Instruction validation failed")
	}
	return nil
}

func cmdInstructionApply(cfg *config.Config, args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye instructions apply <session-uuid> --type <type> --content <text>")
		return nil
	}

	sessionUUID := args[0]
	var instrType, content string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--type", "-t":
			if i+1 < len(args) {
				i++
				instrType = args[i]
			}
		case "--content", "-c":
			if i+1 < len(args) {
				i++
				content = args[i]
			}
		}
	}

	if instrType == "" || content == "" {
		fmt.Println("Usage: hawkeye instructions apply <session-uuid> --type <type> --content <text>")
		return nil
	}

	client := api.NewClient(cfg)
	if err := client.ApplySessionInstruction(sessionUUID, instrType, content); err != nil {
		return fmt.Errorf("applying instruction: %w", err)
	}

	if jsonOutput {
		return printJSON(map[string]string{"applied": sessionUUID, "type": instrType})
	}

	display.Success(fmt.Sprintf("Instruction applied to session %s", sessionUUID))
	return nil
}

// ─── rerun ──────────────────────────────────────────────────────────────────

func cmdRerun(args []string) error {
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
		fmt.Println("Usage: hawkeye rerun <session-uuid>")
		return nil
	}

	client := api.NewClient(cfg)
	resp, err := client.RerunSession(sessionUUID)
	if err != nil {
		return fmt.Errorf("rerunning session: %w", err)
	}

	if jsonOutput {
		return printJSON(resp)
	}

	display.Success(fmt.Sprintf("Rerun started for session %s", sessionUUID))
	if resp.SessionUUID != "" && resp.SessionUUID != sessionUUID {
		display.Info("New session:", resp.SessionUUID)
	}
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
  set project <uuid>               Set the active project UUID
  config                           Show current configuration

%sProjects:%s
  projects                         List available projects
  projects info <uuid>             Get project details
  projects create <name>           Create a new project
    --description <text>           Project description
  projects update <uuid>           Update a project
    --name <name>                  New project name
    --description <text>           New description
  projects delete <uuid>           Delete a project
    --confirm                      Skip confirmation prompt

%sSettings:%s
  set server <url>          Override the server URL
  set project <uuid>        Set the active project UUID
  set token <jwt>           Manually set the auth token
  set org <uuid>            Set the organization UUID

%sInvestigation:%s
  investigate|ask "<question>"         Run an AI-powered investigation (streams output)
    -s, --session <uuid>               Continue in an existing session
  investigate-alert <alert-id>         Investigate from an alert
    --project <uuid>                   Override project UUID
  queries [session-uuid]               Show investigation queries
  link [session-uuid]                  Get web UI URL for a session

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
  connections                              List data source connections
  connections resources <conn-uuid>        List resources for a connection
  connections types                        List supported connection types
  connections info <conn-uuid>             Get connection details
  connections create <type> <name>         Create a connection
  connections sync <conn-uuid>             Wait for connection sync
    --timeout <seconds>                    Timeout in seconds (default: 300)
  connections add <conn-uuid>              Add connection to current project
  connections remove <conn-uuid>           Remove connection from project
    --confirm                              Skip confirmation prompt
  connections project                      List project connections

%sInstructions:%s
  instructions                     List project instructions
  instructions info <uuid>         Get instruction details
  instructions create <name>       Create an instruction
    --type <filter|system|grouping|rca>  Instruction type
    --content <text>               Instruction content
  instructions enable <uuid>       Enable an instruction
  instructions disable <uuid>      Disable an instruction
  instructions delete <uuid>       Delete an instruction
    --confirm                      Skip confirmation prompt
  instructions validate            Validate instruction content
    --type <type>                  Instruction type
    --content <text>               Content to validate
  instructions apply <session-uuid>  Apply instruction to session
    --type <type>                  Instruction type
    --content <text>               Instruction content
  rerun <session-uuid>             Rerun an investigation

%sDiscovery & Reports:%s
  discover                         Discover project resources
    --telemetry-type <type>        Filter by telemetry type (metric, log, trace)
    --connection-type <type>       Filter by connection type (aws, datadog, etc.)
  resource-types <conn> <telemetry>  List resource types (static)
  session-report <uuid> [<uuid>...]  Per-session report with time-saved metrics

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
		display.Cyan, display.Reset, // Usage
		display.Cyan, display.Reset, // Global Options
		display.Cyan, display.Reset, // Getting Started
		display.Cyan, display.Reset, // Projects
		display.Cyan, display.Reset, // Settings
		display.Cyan, display.Reset, // Investigation
		display.Cyan, display.Reset, // Sessions
		display.Cyan, display.Reset, // Analysis
		display.Cyan, display.Reset, // Connections
		display.Cyan, display.Reset, // Instructions
		display.Cyan, display.Reset, // Discovery & Reports
		display.Cyan, display.Reset, // Library
		display.Cyan, display.Reset, // Profiles
		display.Cyan, display.Reset) // Examples
}
