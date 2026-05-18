# Atlas-Recon — Implementation Plan

**2026-05-19** · **Status:** Phases 1-4 complete, Phase 5 next
**Commit:** `ddc6a55` · **Binary:** 16MB · **Tests:** 77+ across 8 packages
**License:** MIT · **Use:** Educational and authorized testing only

---

## Project Principles

Atlas-Recon is an **open-source educational toolkit**. Every feature is built with these invariants:

- **Authorization first**: All scanning, crawling, and probing requires explicit written authorization on targets you don't own. The README's [Legal Disclaimer](../README.md#legal-disclaimer) applies to every command.
- **No attack capabilities**: This is a reconnaissance and discovery tool — not an exploit scanner, credential tester, or vulnerability attack framework.
- **Conservative defaults**: LAN crawling defaults to same-host scope, bounded depth, and explicit CIDR requirements. No feature ever defaults to scanning public ranges.
- **Contributor-ready**: MIT licensed, buildable with `./build.sh`, testable with `go test ./...`, and documented for external contributors. See [CONTRIBUTING.md](../CONTRIBUTING.md).

---

## Quick Summary

Atlas-Recon now has **4 native Go commands** (scan, dns, web, lan discover) — dns and web no longer need Python. The LAN crawler foundation is laid: CIDR parsing, TCP discovery, HTTP(S) service inspection. Next: graph storage and exporters.

| Done | Phase | What shipped |
|---|---|---|
| ✅ | 1: Tests & Contracts | 21 tests, `docs/result-envelope.md`, plugin output contract |
| ✅ | 2: Native DNS + Web | `internal/dns/`, `internal/web/` — `--plugin` fallback preserved |
| ✅ | 3: LAN Discovery | `internal/crawl/scope.go`, `discover.go` — `ct lan discover --local` |
| ✅ | 4: HTTP Inspection | `internal/crawl/inspect.go` — `--inspect` flag, TLS extraction |
| ⬜ | **5: Graph & Exporters** | `internal/graph/` — SQLite store + JSON/Markdown/DOT exporters |
| ⬜ | 6: Bounded Crawler | URL frontier, page crawler, scope enforcement |
| ⬜ | 7: TUI LAN | Dashboard integration for lan commands |
| ⬜ | 8: Plugin Migration | Native replacements for remaining 10 Python plugins |
| ⬜ | 9: Infrastructure | Rate limiter, HTTP client factory, version embedding |

---

## Verify This Delivery

```bash
go build ./...                    # expect: no errors
go test ./... -count=1            # expect: 8 packages, all pass
./ct --help                       # 16 commands (15 original + lan)
./ct dns cloudflare.com --json    # native, no Python
./ct web example.com --json       # native, no Python
./ct lan discover --help           # --cidr, --local, --inspect flags
```

---

## What's Next: Phase 5 — Graph Model & Exporters

### Files to create

| File | Purpose |
|---|---|
| `internal/graph/model.go` | Node/edge types: Scan, Host, Service, Page, Cert, Link, Redirect |
| `internal/graph/store.go` | SQLite persistence: `lan_scans`, `lan_hosts`, `lan_services`, `lan_pages`, `lan_links`, `lan_certs` |
| `internal/graph/store_test.go` | CRUD, dedup, host→service→page relationship tests |
| `internal/graph/export.go` | `ExportJSON`, `ExportMarkdown`, `ExportDOT` |
| `internal/graph/export_test.go` | Snapshot tests per format |

### CLI surface

```bash
ct lan map --scan-id <id> --format json|markdown|dot
ct lan discover ... --no-store   # skip persistence
```

### Test strategy

- **Store**: temp SQLite per test, verify save→retrieve, dedup, foreign-key relationships
- **Exporters**: known input → snapshot output per format (JSON valid, Markdown has sections, DOT starts with `digraph`)

---

## Remaining Phases (Reference)

### Phase 6: Bounded Crawler

URL frontier with BFS, depth/page limits, **same-host scope by default**. `ct lan crawl --depth 1 --max-pages 500`. The crawler never leaves authorized LAN boundaries unless `--allow-external-links` is explicitly set. Tests via `httptest` servers with interlinked HTML. No form submission, no JavaScript execution, no credential testing.

### Phase 7: TUI Integration

Add `lan discover`/`lan crawl` to dashboard menu. Stream progress via `streamLineMsg`. Manual smoke test.

### Phase 8: Plugin Migration

Native Go replacements for recon, report, osint, geo, phone, flight, conflict, markets, social, war. Each gets `--plugin` fallback. httptest-based tests per API client.

### Phase 9: Shared Infrastructure

Rate limiter (`internal/ratelimit/`), HTTP client factory (`internal/httpclient/`), `ct version` with build-time vars.

---

## Non-Goals

- No passive LAN discovery (mDNS, UPnP, ARP)
- No interactive TUI graph visualization
- No vulnerability scanning, credential testing, or exploit payloads
- No scanning of targets without authorization — authorization is the user's responsibility
- No scheduled scans or REST API

---

## Review & Testing Checklist

### Automated

- [x] `go build ./...` — no errors
- [x] `go test ./... -count=1` — 8 packages pass
- [x] Binary smoke: `./ct --help`, `./ct lan --help`
- [x] Binary size 16MB (cap: 30MB)

### Manual (reviewer to verify)

- [ ] **`internal/cli/cli.go`** (high risk) — accumulated wiring. Check: no dead code, all flags in help text, error paths consistent
- [ ] **`internal/crawl/scope.go`** — CIDR edge cases: /31, /32, large ranges
- [ ] **`internal/crawl/inspect.go`** — TLS self-signed heuristic, fingerprint format
- [ ] **`internal/dns/dns.go`** — behavior when DNS resolver offline
- [ ] **TUI smoke test** — `./ct dashboard`, verify no crash

### Not covered by automated tests

| Area | Reason | Mitigation |
|---|---|---|
| `internal/tui/tui.go` | Bubble Tea | Manual dashboard smoke test |
| `internal/plugins/embed.go` | Embedded FS | Covered by CLI tests that route through plugin path |

---

## Risk Table

| Risk | Mitigation |
|---|---|
| DNS/web break existing workflows | `--plugin` flag preserves Python fallback |
| Crawler escapes LAN scope | Same-host default, `--allow-external-links` opt-in |
| Slow/noisy LAN scans | Conservative defaults (256 hosts, 500 pages, short timeouts) |
| Self-signed TLS | `--insecure` flag |
| Python migration breaks users | Compatibility runner stays for 2+ releases |
