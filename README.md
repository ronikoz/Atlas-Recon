# CLI Tools

Multi-command security toolkit for scanning, DNS/OSINT, recon, and web testing. The core CLI is written in Go for speed and cross-platform distribution, with Python plugins for specialized tasks.

## Features
- Go CLI with subcommands: `scan`, `dns`, `osint`, `recon`, `web`, `report`, `dashboard`
- TUI dashboard (Bubble Tea)
- Python plugin runner with JSON result capture
- Dependency checks with user consent and OS-aware installers
- Config file support (`configs/default.yaml`)

## Quick start
```bash
# build (requires Go)
go build -o ct ./cmd/ct

# run
./ct scan example.com --ports 80,443
./ct dns example.com
./ct osint example.com
./ct recon example.com
./ct dashboard
```

## Configuration
Default config: `configs/default.yaml`

Override config path:
```bash
CT_CONFIG=/path/to/config.yaml ./ct scan example.com
# or
./ct --config /path/to/config.yaml scan example.com
```

Config keys:
- `concurrency`: job queue worker count
- `timeouts.command_seconds`: max runtime for a command
- `output.json`: default JSON output
- `paths.python`: Python interpreter path
- `paths.nmap`, `paths.nslookup`, `paths.whois`: tool path overrides

## Output
Use `--json` on commands to emit a JSON result schema that includes:
- `id`, `command`, `args`, `started_at`, `finished_at`, `duration_ms`, `exit_code`, `status`, `stdout`, `stderr`, `error`

## Plugins
Python plugins live in `plugins/python` and are executed via the Go runner. Current plugins:
- `scan_nmap.py` (requires `nmap`)
- `dns_lookup.py` (uses `dnspython` for robust DNS queries; see `plugins/python/README.md`)
- `osint_domain.py` (requires `whois`)
- `osint_suite.py` (multi-source OSINT suite; see `plugins/python/README_osint.md`)
- `recon_subdomains.py` (uses crt.sh)

## Notes
- Missing dependencies will prompt for consent and attempt installation using brew/apt/dnf/pacman/winget/choco.
- Network access is required for crt.sh-based recon.

## Roadmap
- Wire job queue into the TUI dashboard
- Add structured parsing for plugin outputs
- Add report generation and results storage

## Contributing
Contributions are welcome. Please see `CONTRIBUTING.md` for guidelines.

## License
MIT. See `LICENSE`.


Signed-off-by: ronikoz
