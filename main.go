package main

import (
	"fmt"
	"os"
	"strings"

	"hawkeye-cli/internal/api"
	"hawkeye-cli/internal/config"
	"hawkeye-cli/internal/display"
	"hawkeye-cli/internal/tui"
)

const version = "0.1.0"

var activeProfile string

func main() {
	args := os.Args[1:]

	// Parse global flags first (--profile)
	args = parseGlobalFlags(args)

	// No args or explicit -i → launch interactive mode
	if len(args) == 0 || args[0] == "-i" || args[0] == "--interactive" || args[0] == "interactive" {
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
		fmt.Println("  hawkeye login https://littlebird.app.neubird.ai/ -u user@company.com -p pass")
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

	fmt.Println()
	if cfg.ProjectID == "" {
		fmt.Printf("  %sNext:%s Run %shawkeye%s to start interactive mode.\n",
			display.Dim, display.Reset, display.Cyan, display.Reset)
		fmt.Printf("        Then use %s/projects%s to list and %s/set project <uuid>%s to select.\n\n",
			display.Cyan, display.Reset, display.Cyan, display.Reset)
	} else {
		fmt.Printf("  %sReady!%s Project is already set to %s.\n",
			display.Dim, display.Reset, cfg.ProjectID)
		fmt.Printf("  %sNext:%s Run %shawkeye%s to start investigating.\n\n",
			display.Dim, display.Reset, display.Cyan, display.Reset)
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

// ─── profiles ───────────────────────────────────────────────────────────────

func cmdProfiles() error {
	profiles, err := config.ListProfiles()
	if err != nil {
		return err
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

func parseGlobalFlags(args []string) []string {
	var remaining []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" {
			if i+1 < len(args) {
				i++
				activeProfile = args[i]
			}
			continue
		}
		remaining = append(remaining, args[i])
	}
	return remaining
}

// ─── usage ──────────────────────────────────────────────────────────────────

func printUsage() {
	fmt.Printf(`%sHawkeye CLI%s — Neubird AI SRE Platform (v%s)

%sUsage:%s
  hawkeye                             Launch interactive mode (default)
  hawkeye [--profile <name>] <cmd>    Run a specific command

%sSetup Commands:%s
  login <url> -u <user> -p <pass>     Authenticate with Hawkeye server
  set <key> <value>                   Set configuration (server, project, token, org)
  config                              Show current configuration
  profiles                            List all config profiles

%sInteractive Mode:%s
  Just run %shawkeye%s to enter interactive mode, then use slash commands:
    /login        Login to server
    /projects     List available projects
    /set project  Set the active project
    /sessions     List recent sessions
    /inspect      View session details
    /summary      Get session summary
    /prompts      Browse investigation prompts
    /help         Show all commands

%sExamples:%s
  hawkeye login https://myenv.app.neubird.ai/ -u admin@company.com -p secret
  hawkeye set project 66520f61-6a43-48ac-8286-a7e7cf9755c5
  hawkeye                              # Start investigating!
  hawkeye --profile staging login ...  # Use a named profile

`, display.Bold, display.Reset, version,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset,
		display.Cyan, display.Reset)
}
