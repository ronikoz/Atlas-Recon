# Atlas-Recon — Comprehensive Tasks Document

**Date:** 2026-05-19
**Method:** KISS — every task is atomic, testable, and <200 lines changed where possible.
**Progress:** Phases 1-4 complete (44/72 tasks done)

---

## Legend

| Prefix | Meaning | Status marker |
|---|---|---|
| `[TEST]` | Test-only task | ✅ done / ⬜ pending |
| `[CODE]` | Implementation task | ✅ done / ⬜ pending |
| `[DOCS]` | Documentation task | ✅ done / ⬜ pending |
| `[INFRA]` | Build/CI/tooling task | ⬜ pending |
| `[REFAC]` | Refactor with no behavior change | ⬜ pending |

---

## Phase 1: Testing & Contract Foundation ✅

### Task 1.1 [TEST] ✅ — Add CLI contract test file
**File:** `internal/cli/cli_test.go` (new)
- TestRootHelp, TestScanHelp, TestResultsHelp, TestDNSHelp
- TestJSONPersistentFlag, TestBadConfigPath
- **Done.** 6 tests, all pass.

### Task 1.2 [TEST] ✅ — Add scan command contract tests
**File:** `internal/cli/cli_test.go`
- TestScanDefaultPorts, TestScanJSONOutput, TestScanInvalidPorts
- **Done.** 3 tests, all pass.

### Task 1.3 [TEST] ✅ — Add results contract tests
**File:** `internal/cli/cli_test.go`
- TestResultsClear, TestResultsOlderThan, TestResultsInvalidDuration, TestResultsJSON
- **Done.** 4 tests, all pass.

### Task 1.4 [TEST] ✅ — Add result envelope snapshot tests
**File:** `internal/runner/result_test.go` (new)
- TestResultJSONRoundtrip, TestResultStatusConsts, TestResultEmptyFields, TestResultErrorOmitEmpty
- **Done.** 4 tests, all pass.

### Task 1.5 [TEST] ✅ — Add config loading error tests
**File:** `internal/config/config_test.go`
- TestLoadMissingFile, TestLoadInvalidYAML, TestEnvOverride, TestDefaultsFill
- **Done.** 4 tests, all pass.

### Task 1.6 [DOCS] ✅ — Document result envelope schema
**File:** `docs/result-envelope.md` (new)
- runner.Result, scanner.ScanResult, storage.Record schemas
- Command→shape mapping for all 15 commands
- Schema stability guarantees
- **Done.** 197 lines.

### Task 1.7 [DOCS] ✅ — Add plugin output contract to plugin README
**File:** `plugins/python/README.md`
- Expected JSON Output section with per-plugin stdout format table
- Cross-link to docs/result-envelope.md
- **Done.**

**Phase 1 gate:** ✅ `go test ./...` passes (21 new tests). Both docs exist.

---

## Phase 2: Native Go Modules (DNS + Web) ✅

### Task 2.1 [CODE] ✅ — Create internal/dns package skeleton
**File:** `internal/dns/dns.go`
- Package doc, Record struct, Lookup func signature
- **Done.** Part of complete dns.go.

### Task 2.2 [CODE] ✅ — Implement DNS A/AAAA lookup
**File:** `internal/dns/dns.go`
- net.LookupIP → filter IPv4/IPv6
- **Done.**

### Task 2.3 [CODE] ✅ — Implement DNS MX lookup
**File:** `internal/dns/dns.go`
- net.LookupMX → records with host+pref
- **Done.**

### Task 2.4 [CODE] ✅ — Implement DNS NS/TXT/CNAME lookup
**File:** `internal/dns/dns.go`
- NS, TXT, CNAME via net.Lookup*
- Unsupported type returns error
- **Done.**

### Task 2.5 [CODE] ✅ — Implement DNS result formatting
**File:** `internal/dns/dns.go`
- FormatRecords (human-readable table)
- RecordsToJSON (structured JSON)
- **Done.**

### Task 2.6 [TEST] ✅ — Add DNS unit tests
**File:** `internal/dns/dns_test.go`
- TestLookupA, TestLookupMX, TestLookupInvalidDomain, TestFormatRecords, TestRecordsToJSON, TestLookupUnsupportedType
- **Done.** 8 tests, all pass.

### Task 2.7 [CODE] ✅ — Wire native DNS into CLI
**File:** `internal/cli/cli.go`
- `--types` flag (default: "A,AAAA,MX,NS,TXT,CNAME")
- `--plugin` fallback flag
- **Done.** `runNativeDNS()` implementation.

### Task 2.8 [CODE] ✅ — Create internal/web package skeleton
**File:** `internal/web/web.go`
- ProbeOptions, ProbeResult, TLSInfo types
- **Done.** Part of complete web.go.

### Task 2.9 [CODE] ✅ — Implement HTTP probing
**File:** `internal/web/web.go`
- Scheme auto-detection, GET request, title extraction
- Redirect chain recording
- **Done.**

### Task 2.10 [CODE] ✅ — Implement TLS certificate extraction
**File:** `internal/web/web.go`
- Subject, Issuer, DNSNames, dates, SHA256 fingerprint
- Self-signed detection
- **Done.**

### Task 2.11 [CODE] ✅ — Implement web result formatting
**File:** `internal/web/web.go`
- FormatProbe (human-readable)
- ProbeToJSON (structured JSON)
- **Done.**

### Task 2.12 [TEST] ✅ — Add web unit tests
**File:** `internal/web/web_test.go`
- TestProbeHTTP, TestProbeHTTPS, TestProbeRedirect, TestProbeTimeout, TestProbeInvalidURL, TestFormatProbe, TestProbeToJSON
- **Done.** 9 tests, all pass.

### Task 2.13 [CODE] ✅ — Wire native web into CLI
**File:** `internal/cli/cli.go`
- `--plugin`, `--insecure`, `--timeout` flags
- **Done.** `runNativeWeb()` implementation.

**Phase 2 gate:** ✅ `./ct dns cloudflare.com --json` works without Python. `./ct web example.com --json` works without Python. 17 new tests pass.

---

## Phase 3: LAN Crawler — Core Discovery ✅

### Task 3.1 [CODE] ✅ — Add CIDR parsing and validation
**File:** `internal/crawl/scope.go`
- ParseCIDR, ValidateScope, IsPrivateRange, CIDRContainsIP, ParseExcludedCIDRs
- **Done.**

### Task 3.2 [CODE] ✅ — Add local interface CIDR discovery
**File:** `internal/crawl/scope.go`
- DiscoverLocalCIDRs via net.InterfaceAddrs()
- Filter private, deduplicate
- **Done.**

### Task 3.3 [CODE] ✅ — Add host enumeration from CIDR
**File:** `internal/crawl/scope.go`
- EnumerateHosts with maxHosts cap
- **Done.**

### Task 3.4 [TEST] ✅ — Add scope unit tests
**File:** `internal/crawl/scope_test.go`
- TestParseCIDR, TestValidateScope, TestIsPrivateRange, TestCIDRContainsIP, TestParseExcludedCIDRs, TestEnumerateHosts, TestDiscoverLocalCIDRs
- **Done.** 22 tests, all pass.

### Task 3.5 [CODE] ✅ — Add `lan` parent command to CLI
**File:** `internal/cli/cli.go`
- lanCmd() with discover/crawl/map subcommands
- **Done.**

### Task 3.6 [CODE] ✅ — Implement LAN discovery subcommand
**File:** `internal/cli/cli.go`
- `--cidr`, `--local`, `--ports`, `--max-hosts`, `--timeout` flags
- MarkFlagsOneRequired("cidr", "local")
- **Done.** `runLanDiscover()` implementation.

### Task 3.7 [CODE] ✅ — Implement LAN discovery storage
**File:** `internal/crawl/discover.go`
- DiscoveryResult, HostResult types
- RunDiscovery using scanner.Scanner
- **Done.**

### Task 3.8 [TEST] ✅ — Add discovery integration tests
**File:** `internal/crawl/discover_test.go`
- TestRunDiscoveryLoopback, TestRunDiscoveryWithOpenPort, TestDiscoveryResultJSON, TestRunDiscoveryMaxHosts, edge case tests
- **Done.** 10 tests, all pass.

**Phase 3 gate:** ✅ `./ct lan discover --local --ports 80,443 --json` works. 32 crawl tests pass.

---

## Phase 4: LAN Crawler — HTTP Inspection ✅

### Task 4.1 [CODE] ✅ — Implement service inspector
**File:** `internal/crawl/inspect.go`
- InspectService, InspectHostServices
- Scheme auto-detection (port heuristics)
- TLS certificate extraction (SHA256 fingerprint, self-signed detection)
- FormatServiceInfo, ServiceInfoToJSON
- **Done.** 160 lines.

### Task 4.2 [TEST] ✅ — Add service inspector tests
**File:** `internal/crawl/inspect_test.go`
- TestInspectHTTPService, TestInspectHTTPSService, TestInspectClosedPort, TestInspectRedirect, TestFormatServiceInfo, TestServiceInfoToJSON, TestInspectHostServices
- **Done.** 7 tests, all pass.

### Task 4.3 [CODE] ✅ — Wire inspection into LAN discover
**File:** `internal/cli/cli.go`
- `--inspect` flag (default: true)
- `--insecure` flag for TLS verification skip
- Enriched JSON output with services array
- **Done.**

**Phase 4 gate:** ✅ `./ct lan discover --local --ports 80,443 --inspect --json` works. 37 crawl tests pass.

---

## Phase 5: Graph Model & Exporters ⬜

### Task 5.1 [CODE] ⬜ — Define graph data model
**File:** `internal/graph/model.go` (new)
- Types: ScanNode, HostNode, ServiceNode, PageNode, CertNode, LinkEdge, RedirectEdge
- UUID IDs, timestamps, JSON tags

### Task 5.2 [CODE] ⬜ — Create graph SQLite storage
**File:** `internal/graph/store.go` (new)
- Schema: lan_scans, lan_hosts, lan_services, lan_pages, lan_links, lan_certs
- OpenGraph, SaveScan, SaveHost, SaveService, GetScanHosts, GetHostServices

### Task 5.3 [TEST] ⬜ — Add graph store tests
**File:** `internal/graph/store_test.go` (new)
- TestSaveAndRetrieveScan, TestSaveHostDedup, TestHostServiceRelation,
  TestPageLinkGraph
- Temp SQLite DB per test

### Task 5.4 [CODE] ⬜ — Implement JSON graph exporter
**File:** `internal/graph/export.go` (new)
- ExportJSON(store, scanID) → structured JSON with nodes + edges

### Task 5.5 [CODE] ⬜ — Implement Markdown graph exporter
**File:** `internal/graph/export.go`
- ExportMarkdown(store, scanID) → summary tables per host

### Task 5.6 [CODE] ⬜ — Implement DOT graph exporter
**File:** `internal/graph/export.go`
- ExportDOT(store, scanID) → Graphviz digraph

### Task 5.7 [TEST] ⬜ — Add exporter tests
**File:** `internal/graph/export_test.go` (new)
- TestExportJSON, TestExportMarkdown, TestExportDOT
- Known input data, snapshot output

### Task 5.8 [CODE] ⬜ — Wire graph storage into LAN discovery
**File:** `internal/cli/cli.go`
- After discovery: save scan + hosts + services to graph store
- `--no-store` flag to skip persistence
- Graph DB path: `~/.cache/atlas-recon/lan-graph.db`

### Task 5.9 [CODE] ⬜ — Implement `lan map` subcommand
**File:** `internal/cli/cli.go`
- `--scan-id`, `--format json|markdown|dot` flags
- Default: most recent scan

**Phase 5 gate:** `./ct lan map --format markdown` displays stored graph. All graph tests pass.

---

## Phase 6: Bounded LAN Crawler ⬜

### Task 6.1 [CODE] ⬜ — Implement URL frontier
**File:** `internal/crawl/frontier.go` (new)
- Frontier type: maxDepth, maxPages, visited set, FIFO queue
- Add(), Next(), Visited(), Remaining()

### Task 6.2 [CODE] ⬜ — Implement URL normalizer
**File:** `internal/crawl/frontier.go`
- NormalizeURL: resolve relative, lowercase scheme+host, remove default ports, strip fragments

### Task 6.3 [CODE] ⬜ — Implement scope checker
**File:** `internal/crawl/frontier.go`
- IsInScope: same-host check, same-LAN CIDR check, allowExternal override

### Task 6.4 [TEST] ⬜ — Add frontier tests
**File:** `internal/crawl/frontier_test.go` (new)
- TestFrontierDepthLimit, TestFrontierPageLimit, TestFrontierVisitedDedup,
  TestNormalizeURL (fragments, default ports, trailing slashes),
  TestIsInScope (same host, different host, allowExternal)

### Task 6.5 [CODE] ⬜ — Implement page crawler
**File:** `internal/crawl/crawler.go` (new)
- CrawlPage: fetch, extract links from HTML (<a href>, <link href>)
- PageResult: URL, status, title, content_type, links_found, external_links_skipped

### Task 6.6 [CODE] ⬜ — Implement crawl orchestrator
**File:** `internal/crawl/crawler.go`
- RunCrawl: BFS, worker pool, progress tracking
- CrawlOptions: maxDepth, maxPages, scopeHosts, allowExternal, timeout, concurrency

### Task 6.7 [TEST] ⬜ — Add crawler tests
**File:** `internal/crawl/crawler_test.go` (new)
- TestCrawlSinglePage, TestCrawlDepth1, TestCrawlDepth2,
  TestCrawlExternalLink, TestCrawlMaxPages, TestCrawlRedirectExternal,
  TestCrawlTimeout, TestCrawlRedirectChain
- All use httptest servers with interlinked HTML

### Task 6.8 [CODE] ⬜ — Wire crawl into CLI
**File:** `internal/cli/cli.go`
- Implement lanCrawlCmd: --cidr/--local/--ports/--depth/--max-pages/--allow-external-links/--insecure
- Flow: discover → inspect → crawl → store graph → emit

### Task 6.9 [CODE] ⬜ — Add crawl progress callbacks
**File:** `internal/crawl/crawler.go`
- ProgressCallback func(ProgressEvent)
- CLI prints progress dots in non-JSON mode

**Phase 6 gate:** `./ct lan crawl --local --depth 1 --max-pages 10 --json` runs end-to-end.

---

## Phase 7: TUI Integration for LAN ⬜

### Task 7.1 [CODE] ⬜ — Add LAN commands to TUI menu
**File:** `internal/tui/tui.go`
- commandDef entries for: lan discover, lan crawl, lan map
- Native Go path (no Python script)

### Task 7.2 [CODE] ⬜ — Stream LAN discovery progress to TUI
**File:** `internal/tui/tui.go`
- LineCallback on RunDiscovery → streamLineMsg
- Show "Scanning X.X.X.X/24... 45/256 hosts, 3 services found"

### Task 7.3 [CODE] ⬜ — Stream LAN crawl progress to TUI
**File:** `internal/tui/tui.go`
- ProgressCallback → streamLineMsg
- Show "Crawling... Page 12/500, Depth 1/2"

### Task 7.4 [CODE] ⬜ — Add map view action to TUI
**File:** `internal/tui/tui.go`
- lan map renders Markdown summary in viewport

**Phase 7 gate:** TUI can launch `lan discover` and `lan crawl`, show live progress.

---

## Phase 8: Python Plugin Migration (Incremental) ⬜

### Task 8.1 [CODE] ⬜ — Native recon module
**File:** `internal/recon/recon.go` (new)
- EnumerateSubdomains via crt.sh API
- Wire into reconCmd() with --plugin fallback

### Task 8.2 [TEST] ⬜ — Recon tests
**File:** `internal/recon/recon_test.go` (new)
- httptest server with crt.sh-like JSON
- Dedup, wildcard filter, empty, API error

### Task 8.3 [CODE] ⬜ — Native report module
**File:** `internal/report/report.go` (new)
- GenerateReport: markdown, text, json-summary
- Wire into reportCmd()

### Task 8.4 [TEST] ⬜ — Report tests
**File:** `internal/report/report_test.go` (new)
- Snapshot tests per format

### Task 8.5 [CODE] ⬜ — Native geo module
**File:** `internal/geo/geo.go` (new)
- Geocode via Nominatim API
- Wire into geoCmd()

### Task 8.6 [CODE] ⬜ — Native OSINT module
**File:** `internal/osint/osint.go` (new)
- DomainLookup: crt.sh + WHOIS + DNS enrichment
- Wire into osintCmd()

### Tasks 8.7-8.12 [CODE/TEST] ⬜ — Remaining modules
- `internal/flight/` — OpenSky API client + httptest tests
- `internal/markets/` — Polymarket API client + httptest tests
- `internal/conflict/` — GDELT API client + httptest tests
- `internal/social/` — Bluesky API client + httptest tests
- `internal/phone/` — phone number parsing + tests
- `internal/war/` — ISW report scraping + offline snapshot tests

**Phase 8 gate:** All 10 Python-backed commands have native Go implementations. `--plugin` flag as fallback.

---

## Phase 9: Shared Infrastructure ⬜

### Task 9.1 [CODE] ⬜ — Rate limiter package
**File:** `internal/ratelimit/ratelimit.go` (new)
- Token bucket: NewLimiter, Wait with context

### Task 9.2 [TEST] ⬜ — Rate limiter tests
**File:** `internal/ratelimit/ratelimit_test.go` (new)
- Burst, rate, cancellation

### Task 9.3 [CODE] ⬜ — HTTP client factory
**File:** `internal/httpclient/client.go` (new)
- NewClient: timeout, user-agent, insecure
- User-agent: Atlas-Recon/<version> or Atlas-Recon/dev

### Task 9.4 [CODE] ⬜ — Version embedding
**File:** `cmd/ct/main.go` + `internal/version/version.go` (new)
- Build-time vars: Version, Commit, BuildDate
- `ct version` subcommand

### Task 9.5 [INFRA] ⬜ — Update .goreleaser.yaml
**File:** `.goreleaser.yaml`
- ldflags for version injection

### Task 9.6 [INFRA] ⬜ — Add `ct version` to CI smoke test
**File:** `.github/workflows/ci.yml`
- `./ct version` outputs non-empty version

### Task 9.7 [CODE] ⬜ — Add `--engine` flag to all plugin-backed commands
**File:** `internal/cli/cli.go`
- `--engine native|plugin` on each command
- Default: native if available, else plugin

**Phase 9 gate:** `./ct version` prints version info. `--engine` flag works.

---

## Dependency Graph

```
✅ Phase 1 (Tests/Contracts)
  ├─→ ✅ Phase 2 (Native DNS + Web)
  ├─→ ✅ Phase 3 (LAN Discovery) ──→ ✅ Phase 4 (HTTP Inspection)
                                        └─→ ⬜ Phase 5 (Graph) ──→ ⬜ Phase 6 (Crawler) ──→ ⬜ Phase 7 (TUI LAN)
                                                                                                  ⬜ Phase 8 (Plugin Migration)
                                                                                                  ⬜ Phase 9 (Shared Infra)
```

---

## Summary Statistics

| Phase | Tasks | Done | New Files | Status |
|---|---|---|---|---|
| Phase 1: Tests & Contracts | 7 | 7 | 2 | ✅ |
| Phase 2: Native DNS + Web | 13 | 13 | 4 | ✅ |
| Phase 3: LAN Discovery Core | 8 | 8 | 1 | ✅ |
| Phase 4: HTTP Inspection | 3 | 3 | 1 | ✅ |
| Phase 5: Graph Model & Exporters | 9 | 0 | 4 | ⬜ |
| Phase 6: Bounded Crawler | 9 | 0 | 2 | ⬜ |
| Phase 7: TUI LAN Integration | 4 | 0 | 0 | ⬜ |
| Phase 8: Plugin Migration | 12 | 0 | 10+ | ⬜ |
| Phase 9: Shared Infrastructure | 7 | 0 | 4 | ⬜ |
| **Total** | **72** | **31** | **12** | **43%** |

---

## Test Command

```bash
# Run all Go tests
go test ./... -v -count=1

# Run specific phases
go test ./internal/cli/... ./internal/dns/... ./internal/web/... -v
go test ./internal/crawl/... -v
go test ./internal/graph/... -v  # Phase 5

# Build check
go build ./...
```
