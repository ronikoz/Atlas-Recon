# Result Envelope — Schema Reference

Canonical JSON output schemas for Atlas-Recon commands. Every command that supports `--json` emits one of the shapes documented below.

---

## `runner.Result` — Universal Plugin Envelope

**Package:** `internal/runner`
**Go type:** `runner.Result`

Used by Python-plugin-backed commands (`osint`, `recon`, `phone`, `geo`, `conflict`, `markets`, `social`, `flight`, `war`, `report`) and by native commands when explicitly run with `--plugin` (`dns --plugin`, `web --plugin`, or either command with `--targets-file`). The CLI wraps each plugin invocation in this envelope so automation has a single stable outer shape regardless of which plugin ran.

### Fields

| Field | JSON key | Type | Description |
|---|---|---|---|
| `ID` | `id` | string | UUID in `{prefix}-{uuid}` form (e.g. `dns-a1b2c3d4-...`). Unique per invocation. Always present. |
| `Command` | `command` | string | The Python executable used (`python3` or path). Always present. |
| `Args` | `args` | array of strings | Full argument list passed to the subprocess: `[script_path, target, flags...]`. Always present. |
| `StartedAt` | `started_at` | string (RFC 3339) | Wall-clock time when the subprocess was started. Always present. |
| `FinishedAt` | `finished_at` | string (RFC 3339) | Wall-clock time when the subprocess exited. Always present. |
| `DurationMs` | `duration_ms` | integer | Wall-clock duration in milliseconds. Computed as `finished_at - started_at`. Always present. |
| `ExitCode` | `exit_code` | integer | OS exit code of the subprocess. `0` on success, non-zero on failure. Always present. |
| `Status` | `status` | string | One of `"success"` or `"failed"`. Derived from exit code. Always present. |
| `Stdout` | `stdout` | string | Full captured standard output of the plugin (plaintext or plugin-specific JSON). Empty string when nothing was printed. |
| `Stderr` | `stderr` | string | Full captured standard error of the plugin. Typically empty (`""`) on success. |
| `Error` | `error` | string | Go-level error message when the runner itself failed (e.g. Python not found, timeout). Omitted from JSON when empty (`omitempty`). |

### Example — Successful invocation

```json
{
  "id": "dns-a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "command": "python3",
  "args": [
    "/Users/me/Library/Caches/atlas-recon/plugins/dns_lookup.py",
    "example.com",
    "--types",
    "A,MX"
  ],
  "started_at": "2026-05-19T10:00:00Z",
  "finished_at": "2026-05-19T10:00:03Z",
  "duration_ms": 3127,
  "exit_code": 0,
  "status": "success",
  "stdout": "A     example.com.  93.184.216.34\nMX    example.com.  mail.example.com.\n",
  "stderr": ""
}
```

### Example — Failed invocation

```json
{
  "id": "dns-bad00000-0000-0000-0000-000000000000",
  "command": "python3",
  "args": [
    "/Users/me/Library/Caches/atlas-recon/plugins/dns_lookup.py",
    "---invalid---"
  ],
  "started_at": "2026-05-19T10:00:00Z",
  "finished_at": "2026-05-19T10:00:01Z",
  "duration_ms": 423,
  "exit_code": 1,
  "status": "failed",
  "stdout": "",
  "stderr": "Error: invalid domain\n",
  "error": "python runner failed: exit status 1"
}
```

### Multi-target output (JSON array)

When `--targets-file` is used with `--json`, the output is a JSON **array** of `runner.Result` objects:

```json
[
  { "id": "dns-...", "command": "python3", "args": [...], ... },
  { "id": "dns-...", "command": "python3", "args": [...], ... }
]
```

---

## `scanner.ScanResult` — Native TCP Scan Envelope

**Package:** `internal/scanner`
**Go type:** `scanner.ScanResult`

Used exclusively by the native `scan` command. This is the only command that does not go through the Python plugin runner.

### Fields

| Field | JSON key | Type | Description |
|---|---|---|---|
| `Host` | `host` | string | Resolved IP address that was scanned. Always present. |
| `Ports` | `ports` | array of `PortResult` | One entry per port scanned, sorted by port number. Always present (may be empty if error occurred before scanning). |
| `StartTime` | `start_time` | string (RFC 3339) | Wall-clock time when the scan started. Always present. |
| `EndTime` | `end_time` | string (RFC 3339) | Wall-clock time when the scan finished. Always present. |
| `Error` | `error` | string | Top-level scan error (e.g. host resolution failure). Omitted when empty (`omitempty`). |

### `PortResult` sub-struct

| Field | JSON key | Type | Description |
|---|---|---|---|
| `Port` | `port` | integer | Port number (1–65535). Always present. |
| `State` | `state` | string | One of `"open"`, `"closed"`, or `"filtered"`. Always present. |
| `Service` | `service` | string | Common service name for the port (e.g. `"HTTP"`, `"SSH"`, `"Unknown"`). Always present. |
| `Error` | `error` | string | Per-port error. Omitted when empty (`omitempty`). |

### Example

```json
{
  "host": "93.184.216.34",
  "ports": [
    { "port": 22, "state": "filtered", "service": "SSH" },
    { "port": 80, "state": "open", "service": "HTTP" },
    { "port": 443, "state": "open", "service": "HTTPS" }
  ],
  "start_time": "2026-05-19T10:00:00Z",
  "end_time": "2026-05-19T10:00:02Z"
}
```

---

## Native DNS, Web, And LAN Payloads

Native commands emit typed JSON directly and store that typed JSON in `storage.Record.payload`.

| Command | Runtime JSON shape | Stored record kind |
|---|---|---|
| `dns` | `{"records": [dns.Record, ...]}` | `dns` |
| `web` | `web.ProbeResult` | `web` |
| `lan discover` | Array of discovery objects with `cidr`, `hosts`, `open_ports`, and optional `services` | `lan_discover` |
| `lan crawl` | `{"scan_id": "...", "pages": [crawl.PageResult, ...], "links": [crawl.LinkResult, ...]}` | `lan_crawl` |

`scan` is also native and stores its full `scanner.ScanResult` payload with `kind: "scan"`.

---

## `storage.Record` — Stored Result

**Package:** `internal/storage`
**Go type:** `storage.Record`

Used by the `results` command when listing stored invocations (`--json`). The output is a JSON **array** of records.

### Fields

| Field | JSON key | Type | Description |
|---|---|---|---|
| `ID` | `id` | string | UUID from the original result. |
| `Kind` | `kind` | string | `"command"` for plugin invocations, or a native kind such as `"scan"`, `"dns"`, `"web"`, or `"lan_discover"`. |
| `Command` | `command` | string | Subcommand name: `"dns"`, `"scan"`, `"geo"`, etc. |
| `Args` | `args` | array of strings | Argument list from the invocation. |
| `StartedAt` | `started_at` | string (RFC 3339) | Original start time. |
| `FinishedAt` | `finished_at` | string (RFC 3339) | Original finish time. |
| `DurationMs` | `duration_ms` | integer | Duration in milliseconds. |
| `ExitCode` | `exit_code` | integer | Exit code. |
| `Status` | `status` | string | `"success"` or `"failed"`. |
| `Stdout` | `stdout` | string | Captured stdout. |
| `Stderr` | `stderr` | string | Captured stderr. |
| `Error` | `error` | string | Error message. |
| `Payload` | `payload` | string | JSON-serialized native command payload for native records. Empty string for `kind: "command"` plugin records. |

---

## Command → Shape Mapping

| Command | `--json` output shape | Notes |
|---|---|---|
| `scan` | `scanner.ScanResult` | Native Go. No Python involved. |
| `dns` | `{"records": [dns.Record, ...]}` | Native Go by default. `--plugin` or `--targets-file` uses `runner.Result`. |
| `osint` | `runner.Result` | Python plugin. Multi-target: JSON array of Results. |
| `recon` | `runner.Result` | Python plugin. |
| `web` | `web.ProbeResult` | Native Go by default. `--plugin` or `--targets-file` uses `runner.Result`. |
| `report` | `runner.Result` | Python plugin. |
| `phone` | `runner.Result` | Python plugin. |
| `geo` | `runner.Result` | Python plugin. |
| `conflict` | `runner.Result` | Python plugin. |
| `markets` | `runner.Result` | Python plugin. |
| `social` | `runner.Result` | Python plugin. |
| `flight` | `runner.Result` | Python plugin. |
| `war` | `runner.Result` | Python plugin. |
| `dashboard` | N/A | Interactive TUI. No JSON output. |
| `results` | Array of `storage.Record` | Native Go. Lists stored invocation history. |
| `lan discover` | Array of discovery objects | Native Go. Stores records as `kind: "lan_discover"`. |
| `lan crawl` | Crawl result object | Native Go. Discovers services, crawls bounded in-scope links, and stores a graph scan. |
| `lan map` | JSON, Markdown, or DOT graph export | Native Go. Exports a stored graph scan; defaults to the most recent scan. |

---

## Schema Stability Guarantees

**Guaranteed:**
- All fields listed above are stable and will not be removed in minor/patch releases.
- Field names in JSON use `snake_case` and will not change.
- `status` will always be `"success"` or `"failed"` (no other values introduced without a major version).
- `stdout` and `stderr` are always strings (never null).
- `exit_code` is always an integer.
- Timestamps are always RFC 3339 strings.

**May change (automation should be tolerant):**
- `stdout` content format is plugin-specific. Plugins may change their plaintext output between versions. Parse `stdout` loosely or anchor on known patterns.
- Some plugins may emit their own JSON within `stdout` in future releases. Check `Content-Type`-like conventions if added later.
- New fields may be added to the envelope objects. Consumers should ignore unknown fields.
- `duration_ms` precision may improve (nanoseconds truncated to ms today).

**Not guaranteed:**
- `args[0]` (script path) is an internal implementation detail. Its path may change across installs, OSes, and releases.
- Order of `ports` in `ScanResult` is sorted by port number today but sort stability is not contractually guaranteed for edge cases.

---

## Quick Reference

| I want to... | Look at |
|---|---|
| Parse output from a plugin command | `runner.Result` — universal envelope |
| Parse output from `ct scan` | `scanner.ScanResult` + `PortResult` |
| Parse output from `ct results` | `storage.Record` (array) |
| Parse output from `ct dns` (native) | `dns.Record` — the `records` JSON key |
| Parse output from `ct web` (native) | `web.ProbeResult` |
| Parse output from `ct lan discover` | `crawl.DiscoveryResult` (see `internal/crawl/discover.go`) |
| Know which shape a command uses | Command→Shape table (above) |
| Understand what fields are stable | Schema Stability (above) |
