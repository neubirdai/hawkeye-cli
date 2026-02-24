# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hawkeye CLI is a Go command-line client for the Neubird Hawkeye AI SRE platform. It enables AI-powered incident investigations with real-time SSE streaming, interactive session management, and chain-of-thought reasoning visualization.

## Build Commands

```bash
make build      # Compile binary (output: ./hawkeye)
make install    # Build and copy to /usr/local/bin/
make clean      # Remove built binary
make release    # Cross-compile for linux/darwin/windows (amd64+arm64)
go build .      # Quick dev build without stripped symbols
```

## Testing & Validation

```bash
make test       # Run all tests (go test ./... -count=1 -timeout 30s)
make lint       # Run golangci-lint (errcheck, govet, staticcheck, unused, ineffassign)
make check      # Run lint then test — the single command to validate everything
```

### Testing Workflow

**After ANY code change**, run `make check`. Fix all lint and test failures before considering the change done. This is the autonomous validation loop — never skip it.

### When Adding or Modifying Functions

- **Always add minimal, compact tests** for any code change. Extract testable logic into pure helper functions when possible.
- Write table-driven tests for any new pure/deterministic function
- Use stdlib `testing` + `net/http/httptest` only — zero external test dependencies
- Use `t.Setenv("HOME", t.TempDir())` for config tests (isolates from real config)
- Use `httptest.NewServer` for API client tests

### Test File Locations

| Package | Test file | What's tested |
|---------|-----------|---------------|
| `main` | `main_test.go` | `wrapText`, `truncate` |
| `internal/api` | `client_test.go` | `IsDeltaTrue`, `newUUID`, `doJSON`, `setHeaders`, `Login`, `ProcessPromptStream`, `GetIncidentReport`, `ListConnections`, `ListConnectionResources` |
| `internal/api` | `stream_display_test.go` | All pure functions: progress, COT formatting, source labels |
| `internal/config` | `config_test.go` | `Validate`, `ValidateProject`, `Load`/`Save` round-trip |
| `internal/display` | `display_test.go` | Label functions, `FormatTime` |
| `internal/service` | `*_test.go` (7 files) | Pure business logic: session filters, scores, reports, links, connections, projects, stream helpers |
| `internal/tui` | `model_test.go` | Model state transitions, command dispatch, slash command routing (uses mock `HawkeyeAPI`) |
| `internal/tui` | `stream_test.go` | `parseSourceLabel`, `parseCOTFields`, `findActiveCOTPart`, `isTrivialContent` |

## Architecture

**Zero external dependencies** — the entire CLI uses only Go standard library (go 1.21+). No CLI framework (cobra, etc.).

### Module Layout

- **`main.go`** — Entry point, manual argument parsing, 16 command handlers (`login`, `set`, `config`, `investigate`, `sessions`, `inspect`, `summary`, `prompts`, `projects`, `feedback`, `score`, `link`, `report`, `connections`, `profiles`, `help`). Contains interactive prompts and text formatting helpers.
- **`internal/api/client.go`** — HTTP client (`Client` struct), all API endpoint methods, SSE stream parser. Login uses multi-endpoint fallback (4 paths). Streaming uses `bufio.Scanner` with 1MB buffer and no timeout (investigations can run 30+ min).
- **`internal/api/iface.go`** — `HawkeyeAPI` interface extracted from `Client` for testability. The TUI uses this interface to allow mock injection in tests.
- **`internal/api/stream_display.go`** — `StreamDisplay` state machine for real-time terminal output. Handles event deduplication, background spinner (goroutine + mutex), chain-of-thought round tracking, delta-aware text accumulation, and source formatting. This is the most complex module (~950 lines).
- **`internal/config/config.go`** — Reads/writes `~/.hawkeye/config.json` (0600 permissions). Stores server URL, JWT token, org UUID, project UUID. Supports named profiles via `--profile`.
- **`internal/display/display.go`** — ANSI color constants and terminal formatting helpers (headers, status labels, spinners).
- **`internal/service/`** — Pure business logic layer (no I/O). Contains formatters and transformers for sessions, scores, reports, links, connections, projects, and stream helpers. Both CLI and TUI share this layer.
- **`internal/tui/`** — Bubble Tea interactive TUI. `model.go` (state machine + slash command registry), `commands.go` (async command handlers), `stream.go` (streaming display adapter).
- **`internal/api/markdown.go`** — Stub markdown processor (currently passthrough).

### Key Data Flow

Investigation command: `Config.ValidateProject()` → `Client.NewSession()` → `Client.ProcessPromptStream()` (SSE) → `StreamDisplay.HandleEvent()` per event → terminal output.

The SSE parser handles both direct JSON and gRPC-gateway envelope formats. `StreamDisplay` tracks state across event types: progress → chain-of-thought steps → sources → chat response → follow-ups.

### API Pattern

All API calls go through `Client.doJSON()` which handles JSON marshaling and Bearer token auth via `setHeaders()`. Streaming is separate — raw `net/http` response body read line-by-line as SSE.
