# TUI (Bubble Tea) Testing with expect

## Why expect?

Bubble Tea requires a real PTY — it won't start without one. Approaches that fail:
- FIFO pipes (`mkfifo` + stdin redirect) — Bubble Tea detects non-TTY and won't initialize
- `script` command — "Operation not supported on socket" on macOS
- Direct stdin piping — same TTY detection issue

`/usr/bin/expect` (Tcl-based) allocates a real PTY and works reliably.

## The Autocomplete Problem

Hawkeye's TUI has slash-command autocomplete (`matchCommands()` in `model.go`). When typing `/score`, the autocomplete menu opens and **Enter selects the suggestion instead of dispatching the command**. The selection sets input to `/score ` (with trailing space) and returns without executing.

### What didn't work
1. **Fast `send "/score\r"`** — autocomplete intercepts Enter
2. **Slow send (`send -s`)** — worse, autocomplete activates on each keystroke
3. **Double-enter (`\r\r`)** — loading spinner appears but async results never display (unclear root cause, possibly tea.Sequence interaction with autocomplete state)
4. **Escape key to dismiss menu** — didn't close the autocomplete

### What works

**Trailing space before Enter**: `send "/score \r"`
- `matchCommands("/score ")` returns no matches (no command starts with "/score ")
- So `cmdMenuOpen` is false when Enter arrives, and the command dispatches directly
- This is the most reliable pattern

**Commands with arguments**: `send "/score 699614c8a603235d2b22fbfb\r"`
- Also bypasses autocomplete naturally since the full string doesn't prefix-match any command

## expect Script Template

```tcl
#!/usr/bin/expect -f
set timeout 15
log_file -noappend /tmp/test_output.log

# Launch with PTY
spawn ./hawkeye

# Wait for the prompt (the orange arrow)
expect "help"

# Dispatch a slash command (trailing space bypasses autocomplete)
send "/score \r"

# Wait for async result (API calls ~0.3s, give generous timeout)
sleep 8

# Send quit with Ctrl-C
send "\x03"
expect eof
```

## Parsing Output

Bubble Tea output is heavily ANSI-coded (colors, cursor movement, alternate screen). To read log files:

```bash
# Strip ANSI escape sequences
sed $'s/\x1b\[[0-9;]*[A-Za-z]//g' /tmp/test_output.log | sed $'s/\x1b(B//g' | tr -d '\r'
```

Key markers in stripped output:
- `Loading scores for` — async command acknowledged (spinner)
- `RCA Quality Scores` — results arrived
- `Not logged in` — auth error (expected for unauth tests)
- `Session set to:` — session auto-selected

## Testing Unauthenticated State

Use `--profile <name>` to create an isolated config with no credentials:

```tcl
spawn ./hawkeye --profile testnoauth
```

Any command should show: `Not logged in. Run /login first.`

## Verifying Tab Completion

Type `/` and wait — Bubble Tea renders the full command list. Check log for all expected command names:

```tcl
send "/"
sleep 2
# Log will contain the autocomplete menu with all slash commands
```

## Key Timing Notes

- `expect "help"` — wait for initial prompt to fully render before sending commands
- `sleep 8` after command — async tea.Cmd results need time. API calls take ~0.3s but Bubble Tea rendering + log flushing needs buffer
- `set timeout 15` — generous timeout prevents false failures on slow networks
- Avoid `send -s` (slow character send) — it triggers autocomplete keystroke-by-keystroke

## Test Checklist for New TUI Commands

1. Command dispatches correctly (trailing space pattern)
2. Loading indicator appears
3. Async result renders
4. Works without arguments (falls back to active session where applicable)
5. Shows auth error when not logged in (`--profile` isolation)
6. Appears in autocomplete menu (type `/` and verify)
