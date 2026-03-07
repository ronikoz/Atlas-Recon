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
		scanCmd(), dnsCmd(), osintCmd(), reconCmd(), webCmd(),
		reportCmd(), dashboardCmd(), resultsCmd(), phoneCmd(),
		geoCmd(), conflictCmd(), marketsCmd(), socialCmd(), flightCmd(), warCmd(),
	)

	cobra.OnFinalize(func() {
		if resultStore != nil {
			_ = resultStore.Close()
		}
	})

	return root
}

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
			return runPluginHelper("dns", "dns_lookup.py", "usage: ct dns <domain>",
				args, nil, []runner.Dependency{nslookupDependency()}, targetsFile)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	return cmd
}

func osintCmd() *cobra.Command {
	var targetsFile string
	var useSuite bool
	cmd := &cobra.Command{
		Use:   "osint <domain>",
		Short: "Run OSINT tasks and data enrichment",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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
	cmd.Flags().BoolVar(&useSuite, "suite", false, "use multi-source OSINT suite instead of domain-specific")
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

// --- implementation functions ---

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
	hadError := false
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
		if err != nil {
			hadError = true
			if !cfg.Output.JSON {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		}
		results = append(results, result)
	}
	if cfg.Output.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
	}
	if hadError {
		return fmt.Errorf("one or more targets failed")
	}
	return nil
}

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
		d, err := parseDuration(olderThan)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", olderThan, err)
		}
		n, err := resultStore.PruneOldRecords(d)
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
			"darwin": {{Name: "brew", Command: []string{"brew", "install", "bind"}}},
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
