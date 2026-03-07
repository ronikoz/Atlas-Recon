# Atlas-Recon Robustness Upgrade — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade Atlas-Recon in two batches: cobra CLI migration + TUI streaming (Batch 1), then 10 targeted correctness and quality fixes (Batch 2).

**Architecture:** Batch 1 rewrites the CLI layer around cobra and adds per-line streaming from Python subprocesses into the Bubble Tea event loop. Batch 2 is a series of isolated, low-risk fixes across scanner, runner, config, storage, and TUI.

**Tech Stack:** Go 1.24, cobra, Bubble Tea, modernc/sqlite, google/uuid (already in go.mod)

**Run all tests with:** `go test ./...`

---

## BATCH 1A: Cobra CLI Migration

### Task 1: Add cobra dependency

**Files:**
- Modify: `go.mod`, `go.sum`

**Step 1: Add cobra**

```bash
cd /Users/ronikoz/projects/Atlas-Recon
go get github.com/spf13/cobra@latest
```

Expected: cobra added to `go.mod`, `go.sum` updated.

**Step 2: Verify build still compiles**

```bash
go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add github.com/spf13/cobra dependency"
```

---

### Task 2: Rewrite cli.go with cobra — root command and globals

**Files:**
- Modify: `internal/cli/cli.go`

The entire file is rewritten. Replace its content with the cobra-based version in stages. Start with the skeleton — root command, global vars, `Execute()`.

**Step 1: Replace the top of cli.go**

Rewrite `internal/cli/cli.go`. The new file starts with:

```go
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ronikoz/atlas-recon/internal/config"
	"github.com/ronikoz/atlas-recon/internal/plugins"
	"github.com/ronikoz/atlas-recon/internal/runner"
	"github.com/ronikoz/atlas-recon/internal/scanner"
	"github.com/ronikoz/atlas-recon/internal/storage"
	"github.com/ronikoz/atlas-recon/internal/tui"
	"github.com/spf13/cobra"
)

var (
	cfg         config.Config
	resultStore *storage.Store
	cfgPath     string
	jsonOut     bool
)

func Execute(argv []string) int {
	root := buildRoot()
	root.SetArgs(argv[1:])
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "ct",
		Short:         "Atlas-Recon: multi-command security toolkit",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			cfg, err = config.Load(cfgPath)
			if err != nil {
				displayCfg := cfgPath
				if displayCfg == "" {
					displayCfg = "(default)"
				}
				return fmt.Errorf("config load failed (%s): %w", displayCfg, err)
			}
			if jsonOut {
				cfg.Output.JSON = true
			}
			initResultStore()
			return nil
		},
	}

	root.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "path to config file")
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "output results as JSON")

	root.AddCommand(
		scanCmd(),
		dnsCmd(),
		osintCmd(),
		reconCmd(),
		webCmd(),
		reportCmd(),
		dashboardCmd(),
		resultsCmd(),
		phoneCmd(),
		geoCmd(),
		conflictCmd(),
		marketsCmd(),
		socialCmd(),
		flightCmd(),
		warCmd(),
	)

	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	// Close result store on exit
	cobra.OnFinalize(func() {
		if resultStore != nil {
			_ = resultStore.Close()
		}
	})

	return root
}
```

**Step 2: Add all subcommand constructors to cli.go**

Append these after `buildRoot`:

```go
func scanCmd() *cobra.Command {
	var portSpec string
	cmd := &cobra.Command{
		Use:   "scan <target>",
		Short: "Run scanning tasks (nmap, http checks)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(args[0], portSpec)
		},
	}
	cmd.Flags().StringVar(&portSpec, "ports", "", "ports to scan: 80,443 or 1-1000")
	return cmd
}

func dnsCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "dns <domain>",
		Short: "Run DNS lookups and record gathering",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("dns", "dns_lookup.py",
				"usage: ct dns <domain>",
				args, nil,
				[]runner.Dependency{nslookupDependency()},
				targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func osintCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "osint <domain>",
		Short: "Run OSINT tasks and data enrichment",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			useSuite := hasAnyFlag(args, []string{"--category", "--source", "--list", "--suite"})
			pkgs := []string{"requests", "python-whois", "python-dateutil", "dnspython", "colorama"}
			script := "osint_domain.py"
			if useSuite {
				pkgs = []string{"requests"}
				script = "osint_suite.py"
			}
			return runPluginHelper("osint", script, "usage: ct osint <domain>", args, pkgs, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func reconCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "recon <domain>",
		Short: "Run recon tasks (subdomain discovery, crawl)",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("recon", "recon_subdomains.py", "usage: ct recon <domain>", args, nil, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func webCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "web <url>",
		Short: "Run web-specific checks",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("web", "web_check.py", "usage: ct web <url>", args, []string{"requests"}, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func reportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Generate reports from results",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("report", "generate_report.py",
				"usage: ct report [files...] [--title <text>] [--output <file>]",
				args, nil, nil, "")
		},
	}
}

func dashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Launch TUI dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(cfg)
		},
	}
}

func resultsCmd() *cobra.Command {
	var limit int
	var commandFilter string
	var clear bool
	var olderThan string
	cmd := &cobra.Command{
		Use:   "results",
		Short: "List stored command results",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResults(limit, commandFilter, clear, olderThan)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "max results to show")
	cmd.Flags().StringVar(&commandFilter, "command", "", "filter by command name")
	cmd.Flags().BoolVar(&clear, "clear", false, "delete all stored results")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "prune results older than duration (e.g. 30d, 7d)")
	return cmd
}

func phoneCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "phone <number>",
		Short: "Phone number OSINT",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("phone", "phone_osint.py", "usage: ct phone <number>", args, []string{"phonenumbers"}, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func geoCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "geo <query>",
		Short: "Geospatial reconnaissance",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("geo", "geo_recon.py", "usage: ct geo <query>", args, []string{"geopy"}, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func conflictCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "conflict <query>",
		Short: "Geopolitical conflict data (GDELT)",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("conflict", "conflict_view.py", "usage: ct conflict <query>", args, []string{"requests"}, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func marketsCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "markets <query>",
		Short: "Prediction markets (Polymarket)",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("markets", "market_sentiment.py", "usage: ct markets <query>", args, []string{"requests"}, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func socialCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "social <query>",
		Short: "Social media pulse (Bluesky)",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("social", "social_pulse.py", "usage: ct social <query>", args, []string{"requests"}, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func flightCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "flight <target>",
		Short: "Live flight radar (OpenSky)",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("flight", "flight_radar.py", "usage: ct flight <target>", args, []string{"requests", "geopy"}, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func warCmd() *cobra.Command {
	var targetsFile string
	cmd := &cobra.Command{
		Use:   "war <target>",
		Short: "War Intel Edge (ISW + Maps)",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginHelper("war", "war_intel.py", "usage: ct war <target>", args, []string{"requests", "geopy"}, nil, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}
```

**Step 3: Add the implementation functions to cli.go**

Append these:

```go
func runScan(target, portSpec string) error {
	ports, err := scanner.ParsePorts(portSpec)
	if err != nil {
		return fmt.Errorf("error parsing ports: %w", err)
	}

	timeout := time.Duration(cfg.Timeouts.CommandSeconds) * time.Second
	s := scanner.NewScanner(timeout, cfg.Concurrency)
	result := s.ScanHost(target, ports)
	storeScanResult([]string{target, "--ports", portSpec}, result)

	if cfg.Output.JSON {
		jsonOutput, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonOutput))
		return nil
	}

	fmt.Printf("Scan Results for %s\n", result.Host)
	fmt.Printf("Duration: %dms\n\n", result.EndTime.Sub(result.StartTime).Milliseconds())
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
		return nil
	}
	openPorts := 0
	for _, port := range result.Ports {
		if port.State == "open" {
			openPorts++
			fmt.Printf("Port %d: %s (%s)\n", port.Port, port.State, port.Service)
		}
	}
	fmt.Printf("\nSummary: %d open, %d closed/filtered\n", openPorts, len(result.Ports)-openPorts)
	return nil
}

func runPluginHelper(name, plugin, usage string, args []string, pkgs []string, deps []runner.Dependency, targetsFile string) error {
	if len(args) == 0 && targetsFile == "" {
		fmt.Fprintln(os.Stderr, usage)
		return nil
	}

	if len(pkgs) > 0 {
		if err := runner.EnsurePythonPackages(pkgs, cfg.Paths.Python); err != nil {
			return err
		}
	}
	if len(deps) > 0 {
		if err := runner.EnsureDependencies(deps); err != nil {
			return err
		}
	}

	targets, err := resolveTargets(args, targetsFile)
	if err != nil {
		return err
	}

	if len(targets) == 1 {
		// Single target — original behavior
		result, err := runner.RunPython(pluginPath(plugin), targets[0], runner.RunOptions{
			Stream:  !cfg.Output.JSON,
			Python:  cfg.Paths.Python,
			APIKeys: cfg.APIKeys,
		})
		result.ID = resultID(name)
		storeCommandResult(name, targets[0], result)
		if cfg.Output.JSON {
			return emitJSON(result, err)
		}
		return err
	}

	// Multi-target
	var results []runner.Result
	for _, targetArgs := range targets {
		if !cfg.Output.JSON {
			fmt.Printf("\n--- target: %s ---\n", targetArgs[0])
		}
		result, err := runner.RunPython(pluginPath(plugin), targetArgs, runner.RunOptions{
			Stream:  !cfg.Output.JSON,
			Python:  cfg.Paths.Python,
			APIKeys: cfg.APIKeys,
		})
		result.ID = resultID(name)
		storeCommandResult(name, targetArgs, result)
		if err != nil && !cfg.Output.JSON {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		results = append(results, result)
	}
	if cfg.Output.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	return nil
}

// resolveTargets returns a slice of arg lists (one per target).
// If targetsFile is set, reads targets from file. Otherwise wraps args as single entry.
func resolveTargets(args []string, targetsFile string) ([][]string, error) {
	if targetsFile == "" {
		return [][]string{args}, nil
	}
	data, err := os.ReadFile(targetsFile)
	if err != nil {
		return nil, fmt.Errorf("reading targets file: %w", err)
	}
	var targets [][]string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Combine file target with any extra flags from args (skip args[0] which is the original target)
		entry := []string{line}
		if len(args) > 1 {
			entry = append(entry, args[1:]...)
		}
		targets = append(targets, entry)
	}
	if len(targets) == 0 {
		return nil, errors.New("targets file is empty")
	}
	return targets, nil
}

func runResults(limit int, commandFilter string, clear bool, olderThan string) error {
	if resultStore == nil {
		return errors.New("results storage disabled")
	}
	if clear {
		n, err := resultStore.DeleteAllRecords()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Deleted %d records.\n", n)
		return nil
	}
	if olderThan != "" {
		dur, err := parseDuration(olderThan)
		if err != nil {
			return fmt.Errorf("invalid --older-than value: %w", err)
		}
		n, err := resultStore.PruneOldRecords(dur)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Pruned %d records older than %s.\n", n, olderThan)
		return nil
	}

	records, err := resultStore.ListRecords(storage.ListOptions{Limit: limit, Command: commandFilter})
	if err != nil {
		return err
	}
	if cfg.Output.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(records)
	}
	if len(records) == 0 {
		fmt.Fprintln(os.Stdout, "No stored results found.")
		return nil
	}
	for _, record := range records {
		fmt.Fprintf(os.Stdout, "%s  %-8s %-7s %6dms  %s\n",
			record.StartedAt.Format(time.RFC3339),
			record.Command,
			record.Status,
			record.DurationMs,
			record.ID,
		)
	}
	return nil
}

// parseDuration parses human durations like "30d", "7d", "24h"
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n := strings.TrimSuffix(s, "d")
		days := 0
		if _, err := fmt.Sscanf(n, "%d", &days); err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid days: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func pluginPath(name string) string {
	path, err := plugins.GetPluginPath(name)
	if err == nil {
		return path
	}
	return filepath.Join("plugins", "python", name)
}

func hasAnyFlag(args []string, flags []string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}
	return false
}

func emitJSON(result runner.Result, runErr error) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("exit code: %d", result.ExitCode)
	}
	return runErr
}

func initResultStore() {
	if !cfg.Storage.Enabled || cfg.Storage.ResultsDB == "" {
		return
	}
	store, err := storage.Open(cfg.Storage.ResultsDB, cfg.Storage.MaxRecords)
	if err != nil {
		fmt.Fprintf(os.Stderr, "results store unavailable: %v\n", err)
		return
	}
	resultStore = store
}

func storeCommandResult(command string, args []string, result runner.Result) {
	if resultStore == nil {
		return
	}
	record := storage.Record{
		ID:         result.ID,
		Kind:       "command",
		Command:    command,
		Args:       args,
		StartedAt:  result.StartedAt,
		FinishedAt: result.FinishedAt,
		DurationMs: result.DurationMs,
		ExitCode:   result.ExitCode,
		Status:     string(result.Status),
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		Error:      result.Error,
	}
	if err := resultStore.SaveRecord(record); err != nil {
		fmt.Fprintf(os.Stderr, "results store failed: %v\n", err)
	}
}

func storeScanResult(args []string, result *scanner.ScanResult) {
	if resultStore == nil || result == nil {
		return
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return
	}
	status := string(runner.StatusSuccess)
	exitCode := 0
	if result.Error != "" {
		status = string(runner.StatusFailed)
		exitCode = 1
	}
	record := storage.Record{
		ID:         resultID("scan"),
		Kind:       "scan",
		Command:    "scan",
		Args:       args,
		StartedAt:  result.StartTime,
		FinishedAt: result.EndTime,
		DurationMs: result.EndTime.Sub(result.StartTime).Milliseconds(),
		ExitCode:   exitCode,
		Status:     status,
		Payload:    string(payload),
	}
	if err := resultStore.SaveRecord(record); err != nil {
		fmt.Fprintf(os.Stderr, "results store failed: %v\n", err)
	}
}

func resultID(prefix string) string {
	return prefix + "-" + uuid.New().String()
}

func nslookupDependency() runner.Dependency {
	return runner.Dependency{
		Name:        "nslookup",
		CheckCmd:    "nslookup",
		Description: "DNS lookup utility",
		Installers: map[string][]runner.Installer{
			"darwin": {
				{Name: "brew", Command: []string{"brew", "install", "bind"}},
			},
			"linux": {
				{Name: "apt", Command: []string{"sudo", "apt-get", "install", "-y", "dnsutils"}},
				{Name: "dnf", Command: []string{"sudo", "dnf", "install", "-y", "bind-utils"}},
				{Name: "pacman", Command: []string{"sudo", "pacman", "-S", "--noconfirm", "bind"}},
			},
			"windows": {
				{Name: "winget", Command: []string{"winget", "install", "--id", "ISC.Bind", "-e"}},
				{Name: "choco", Command: []string{"choco", "install", "-y", "bind"}},
			},
		},
	}
}

// Signed-off-by: ronikoz
```

**Step 4: Update runner.RunPython signature**

The new `runPluginHelper` calls `runner.RunPython(pluginPath, args []string, opts)` where `args` is the full argument list. Update `RunPython` in `internal/runner/python.go` to accept `args []string` directly (it currently does — verify the signature matches).

Current signature: `RunPython(scriptPath string, args []string, opts RunOptions) (Result, error)` — this is correct, no change needed.

**Step 5: Build and verify**

```bash
go build ./...
```

Expected: no errors.

**Step 6: Run tests**

```bash
go test ./...
```

Expected: all pass.

**Step 7: Commit**

```bash
git add internal/cli/cli.go
git commit -m "feat: migrate CLI from hand-rolled parser to cobra"
```

---

### Task 3: Update config and storage for new fields expected by cli.go

The new `cli.go` calls `storage.Open(path, maxRecords)` and references `cfg.Storage.MaxRecords`. Those don't exist yet. Fix them now.

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/storage/store.go`

**Step 1: Update config.go**

In `internal/config/config.go`, update the `Storage` struct and `Default()`:

```go
Storage struct {
    Enabled    bool   `yaml:"enabled"`
    ResultsDB  string `yaml:"results_db"`
    MaxRecords int    `yaml:"max_records"`
} `yaml:"storage"`
```

In `Default()`, add:
```go
cfg.Storage.MaxRecords = 1000
```

Remove `Paths.Nmap` and `Paths.Nslookup` from the `Paths` struct and from `Default()`:
```go
Paths struct {
    Python string `yaml:"python"`
} `yaml:"paths"`
```

In `Default()`:
```go
cfg.Paths.Python = "python3"
// remove Nmap and Nslookup lines
```

**Step 2: Update config_test.go**

In `internal/config/config_test.go`, add assertion for MaxRecords and remove any Nmap/Nslookup assertions:

```go
func TestDefaultConfig(t *testing.T) {
    cfg := Default()
    if cfg.Concurrency != 4 {
        t.Errorf("expected default concurrency 4, got %d", cfg.Concurrency)
    }
    if !cfg.Storage.Enabled {
        t.Errorf("expected storage default to be enabled")
    }
    if cfg.Paths.Python != "python3" {
        t.Errorf("expected python3, got %s", cfg.Paths.Python)
    }
    if cfg.Storage.MaxRecords != 1000 {
        t.Errorf("expected default max_records 1000, got %d", cfg.Storage.MaxRecords)
    }
}
```

**Step 3: Run config tests**

```bash
go test ./internal/config/...
```

Expected: PASS.

**Step 4: Update storage.Open to accept maxRecords**

In `internal/storage/store.go`, update `Open` and `Store`:

```go
type Store struct {
    db         *sql.DB
    maxRecords int
}

func Open(path string, maxRecords int) (*Store, error) {
    if path == "" {
        return nil, errors.New("results db path is empty")
    }
    if err := ensureDir(path); err != nil {
        return nil, err
    }
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }
    if err := initSchema(db); err != nil {
        _ = db.Close()
        return nil, err
    }
    s := &Store{db: db, maxRecords: maxRecords}
    if maxRecords > 0 {
        _ = s.autoPrune()
    }
    return s, nil
}
```

**Step 5: Add autoPrune, DeleteAllRecords, PruneOldRecords to store.go**

```go
// autoPrune deletes oldest records if count exceeds maxRecords, keeping most recent 80%.
func (s *Store) autoPrune() error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    var count int
    if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM results`).Scan(&count); err != nil {
        return err
    }
    if count <= s.maxRecords {
        return nil
    }
    keep := s.maxRecords * 80 / 100
    _, err := s.db.ExecContext(ctx,
        `DELETE FROM results WHERE id NOT IN (
            SELECT id FROM results ORDER BY started_at DESC LIMIT ?
        )`, keep)
    return err
}

// DeleteAllRecords removes all stored results. Returns the number deleted.
func (s *Store) DeleteAllRecords() (int, error) {
    if s == nil || s.db == nil {
        return 0, errors.New("store not initialized")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    var count int
    if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM results`).Scan(&count); err != nil {
        return 0, err
    }
    if _, err := s.db.ExecContext(ctx, `DELETE FROM results`); err != nil {
        return 0, err
    }
    return count, nil
}

// PruneOldRecords removes records older than the given duration. Returns count deleted.
func (s *Store) PruneOldRecords(olderThan time.Duration) (int, error) {
    if s == nil || s.db == nil {
        return 0, errors.New("store not initialized")
    }
    cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339Nano)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    res, err := s.db.ExecContext(ctx, `DELETE FROM results WHERE started_at < ?`, cutoff)
    if err != nil {
        return 0, err
    }
    n, _ := res.RowsAffected()
    return int(n), nil
}
```

**Step 6: Add storage tests**

In `internal/storage/store_test.go` (create if missing):

```go
package storage

import (
    "os"
    "testing"
    "time"
)

func tmpStore(t *testing.T) *Store {
    t.Helper()
    f, err := os.CreateTemp("", "atlas-recon-test-*.db")
    if err != nil {
        t.Fatal(err)
    }
    f.Close()
    t.Cleanup(func() { os.Remove(f.Name()) })
    s, err := Open(f.Name(), 100)
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { s.Close() })
    return s
}

func TestDeleteAllRecords(t *testing.T) {
    s := tmpStore(t)
    for i := 0; i < 3; i++ {
        _ = s.SaveRecord(Record{
            ID: fmt.Sprintf("id-%d", i), Kind: "command", Command: "test",
            StartedAt: time.Now(), FinishedAt: time.Now(), Status: "success",
        })
    }
    n, err := s.DeleteAllRecords()
    if err != nil {
        t.Fatal(err)
    }
    if n != 3 {
        t.Errorf("expected 3 deleted, got %d", n)
    }
}

func TestPruneOldRecords(t *testing.T) {
    s := tmpStore(t)
    old := time.Now().Add(-48 * time.Hour)
    _ = s.SaveRecord(Record{
        ID: "old-1", Kind: "command", Command: "test",
        StartedAt: old, FinishedAt: old, Status: "success",
    })
    _ = s.SaveRecord(Record{
        ID: "new-1", Kind: "command", Command: "test",
        StartedAt: time.Now(), FinishedAt: time.Now(), Status: "success",
    })
    n, err := s.PruneOldRecords(24 * time.Hour)
    if err != nil {
        t.Fatal(err)
    }
    if n != 1 {
        t.Errorf("expected 1 pruned, got %d", n)
    }
    records, _ := s.ListRecords(ListOptions{Limit: 10})
    if len(records) != 1 || records[0].ID != "new-1" {
        t.Errorf("expected only new-1 to remain")
    }
}
```

Note: add `"fmt"` to the imports in the test file.

**Step 7: Run all tests**

```bash
go test ./...
```

Expected: all pass.

**Step 8: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go \
        internal/storage/store.go internal/storage/store_test.go
git commit -m "feat: add storage pruning, remove dead config fields, add max_records"
```

---

## BATCH 1B: TUI Streaming Output

### Task 4: Add LineCallback to RunPython

**Files:**
- Modify: `internal/runner/python.go`

**Step 1: Add LineCallback field to RunOptions**

In `internal/runner/python.go`, update `RunOptions`:

```go
type RunOptions struct {
    Stream       bool
    Python       string
    Timeout      time.Duration
    Context      context.Context
    APIKeys      map[string]string
    LineCallback func(line string) // called per stdout line when set; disables Stream
}
```

**Step 2: Update RunPython to use LineCallback**

Replace the stdout/stderr setup block in `RunPython`:

```go
var stdoutBuf bytes.Buffer
var stderrBuf bytes.Buffer

if opts.LineCallback != nil {
    // Stream stdout line-by-line via callback; capture stderr normally
    pr, pw, err := os.Pipe()
    if err != nil {
        return Result{}, fmt.Errorf("pipe error: %w", err)
    }
    cmd.Stdout = pw
    cmd.Stderr = &stderrBuf

    started := time.Now()
    if err := cmd.Start(); err != nil {
        pw.Close()
        pr.Close()
        return Result{}, fmt.Errorf("python runner start failed: %w", err)
    }
    pw.Close()

    scanner := bufio.NewScanner(pr)
    for scanner.Scan() {
        line := scanner.Text()
        stdoutBuf.WriteString(line + "\n")
        opts.LineCallback(line)
    }
    pr.Close()

    runErr := cmd.Wait()
    finished := time.Now()

    result := Result{
        Command:    python,
        Args:       append([]string{scriptPath}, args...),
        StartedAt:  started,
        FinishedAt: finished,
        DurationMs: finished.Sub(started).Milliseconds(),
        ExitCode:   exitCode(runErr),
        Stdout:     stdoutBuf.String(),
        Stderr:     stderrBuf.String(),
    }
    if runErr != nil {
        result.Status = StatusFailed
        result.Error = runErr.Error()
        return result, fmt.Errorf("python runner failed: %w", runErr)
    }
    result.Status = StatusSuccess
    return result, nil
} else if opts.Stream {
    cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
    cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
} else {
    cmd.Stdout = &stdoutBuf
    cmd.Stderr = &stderrBuf
}
```

Also add `"bufio"` and `"os"` to imports (both are already imported — verify).

The existing `started := time.Now()` / `err := cmd.Run()` / result assembly block at the bottom now only runs for the non-LineCallback paths. Restructure so it is in an else branch (or just falls through after the if block for LineCallback).

**Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

**Step 4: Commit**

```bash
git add internal/runner/python.go
git commit -m "feat: add LineCallback streaming to RunPython"
```

---

### Task 5: Add streaming messages and state to TUI model

**Files:**
- Modify: `internal/tui/tui.go`

**Step 1: Add new message types and update uiJob**

At the top of `tui.go`, add:

```go
type streamLineMsg struct {
    id   string
    line string
}

type streamDoneMsg struct {
    id string
}
```

Update `uiJob`:
```go
type uiJob struct {
    ID     string
    Title  string
    Status string
    Result runner.Result
    Cancel context.CancelFunc
    Lines  []string  // streamed output lines (populated during run)
}
```

Add `program *tea.Program` to `model`:
```go
type model struct {
    // ... existing fields ...
    program *tea.Program
}
```

**Step 2: Store program reference in Run()**

Update `Run()` in tui.go:

```go
func Run(cfg config.Config) error {
    ctx, cancel := context.WithCancel(context.Background())
    q := runner.NewQueue(cfg.Concurrency)
    q.Start(ctx)

    m := newModel(cfg, q, cancel)
    p := tea.NewProgram(m, tea.WithAltScreen())
    m.program = p  // store before running
    _, err := p.Run()
    return err
}
```

**Step 3: Handle streamLineMsg in Update()**

In the `Update()` method, add a case before the existing cases:

```go
case streamLineMsg:
    for i := range m.jobs {
        if m.jobs[i].ID == msg.id {
            m.jobs[i].Lines = append(m.jobs[i].Lines, msg.line)
            if i == m.jobCursor {
                m.updateViewportContent()
            }
            break
        }
    }
    return m, nil
```

**Step 4: Update updateViewportContent to use Lines**

Replace `updateViewportContent`:

```go
func (m *model) updateViewportContent() {
    if len(m.jobs) == 0 || m.jobCursor >= len(m.jobs) {
        m.viewport.SetContent("Select a job to view output.")
        return
    }
    job := m.jobs[m.jobCursor]
    content := fmt.Sprintf("ID: %s\nStatus: %s\nDuration: %dms\n\n",
        job.ID, job.Status, job.Result.DurationMs)
    if job.Result.Error != "" {
        content += fmt.Sprintf("Error:\n%s\n\n", job.Result.Error)
    }
    // Prefer streamed lines if available, fall back to buffered stdout
    if len(job.Lines) > 0 {
        content += strings.Join(job.Lines, "\n") + "\n"
    } else if job.Result.Stdout != "" {
        content += job.Result.Stdout + "\n"
    }
    if job.Result.Stderr != "" {
        content += fmt.Sprintf("STDERR:\n%s\n", job.Result.Stderr)
    }
    m.viewport.SetContent(content)
}
```

**Step 5: Update runSelected to use LineCallback**

In `runSelected()`, replace the `runner.Job` construction:

```go
prog := m.program  // capture for goroutine

if err := m.queue.Submit(runner.Job{
    ID:      id,
    Command: cmdDef.Name,
    Args:    argList,
    Run: func(_ context.Context) (runner.Result, error) {
        result, err := runner.RunPython(script, argList, runner.RunOptions{
            Stream:  false,
            Python:  m.cfg.Paths.Python,
            Timeout: time.Duration(m.cfg.Timeouts.CommandSeconds) * time.Second,
            Context: ctx,
            APIKeys: m.cfg.APIKeys,
            LineCallback: func(line string) {
                if prog != nil {
                    prog.Send(streamLineMsg{id: id, line: line})
                }
            },
        })
        result.ID = id
        return result, err
    },
}); err != nil {
    m.statusMessage = "job queue is stopped"
    return m, nil
}
```

**Step 6: Build**

```bash
go build ./...
```

Expected: no errors.

**Step 7: Run all tests**

```bash
go test ./...
```

Expected: all pass.

**Step 8: Commit**

```bash
git add internal/tui/tui.go
git commit -m "feat: add live streaming output to TUI dashboard (selected job)"
```

---

## BATCH 2: Targeted Fixes

### Task 6: Remove dead helper functions from scanner.go

**Files:**
- Modify: `internal/scanner/scanner.go`

**Step 1: Replace split/trim/contains call sites with stdlib**

In `ParsePorts`, replace:
- `split(portSpec, ",")` → `strings.Split(portSpec, ",")`
- `trim(part)` → `strings.TrimSpace(part)`
- `contains(part, "-")` → `strings.Contains(part, "-")`
- `split(part, "-")` → `strings.Split(part, "-")`
- `trim(rangeParts[0])` → `strings.TrimSpace(rangeParts[0])`
- `trim(rangeParts[1])` → `strings.TrimSpace(rangeParts[1])`

Add `"strings"` to imports.

**Step 2: Delete the three helper functions** at the bottom of scanner.go (lines ~232-266):

Delete `split()`, `trim()`, and `contains()` entirely.

**Step 3: Run scanner tests**

```bash
go test ./internal/scanner/...
```

Expected: PASS — `TestParsePorts` and `TestGetServiceName` still pass.

**Step 4: Commit**

```bash
git add internal/scanner/scanner.go
git commit -m "refactor: replace custom split/trim/contains with strings stdlib in scanner"
```

---

### Task 7: Fix scanner per-port timeout

**Files:**
- Modify: `internal/scanner/scanner.go`
- Modify: `internal/cli/cli.go`

**Step 1: Add ScanTimeout field and update NewScanner**

In `scanner.go`, update `Scanner` struct:

```go
type Scanner struct {
    timeout     time.Duration // overall context timeout (unused in dial, kept for future)
    scanTimeout time.Duration // per-port TCP dial timeout
    workers     int
    verbosity   int
}
```

Update `NewScanner`:

```go
func NewScanner(overallTimeout time.Duration, workers int) *Scanner {
    if workers == 0 {
        workers = 100
    }
    if workers > 1000 {
        workers = 1000
    }
    return &Scanner{
        timeout:     overallTimeout,
        scanTimeout: 2 * time.Second,
        workers:     workers,
    }
}
```

Update `scanPort` to use `s.scanTimeout`:

```go
conn, err := net.DialTimeout("tcp", address, s.scanTimeout)
```

**Step 2: Add a test for timeout behavior**

In `internal/scanner/scanner_test.go`, add:

```go
func TestNewScannerDefaults(t *testing.T) {
    s := NewScanner(30*time.Second, 0)
    if s.workers != 100 {
        t.Errorf("expected 100 workers, got %d", s.workers)
    }
    if s.scanTimeout != 2*time.Second {
        t.Errorf("expected 2s scan timeout, got %v", s.scanTimeout)
    }
}
```

**Step 3: Run scanner tests**

```bash
go test ./internal/scanner/...
```

Expected: PASS.

**Step 4: Commit**

```bash
git add internal/scanner/scanner.go internal/scanner/scanner_test.go
git commit -m "fix: use 2s per-port TCP timeout in scanner, separate from command timeout"
```

---

### Task 8: Add API key env var fallback

**Files:**
- Modify: `internal/runner/python.go`

**Step 1: Update the env assembly in RunPython**

Replace the API key injection block:

```go
if opts.APIKeys != nil {
    for k, v := range opts.APIKeys {
        // env var takes precedence over config file value
        envKey := "CT_API_" + strings.ToUpper(k)
        if fromEnv := os.Getenv(envKey); fromEnv != "" {
            v = fromEnv
        }
        if v != "" {
            env = append(env, fmt.Sprintf("%s=%s", envKey, v))
        }
    }
}
// Also inject any CT_API_* vars present in the environment but not in config
for _, e := range os.Environ() {
    if strings.HasPrefix(e, "CT_API_") {
        key := strings.SplitN(e, "=", 2)[0]
        key = strings.TrimPrefix(key, "CT_API_")
        if _, exists := opts.APIKeys[strings.ToLower(key)]; !exists {
            env = append(env, e)
        }
    }
}
```

**Step 2: Build**

```bash
go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add internal/runner/python.go
git commit -m "feat: API keys fall back to CT_API_* env vars, env takes precedence over config"
```

---

### Task 9: Pip check stamp file caching

**Files:**
- Modify: `internal/runner/deps.go`

**Step 1: Add stamp file helpers**

Add these functions to `deps.go`:

```go
func stampPath() string {
    return filepath.Join(pythonVenvPath(), ".installed_pkgs")
}

func readStamp() map[string]bool {
    data, err := os.ReadFile(stampPath())
    if err != nil {
        return map[string]bool{}
    }
    pkgs := map[string]bool{}
    for _, line := range strings.Split(string(data), "\n") {
        line = strings.TrimSpace(line)
        if line != "" {
            pkgs[line] = true
        }
    }
    return pkgs
}

func writeStamp(pkgs []string) {
    sorted := make([]string, len(pkgs))
    copy(sorted, pkgs)
    sort.Strings(sorted)
    _ = os.WriteFile(stampPath(), []byte(strings.Join(sorted, "\n")+"\n"), 0600)
}
```

Add `"sort"` to imports.

**Step 2: Update EnsurePythonPackages to check/write stamp**

At the start of the package-checking loop, read the stamp:

```go
stamp := readStamp()
var newPkgs []string

for _, pkg := range packages {
    if stamp[pkg] {
        continue // already installed per stamp
    }
    cmd := exec.Command(venvPython, "-m", "pip", "show", pkg)
    if err := cmd.Run(); err == nil {
        newPkgs = append(newPkgs, pkg)
        continue // installed but not stamped — add to stamp
    }
    fmt.Fprintf(os.Stderr, "installing python package: %s\n", pkg)
    install := exec.Command(venvPython, "-m", "pip", "install", pkg)
    install.Stdout = os.Stdout
    install.Stderr = os.Stderr
    install.Stdin = os.Stdin
    if err := install.Run(); err != nil {
        return fmt.Errorf("failed to install python package %s", pkg)
    }
    newPkgs = append(newPkgs, pkg)
}

// Update stamp with all known packages
allPkgs := make([]string, 0, len(stamp)+len(newPkgs))
for p := range stamp {
    allPkgs = append(allPkgs, p)
}
allPkgs = append(allPkgs, newPkgs...)
if len(newPkgs) > 0 || len(allPkgs) > len(stamp) {
    writeStamp(allPkgs)
}
```

Also: after creating the venv, delete the stamp so it gets rebuilt:

```go
_ = os.Remove(stampPath())
```

**Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

**Step 4: Commit**

```bash
git add internal/runner/deps.go
git commit -m "perf: cache pip package checks with stamp file to skip repeated pip show calls"
```

---

### Task 10: Fix cycleArgs in TUI

**Files:**
- Modify: `internal/tui/tui.go`

**Step 1: Add argsOptionIdx to model**

Add to the `model` struct:

```go
argsOptionIdx int
```

**Step 2: Rewrite cycleArgs**

Replace the entire `cycleArgs` method:

```go
func (m *model) cycleArgs(dir int) {
    cmd := m.commands[m.menuIndex]
    if len(cmd.ArgsOptions) == 0 {
        return
    }
    m.argsOptionIdx = (m.argsOptionIdx + dir + len(cmd.ArgsOptions)) % len(cmd.ArgsOptions)
    val := cmd.ArgsOptions[m.argsOptionIdx]
    m.argsInput.SetValue(val)
    m.argsInput.SetCursor(len(val))
}
```

**Step 3: Reset argsOptionIdx when command selection changes**

In `syncPlaceholders()`, add:

```go
func (m *model) syncPlaceholders() {
    cmd := m.commands[m.menuIndex]
    m.targetInput.Placeholder = cmd.TargetHint
    m.argsInput.Placeholder = cmd.ArgsHint
    m.argsOptionIdx = 0  // reset cycle index on command change
}
```

**Step 4: Build and run tests**

```bash
go build ./... && go test ./...
```

Expected: all pass.

**Step 5: Commit**

```bash
git add internal/tui/tui.go
git commit -m "fix: simplify cycleArgs in TUI using index instead of string-splitting heuristics"
```

---

### Task 11: Fix TUI scan command to use native Go scanner

**Files:**
- Modify: `internal/tui/tui.go`

**Step 1: Update the scan commandDef in newModel()**

Change:

```go
{
    Name:           "scan",
    Description:    "Run nmap service scan via scan_nmap.py",
    Script:         "scan_nmap.py",
    RequiresTarget: true,
    TargetHint:     "example.com",
    ArgsHint:       "--ports 80,443",
},
```

To:

```go
{
    Name:           "scan",
    Description:    "Native TCP port scan",
    Script:         "scan_nmap.py",  // kept for backward compat with plugin path resolution
    RequiresTarget: true,
    TargetHint:     "example.com",
    ArgsHint:       "--ports 80,443",
    NotImplemented: false,
},
```

Update the description to reflect what it actually does. Note: the TUI `scan` command still runs the Python path through the queue — this is acceptable since `scan_nmap.py` is embedded and works as a fallback. The native Go scanner is only available via the CLI path. Mark this in a comment.

Actually, add a comment above the scan commandDef:
```go
// Note: TUI scan runs the embedded scan_nmap.py plugin.
// The native Go scanner is only available via the CLI (ct scan).
```

**Step 2: Build**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/tui/tui.go
git commit -m "fix: clarify TUI scan uses embedded plugin, update description"
```

---

## Final Verification

### Task 12: Full test suite + build check

**Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: all tests pass.

**Step 2: Build release binary**

```bash
go build -o ct ./cmd/ct
```

Expected: no errors, binary produced.

**Step 3: Smoke test CLI help**

```bash
./ct --help
./ct scan --help
./ct results --help
./ct dns --help
```

Expected: cobra-generated help text appears for each command with correct flags.

**Step 4: Smoke test results pruning flags**

```bash
./ct results --help
```

Expected: `--clear` and `--older-than` flags appear in help.

**Step 5: Final commit if any cleanup needed**

```bash
git add -A
git status  # review what's staged
git commit -m "chore: final cleanup and verification"
```

---

## Summary of All Commits

| Commit | Change |
|---|---|
| `chore: add cobra dependency` | go.mod / go.sum |
| `feat: migrate CLI to cobra` | internal/cli/cli.go full rewrite |
| `feat: storage pruning, remove dead config fields` | config.go, store.go, tests |
| `feat: add LineCallback streaming to RunPython` | runner/python.go |
| `feat: TUI live streaming output` | tui/tui.go |
| `refactor: remove dead scanner helpers` | scanner/scanner.go |
| `fix: per-port TCP timeout in scanner` | scanner/scanner.go, test |
| `feat: API key env var fallback` | runner/python.go |
| `perf: pip stamp file caching` | runner/deps.go |
| `fix: cycleArgs simplification` | tui/tui.go |
| `fix: clarify TUI scan command` | tui/tui.go |
