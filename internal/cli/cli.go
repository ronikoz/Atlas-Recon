package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ronikoz/atlas-recon/internal/config"
	"github.com/ronikoz/atlas-recon/internal/crawl"
	"github.com/ronikoz/atlas-recon/internal/dns"
	"github.com/ronikoz/atlas-recon/internal/graph"
	"github.com/ronikoz/atlas-recon/internal/plugins"
	"github.com/ronikoz/atlas-recon/internal/runner"
	"github.com/ronikoz/atlas-recon/internal/scanner"
	"github.com/ronikoz/atlas-recon/internal/storage"
	"github.com/ronikoz/atlas-recon/internal/tui"
	"github.com/ronikoz/atlas-recon/internal/web"
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
		lanCmd(),
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
		Short: "Run native TCP port scans",
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
	var typesStr string
	var usePlugin bool
	cmd := &cobra.Command{
		Use:   "dns <domain>",
		Short: "Run DNS lookups and record gathering",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if usePlugin || targetsFile != "" {
				return runPluginHelper("dns", "dns_lookup.py", "usage: ct dns <domain>",
					args, []string{"dnspython"}, nil, targetsFile)
			}
			if len(args) == 0 {
				fmt.Fprintln(os.Stderr, "usage: ct dns <domain>")
				return nil
			}
			return runNativeDNS(args[0], typesStr)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	cmd.Flags().StringVar(&typesStr, "types", "A,AAAA,MX,NS,TXT,CNAME", "record types to query")
	cmd.Flags().BoolVar(&usePlugin, "plugin", false, "use Python plugin instead of native resolver")
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
	var webInsecure bool
	var webTimeout int
	var usePlugin bool
	cmd := &cobra.Command{
		Use:   "web <url>",
		Short: "Run web-specific checks",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if usePlugin || targetsFile != "" {
				return runPluginHelper("web", "web_check.py", "usage: ct web <url>", args, []string{"requests"}, nil, targetsFile)
			}
			if len(args) == 0 {
				fmt.Fprintln(os.Stderr, "usage: ct web <url>")
				return nil
			}
			return runNativeWeb(args[0], webTimeout, webInsecure)
		},
	}
	cmd.Flags().StringVar(&targetsFile, "targets-file", "", "file with one target per line")
	cmd.Flags().BoolVar(&webInsecure, "insecure", false, "skip TLS certificate verification")
	cmd.Flags().IntVar(&webTimeout, "timeout", 0, "request timeout in seconds (default from config)")
	cmd.Flags().BoolVar(&usePlugin, "plugin", false, "use Python plugin instead of native probe")
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

// --- lan commands ---

func lanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lan",
		Short: "Authorized LAN discovery, crawl, and mapping",
	}
	cmd.AddCommand(lanDiscoverCmd(), lanCrawlCmd(), lanMapCmd())
	return cmd
}

func lanDiscoverCmd() *cobra.Command {
	var cidrStr string
	var useLocal bool
	var portSpec string
	var maxHosts int
	var timeoutSec int
	var inspect bool
	var insecure bool
	var noStore bool

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover live hosts and services in a LAN range",
		Long: `Scan an explicit CIDR range or auto-detect local private interfaces to find
live hosts and open TCP ports. Requires --cidr or --local.

Use --inspect to probe discovered HTTP(S) services for status, title, and TLS metadata.

Examples:
  ct lan discover --cidr [IP_ADDRESS]/24 --ports 80,443,8080,8443
  ct lan discover --local --ports 80,443 --inspect --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLanDiscover(cidrStr, useLocal, portSpec, maxHosts, timeoutSec, inspect, insecure, noStore)
		},
	}
	cmd.Flags().StringVar(&cidrStr, "cidr", "", "CIDR range to scan (e.g. [IP_ADDRESS]/24)")
	cmd.Flags().BoolVar(&useLocal, "local", false, "auto-detect local private interface CIDRs")
	cmd.Flags().StringVar(&portSpec, "ports", "80,443,8080,8443", "ports to scan")
	cmd.Flags().IntVar(&maxHosts, "max-hosts", 256, "maximum hosts to scan")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 0, "per-host scan timeout in seconds (default from config)")
	cmd.Flags().BoolVar(&inspect, "inspect", true, "probe HTTP(S) services for metadata")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "skip TLS certificate verification")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "skip storing LAN graph data")
	cmd.MarkFlagsOneRequired("cidr", "local")
	return cmd
}

func lanCrawlCmd() *cobra.Command {
	var cidrStr string
	var useLocal bool
	var portSpec string
	var maxHosts int
	var timeoutSec int
	var depth int
	var maxPages int
	var insecure bool
	var allowExternal bool
	var noStore bool

	cmd := &cobra.Command{
		Use:   "crawl",
		Short: "Crawl HTTP(S) services discovered on a LAN",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLanCrawl(cidrStr, useLocal, portSpec, maxHosts, timeoutSec, depth, maxPages, insecure, allowExternal, noStore)
		},
	}
	cmd.Flags().StringVar(&cidrStr, "cidr", "", "CIDR range to scan and crawl")
	cmd.Flags().BoolVar(&useLocal, "local", false, "auto-detect local private interface CIDRs")
	cmd.Flags().StringVar(&portSpec, "ports", "80,443,8080,8443", "ports to scan before crawling")
	cmd.Flags().IntVar(&maxHosts, "max-hosts", 256, "maximum hosts to scan")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 0, "per-request timeout in seconds (default from config)")
	cmd.Flags().IntVar(&depth, "depth", 1, "maximum crawl depth")
	cmd.Flags().IntVar(&maxPages, "max-pages", 500, "maximum pages to crawl")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "skip TLS certificate verification during service inspection")
	cmd.Flags().BoolVar(&allowExternal, "allow-external-links", false, "allow crawling outside discovered LAN service hosts")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "skip storing LAN graph data")
	cmd.MarkFlagsOneRequired("cidr", "local")
	return cmd
}

func lanMapCmd() *cobra.Command {
	var scanID string
	var format string
	cmd := &cobra.Command{
		Use:   "map",
		Short: "Export a stored LAN scan map",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLanMap(scanID, format)
		},
	}
	cmd.Flags().StringVar(&scanID, "scan-id", "", "scan id to export (default: most recent)")
	cmd.Flags().StringVar(&format, "format", "markdown", "export format: json, markdown, or dot")
	return cmd
}

func runLanDiscover(cidrStr string, useLocal bool, portSpec string, maxHosts int, timeoutSec int, inspect bool, insecure bool, noStore bool) error {
	started := time.Now()
	// Resolve CIDR ranges
	var cidrs []*net.IPNet // shadowed; use net package below
	if useLocal {
		localCIDRs, err := crawl.DiscoverLocalCIDRs()
		if err != nil {
			return fmt.Errorf("detecting local interfaces: %w", err)
		}
		if len(localCIDRs) == 0 {
			return fmt.Errorf("no private network interfaces found")
		}
		cidrs = localCIDRs
		if !cfg.Output.JSON {
			for _, cidr := range cidrs {
				fmt.Printf("Auto-detected interface: %s\n", cidr)
			}
		}
	} else {
		cidr, err := crawl.ParseCIDR(cidrStr)
		if err != nil {
			return fmt.Errorf("invalid CIDR: %w", err)
		}
		if err := crawl.ValidateScope(cidr); err != nil {
			return err
		}
		cidrs = []*net.IPNet{cidr}
	}

	ports, err := scanner.ParsePorts(portSpec)
	if err != nil {
		return fmt.Errorf("parsing ports: %w", err)
	}

	timeout := time.Duration(cfg.Timeouts.CommandSeconds) * time.Second
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	// Scan each CIDR
	if !cfg.Output.JSON {
		fmt.Fprintf(os.Stderr, "Starting LAN discovery...\n")
	}

	var allResults []*crawl.DiscoveryResult
	for _, cidr := range cidrs {
		result, err := crawl.RunDiscovery(cidr, ports, maxHosts, cfg.Concurrency, timeout)
		if err != nil {
			if !cfg.Output.JSON {
				fmt.Fprintf(os.Stderr, "discovery error for %s: %v\n", cidr, err)
			}
			continue
		}
		allResults = append(allResults, result)

		if !cfg.Output.JSON {
			totalOpen := 0
			for _, h := range result.Hosts {
				totalOpen += len(h.OpenPorts)
			}
			fmt.Printf("CIDR %s: %d hosts, %d open ports\n", cidr, len(result.Hosts), totalOpen)
		}
	}

	if len(allResults) == 0 {
		return fmt.Errorf("no results from any scanned CIDR")
	}

	// Run HTTP(S) inspection if requested
	type enrichedHost struct {
		IP       string               `json:"ip"`
		Ports    []scanner.PortResult `json:"open_ports"`
		Services []crawl.ServiceInfo  `json:"services,omitempty"`
	}
	type discoverOutput struct {
		CIDR  string         `json:"cidr"`
		Hosts []enrichedHost `json:"hosts"`
	}

	var outputs []discoverOutput
	graphScan := &graph.ScanNode{
		ID:        resultID("lan"),
		StartedAt: started,
	}
	for _, result := range allResults {
		graphScan.CIDRs = append(graphScan.CIDRs, result.CIDR)
		out := discoverOutput{CIDR: result.CIDR}
		for _, host := range result.Hosts {
			eh := enrichedHost{IP: host.IP, Ports: host.OpenPorts}
			if inspect && len(host.OpenPorts) > 0 {
				eh.Services = crawl.InspectHostServices(host, crawl.InspectOptions{
					Timeout:  timeout,
					Insecure: insecure,
				})
				if !cfg.Output.JSON {
					for _, svc := range eh.Services {
						fmt.Print(crawl.FormatServiceInfo(&svc))
					}
				}
			}
			out.Hosts = append(out.Hosts, eh)
			graphScan.Hosts = append(graphScan.Hosts, graphHost(result.CIDR, host.IP, eh.Ports, eh.Services))
		}
		outputs = append(outputs, out)
	}
	finished := time.Now()
	graphScan.EndedAt = finished
	storeNativePayload("lan discover", "lan_discover", []string{cidrArg(cidrStr, useLocal), "--ports", portSpec}, outputs, started, finished, "")
	if !noStore {
		if err := saveGraphScan(graphScan); err != nil {
			return err
		}
		if !cfg.Output.JSON {
			fmt.Fprintf(os.Stdout, "Stored LAN graph scan: %s\n", graphScan.ID)
		}
	}

	if cfg.Output.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(outputs)
	}

	return nil
}

func runLanMap(scanID string, format string) error {
	store, err := graph.Open(graph.DefaultDBPath())
	if err != nil {
		return fmt.Errorf("opening graph store: %w", err)
	}
	defer store.Close()
	scan, err := store.LoadScan(scanID)
	if err != nil {
		if scanID == "" {
			return fmt.Errorf("loading latest LAN graph: %w", err)
		}
		return fmt.Errorf("loading LAN graph %s: %w", scanID, err)
	}
	switch strings.ToLower(format) {
	case "json":
		data, err := graph.ExportJSON(scan)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
	case "markdown", "md":
		fmt.Print(graph.ExportMarkdown(scan))
	case "dot":
		fmt.Print(graph.ExportDOT(scan))
	default:
		return fmt.Errorf("unsupported map format %q", format)
	}
	return nil
}

func runLanCrawl(cidrStr string, useLocal bool, portSpec string, maxHosts int, timeoutSec int, depth int, maxPages int, insecure bool, allowExternal bool, noStore bool) error {
	started := time.Now()
	cidrs, err := resolveLanCIDRs(cidrStr, useLocal)
	if err != nil {
		return err
	}
	ports, err := scanner.ParsePorts(portSpec)
	if err != nil {
		return fmt.Errorf("parsing ports: %w", err)
	}
	timeout := time.Duration(cfg.Timeouts.CommandSeconds) * time.Second
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}
	graphScan := &graph.ScanNode{ID: resultID("lan-crawl"), StartedAt: started}
	var seedURLs []string
	for _, cidr := range cidrs {
		graphScan.CIDRs = append(graphScan.CIDRs, cidr.String())
		result, err := crawl.RunDiscovery(cidr, ports, maxHosts, cfg.Concurrency, timeout)
		if err != nil {
			if !cfg.Output.JSON {
				fmt.Fprintf(os.Stderr, "discovery error for %s: %v\n", cidr, err)
			}
			continue
		}
		for _, host := range result.Hosts {
			services := crawl.InspectHostServices(host, crawl.InspectOptions{Timeout: timeout, Insecure: insecure})
			graphScan.Hosts = append(graphScan.Hosts, graphHost(result.CIDR, host.IP, host.OpenPorts, services))
			for _, service := range services {
				if service.Error == "" && service.Scheme != "" {
					seedURLs = append(seedURLs, fmt.Sprintf("%s://%s:%d/", service.Scheme, service.Host, service.Port))
				}
			}
		}
	}
	crawlResult := crawl.RunCrawl(seedURLs, crawl.CrawlOptions{
		MaxDepth:      depth,
		MaxPages:      maxPages,
		Timeout:       timeout,
		AllowExternal: allowExternal,
		AllowedCIDRs:  cidrs,
	})
	for _, page := range crawlResult.Pages {
		graphScan.Pages = append(graphScan.Pages, graph.PageNode{
			URL:         page.URL,
			StatusCode:  page.StatusCode,
			Title:       page.Title,
			ContentType: page.ContentType,
			Depth:       page.Depth,
		})
	}
	for _, link := range crawlResult.Links {
		graphScan.Links = append(graphScan.Links, graph.LinkEdge{FromURL: link.FromURL, ToURL: link.ToURL})
	}
	finished := time.Now()
	graphScan.EndedAt = finished
	output := struct {
		ScanID string             `json:"scan_id"`
		Pages  []crawl.PageResult `json:"pages"`
		Links  []crawl.LinkResult `json:"links"`
	}{
		ScanID: graphScan.ID,
		Pages:  crawlResult.Pages,
		Links:  crawlResult.Links,
	}
	storeNativePayload("lan crawl", "lan_crawl", []string{cidrArg(cidrStr, useLocal), "--ports", portSpec}, output, started, finished, "")
	if !noStore {
		if err := saveGraphScan(graphScan); err != nil {
			return err
		}
	}
	if cfg.Output.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}
	fmt.Fprintf(os.Stdout, "Crawled %d pages from %d seed services.\n", len(crawlResult.Pages), len(seedURLs))
	if !noStore {
		fmt.Fprintf(os.Stdout, "Stored LAN graph scan: %s\n", graphScan.ID)
	}
	return nil
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

func runNativeDNS(domain, typesStr string) error {
	started := time.Now()
	rtypes := strings.Split(typesStr, ",")
	for i := range rtypes {
		rtypes[i] = strings.TrimSpace(rtypes[i])
	}
	records, err := dns.Lookup(domain, rtypes)
	if err != nil {
		return err
	}
	payload := struct {
		Records []dns.Record `json:"records"`
	}{Records: records}
	storeNativePayload("dns", "dns", []string{domain, "--types", typesStr}, payload, started, time.Now(), "")
	if cfg.Output.JSON {
		data, err := dns.RecordsToJSON(records)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(dns.FormatRecords(records))
	return nil
}

func runNativeWeb(target string, timeoutSec int, insecure bool) error {
	started := time.Now()
	timeout := time.Duration(cfg.Timeouts.CommandSeconds)
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec)
	}
	timeout *= time.Second

	result, err := web.Probe(target, web.ProbeOptions{
		Timeout:  timeout,
		Insecure: insecure,
	})
	if err != nil {
		return err
	}
	status := ""
	if result.Error != "" {
		status = string(runner.StatusFailed)
	}
	storeNativePayload("web", "web", []string{target}, result, started, time.Now(), status)

	if cfg.Output.JSON {
		data, err := web.ProbeToJSON(result)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		if result.Error != "" {
			return fmt.Errorf("%s", result.Error)
		}
		return nil
	}

	fmt.Print(web.FormatProbe(result))
	if result.Error != "" {
		fmt.Println()
		return fmt.Errorf("%s", result.Error)
	}
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

func saveGraphScan(scan *graph.ScanNode) error {
	store, err := graph.Open(graph.DefaultDBPath())
	if err != nil {
		return fmt.Errorf("opening graph store: %w", err)
	}
	defer store.Close()
	if err := store.SaveScan(scan); err != nil {
		return fmt.Errorf("saving graph scan: %w", err)
	}
	return nil
}

func graphHost(cidr string, ip string, ports []scanner.PortResult, services []crawl.ServiceInfo) graph.HostNode {
	host := graph.HostNode{IP: ip, CIDR: cidr}
	for _, port := range ports {
		host.OpenPorts = append(host.OpenPorts, graph.PortNode{
			Port:    port.Port,
			State:   port.State,
			Service: port.Service,
		})
	}
	for _, service := range services {
		node := graph.ServiceNode{
			HostIP:     service.Host,
			Port:       service.Port,
			Scheme:     service.Scheme,
			Protocol:   service.Scheme,
			StatusCode: service.StatusCode,
			Title:      service.Title,
			Error:      service.Error,
		}
		if service.TLS != nil {
			node.TLSSubject = service.TLS.Subject
			node.TLSIssuer = service.TLS.Issuer
			node.TLSNotBefore = service.TLS.NotBefore
			node.TLSNotAfter = service.TLS.NotAfter
			node.TLSFingerprint = service.TLS.FingerprintSHA256
		}
		host.Services = append(host.Services, node)
	}
	return host
}

func resolveLanCIDRs(cidrStr string, useLocal bool) ([]*net.IPNet, error) {
	if useLocal {
		localCIDRs, err := crawl.DiscoverLocalCIDRs()
		if err != nil {
			return nil, fmt.Errorf("detecting local interfaces: %w", err)
		}
		if len(localCIDRs) == 0 {
			return nil, fmt.Errorf("no private network interfaces found")
		}
		return localCIDRs, nil
	}
	cidr, err := crawl.ParseCIDR(cidrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}
	if err := crawl.ValidateScope(cidr); err != nil {
		return nil, err
	}
	return []*net.IPNet{cidr}, nil
}

func storeNativePayload(command string, kind string, args []string, payload any, started time.Time, finished time.Time, status string) {
	if resultStore == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "results store failed: %v\n", err)
		return
	}
	if status == "" {
		status = string(runner.StatusSuccess)
	}
	exitCode := 0
	if status == string(runner.StatusFailed) {
		exitCode = 1
	}
	record := storage.Record{
		ID:         resultID(strings.ReplaceAll(command, " ", "-")),
		Kind:       kind,
		Command:    command,
		Args:       args,
		StartedAt:  started,
		FinishedAt: finished,
		DurationMs: finished.Sub(started).Milliseconds(),
		ExitCode:   exitCode,
		Status:     status,
		Payload:    string(data),
	}
	if err := resultStore.SaveRecord(record); err != nil {
		fmt.Fprintf(os.Stderr, "results store failed: %v\n", err)
	}
}

func cidrArg(cidrStr string, useLocal bool) string {
	if useLocal {
		return "--local"
	}
	return "--cidr=" + cidrStr
}

func resultID(prefix string) string {
	return prefix + "-" + uuid.New().String()
}
