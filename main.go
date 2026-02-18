package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/neubirdai/hawkeye-cli/internal/api"
	"github.com/neubirdai/hawkeye-cli/internal/config"
	"github.com/neubirdai/hawkeye-cli/internal/display"
)

const version = "0.1.0"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	var err error

	switch args[0] {
	case "set":
		err = cmdSet(args[1:])
	case "config":
		err = cmdConfig()
	case "investigate":
		err = cmdInvestigate(args[1:])
	case "sessions":
		err = cmdSessions(args[1:])
	case "inspect":
		err = cmdInspect(args[1:])
	case "summary":
		err = cmdSummary(args[1:])
	case "prompts":
		err = cmdPrompts()
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

	cfg, err := config.Load()
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
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	display.Header("Hawkeye CLI Configuration")

	server := cfg.Server
	if server == "" {
		server = display.Dim + "(not set)" + display.Reset
	}
	display.Info("Server:", server)

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
	fmt.Println()

	return nil
}

// ─── investigate ────────────────────────────────────────────────────────────

func cmdInvestigate(args []string) error {
	var sessionUUID string
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

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	client := api.NewClient(cfg)

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

	display.Header("Investigation")
	fmt.Printf("  %sPrompt:%s  %s\n", display.Dim, display.Reset, prompt)
	fmt.Printf("  %sSession:%s %s\n", display.Dim, display.Reset, sessionUUID)
	fmt.Println()
	fmt.Println(strings.Repeat("─", 80))

	lastContentType := ""

	err = client.ProcessPromptStream(cfg.ProjectID, sessionUUID, prompt, func(resp *api.ProcessPromptResponse) {
		if resp.Error != "" {
			display.Error(resp.Error)
			return
		}
		if resp.Message == nil || resp.Message.Content == nil {
			return
		}

		ct := resp.Message.Content.ContentType
		parts := resp.Message.Content.Parts

		if ct != lastContentType && ct != "" {
			if lastContentType == "CONTENT_TYPE_CHAT_RESPONSE" ||
				lastContentType == "CONTENT_TYPE_CHAIN_OF_THOUGHT" {
				fmt.Println()
			}
			fmt.Printf("\n%s\n", display.ContentTypeLabel(ct))
			lastContentType = ct
		}

		switch ct {
		case "CONTENT_TYPE_CHAT_RESPONSE", "CONTENT_TYPE_CHAIN_OF_THOUGHT":
			for _, part := range parts {
				fmt.Print(part)
			}
		case "CONTENT_TYPE_PROGRESS_STATUS":
			for _, part := range parts {
				fmt.Printf("  %s%s%s\n", display.Yellow, part, display.Reset)
			}
		case "CONTENT_TYPE_SOURCES":
			for _, part := range parts {
				fmt.Printf("  %s•%s %s\n", display.Blue, display.Reset, part)
			}
		case "CONTENT_TYPE_FOLLOW_UP_SUGGESTIONS":
			for i, part := range parts {
				fmt.Printf("  %s%d.%s %s\n", display.Cyan, i+1, display.Reset, part)
			}
		case "CONTENT_TYPE_SESSION_NAME":
			for _, part := range parts {
				fmt.Printf("  %s\n", part)
			}
		case "CONTENT_TYPE_ERROR_MESSAGE":
			for _, part := range parts {
				display.Error(part)
			}
		case "CONTENT_TYPE_EXECUTION_TIME":
			for _, part := range parts {
				fmt.Printf("  %s%s%s\n", display.Gray, part, display.Reset)
			}
		case "CONTENT_TYPE_DONE_INDICATOR":
			// no-op
		default:
			for _, part := range parts {
				fmt.Printf("  %s\n", part)
			}
		}
	})

	fmt.Println()
	fmt.Println(strings.Repeat("─", 80))

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
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	client := api.NewClient(cfg)

	resp, err := client.SessionList(cfg.ProjectID, limit)
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
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
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye inspect <session-uuid>")
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	client := api.NewClient(cfg)
	sessionUUID := args[0]

	resp, err := client.SessionInspect(cfg.ProjectID, sessionUUID)
	if err != nil {
		return fmt.Errorf("inspecting session: %w", err)
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
					for _, line := range wrapText(cot.Description, 72) {
						fmt.Printf("      %s\n", line)
					}
				}

				if cot.Investigation != "" {
					fmt.Printf("      %sInvestigation:%s\n", display.Dim, display.Reset)
					for _, line := range wrapText(cot.Investigation, 72) {
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
			for _, line := range wrapText(pc.FinalAnswer, 76) {
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
	if len(args) == 0 {
		fmt.Println("Usage: hawkeye summary <session-uuid>")
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.ValidateProject(); err != nil {
		return err
	}

	client := api.NewClient(cfg)
	sessionUUID := args[0]

	resp, err := client.GetSessionSummary(cfg.ProjectID, sessionUUID)
	if err != nil {
		return fmt.Errorf("getting summary: %w", err)
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
			for _, line := range wrapText(summary.ShortSummary.Question, 76) {
				fmt.Printf("    %s\n", line)
			}
		}
		if summary.ShortSummary.Analysis != "" {
			fmt.Printf("\n  %sQuick Analysis:%s\n", display.Dim, display.Reset)
			for _, line := range wrapText(summary.ShortSummary.Analysis, 76) {
				fmt.Printf("    %s\n", line)
			}
		}
	}

	if summary.Analysis != "" {
		fmt.Printf("\n  %s📋 Full Analysis:%s\n", display.Green, display.Reset)
		for _, line := range wrapText(summary.Analysis, 76) {
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

// ─── prompts ────────────────────────────────────────────────────────────────

func cmdPrompts() error {
	cfg, err := config.Load()
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

// ─── helpers ────────────────────────────────────────────────────────────────

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
  hawkeye <command> [arguments]

%sSetup:%s
  set server <url>          Set the Hawkeye server URL
  set project <uuid>        Set the active project UUID
  set token <jwt>           Set the authentication token
  set org <uuid>            Set the organization UUID
  config                    Show current configuration

%sInvestigation:%s
  investigate "<question>"  Run an AI-powered investigation (streams output)
    -s, --session <uuid>    Continue in an existing session

%sSessions:%s
  sessions                  List recent investigation sessions
    -n, --limit <count>     Number of sessions to list (default: 20)
  inspect <session-uuid>    View session details with chain-of-thought
  summary <session-uuid>    Get executive summary with action items

%sLibrary:%s
  prompts                   Browse available investigation prompts

%sExamples:%s
  hawkeye set server http://hawkeye.internal:8080
  hawkeye set project abc-123-def
  hawkeye investigate "Why is the API returning 500 errors?"
  hawkeye investigate "Check DB connections" -s <session-uuid>
  hawkeye sessions
  hawkeye inspect <session-uuid>

`, display.Bold, display.Reset, version,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset)
}
