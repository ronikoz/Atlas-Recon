# Python Plugins

This directory contains the source Python plugins embedded into the Atlas-Recon Go binary.

`./build.sh` copies these files into `internal/plugins/python/` before compiling so the binary can extract and run them from the operating system's user cache directory.

## Dependency Handling

The Go CLI creates a managed Python virtual environment for plugin package dependencies. For direct plugin development, install the dependencies needed by the plugin you are editing into your own virtual environment.

Example for DNS lookup development:

```bash
python -m venv .venv
. .venv/bin/activate
python -m pip install dnspython
python plugins/python/dns_lookup.py example.com --types A,MX,TXT --json
```

## Direct Usage Examples

```bash
python plugins/python/dns_lookup.py example.com --types A,MX,TXT --json
python plugins/python/dns_lookup.py --file domains.txt --types A,AAAA,NS --csv
python plugins/python/dns_lookup.py 8.8.8.8 --types PTR
```

Prefer invoking plugins through `./ct` for normal use so configuration, dependency setup, result storage, and JSON handling are consistent.

## Expected JSON Output

When invoked through `./ct <command> --json`, every plugin-backed command emits output wrapped in the `runner.Result` envelope. The CLI handles the envelope; plugins write plaintext (or their own JSON) to stdout.

**Wrapper shape:**

```json
{
  "id": "<prefix>-<uuid>",
  "command": "python3",
  "args": ["<script-path>", "<target>", "<flags...>"],
  "started_at": "<RFC 3339>",
  "finished_at": "<RFC 3339>",
  "duration_ms": <integer>,
  "exit_code": <integer>,
  "status": "success" | "failed",
  "stdout": "<plugin output>",
  "stderr": "<plugin stderr>",
  "error": "<runner error, omitted if empty>"
}
```

**Key points:**

- `stdout` contains whatever the plugin printed. Currently all plugins emit human-readable plaintext by default. Some plugins support their own `--json` flag that changes `stdout` to structured JSON (e.g., `phone_osint.py --json`, `conflict_view.py --json`).
- `stderr` is typically empty on success. It captures warnings, diagnostics, and Python tracebacks on failure.
- `status` is `"success"` when `exit_code` is 0, and `"failed"` otherwise.
- Multi-target runs (`--targets-file`) emit a JSON **array** of Result objects.

For the authoritative envelope schema, including `scanner.ScanResult` (native `scan` command) and `storage.Record` (`results` command), see [`docs/result-envelope.md`](../docs/result-envelope.md).

**Per-plugin deviations:**

| Plugin | `stdout` in `--json` mode | Notes |
|---|---|---|
| `dns_lookup.py` | Plaintext table | Records formatted as `TYPE  name  value` |
| `osint_domain.py` | Plaintext sections | crt.sh certs, WHOIS, DNS enrichment |
| `osint_suite.py` | Plaintext per-category | Each `--category` prints a header + results block |
| `recon_subdomains.py` | Plaintext list | One subdomain per line |
| `web_check.py` | Plaintext status | HTTP status, headers, redirect chain |
| `phone_osint.py` | Structured JSON | When `--json` is passed to the plugin itself |
| `geo_recon.py` | Plaintext summary | Lat/lon, map links, precision |
| `conflict_view.py` | Structured JSON | When `--json` is passed to the plugin itself |
| `market_sentiment.py` | Plaintext markets | Polymarket event summaries |
| `social_pulse.py` | Plaintext posts | Bluesky search results |
| `flight_radar.py` | Plaintext flights | OpenSky state vectors |
| `war_intel.py` | Structured JSON | When `--json` is passed to the plugin itself |
| `generate_report.py` | N/A (writes to file) | Report content written to `--output` file, stdout has status |
| `scan_nmap.py` | Plaintext scan | Used by TUI; CLI uses native Go scanner instead |

Plugins that accept their own `--json` flag (`phone_osint.py`, `conflict_view.py`, `war_intel.py`) will emit structured JSON inside `stdout` when that flag is passed. The outer envelope remains `runner.Result` regardless.
