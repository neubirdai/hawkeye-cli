# Hawkeye CLI

Command-line client for the **Neubird Hawkeye AI SRE** platform. Run AI-powered incident investigations, stream chain-of-thought reasoning, and review session results — all from your terminal.

## Quick Start

```bash
# Build
go build -o hawkeye .

# Configure
./hawkeye set server http://your-hawkeye-server:8080
./hawkeye set project <your-project-uuid>

# Optional
./hawkeye set token <your-jwt-token>
./hawkeye set org <your-org-uuid>

# Investigate
./hawkeye investigate "Why is the API returning 500 errors?"
```

## Installation

### From Source

```bash
git clone https://github.com/neubird/hawkeye-cli.git
cd hawkeye-cli
go build -o hawkeye .

# Optional: install to PATH
sudo mv hawkeye /usr/local/bin/
```

Requires **Go 1.22+**.

## Configuration

Configuration is stored in `~/.hawkeye/config.json`.

| Command | Description |
|---|---|
| `hawkeye set server <url>` | Set the Hawkeye server URL (required) |
| `hawkeye set project <uuid>` | Set the active project UUID (required) |
| `hawkeye set token <jwt>` | Set the authentication bearer token |
| `hawkeye set org <uuid>` | Set the organization UUID |
| `hawkeye config` | Show current configuration |

## Commands

### `investigate` — Run an AI Investigation

Start a new investigation and stream the AI's analysis in real time:

```bash
# New investigation
hawkeye investigate "High latency on checkout service"

# Continue in an existing session
hawkeye investigate "What about the database connections?" --session <uuid>
```

The output streams in real time, showing:
- 🧠 **Chain-of-thought** reasoning steps
- ⟳ **Progress** status updates
- 📎 **Sources** being consulted (logs, metrics, traces, configs)
- 💬 **Response** with the AI's analysis
- 💡 **Follow-up suggestions**

### `sessions` — List Sessions

```bash
# List recent sessions
hawkeye sessions

# Limit results
hawkeye sessions -n 10
```

### `inspect` — View Session Details

Drill into a specific session to see every prompt cycle, chain-of-thought step, sources consulted, and answers:

```bash
hawkeye inspect <session-uuid>
```

### `summary` — Executive Summary

Get a concise summary with action items:

```bash
hawkeye summary <session-uuid>
```

### `prompts` — Browse Prompt Library

See pre-built investigation prompts for your project:

```bash
hawkeye prompts
```

## Example Workflow

```bash
# 1. Set up
hawkeye set server https://hawkeye.internal.company.com
hawkeye set project abc-123-def
hawkeye set token eyJhbGc...

# 2. Browse available prompts
hawkeye prompts

# 3. Run investigation
hawkeye investigate "Investigate the PagerDuty alert for high error rate on payments-api"

# 4. Review
hawkeye sessions
hawkeye inspect <session-uuid>
hawkeye summary <session-uuid>

# 5. Follow up in the same session
hawkeye investigate "Can you check the database connection pool metrics?" -s <session-uuid>
```

## Project Structure

```
hawkeye-cli/
├── main.go                      # Entry point + all commands
├── go.mod                       # Zero external dependencies
├── .gitignore
├── internal/
│   ├── api/
│   │   └── client.go            # HTTP client for Hawkeye API
│   ├── config/
│   │   └── config.go            # Persistent config (~/.hawkeye/)
│   └── display/
│       └── display.go           # Terminal formatting & colors
└── README.md
```

## Dependencies

**None.** This CLI uses only the Go standard library. No cobra, no viper, no external packages — just `net/http`, `encoding/json`, and friends. This makes it trivial to build and cross-compile.

## API Coverage

This CLI currently covers the core investigation workflow:

| API Endpoint | CLI Command |
|---|---|
| `POST /v1/inference/new_session` | `investigate` (auto-creates) |
| `POST /v1/inference/session` | `investigate` (streams response) |
| `POST /v1/inference/session/list` | `sessions` |
| `POST /v1/inference/session/inspect` | `inspect` |
| `GET  /v1/inference/session/summary/{id}` | `summary` |
| `GET  /v1/inference/prompt-library` | `prompts` |

Additional endpoints (ratings, file uploads, instructions, xray, watch, incidents, etc.) can be added as needed.

## License

MIT
