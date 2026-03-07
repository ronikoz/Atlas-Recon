package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ronikoz/atlas-recon/internal/config"
	"github.com/ronikoz/atlas-recon/internal/plugins"
	"github.com/ronikoz/atlas-recon/internal/runner"
	"github.com/ronikoz/atlas-recon/internal/scanner"
	"github.com/ronikoz/atlas-recon/internal/storage"
	"github.com/ronikoz/atlas-recon/internal/tui"
)

type command struct {
	name        string
	description string
	run         func(args []string) error
}

var cfg config.Config
var resultStore *storage.Store

func Execute(argv []string) int {
	var err error
	cfgPath, argv := extractConfigPath(argv)
	cfg, err = config.Load(cfgPath)
	if err != nil {
		displayCfg := cfgPath
		if displayCfg == "" {
			displayCfg = "(default)"
		}
		fmt.Fprintf(os.Stderr, "config load failed (%s): %v\n", displayCfg, err)
		return 1
	}
	initResultStore()
	if resultStore != nil {
		defer func() {
			if err := resultStore.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "results store close failed: %v\n", err)
			}
		}()
	}

	cmds := commandSet()
	if len(argv) < 2 {
		printUsage(cmds)
		return 1
	}

	name := argv[1]
	if name == "help" || name == "-h" || name == "--help" {
		printUsage(cmds)
		return 0
	}

	cmd, ok := cmds[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", name)
		printUsage(cmds)
		return 1
	}

	if err := cmd.run(argv[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s failed: %v\n", name, err)
		return 1
	}
	return 0
}

func commandSet() map[string]command {
	return map[string]command{
		"scan": {
			name:        "scan",
			description: "Run scanning tasks (nmap, http checks)",
			run:         runScan,
		},
		"dns": {
			name:        "dns",
			description: "Run DNS lookups and record gathering",
			run:         runDNS,
		},
		"osint": {
			name:        "osint",
			description: "Run OSINT tasks and data enrichment",
			run:         runOSINT,
		},
		"recon": {
			name:        "recon",
			description: "Run recon tasks (subdomain discovery, crawl)",
			run:         runRecon,
		},
		"web": {
			name:        "web",
			description: "Run web-specific checks",
			run:         runWeb,
		},
		"report": {
			name:        "report",
			description: "Generate reports from results",
			run:         runReport,
		},
		"dashboard": {
			name:        "dashboard",
			description: "Launch TUI dashboard",
			run:         runDashboard,
		},
		"results": {
			name:        "results",
			description: "List stored command results",
			run:         runResults,
		},
		"phone": {
			name:        "phone",
			description: "Phone number OSINT",
			run:         runPhone,
		},
		"geo": {
			name:        "geo",
			description: "Geospatial reconnaissance",
			run:         runGeo,
		},
		"conflict": {
			name:        "conflict",
			description: "Geopolitical conflict data (GDELT)",
			run:         runConflict,
		},
		"markets": {
			name:        "markets",
			description: "Prediction markets (Polymarket)",
			run:         runMarkets,
		},
		"social": {
			name:        "social",
			description: "Social media pulse (Bluesky)",
			run:         runSocial,
		},
		"flight": {
			name:        "flight",
			description: "Live flight radar (OpenSky)",
			run:         runFlight,
		},
		"war": {
			name:        "war",
			description: "War Intel Edge (ISW + Maps)",
			run:         runWar,
		},
	}
}

func printUsage(cmds map[string]command) {
	fmt.Fprintln(os.Stderr, "Atlas-Recon: multi-command security toolkit")
	fmt.Fprintln(os.Stderr, "\nUsage: ct [--config path] <command> [args]")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	order := []string{"scan", "dns", "osint", "phone", "geo", "conflict", "markets", "social", "flight", "war", "recon", "web", "report", "results", "dashboard"}
	for _, name := range order {
		if cmd, ok := cmds[name]; ok {
			fmt.Fprintf(os.Stderr, "  %-10s %s\n", cmd.name, cmd.description)
		}
	}
	fmt.Fprintln(os.Stderr, "\nRun: ct <command> --help for command-specific help")
}

func runScan(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct scan <target> [--ports 80,443] [--json]")
		return nil
	}

	target := parsed.args[0]
	portSpec := ""

	// Extract --ports argument
	for i, arg := range parsed.args {
		if arg == "--ports" && i+1 < len(parsed.args) {
			portSpec = parsed.args[i+1]
			break
		}
	}

	// Parse ports
	ports, err := scanner.ParsePorts(portSpec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing ports: %v\n", err)
		return err
	}

	// Run native scanner
	timeout := time.Duration(cfg.Timeouts.CommandSeconds) * time.Second
	s := scanner.NewScanner(timeout, cfg.Concurrency)
	result := s.ScanHost(target, ports)
	storeScanResult(parsed.args, result)

	if parsed.json {
		jsonOutput, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Printf("Scan Results for %s\n", result.Host)
		fmt.Printf("Duration: %dms\n\n", result.EndTime.Sub(result.StartTime).Milliseconds())

		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
		} else {
			openPorts := 0
			for _, port := range result.Ports {
				if port.State == "open" {
					openPorts++
					fmt.Printf("Port %d: %s (%s)\n", port.Port, port.State, port.Service)
				}
			}

			closedPorts := len(result.Ports) - openPorts
			fmt.Printf("\nSummary: %d open, %d closed/filtered\n", openPorts, closedPorts)
		}
	}

	return nil
}

func runPluginHelper(name, plugin, usage string, args []string, pkgs []string, deps []runner.Dependency) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
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

	result, err := runner.RunPython(pluginPath(plugin), parsed.args, runner.RunOptions{
		Stream:  !parsed.json,
		Python:  cfg.Paths.Python,
		APIKeys: cfg.APIKeys,
	})
	result.ID = resultID(name)
	storeCommandResult(name, parsed.args, result)
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runDNS(args []string) error {
	return runPluginHelper("dns", "dns_lookup.py", "usage: ct dns <domain> [--json]", args, nil, []runner.Dependency{nslookupDependency()})
}

func runOSINT(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct osint <domain> [--json]\n   or: ct osint --category <name> [--source <name>] [--json]")
		return nil
	}

	useSuite := hasAnyFlag(parsed.args, []string{"--category", "--source", "--list", "--suite"})
	pkgs := []string{"requests", "python-whois", "python-dateutil", "dnspython", "colorama"}
	script := "osint_domain.py"
	if useSuite {
		pkgs = []string{"requests"}
		script = "osint_suite.py"
	}

	// Temporarily construct the args so it bypasses runPluginHelper's help check
	// because the helper uses the same `parseArgs`, but since we already parsed it, we pass original args
	return runPluginHelper("osint", script, "usage: ct osint <domain> [--json]", args, pkgs, nil)
}

func runRecon(args []string) error {
	return runPluginHelper("recon", "recon_subdomains.py", "usage: ct recon <domain> [--json]", args, nil, nil)
}

func runWeb(args []string) error {
	return runPluginHelper("web", "web_check.py", "usage: ct web <url> [--json]", args, []string{"requests"}, nil)
}

func runReport(args []string) error {
	return runPluginHelper("report", "generate_report.py", "usage: ct report [files...] [--title <text>] [--output <file>]", args, nil, nil)
}

func runPhone(args []string) error {
	return runPluginHelper("phone", "phone_osint.py", "usage: ct phone <number> [--json]", args, []string{"phonenumbers"}, nil)
}

func runGeo(args []string) error {
	return runPluginHelper("geo", "geo_recon.py", "usage: ct geo <query> [--json]", args, []string{"geopy"}, nil)
}

func runConflict(args []string) error {
	return runPluginHelper("conflict", "conflict_view.py", "usage: ct conflict <query> [--json]", args, []string{"requests"}, nil)
}

func runMarkets(args []string) error {
	return runPluginHelper("markets", "market_sentiment.py", "usage: ct markets <query> [--json]", args, []string{"requests"}, nil)
}

func runSocial(args []string) error {
	return runPluginHelper("social", "social_pulse.py", "usage: ct social <query> [--json]", args, []string{"requests"}, nil)
}

func runFlight(args []string) error {
	return runPluginHelper("flight", "flight_radar.py", "usage: ct flight <target> [--radius km] [--json]", args, []string{"requests", "geopy"}, nil)
}

func runWar(args []string) error {
	return runPluginHelper("war", "war_intel.py", "usage: ct war <target> [--json]", args, []string{"requests", "geopy"}, nil)
}

func runDashboard(args []string) error {
	return tui.Run(cfg)
}

func runResults(args []string) error {
	parsed := parseArgs(args)
	if isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct results [--limit N] [--command name] [--json]")
		return nil
	}
	if resultStore == nil {
		return errors.New("results storage disabled")
	}
	limit := 20
	commandFilter := ""
	for i := 0; i < len(parsed.args); i++ {
		arg := parsed.args[i]
		if arg == "--limit" && i+1 < len(parsed.args) {
			value, err := strconv.Atoi(parsed.args[i+1])
			if err != nil {
				return fmt.Errorf("invalid --limit value: %v", err)
			}
			limit = value
			i++
			continue
		}
		if strings.HasPrefix(arg, "--limit=") {
			value, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit="))
			if err != nil {
				return fmt.Errorf("invalid --limit value: %v", err)
			}
			limit = value
			continue
		}
		if arg == "--command" && i+1 < len(parsed.args) {
			commandFilter = parsed.args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--command=") {
			commandFilter = strings.TrimPrefix(arg, "--command=")
			continue
		}
	}

	records, err := resultStore.ListRecords(storage.ListOptions{Limit: limit, Command: commandFilter})
	if err != nil {
		return err
	}
	if parsed.json {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(records)
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

func pluginPath(name string) string {
	// Use the embedded plugin resolver
	path, err := plugins.GetPluginPath(name)
	if err == nil {
		return path
	}
	// Fallback to relative path (will error in runner with clear message)
	return filepath.Join("plugins", "python", name)
}

func isHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
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

type parsedFlags struct {
	args []string
	json bool
}

func parseArgs(args []string) parsedFlags {
	parsed := parsedFlags{args: make([]string, 0, len(args)), json: cfg.Output.JSON}
	for _, arg := range args {
		if arg == "--json" {
			parsed.json = true
			continue
		}
		parsed.args = append(parsed.args, arg)
	}
	return parsed
}

func emitJSON(result runner.Result, runErr error) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return err
	}
	// Return error to propagate exit code
	if result.ExitCode != 0 {
		return fmt.Errorf("exit code: %d", result.ExitCode)
	}
	return runErr
}

func initResultStore() {
	if !cfg.Storage.Enabled || cfg.Storage.ResultsDB == "" {
		return
	}
	store, err := storage.Open(cfg.Storage.ResultsDB)
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
		Payload:    "",
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
		fmt.Fprintf(os.Stderr, "results store failed: %v\n", err)
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
		Stdout:     "",
		Stderr:     "",
		Error:      result.Error,
		Payload:    string(payload),
	}
	if err := resultStore.SaveRecord(record); err != nil {
		fmt.Fprintf(os.Stderr, "results store failed: %v\n", err)
	}
}

func resultID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func extractConfigPath(argv []string) (string, []string) {
	var cfgPath string
	filtered := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		if argv[i] == "--config" || argv[i] == "-c" {
			if i+1 < len(argv) {
				cfgPath = argv[i+1]
				i++
				continue
			}
		}
		filtered = append(filtered, argv[i])
	}
	return cfgPath, filtered
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
