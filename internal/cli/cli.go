package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cli-tools/internal/config"
	"cli-tools/internal/plugins"
	"cli-tools/internal/runner"
	"cli-tools/internal/scanner"
	"cli-tools/internal/tui"
)

type command struct {
	name        string
	description string
	run         func(args []string) error
}

var cfg config.Config

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
	fmt.Fprintln(os.Stderr, "CLI Tools: multi-command security toolkit")
	fmt.Fprintln(os.Stderr, "\nUsage: ct [--config path] <command> [args]")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	order := []string{"scan", "dns", "osint", "phone", "geo", "conflict", "markets", "social", "flight", "war", "recon", "web", "report", "dashboard"}
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
	s := scanner.NewScanner(3*time.Second, 100)
	result := s.ScanHost(target, ports)

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

func runDNS(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct dns <domain> [--json]")
		return nil
	}

	if err := runner.EnsureDependencies([]runner.Dependency{nslookupDependency()}); err != nil {
		return err
	}

	script := pluginPath("dns_lookup.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("dns")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runOSINT(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct osint <domain> [--json]")
		fmt.Fprintln(os.Stderr, "   or: ct osint --category <name> [--source <name>] [--json]")
		return nil
	}

	useSuite := hasAnyFlag(parsed.args, []string{"--category", "--source", "--list", "--suite"})
	var pyPkgs []string
	var script string
	if useSuite {
		pyPkgs = []string{"requests"}
		script = pluginPath("osint_suite.py")
	} else {
		// Ensure Python and required pip packages for the OSINT domain plugin
		pyPkgs = []string{"requests", "python-whois", "python-dateutil", "dnspython", "colorama"}
		script = pluginPath("osint_domain.py")
	}
	if err := runner.EnsurePythonPackages(pyPkgs, cfg.Paths.Python); err != nil {
		return err
	}

	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("osint")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runRecon(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct recon <domain> [--json]")
		return nil
	}

	script := pluginPath("recon_subdomains.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("recon")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runWeb(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct web <url> [--json]")
		return nil
	}

	if err := runner.EnsurePythonPackages([]string{"requests"}, cfg.Paths.Python); err != nil {
		return err
	}

	script := pluginPath("web_check.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("web")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runReport(args []string) error {
	parsed := parseArgs(args)
	// Report doesn't strictly need a target, but often takes files.
	// If no args, show help
	if len(parsed.args) == 0 && !hasAnyFlag(parsed.args, []string{"--title", "--output"}) {
		fmt.Fprintln(os.Stderr, "usage: ct report [files...] [--title <text>] [--output <file>]")
		return nil
	}

	script := pluginPath("generate_report.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("report")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runPhone(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct phone <number> [--json]")
		return nil
	}

	if err := runner.EnsurePythonPackages([]string{"phonenumbers"}, cfg.Paths.Python); err != nil {
		return err
	}

	script := pluginPath("phone_osint.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("phone")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runGeo(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct geo <query> [--json]")
		return nil
	}

	if err := runner.EnsurePythonPackages([]string{"geopy"}, cfg.Paths.Python); err != nil {
		return err
	}

	script := pluginPath("geo_recon.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("geo")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runConflict(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct conflict <query> [--json]")
		return nil
	}
	if err := runner.EnsurePythonPackages([]string{"requests"}, cfg.Paths.Python); err != nil {
		return err
	}
	script := pluginPath("conflict_view.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("conflict")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runMarkets(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct markets <query> [--json]")
		return nil
	}
	if err := runner.EnsurePythonPackages([]string{"requests"}, cfg.Paths.Python); err != nil {
		return err
	}
	script := pluginPath("market_sentiment.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("markets")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runSocial(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct social <query> [--json]")
		return nil
	}
	// atproto requires requests too, but let's check basic requests first as it is used in the plugin manually
	if err := runner.EnsurePythonPackages([]string{"requests"}, cfg.Paths.Python); err != nil {
		return err
	}
	script := pluginPath("social_pulse.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("social")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runFlight(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct flight <target> [--radius km] [--json]")
		return nil
	}
	if err := runner.EnsurePythonPackages([]string{"requests", "geopy"}, cfg.Paths.Python); err != nil {
		return err
	}
	script := pluginPath("flight_radar.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("flight")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runWar(args []string) error {
	parsed := parseArgs(args)
	if len(parsed.args) == 0 || isHelp(parsed.args) {
		fmt.Fprintln(os.Stderr, "usage: ct war <target> [--json]")
		return nil
	}
	if err := runner.EnsurePythonPackages([]string{"requests", "geopy"}, cfg.Paths.Python); err != nil {
		return err
	}
	script := pluginPath("war_intel.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("war")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
}

func runDashboard(args []string) error {
	return tui.Run(cfg)
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
	joined := strings.Join(args, " ")
	return strings.Contains(joined, "-h") || strings.Contains(joined, "--help")
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

func nmapDependency() runner.Dependency {
	return runner.Dependency{
		Name:        "nmap",
		CheckCmd:    "nmap",
		Description: "port scanner",
		Installers:  runner.BaseInstallers("nmap", "Nmap.Nmap", "nmap"),
	}
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

func whoisDependency() runner.Dependency {
	return runner.Dependency{
		Name:        "whois",
		CheckCmd:    "whois",
		Description: "WHOIS client",
		Installers:  runner.BaseInstallers("whois", "Sysinternals.Whois", "whois"),
	}
}

// Signed-off-by: ronikoz
