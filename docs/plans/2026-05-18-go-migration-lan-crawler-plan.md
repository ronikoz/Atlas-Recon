# Atlas-Recon Go Migration and LAN Crawler — Implementation Plan

**Goal:** Move Atlas-Recon toward a Go-native core while adding an authorized LAN crawler that discovers local network services, inspects HTTP(S) surfaces, and exports a navigable network map.

**Architecture:** Treat the LAN crawler as the forcing function for a shared Go discovery layer. New native modules should live under `internal/` and expose typed results that can be reused by CLI commands, storage, reports, and the TUI. Python plugins remain supported during migration but should become compatibility extensions instead of the primary runtime path.

**Tech Stack:** Go 1.24, cobra, Bubble Tea, modernc/sqlite, Go standard library networking/HTTP/TLS packages.

**Run all tests with:**

```bash
go test ./...
python3 -m pytest -q
go build ./...
```

---

## Principles

- Keep existing CLI commands backward compatible unless a breaking change is explicitly planned.
- Prefer Go-native implementations for deterministic network, DNS, HTTP, storage, and reporting behavior.
- Keep scanning and crawling conservative by default: explicit target scope, bounded concurrency, bounded depth, and clear authorization language.
- Store facts separately from inferences. Use confidence fields where detection is heuristic.
- Make crawler output useful for both humans and automation: JSON graph first, text/Markdown/DOT exports after.

---

## Track 1: Migrate More Functionality To Go

### Target End State

```text
internal/dns/       Native DNS lookup and enrichment
internal/web/       HTTP(S), TLS, headers, titles, redirects, technology hints
internal/recon/     Subdomain and target expansion
internal/osint/     Native API clients where practical
internal/crawl/     LAN/service crawler and page traversal
internal/graph/     Network graph model and exporters
internal/report/    Native report generation
internal/plugins/   Compatibility layer for remaining Python plugins
```

### Migration Order

1. **Contract foundation**
   - Define a shared result envelope for native and plugin-backed commands.
   - Add CLI tests for `--json`, command errors, command routing, and result storage.
   - Keep current Python plugin commands working while native replacements are added.

2. **Low-risk native replacements**
   - Replace DNS lookup behavior with `internal/dns` using Go resolver APIs.
   - Replace web checks with `internal/web` using `net/http`, `crypto/tls`, and redirect handling.
   - Replace report generation with `internal/report` for Markdown, JSON, and text output.
   - Replace simple subdomain validation/expansion helpers with `internal/recon`.

3. **Medium-complexity native modules**
   - Split `osint_domain.py` behavior into native clients for crt.sh, DNS enrichment, and WHOIS fallback.
   - Move geocoding API behavior into a Go client with configured timeouts and rate limits.
   - Move API-backed intelligence plugins only after their response schemas are stable.

4. **Compatibility cleanup**
   - Mark each command as `native`, `plugin`, or `deprecated-plugin`.
   - Keep Python runner optional for plugin extension and legacy compatibility.
   - Document plugin extension points separately from core command behavior.

### Acceptance Criteria

- Native DNS and web commands do not require Python.
- JSON output for native and plugin-backed commands is documented and covered by tests.
- Python-backed commands still work through the compatibility runner.
- README and plugin docs distinguish native commands from plugin commands.
- CI runs Go tests, Python plugin tests, and a built-binary smoke check.

---

## Track 2: Authorized LAN Crawler and Mapper

### Definition

The LAN crawler discovers hosts and services inside explicitly authorized local network ranges, inspects reachable HTTP(S) services, crawls bounded page depth, and emits a graph of networks, hosts, services, pages, links, redirects, and certificates.

This is not an exploit scanner, credential tester, or vulnerability attack tool.

### Proposed CLI

```bash
# Discover live services in an explicit CIDR.
./ct lan discover --cidr 192.168.1.0/24 --ports 80,443,8080,8443

# Auto-detect local interface CIDRs, then crawl conservatively.
./ct lan crawl --local --depth 1 --max-pages 500

# Crawl an explicit CIDR with deeper inspection.
./ct lan crawl --cidr 192.168.1.0/24 --ports 80,443,8080,8443 --depth 2 --max-pages 1000

# Export the last map.
./ct lan map --format json
./ct lan map --format markdown
./ct lan map --format dot > lan.dot
```

### Safety Defaults

- Require one of `--cidr`, `--target-file`, or `--local`.
- Default ports: `80,443,8080,8443`.
- Default depth: `1`.
- Default max pages: `500`.
- Default per-host timeout: short and configurable.
- Default concurrency: conservative and tied to config.
- Support `--exclude` CIDRs and hosts.
- Do not crawl external hosts unless `--allow-external-links` is explicitly set.
- Do not submit forms, brute force credentials, run exploit payloads, or bypass authentication.

---

## Data Model

### Graph Nodes

```text
Network  cidr, interface, source
Host     ip, names, mac_optional, first_seen, last_seen
Service  host_ip, port, scheme, protocol, state, confidence
Page     url, status, title, content_type, depth
Cert     fingerprint, subject, issuer, dns_names, not_before, not_after
```

### Graph Edges

```text
Network  --contains-->    Host
Host     --exposes-->     Service
Service  --serves-->      Page
Page     --links_to-->    Page
Page     --redirects_to--> Page
Service  --uses_cert-->   Cert
Host     --resolves_as-->  DNS name
```

### Storage

Start with SQLite tables under the existing storage package or a new `internal/graph` package:

```text
lan_scans
lan_hosts
lan_services
lan_pages
lan_links
lan_certs
```

Keep the schema append-friendly so repeated crawls can show deltas later.

---

## Implementation Milestones

### Milestone 1: Native LAN Discovery

**Files likely touched:**
- `internal/cli/cli.go`
- `internal/scanner/scanner.go`
- `internal/graph/` or `internal/lan/`
- `internal/storage/`
- `README.md`

**Tasks:**
1. Add `lan` parent command with `discover`, `crawl`, and `map` subcommands.
2. Add CIDR parsing and local interface CIDR discovery.
3. Reuse or extend the native scanner for host/port probing.
4. Emit JSON for discovered hosts and services.
5. Store discovery runs in SQLite.

**Acceptance Criteria:**
- `./ct lan discover --cidr <range> --ports 80,443` runs without Python.
- CIDR parsing rejects invalid or overly broad ranges unless explicitly forced.
- Discovery output is deterministic enough for snapshot tests.
- Unit tests cover CIDR parsing, port parsing, exclusion matching, and discovery result serialization.

### Milestone 2: HTTP(S) Service Inspection

**Files likely touched:**
- `internal/web/`
- `internal/crawl/`
- `internal/cli/cli.go`

**Tasks:**
1. Add HTTP and HTTPS probing for discovered services.
2. Capture status code, headers, title, redirects, TLS certificate metadata, and response timing.
3. Add scheme detection where a port may serve either HTTP or HTTPS.
4. Persist service inspection facts.

**Acceptance Criteria:**
- `lan discover` identifies HTTP(S) services and basic metadata.
- TLS certificate parsing handles self-signed and expired certificates without failing the scan.
- Redirects are recorded but external redirects are not crawled by default.
- Unit tests use local `httptest` and TLS test servers.

### Milestone 3: Bounded LAN Crawler

**Files likely touched:**
- `internal/crawl/`
- `internal/graph/`
- `internal/storage/`
- `internal/cli/cli.go`

**Tasks:**
1. Implement URL frontier with depth, page, host, and same-scope limits.
2. Parse HTML links and normalize URLs.
3. Enforce same-host or same-LAN scope by default.
4. Record page nodes and link/redirect edges.
5. Add cancellation and timeout handling.

**Acceptance Criteria:**
- `./ct lan crawl --cidr <range> --depth 1 --max-pages 100 --json` emits a graph.
- Crawler never follows external links unless explicitly allowed.
- Crawler respects max depth and max pages.
- Unit tests cover URL normalization, scope checks, depth limits, and link extraction.

### Milestone 4: Map Exporters

**Files likely touched:**
- `internal/graph/`
- `internal/report/`
- `internal/cli/cli.go`

**Tasks:**
1. Add JSON graph export.
2. Add Markdown summary export.
3. Add DOT export for Graphviz.
4. Optionally add Mermaid export for lightweight docs rendering.

**Acceptance Criteria:**
- `./ct lan map --format json` emits machine-readable graph data.
- `./ct lan map --format markdown` emits a concise human report.
- `./ct lan map --format dot` renders network/service/page relationships.
- Exporters have unit tests for stable output.

### Milestone 5: TUI Integration

**Files likely touched:**
- `internal/tui/tui.go`
- `internal/runner/queue.go`
- `internal/graph/`

**Tasks:**
1. Add LAN discovery/crawl actions to the dashboard.
2. Stream discovery and crawl progress into the existing TUI log model.
3. Show summary counts: hosts, services, pages, errors, skipped external links.
4. Add a map/export action.

**Acceptance Criteria:**
- TUI can launch a bounded LAN discovery without invoking Python.
- Long-running scans remain cancellable.
- Progress output does not block the Bubble Tea event loop.

---

## Verification Plan

### Unit Tests

- CIDR parsing and exclusions.
- Port list parsing and scan option validation.
- HTTP title/header/TLS extraction.
- URL normalization and scope checks.
- Graph node/edge deduplication.
- Exporter output stability.

### Integration Tests

- Use `httptest` to stand up local HTTP and HTTPS services.
- Run crawler against loopback test servers.
- Verify graph contains expected hosts, services, pages, links, and cert metadata.

### CLI Smoke Tests

```bash
go test ./...
go build -o /tmp/atlas-recon-ct ./cmd/ct
/tmp/atlas-recon-ct lan discover --cidr 127.0.0.1/32 --ports 1 --json
/tmp/atlas-recon-ct --help
```

### Manual Validation

On an authorized LAN only:

```bash
./ct lan discover --local --ports 80,443,8080,8443
./ct lan crawl --local --depth 1 --max-pages 100
./ct lan map --format markdown
```

---

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Accidental broad scanning | Require explicit CIDR/local mode, add max-host caps, reject broad public ranges by default |
| Crawler escapes LAN scope | Same-LAN/same-host default, explicit `--allow-external-links` for exceptions |
| Slow or noisy scans | Conservative concurrency, short timeouts, per-run caps |
| Ambiguous service detection | Store confidence and raw evidence, avoid overclaiming |
| Python migration breaks users | Keep plugin compatibility until native commands reach parity |
| Result schema drift | Add contract tests and document native/plugin JSON envelopes |

---

## Recommended Execution Sequence

1. Add result envelope and CLI contract tests.
2. Add `internal/web` and `internal/dns` native modules.
3. Add `lan discover` with CIDR parsing and scanner reuse.
4. Add graph storage tables and JSON graph output.
5. Add HTTP(S) inspection.
6. Add bounded crawler.
7. Add map exporters.
8. Add TUI integration.
9. Migrate remaining Python plugins in priority order.
10. Update README and release notes after each milestone.

---

## First Delivery Slice

Deliver this first:

```bash
./ct lan crawl --cidr 192.168.1.0/24 --ports 80,443,8080,8443 --depth 1 --max-pages 500 --json
```

Minimum behavior:

- Discover candidate hosts via native TCP scanning.
- Identify HTTP(S) services.
- Crawl one level deep.
- Enforce LAN scope and max page limits.
- Store scan graph in SQLite.
- Emit JSON graph.

Minimum tests:

- CIDR parsing.
- Exclusion matching.
- HTTP(S) service inspection with `httptest`.
- URL scope and depth limits.
- Graph serialization.

---

## Follow-Up Decisions

- Whether LAN crawling should auto-detect all private interfaces by default or require explicit `--local`.
- Whether to include passive LAN discovery protocols such as mDNS, DNS-SD, UPnP, ARP, or NetBIOS.
- Whether map visualization should stay export-only or become an interactive TUI view.
- Whether native OSINT API clients should share one rate-limit/retry package.
- Whether Python plugin compatibility should remain indefinitely or be deprecated after native parity.
