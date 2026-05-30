# Atlas-Recon

Atlas-Recon is an open-source command-line toolkit for authorized network scanning, LAN discovery, web crawling, DNS reconnaissance, OSINT, and geospatial intelligence.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-blue)](https://go.dev/)
[![Python Version](https://img.shields.io/badge/Python-3.9+-green)](https://www.python.org/)
[![Tests](https://img.shields.io/badge/tests-77+-brightgreen)]()
[![Status](https://img.shields.io/badge/Status-In%20Development-orange)]()

## Legal Disclaimer

This tool is provided for educational use and authorized security testing only. The user is responsible for obtaining written authorization before testing any system they do not own.

| Permitted | Prohibited |
| --- | --- |
| Testing systems you own or are authorized to assess | Unauthorized scanning or access |
| Security research with explicit permission | Credential theft, data harvesting, or abuse |
| Network diagnostics and troubleshooting | Attacks against third-party systems |
| Training in lawful environments | Any criminal or harmful activity |

By using Atlas-Recon, you are responsible for complying with applicable laws, obtaining written authorization where required, and limiting activity to approved targets.

## Current Status

Atlas-Recon is under active development. The codebase includes:

- A Go CLI with 16 commands (15 original + `lan`).
- **Native Go** implementations for `scan`, `dns`, `web`, and `lan discover` — no Python required.
- Native concurrent TCP port scanner.
- LAN discovery and bounded HTTP(S) crawling with graph export.
- 14 embedded Python plugins for OSINT, geospatial, social, market, conflict, and reporting — with `--plugin` fallback on native commands.
- Automatic extraction of embedded plugins and Python virtual environment management.
- Optional SQLite result storage with auto-pruning.
- Structured JSON output for automation.
- A terminal dashboard built with Bubble Tea.

Python 3.9+ is only needed for plugin-backed commands. `scan`, `dns`, `web`, `dashboard`, `results`, and `lan` run entirely in Go.

## Quick Start

### Prerequisites

- Go 1.24+
- Python 3.9+ (only for plugin-backed commands)
- A Unix-like shell for `build.sh`

### Build

```bash
./build.sh
```

Or manually:

```bash
go build -o ct ./cmd/ct
```

### Basic Usage

```bash
# Help
./ct --help

# Native TCP port scanning (Go, no Python)
./ct scan example.com --ports 22,80,443
./ct scan [IP_ADDRESS] --ports 1-1000

# Native DNS lookup (Go, no Python)
./ct dns example.com
./ct dns example.com --types A,AAAA,MX --json

# Native web probe (Go, no Python)
./ct web example.com --json

# LAN discovery (Go, no Python)
./ct lan discover --local --ports 80,443,8080,8443
./ct lan discover --cidr [IP_ADDRESS]/24 --ports 80,443 --inspect --json
./ct lan crawl --cidr [IP_ADDRESS]/24 --ports 80,443 --depth 1 --max-pages 100 --json
./ct lan map --format markdown

# OSINT and reconnaissance (Python plugins)
./ct osint example.com
./ct recon example.com

# Geospatial and intelligence workflows
./ct geo "Paris, France"
./ct flight "London"
./ct conflict "regional security"
./ct markets "cybersecurity"
./ct social "security research"
./ct war "Ukraine"

# Interactive dashboard
./ct dashboard

# JSON output for automation
./ct scan example.com --ports 80,443 --json
./ct geo "Tokyo" --json

# Review stored results
./ct results --limit 10
./ct results --command dns --json
./ct results --clear
./ct results --older-than 30d
```

## Commands

| Command | Purpose | Backend |
| --- | --- | --- |
| `scan` | Concurrent TCP port scanning | Native Go |
| `dns` | DNS record lookup (A, AAAA, MX, NS, TXT, CNAME) | Native Go · `--plugin` for Python fallback |
| `web` | HTTP/HTTPS probing with TLS extraction | Native Go · `--plugin` for Python fallback |
| `lan discover` | LAN host and service discovery | Native Go |
| `lan crawl` | Bounded LAN web crawler | Native Go |
| `lan map` | Graph export (JSON, Markdown, DOT) | Native Go |
| `dashboard` | Interactive terminal UI | Native Go |
| `results` | List, prune, or clear stored results | Native Go |
| `osint` | Domain OSINT lookup | Python plugin |
| `recon` | Subdomain reconnaissance | Python plugin |
| `report` | Generate reports from results | Python plugin |
| `phone` | Phone-number OSINT | Python plugin |
| `geo` | Geocoding and precision assessment | Python plugin |
| `conflict` | Conflict/event intelligence (GDELT) | Python plugin |
| `markets` | Prediction market sentiment (Polymarket) | Python plugin |
| `social` | Bluesky/social pulse lookup | Python plugin |
| `flight` | OpenSky-backed aircraft lookup | Python plugin |
| `war` | War intelligence summaries (ISW) | Python plugin |

## JSON Output

Commands support `--json` for structured output. The canonical schema reference is [docs/result-envelope.md](./docs/result-envelope.md).

| Command | JSON shape |
| --- | --- |
| `scan` | `scanner.ScanResult` — host, ports, timing |
| `dns` (native) | `{"records": [dns.Record, ...]}` |
| `web` (native) | `web.ProbeResult` — status, headers, title, TLS |
| `lan discover` | `crawl.DiscoveryResult` — CIDR, hosts, open ports, services |
| `lan crawl` | `{"scan_id": "...", "pages": [...], "links": [...]}` |
| `results` | Array of `storage.Record` |
| Plugin commands | `runner.Result` — universal envelope with stdout/stderr/timing |

Plugin-backed commands return the `runner.Result` envelope:

```json
{
  "id": "dns-a1b2c3d4-...",
  "command": "python3",
  "args": ["/path/to/plugin.py", "example.com"],
  "started_at": "2026-01-21T10:00:00Z",
  "finished_at": "2026-01-21T10:00:05Z",
  "duration_ms": 5000,
  "exit_code": 0,
  "status": "success",
  "stdout": "command output here",
  "stderr": "",
  "error": ""
}
```

## Configuration

Atlas-Recon loads `configs/default.yaml` when present. Pass an explicit config file with `--config`:

```bash
./ct --config /path/to/config.yaml scan example.com
```

```yaml
concurrency: 4
timeouts:
  command_seconds: 120
output:
  json: false
storage:
  enabled: true
  results_db: /path/to/results.db  # defaults to OS cache directory
  max_records: 1000
paths:
  python: python3
  whois: whois
apikeys:
  nasa: ""
  shodan: ""
  opensky: ""
```

Environment overrides:

- `CT_CONFIG` — config path when `--config` is not set.
- `CT_PYTHON` — Python executable when config doesn't set `paths.python`.
- `CT_API_*` — API keys set via environment take precedence over config file values.

## Architecture

```text
cmd/ct/main.go              CLI entry point
internal/cli/               Cobra command routing
internal/config/            YAML/env/default configuration
internal/scanner/           Native concurrent TCP scanner
internal/dns/               Native DNS resolver (stdlib)
internal/web/               Native HTTP/HTTPS probe + TLS extraction
internal/crawl/             LAN discovery, service inspection, scope management
internal/graph/             LAN graph model, SQLite store, map exporters
internal/plugins/           Embedded plugin discovery and extraction
internal/runner/            Python runner, dependency setup, job queue
internal/storage/           SQLite result storage
internal/tui/               Bubble Tea terminal dashboard
plugins/python/             Source Python plugins
scripts/                    Build and sync helpers
tests/                      Python plugin tests
```

## Plugin System

- Source plugins live in `plugins/python/`.
- `scripts/sync_plugins.sh` copies source plugins into `internal/plugins/python/` for Go embedding.
- `build.sh` runs the sync step before compiling.
- Embedded plugins are extracted on demand into the OS user cache directory.
- Python package dependencies are installed into an Atlas-Recon-managed virtual environment under the OS user cache directory.
- Rebuild after adding or changing plugin source files.

To add a plugin:

1. Add `plugins/python/your_plugin.py`.
2. Implement command-line argument parsing and stable stdout/JSON behavior.
3. Wire the plugin into `internal/cli/cli.go`.
4. Run `./build.sh`.
5. Add or update tests.

## Development

```bash
go test ./...                    # Go tests (77+ across 8 packages)
python -m pytest -q              # Python plugin tests
go build ./...                   # Compile check
```

Before submitting changes:

- Run `gofmt` on Go files.
- Run Go tests for Go changes.
- Run Python tests for plugin changes.
- Keep plugin behavior stable for both human-readable and JSON output.

## Roadmap

| Phase | Status |
| --- | --- |
| CLI tests + result envelope docs | ✅ |
| Native DNS + Web modules | ✅ |
| LAN discovery + HTTP inspection | ✅ |
| Graph model + exporters (JSON/Markdown/DOT) | ✅ |
| Bounded LAN crawler | ✅ initial |
| TUI LAN integration | Next |
| Native replacements for remaining Python plugins | Planned |
| Rate limiter, HTTP client factory, version embedding | Planned |

See [docs/plans/](./docs/plans/) for detailed implementation plans and the [comprehensive tasks document](./docs/plans/2026-05-19-comprehensive-tasks.md).

## Troubleshooting

### Binary Not Found From Other Directories

```bash
go build -o ct ./cmd/ct
cd /tmp
/path/to/ct --help
```

### Plugin Extraction Or Dependency Issues

Remove the Atlas-Recon cache directory for your OS and rebuild:

```bash
# Linux
rm -rf "${XDG_CACHE_HOME:-$HOME/.cache}/atlas-recon"

# macOS
rm -rf "$HOME/Library/Caches/atlas-recon"

./build.sh
```

### Low Geolocation Precision

```bash
./ct geo "Germany"                   # broad
./ct geo "Berlin, Germany"           # better
./ct geo "Potsdamer Platz, Berlin"   # precise
```

### API Rate Limits

Plugins call public APIs (Nominatim, OpenSky, crt.sh, GDELT, etc.) which may throttle or change response formats. Use `--json` and retry conservatively.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT License. See [LICENSE](./LICENSE).

Use responsibly, legally, and only against authorized targets.
