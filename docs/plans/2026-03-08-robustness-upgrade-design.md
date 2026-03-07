# Atlas-Recon Robustness Upgrade — Design

Date: 2026-03-08
Status: Approved

---

## Overview

Two-batch upgrade to improve correctness, robustness, and UX without breaking any existing user-facing interfaces.

- **Batch 1**: Foundational rewrites — cobra CLI migration, TUI streaming output
- **Batch 2**: Targeted fixes — 10 smaller correctness and quality improvements

---

## Batch 1: Foundational Rewrites

### 1A. Cobra CLI Migration

**Goal**: Replace the hand-rolled arg parser in `internal/cli/cli.go` with `cobra`, eliminating fragile manual loops and workarounds.

**Constraints**:
- Zero breaking changes — all existing command names, flags, and output formats stay identical
- `main.go` is untouched — `cli.Execute(os.Args) int` signature preserved
- `runPluginHelper()` helper stays unchanged; cobra-parsed values are passed into it

**Changes**:
- Root `*cobra.Command` holds `--config` / `-c` and `--json` as persistent global flags
- Each of the 15 subcommands becomes its own `*cobra.Command` with `RunE`
- `--ports` on `scan` becomes a `StringVar` cobra flag
- `--limit` and `--command` on `results` become proper cobra flags
- Delete `parseArgs()`, `extractConfigPath()`, and the `hasAnyFlag()` hand-rolled parsers
- `runOSINT` double-parsing workaround is removed — cobra flag detection replaces the `hasAnyFlag` check
- Auto-generated `--help` per subcommand via cobra

**Files affected**: `internal/cli/cli.go` (full rewrite)
**Dependencies added**: `github.com/spf13/cobra`

---

### 1B. TUI Streaming Output (Selected Job Only)

**Goal**: Show live output in the viewport while the selected job is running, instead of waiting until completion.

**Approach**: Goroutine per job pipes stdout line-by-line into the Bubble Tea event loop as `streamLineMsg` messages. Only the selected job's stream updates the viewport; background jobs accumulate lines silently.

**Changes**:

`internal/runner/python.go`:
- Add `LineCallback func(string)` to `RunOptions`
- When `LineCallback` is set, pipe stdout through `bufio.Scanner` in a goroutine, calling the callback per line instead of buffering into `stdoutBuf`
- The existing `Stream: false` buffered path is preserved for CLI usage — no regression

`internal/tui/tui.go`:
- Add `streamLineMsg struct { id string; line string }` message type
- Add `Lines []string` to `uiJob` to accumulate streamed lines
- `runSelected()` creates a `LineCallback` that sends `streamLineMsg` into the Bubble Tea program via `p.Send()`; the `tea.Program` reference is stored on `model`
- `Update()` handles `streamLineMsg`: appends line to matching job's `Lines`; if that job is `m.jobCursor`, calls `updateViewportContent()`
- `updateViewportContent()` renders from `job.Lines` when available, falls back to `job.Result.Stdout` for completed jobs

---

## Batch 2: Targeted Fixes

### 2A. Remove Dead Helper Functions — `internal/scanner/scanner.go`

Delete `split()`, `trim()`, `contains()` (manual reimplementations of stdlib). Replace all call sites with `strings.Split`, `strings.TrimSpace`, `strings.Contains`.

---

### 2B. Fix Scanner Per-Port Timeout — `internal/scanner/scanner.go`

Add `ScanTimeout time.Duration` field to `Scanner` (default: 2s). `NewScanner` accepts it as a parameter. The TCP dial in `scanPort()` uses `ScanTimeout`, not the overall command timeout from config. The overall command timeout remains available for cancellation via context at the CLI level.

`cli.go` passes `ScanTimeout: 2 * time.Second` when constructing the scanner.

---

### 2C. Fix Result ID Collision — `internal/cli/cli.go`

`resultID()` switches from `time.Now().UnixNano()` to `uuid.New().String()` using `github.com/google/uuid` (already in `go.mod`).

---

### 2D. Remove Dead Config Fields — `internal/config/config.go`

Remove `Paths.Nmap` and `Paths.Nslookup` from the `Config` struct and from `Default()`. Remove from any YAML config examples. `Nmap` was replaced by the native scanner; `Nslookup` is only checked for existence via `exec.LookPath`, not configured.

---

### 2E. Pip Check Caching — `internal/runner/deps.go`

`EnsurePythonPackages` writes a stamp file at `~/.cache/ct_plugins/env/.installed_pkgs` containing a newline-separated sorted list of installed package names. On entry:
1. Read stamp file
2. If all requested packages are present in the stamp, return immediately (skip all `pip show` calls)
3. After installing any new package, rewrite the stamp file with the updated list

Stamp is invalidated (deleted) if venv is recreated.

---

### 2F. Storage Pruning — `internal/storage/store.go` + `internal/cli/cli.go`

`store.go`:
- Add `DeleteAllRecords() error`
- Add `PruneOldRecords(keepCount int) (int, error)` — deletes oldest rows beyond `keepCount`

`cli.go` (`results` command):
- Add `--clear` flag: deletes all records, prints count
- Add `--older-than <duration>` flag (e.g. `30d`, `7d`): deletes records older than duration

`config.go`:
- Add `storage.max_records` (default: 1000)
- On `Open()`, auto-prune if row count exceeds `max_records`, keeping the most recent 800

---

### 2G. API Keys via Env Var Fallback — `internal/runner/python.go` + `internal/config/config.go`

Before injecting `cfg.APIKeys[k]` into subprocess env, also check `os.Getenv("CT_API_" + strings.ToUpper(k))`. Env var takes precedence over config file value. This allows keys to be set in the environment without ever writing them to disk.

Logic lives in `RunPython` where the env is assembled.

---

### 2H. Batch / Multi-Target Support — `internal/cli/cli.go`

All plugin-backed commands gain a `--targets-file <path>` cobra flag. When set:
1. Read the file line by line, skip blank lines and `#` comments
2. Run the command once per target sequentially, reusing the same plugin
3. In plain mode: print a header per target, then its output
4. In `--json` mode: output a JSON array of `Result` objects

`runPluginHelper` is extended with a `targetsFile string` parameter. If empty, existing single-target behavior is unchanged.

---

### 2I. Fix `cycleArgs` in TUI — `internal/tui/tui.go`

Add two fields to `model`:
- `argsOptionIndex int` — tracks which `ArgsOptions` entry is selected
- `argsExtra string` — stores any free-text the user typed beyond the category option

Arrow up/down in the args panel (when `ArgsOptions` is non-empty) increment/decrement `argsOptionIndex`. The displayed value is composed as `ArgsOptions[argsOptionIndex] + " " + argsExtra`. When the user types in the args field, changes beyond the category prefix update `argsExtra`. Removes all string-splitting heuristics in the current `cycleArgs`.

---

## File Change Summary

| File | Change |
|---|---|
| `internal/cli/cli.go` | Full rewrite (cobra), uuid IDs, batch targets, results pruning flags |
| `internal/runner/python.go` | `LineCallback` in `RunOptions`, env var API key fallback |
| `internal/runner/deps.go` | Pip stamp file caching |
| `internal/scanner/scanner.go` | Delete dead helpers, add `ScanTimeout` |
| `internal/config/config.go` | Remove dead fields, add `max_records` |
| `internal/storage/store.go` | Add `DeleteAllRecords`, `PruneOldRecords`, auto-prune on open |
| `internal/tui/tui.go` | Streaming msgs, `argsOptionIndex`/`argsExtra`, `tea.Program` ref |
| `go.mod` / `go.sum` | Add `github.com/spf13/cobra` |

---

## Non-Goals

- No changes to Python plugin scripts
- No changes to the JSON output schema
- No new subcommands beyond `--targets-file` and results pruning flags
- No Web API / REST endpoints (deferred)
- No scheduled/recurring scans (deferred)
