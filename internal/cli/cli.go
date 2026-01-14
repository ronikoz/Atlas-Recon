package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cli-tools/internal/config"
	"cli-tools/internal/runner"
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
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
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
	}
}

func printUsage(cmds map[string]command) {
	fmt.Fprintln(os.Stderr, "CLI Tools: multi-command security toolkit")
	fmt.Fprintln(os.Stderr, "\nUsage: ct [--config path] <command> [args]")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	order := []string{"scan", "dns", "osint", "recon", "web", "report", "dashboard"}
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

	if err := runner.EnsureDependencies([]runner.Dependency{nmapDependency()}); err != nil {
		return err
	}

	script := pluginPath("scan_nmap.py")
	result, err := runner.RunPython(script, parsed.args, runner.RunOptions{
		Stream: !parsed.json,
		Python: cfg.Paths.Python,
	})
	result.ID = resultID("scan")
	if parsed.json {
		return emitJSON(result, err)
	}
	return err
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
		return nil
	}

	// Ensure Python and required pip packages for the OSINT plugin
	pyPkgs := []string{"requests", "python-whois", "python-dateutil", "dnspython", "colorama"}
	if err := runner.EnsurePythonPackages(pyPkgs, cfg.Paths.Python); err != nil {
		return err
	}

	script := pluginPath("osint_domain.py")
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
	return errors.New("web command not implemented yet")
}

func runReport(args []string) error {
	return errors.New("report command not implemented yet")
}

func runDashboard(args []string) error {
	return tui.Run()
}

func pluginPath(name string) string {
	return filepath.Join("plugins", "python", name)
}

func isHelp(args []string) bool {
	joined := strings.Join(args, " ")
	return strings.Contains(joined, "-h") || strings.Contains(joined, "--help")
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
