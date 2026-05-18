# Atlas-Recon

Atlas-Recon is an open-source command-line toolkit for authorized network scanning, DNS reconnaissance, OSINT, geospatial lookup, and web intelligence workflows.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-blue)](https://go.dev/)
[![Python Version](https://img.shields.io/badge/Python-3.9+-green)](https://www.python.org/)
[![Status](https://img.shields.io/badge/Status-In%20Development-orange)]()

## Legal Disclaimer

This tool is provided for educational use and authorized security testing only.

| Permitted | Prohibited |
| --- | --- |
| Testing systems you own or are authorized to assess | Unauthorized scanning or access |
| Security research with explicit permission | Credential theft, data harvesting, or abuse |
| Network diagnostics and troubleshooting | Attacks against third-party systems |
| Training in lawful environments | Any criminal or harmful activity |

By using Atlas-Recon, you are responsible for complying with applicable laws, obtaining written authorization where required, and limiting activity to approved targets.

## Current Status

Atlas-Recon is under active development. The current codebase includes:

- A Go CLI with 15 subcommands.
- A native concurrent TCP port scanner.
- 14 embedded Python plugin scripts for DNS, OSINT, web, geospatial, social, market, conflict, and reporting workflows.
- Automatic extraction of embedded plugins into the operating system's user cache directory.
- Automatic Python virtual environment creation for plugin package dependencies.
- Optional SQLite result storage.
- JSON output for automation.
- A terminal dashboard built with Bubble Tea.

The project still depends on a local Python 3.9+ runtime for Python-backed commands and on internet connectivity for plugins that call external APIs.

## Quick Start

### Prerequisites

- Go 1.24+ to build the CLI.
- Python 3.9+ for plugin-backed commands.
- A Unix-like shell for `build.sh`.

### Build

```bash
./build.sh
```

Manual build:

```bash
go build -o ct ./cmd/ct
```

### Basic Usage

```bash
# Display help
./ct --help

# Native TCP port scanning
./ct scan example.com --ports 22,80,443
./ct scan 203.0.113.10 --ports 1-1000

# DNS reconnaissance
./ct dns example.com

# OSINT and reconnaissance
./ct osint example.com
./ct recon example.com
./ct web https://example.com

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
```

### Run From Any Directory

The binary does not require the repository working directory after it is built.

```bash
cd /tmp
/path/to/ct scan example.com --ports 80,443
```

## Configuration

Atlas-Recon loads `configs/default.yaml` from the current working directory when present. You can also pass an explicit config file:

```bash
./ct --config /path/to/config.yaml scan example.com
```

Configuration shape:

```yaml
concurrency: 4
timeouts:
  command_seconds: 120
output:
  json: false
storage:
  enabled: true
  results_db: /path/to/results.db # Optional; defaults to the OS user cache directory.
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

- `CT_CONFIG`: default config path when `--config` is not provided.
- `CT_PYTHON`: Python executable used when config does not set `paths.python`.

## Commands

| Command | Purpose | Implementation |
| --- | --- | --- |
| `scan` | Concurrent TCP port scanning | Native Go |
| `dns` | DNS record lookup | Python plugin, `dnspython` |
| `osint` | Domain OSINT lookup | Python plugin |
| `recon` | Subdomain reconnaissance | Python plugin |
| `web` | Web endpoint checks | Python plugin |
| `report` | Generate reports from files/results | Python plugin |
| `dashboard` | Interactive terminal UI | Native Go |
| `results` | List, prune, or clear stored results | Native Go |
| `phone` | Phone-number OSINT links and metadata | Python plugin |
| `geo` | Geocoding and precision assessment | Python plugin |
| `conflict` | Conflict/event intelligence | Python plugin |
| `markets` | Prediction market sentiment | Python plugin |
| `social` | Bluesky/social pulse lookup | Python plugin |
| `flight` | OpenSky-backed aircraft lookup | Python plugin |
| `war` | War intelligence summaries | Python plugin |

## JSON Output

Commands support `--json` for structured output. Plugin-backed commands return the runner result envelope:

```json
{
  "id": "dns-00000000-0000-0000-0000-000000000000",
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

The native `scan` command emits a scan-specific JSON payload with host, port, state, service, and timing fields.

## Plugin System

- Source plugins live in `plugins/python/`.
- `scripts/sync_plugins.sh` copies source plugins into `internal/plugins/python/` for Go embedding.
- `build.sh` runs the sync step before compiling.
- Embedded plugins are extracted on demand into the OS user cache directory.
- Python package dependencies are installed into an Atlas-Recon-managed virtual environment under the OS user cache directory.
- Rebuilding is required after adding or changing plugin source files.

To add a plugin:

1. Add `plugins/python/your_plugin.py`.
2. Implement command-line argument parsing and stable stdout/JSON behavior.
3. Wire the plugin into `internal/cli/cli.go`.
4. Run `./build.sh`.
5. Add or update tests for the plugin behavior.

## Architecture

```text
cmd/ct/main.go              CLI entry point
internal/cli/               Cobra command routing
internal/config/            YAML/env/default configuration
internal/scanner/           Native concurrent TCP scanner
internal/plugins/           Embedded plugin discovery and extraction
internal/runner/            Python runner, dependency setup, job queue
internal/storage/           SQLite result storage
internal/tui/               Bubble Tea terminal dashboard
plugins/python/             Source Python plugins
scripts/                    Build and sync helpers
tests/                      Python plugin tests
```

## Performance Notes

| Area | Current behavior | Optimization direction |
| --- | --- | --- |
| Port scanning | Concurrent worker pool, capped at 1000 workers | Add per-scan timeout controls and better filtered/closed classification |
| Plugin startup | Cached extraction plus managed virtual environment | Consolidate repeated package checks and preflight dependency diagnostics |
| Result storage | SQLite with max-record pruning | Add indexed search/filter fields for large histories |
| External APIs | Direct plugin calls with basic retry behavior in selected plugins | Standardize retries, rate-limit handling, and user-agent headers across plugins |

## Review Conclusions

The current codebase is usable as a local CLI toolkit, but the highest-value improvements are:

- Add CLI-level tests for command wiring, especially plugin dependency behavior and JSON output.
- Standardize plugin output schemas so automation does not need per-plugin parsing logic.
- Replace interactive dependency installation prompts with explicit setup/preflight commands for non-interactive CI and scripted usage.
- Add a documented Python test setup because `pytest` is required for `tests/` but is not currently declared in a requirements file.
- Add release/version metadata generated at build time so status and binary version cannot drift from documentation.
- Review external API usage for rate limits, user-agent requirements, and graceful offline behavior.

## Troubleshooting

### Binary Not Found From Other Directories

```bash
go build -o ct ./cmd/ct
cd /tmp
/path/to/ct --help
```

### Plugin Extraction Or Dependency Issues

Atlas-Recon writes plugin cache files and the managed Python environment under the operating system's user cache directory. If plugin execution fails, remove the Atlas-Recon cache directory for your OS and rebuild.

Examples:

```bash
# Linux commonly uses:
rm -rf "${XDG_CACHE_HOME:-$HOME/.cache}/atlas-recon"

# macOS commonly uses:
rm -rf "$HOME/Library/Caches/atlas-recon"

./build.sh
```

### Low Geolocation Precision

```bash
./ct geo "Germany"                   # broad result
./ct geo "Berlin, Germany"           # better precision
./ct geo "Potsdamer Platz, Berlin"   # higher precision
```

### API Rate Limits

Some plugins call public APIs such as Nominatim, OpenSky, crt.sh, GDELT, or other third-party services. These services may throttle, fail, or change response formats. Use JSON output and retry conservatively.

## Development

```bash
go test ./...
python -m pytest -q
```

Before submitting changes:

- Run `gofmt` on Go files.
- Run Go tests for Go changes.
- Run Python tests for plugin changes.
- Keep plugin behavior stable for both human-readable and JSON output.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT License. See [LICENSE](./LICENSE).

Use responsibly, legally, and only against authorized targets.
