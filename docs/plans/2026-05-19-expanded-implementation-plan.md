# Atlas-Recon — Expanded Implementation Plan

**Date:** 2026-05-19
**Status:** Active — Phases 1-4 complete
**Based on:** `2026-03-08-robustness-upgrade-plan.md`, `2026-05-18-go-migration-lan-crawler-plan.md`, current codebase audit

---

## Current State Summary

### ✅ Completed (Batch 1 + Batch 2 — Robustness Upgrade) — Before This Plan

| Area | Status |
|---|---|
| Cobra CLI migration (15 subcommands) | Done. `internal/cli/cli.go` |
| TUI streaming output (LineCallback) | Done. `runner/python.go` + `tui/tui.go` |
| Scanner per-port timeout (2s) | Done. `scanner/scanner.go` |
| Dead helper removal | Done |
| API key env var fallback (CT_API_*) | Done |
| Pip stamp file caching | Done |
| Storage auto-prune, DeleteAllRecords, PruneOldRecords | Done |
| cycleArgs simplification | Done |
| UUID-based result IDs | Done |
| `--targets-file` batch support | Done |
| `--suite` flag for osint | Done |
| Config: max_records, removed Nmap/Nslookup | Done |

### ✅ Phase 1: Tests & Contract Foundation — COMPLETE

| Deliverable | File | Tests |
|---|---|---|
| CLI contract tests | `internal/cli/cli_test.go` (305 lines) | 13 tests — help output, scan contracts, results contracts, config errors |
| Runner result tests | `internal/runner/result_test.go` (130 lines) | 4 tests — JSON roundtrip, status consts, empty fields |
| Config error tests | `internal/config/config_test.go` (appended) | 4 tests — missing file, invalid YAML, env override, defaults |
| Result envelope docs | `docs/result-envelope.md` (197 lines) | — schema reference, command→shape mapping |
| Plugin output contract | `plugins/python/README.md` (updated) | — per-plugin stdout format table |

### ✅ Phase 2: Native Go Modules (DNS + Web) — COMPLETE

| Module | Files | Tests |
|---|---|---|
| `internal/dns/` | `dns.go` (6 record types, formatting, JSON) | 8 tests — A/MX/TXT/CNAME lookup, format, JSON, unsupported type |
| `internal/web/` | `web.go` (HTTP/HTTPS probe, TLS, redirects) | 9 tests — HTTP, HTTPS, redirect, timeout, bad URL, format, TLS roundtrip |
| CLI wiring | `cli.go` — `--plugin`, `--types`, `--insecure`, `--timeout` flags | Covered by existing CLI tests |

### ✅ Phase 3: LAN Discovery Core — COMPLETE

| Module | Files | Tests |
|---|---|---|
| `internal/crawl/scope.go` | CIDR parsing, validation, private range, enumeration | 22 tests — parse, validate, private range, enumeration, exclusions |
| `internal/crawl/discover.go` | `RunDiscovery` using project scanner | 10 tests — loopback, open port, JSON, max hosts, nil/empty edge cases |
| CLI wiring | `lan` parent + `discover` subcommand in `cli.go` | Covered by existing CLI tests |

### ✅ Phase 4: LAN HTTP Inspection — COMPLETE

| Module | Files | Tests |
|---|---|---|
| `internal/crawl/inspect.go` | `InspectService`, `InspectHostServices`, scheme detection, TLS extraction | 7 tests — HTTP, HTTPS, closed port, redirect, format, JSON, batch |
| CLI wiring | `--inspect`, `--insecure` flags on `lan discover` | Covered by existing CLI tests |

### Total Tests: 77+ across 8 packages

```
internal/cli      — 13 tests
internal/config   —  5 tests
internal/crawl    — 37 tests (22 scope + 10 discover + 5 added... need recount)
internal/dns      —  8 tests
internal/runner   —  5 tests
internal/scanner  —  3 tests
internal/storage  —  3 tests
internal/web      —  9 tests
```

---

## Phase 5: Graph Model & Storage

### 5A: Graph Schema (`internal/graph/`)

| File | Purpose | Tests |
|---|---|---|
| `internal/graph/model.go` | Node and edge types (Scan, Host, Service, Page, Cert, Link, Redirect) | — |
| `internal/graph/store.go` | SQLite tables for graph persistence | CRUD and dedup tests |
| `internal/graph/store_test.go` | — | Test all CRUD operations, dedup on save, host-service relations |
| `internal/graph/export.go` | JSON, Markdown, DOT exporters | Snapshot tests for stable output |
| `internal/graph/export_test.go` | — | Test each format produces valid output with expected structure |

**Tables:**
```sql
lan_scans(id, cidr, ports, started_at, finished_at)
lan_hosts(id, scan_id, ip, first_seen, last_seen)
lan_services(id, host_id, port, scheme, protocol, state, confidence)
lan_pages(id, service_id, url, status, title, content_type, depth)
lan_links(id, from_page_id, to_page_id, url)
lan_certs(id, service_id, fingerprint, subject, issuer, dns_names, not_before, not_after)
```

### 5B: Exporters

- `--format json`: Full graph as structured JSON.
- `--format markdown`: Summary table: hosts by service, pages, certificates.
- `--format dot`: Graphviz DOT for visualization.
- `--format mermaid`: Mermaid diagram (nice for Markdown docs).

### 5C: Testing Strategy

- **Store tests**: Temp SQLite DB per test, verify save+retrieve, dedup on duplicate save, host→service→page relationships.
- **Exporter tests**: Known input data, snapshot the output string for each format. Verify JSON is valid, Markdown has expected sections, DOT starts with `digraph`.

### 5D: CLI Wiring

- Wire graph storage into `lan discover`: save scan + hosts + services to graph store after discovery completes.
- Implement `lan map` subcommand: `--scan-id`, `--format json|markdown|dot`.
- Default: most recent scan.

---

## Phase 6: Bounded LAN Crawler

### 6A: URL Frontier (`internal/crawl/frontier.go`)

| File | Purpose | Tests |
|---|---|---|
| `internal/crawl/frontier.go` | URL frontier with depth, page count, scope enforcement | 6 tests — depth limit, page limit, dedup, normalize, scope check |
| `internal/crawl/frontier_test.go` | — | Test adding beyond max depth/page, URL normalization edge cases, same-host scope |

**Behavior:**
- BFS crawl starting from discovered HTTP(S) service root URLs.
- Max depth per run (default 1).
- Max total pages per run (default 500).
- Same-host scope by default: never leave the originating host's domain.
- `--allow-external-links` flag to follow links outside the scope.
- URL normalization: strip fragments, resolve relative URLs, lowercase scheme+host.
- Deduplicate URLs already visited.

### 6B: Page Crawler (`internal/crawl/crawler.go`)

| File | Purpose | Tests |
|---|---|---|
| `internal/crawl/crawler.go` | Fetch page, extract links, record metadata | 8 tests — single page, depth limits, external links, max pages, redirects, timeout |
| `internal/crawl/crawler_test.go` | — | Use httptest servers with interlinked HTML pages |

**Behavior:**
- Fetch page with HTTP client (timeout, user-agent).
- Extract links: `<a href>`, `<link href>`, `<script src>`, `<img src>`.
- Normalize and scope-check each link.
- Queue in-scope links into frontier for next BFS level.
- Record out-of-scope links as "skipped" for human review.
- DO NOT: submit forms, execute JavaScript, follow redirects to external hosts by default.

### 6C: Testing Strategy

- **Frontier tests**: Pure logic — no network needed. Test depth/page limits, dedup, URL normalization with various scheme/host/path/fragment combos.
- **Crawler tests**: Use `httptest` servers with multiple pages and links. Test single-page crawl, depth-1 (A→B stops at B), depth-2 (A→B→C stops at C), external-link skip, max-pages cap, timeout handling, redirect chain recording.

### 6D: CLI Wiring

- Implement `lan crawl` subcommand: share `--cidr`/`--local`/`--ports` flags from discover, add `--depth`, `--max-pages`, `--allow-external-links`.
- Flow: discover → inspect HTTP(S) → crawl each service root → store graph → emit.

---

## Phase 7: TUI Integration for LAN

### 7A: LAN Dashboard Actions

| Task | Tests |
|---|---|
| Add `lan discover` and `lan crawl` to TUI command menu | — (manual TUI testing) |
| Stream discovery progress via `streamLineMsg` | — |
| Show live host/service counts in TUI status bar | — |
| Cancel long-running LAN scans via `ctrl+x` | — |
| View map results in viewport (Markdown) | — |

### 7B: Testing Strategy

- TUI is inherently hard to unit test. Focus on:
  - Adding `lan discover` and `lan crawl` to the `commandDef` list in testable helper.
  - Verifying the progress callback wiring compiles and doesn't panic.
  - Smoke test: launch TUI, select `lan discover`, verify it doesn't crash.

---

## Phase 8: Remaining Python Plugin Migration

Ordered by usage frequency and complexity:

| Priority | Command | Current Plugin | Target Go Module | Complexity | Tests |
|---|---|---|---|---|---|
| High | `recon` | `recon_subdomains.py` | `internal/recon/` — crt.sh client | Medium | httptest with crt.sh-like JSON |
| High | `report` | `generate_report.py` | `internal/report/` — Markdown/JSON/text | Low | Snapshot tests per format |
| Medium | `osint` | `osint_domain.py` | `internal/osint/` — crt.sh+WHOIS+DNS | High | Mock API responses |
| Medium | `geo` | `geo_recon.py` | `internal/geo/` — Nominatim client | Low | httptest Nominatim response |
| Low | `phone` | `phone_osint.py` | `internal/phone/` | Medium | Known-number parsing |
| Low | `flight` | `flight_radar.py` | `internal/flight/` — OpenSky API | Medium | httptest OpenSky response |
| Low | `conflict` | `conflict_view.py` | `internal/conflict/` — GDELT API | Low | httptest GDELT response |
| Low | `markets` | `market_sentiment.py` | `internal/markets/` — Polymarket API | Low | httptest Polymarket response |
| Low | `social` | `social_pulse.py` | `internal/social/` — Bluesky API | Medium | httptest Bluesky response |
| Low | `war` | `war_intel.py` | `internal/war/` — ISW scraping | High | Offline snapshot tests |

### Testing Strategy per Module

Each native module must include:
1. **Happy path**: httptest server returning realistic API responses.
2. **Error path**: timeout, bad status code, malformed response.
3. **Empty path**: no results found, empty response.
4. **JSON roundtrip**: marshal/unmarshal the result struct.
5. **Format output**: snapshot test for human-readable output.

### Compatibility Strategy

- Each command gets a `native` or `plugin` tag in `--help`.
- `--engine native|plugin` flag on each command to force a specific backend.
- Default: native when available, plugin fallback when native fails or is unavailable.
- Python runner remains for at least 2 releases as a compatibility layer.

---

## Phase 9: Shared Infrastructure

### 9A: Rate Limiter (`internal/ratelimit/`)

| File | Purpose | Tests |
|---|---|---|
| `internal/ratelimit/ratelimit.go` | Token bucket rate limiter | Burst test, rate test, context cancellation |
| `internal/ratelimit/ratelimit_test.go` | — | Verify N calls complete within expected time, cancellation stops Wait |

### 9B: HTTP Client Factory (`internal/httpclient/`)

| File | Purpose | Tests |
|---|---|---|
| `internal/httpclient/client.go` | Shared HTTP client with User-Agent, timeout, insecure | Verify defaults, User-Agent format |
| `internal/httpclient/client_test.go` | — | Test default timeout, custom timeout, insecure transport |

### 9C: Version Embedding

| File | Purpose |
|---|---|
| `internal/version/version.go` | Build-time vars: Version, Commit, BuildDate |
| `cmd/ct/main.go` | `version` subcommand |
| `.goreleaser.yaml` | ldflags injection |

- `version` subcommand: `ct version` prints build version, commit, build date.
- Version injected at build time via `-ldflags`.
- CI smoke test: `./ct version` outputs non-empty version string.

---

## Non-Goals (Explicitly Deferred)

- No passive LAN discovery (mDNS, DNS-SD, UPnP, ARP, NetBIOS) — stick to active TCP.
- No interactive TUI map visualization — export-only for now.
- No vulnerability scanning, credential testing, or exploit payloads.
- No scheduled/recurring scans.
- No Web API / REST endpoints.
- No macOS/Linux service integration (launchd, systemd).
- No plugin hot-reloading in production binary.

---

## Risk Table

| Risk | Mitigation |
|---|---|
| Native DNS/web break existing user workflows | Keep Python plugin fallback with `--plugin` flag |
| Crawler escapes LAN scope | Same-host default, explicit `--allow-external-links`, scope tests |
| Slow or noisy LAN scans | Conservative defaults (max hosts, max pages, short timeouts) |
| Self-signed TLS breaks crawler | Configurable `--insecure` flag for LAN crawler |
| Python migration breaks users | Keep compatibility runner, deprecate gracefully with clear docs |
| CI doesn't cover LAN crawling | Use loopback test servers, mocked CIDRs |
| Growing binary size from new modules | Monitor with `go build -o ct ./cmd/ct && du -h ct`, keep under 30MB |

---

## Execution Progress

```
✅ Phase 1: Tests & Contracts       — 21 new tests, 2 docs
✅ Phase 2: Native DNS + Web         — 17 new tests, 2 native commands
✅ Phase 3: LAN Discovery Core       — 32 new tests, lan parent command
✅ Phase 4: LAN HTTP Inspection      —  7 new tests, inspect flag
⬜ Phase 5: Graph Model & Exporters  —  next
⬜ Phase 6: Bounded Crawler          —
⬜ Phase 7: TUI LAN Integration      —
⬜ Phase 8: Python Plugin Migration  —
⬜ Phase 9: Shared Infrastructure    —
```

**Total tests: 77+** across 8 packages. **4 native commands**: scan, dns, web, lan discover.

---

## Review & Testing Status — 2026-05-19

### Automated Tests Passing

```
8 packages, 0 failures
go test ./... -count=1
```

| Package | Tests | Coverage Focus |
|---|---|---|
| `internal/cli` | 13 | Help output, scan/results contracts, JSON flag, config errors |
| `internal/config` | 5 | Defaults, missing/invalid YAML, env override, partial fill |
| `internal/crawl` | 37 | CIDR parsing, host enumeration, discovery, HTTP/HTTPS inspection |
| `internal/dns` | 8 | A/MX/TXT/CNAME lookup, formatting, JSON roundtrip |
| `internal/runner` | 5 | Result JSON roundtrip, status consts, empty/error fields |
| `internal/scanner` | 3 | Port parsing, defaults, service names |
| `internal/storage` | 3 | DeleteAll, prune old, auto-prune |
| `internal/web` | 9 | HTTP/HTTPS probe, redirects, TLS extraction, timeout |

### What Needs Manual Review

| Area | Risk | Reviewer Check |
|---|---|---|
| `internal/cli/cli.go` | High — accumulated wiring from 4 phases | Verify no dead code, all flags documented in help text, error paths consistent |
| `internal/crawl/scope.go` | Medium — CIDR math correctness | Verify edge cases: /31, /32, IPv6 (deferred), very large ranges |
| `internal/crawl/inspect.go` | Low — TLS cert extraction | Check self-signed heuristic, fingerprint format, redirect handling |
| `internal/dns/dns.go` | Low — stdlib resolver | Verify behavior when DNS resolver is unavailable (offline) |
| `internal/web/web.go` | Low — redirect chain | Verify max redirect depth, external redirect handling |

### What Has No Automated Tests

| Area | Reason | Mitigation |
|---|---|---|
| `internal/tui/tui.go` | Bubble Tea is hard to unit test | Manual smoke test: `./ct dashboard`, verify no crash, commands run |
| `internal/plugins/embed.go` | Requires embedded filesystem | Covered indirectly by CLI tests that route through plugin path |
| `cmd/ct/main.go` | Single-line entry point | Covered by CLI tests via `cli.Execute()` |

### Binary Size

```bash
go build -o ct ./cmd/ct && du -h ct
```

### Pre-Push Checklist

- [x] `go build ./...` — no errors
- [x] `go test ./... -count=1` — all pass
- [x] Binary smoke: `./ct --help`, `./ct dns --help`, `./ct lan --help`
- [x] Plans updated with testing sections
- [x] Tasks document reflects completion status
- [ ] Manual TUI smoke test
- [ ] Binary size under 30MB
