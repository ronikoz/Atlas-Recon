# 🔍 Atlas-Recon

> **A comprehensive open-source security toolkit for network scanning, DNS reconnaissance, OSINT, and web intelligence gathering.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-blue)](https://golang.org/)
[![Python Version](https://img.shields.io/badge/Python-3.9+-green)](https://www.python.org/)
[![Status](https://img.shields.io/badge/Status-In%20Development-orange)]()

---

## ⚠️ LEGAL DISCLAIMER

**This tool is provided for educational and authorized security testing purposes only.**

| ✅ Permitted | ❌ Prohibited |
|---|---|
| Authorized penetration testing on your own systems | Unauthorized network scanning |
| Security research with explicit permission | Data harvesting or scraping |
| Network diagnostics on systems you own | Hacking, cracking, or cyberattacks |
| Legitimate security training | Criminal activity of any kind |

**By using this tool, you agree to:**
- Comply with all local, state, and federal laws
- Only scan networks and systems you own or have **written permission** to test
- Use this tool exclusively for lawful, ethical purposes
- Accept full responsibility for your actions and any damages caused

**Unauthorized access to computer systems is illegal.** Violators will be prosecuted to the fullest extent of the law.

---

## 📊 Project Status

| Aspect | Status |
|--------|--------|
| **Version** | 2.1.0 |
| **Build** | ✅ All bugs fixed, 15 plugins embedded, fully tested |
| **Go Binary** | ✅ Complete, optimized, cross-platform |
| **Plugin System** | ✅ Fully embedded with caching |
| **Native Port Scanner** | ✅ Built-in (no external nmap required) |
| **External Dependency** | ❌ Eliminated - runs standalone |

---

## 🎯 Core Features

### ⚡ Go CLI Engine
- **14+ powerful subcommands** for various security tasks
- **Native port scanner** - TCP connectivity scanning with worker pool concurrency
- **Interactive TUI dashboard** using Bubble Tea framework
- **15 embedded Python plugins** - all bundled in the binary
- **Job queue system** with goroutine-based concurrency
- **Plugin caching** for instant subsequent runs
- **JSON output** for integration with other tools
- **Configuration support** via YAML files
- **Comprehensive error handling** and timeout management

### 🔌 Embedded Plugins

| Category | Plugins |
|----------|---------|
| 🌐 **Network** | Port Scanner, DNS Lookup, Web Check, Subdomain Recon |
| 🔎 **OSINT** | Domain OSINT, OSINT Suite, Phone Recon |
| 📍 **Geospatial** | Geo Reconnaissance, Flight Radar |
| 📰 **Intelligence** | Conflict Tracking, War Intelligence, Market Sentiment |
| 🎭 **Social** | Social Pulse (Bluesky), Phone OSINT |
| 📑 **Analysis** | Report Generation |

### 🎯 Standout Features
- **Native Port Scanner**: Concurrent TCP scanning without external nmap dependency
- **Geolocation Accuracy**: 5-tier confidence system with precision assessment
- **Flight Tracking**: Real-time aircraft monitoring via OpenSky API
- **Multi-source OSINT**: Aggregated intelligence from multiple databases
- **Report Generation**: Automated security report creation
- **Cross-platform**: Works on macOS, Linux, and other Unix-like systems

---

## 🚀 Quick Start

### Prerequisites
- **Go 1.24+** (to build)
- **Python 3.9+** (for plugins)
- **macOS/Linux** or Unix-like environment

### Build
```bash
# Using build script (recommended)
./build.sh

# Or manual build
go build -o ct ./cmd/ct
```

### Usage Examples
```bash
# Display help
./ct --help

# Network scanning (native Go scanner)
./ct scan localhost --ports 22,80,443
./ct scan 192.168.1.1 --ports 1-1000

# DNS reconnaissance
./ct dns example.com
./ct dns google.com

# OSINT gathering
./ct osint example.com
./ct geo "Paris, France"
./ct flight "London"

# Run interactive dashboard
./ct dashboard

# JSON output for automation
./ct scan localhost --ports 80,443 --json
./ct osint geo "Tokyo" --json

# Review stored results
./ct results --limit 10
./ct results --command dns --json
```

### Run from Any Directory
```bash
# Binary works from anywhere once built
cd /tmp
/Users/ronikoz/Atlas-Recon/ct scan 192.168.1.1 --ports 80,443
```

---

## 📖 Documentation

### Configuration
Default config: `configs/default.yaml`

Override:
```bash
./ct --config /path/to/config.yaml scan example.com
```

Config options:
```yaml
concurrency: 10              # Job queue workers
timeouts:
  command_seconds: 30        # Max runtime per command
output:
  json: false                # Default output format
storage:
  enabled: true              # Store results in SQLite
  results_db: /path/to/db    # Defaults to OS cache dir
paths:
  python: python3            # Python interpreter
```

### JSON Output Schema
All commands support `--json` for structured output:
```json
{
  "id": "unique_job_id",
  "command": "scan",
  "args": ["example.com", "--ports", "80"],
  "started_at": "2026-01-21T10:00:00Z",
  "finished_at": "2026-01-21T10:00:05Z",
  "duration_ms": 5000,
  "exit_code": 0,
  "status": "success",
  "stdout": "command output here",
  "stderr": "",
  "error": null
}
```

---

## 🔧 Plugin System

### How It Works
- All 15 Python plugins are **embedded in the binary** at compile time
- Automatically extracted to `~/.cache/ct_plugins/` on first run
- Subsequent runs use cached versions (sub-millisecond lookup)
- Self-contained with Python dependencies

### Available Plugins

**Network & Scanning:**
- `scan` - Native port scanning with service detection
- `dns_lookup` - DNS queries with validation
- `web_check` - Web server analysis

**OSINT & Reconnaissance:**
- `osint_domain` - Domain intelligence via crt.sh and whois
- `osint_suite` - Multi-source OSINT aggregation
- `recon_subdomains` - Subdomain discovery
- `phone_osint` - Phone number reconnaissance

**Geospatial & Tracking:**
- `geo_recon` - Geolocation with accuracy assessment (5-tier confidence)
- `flight_radar` - Real-time aircraft tracking (OpenSky API)

**Intelligence & Analysis:**
- `conflict_view` - GDELT conflict event data
- `war_intel` - ISW war intelligence tracking
- `market_sentiment` - Prediction market analysis
- `social_pulse` - Bluesky social media analysis
- `generate_report` - Automated report generation

### Adding Custom Plugins
1. Create `plugins/python/your_plugin.py`
2. Implement argument parsing and main logic
3. Return JSON or human-readable output
4. Rebuild: `go build -o ct ./cmd/ct`
5. *That's it!* The new module is now automatically available in the CLI and the TUI dashboard (via dynamic discovery). Use the `/` hotkey in the TUI to search for it.

---

## 📊 Performance

| Operation | Duration | Notes |
|-----------|----------|-------|
| Binary startup | <100ms | Instant |
| First plugin extraction | 100-200ms | Cached after |
| Plugin execution | 500-1500ms | Depends on external APIs |
| Port scan (100 ports) | 1-5s | Concurrent with 100 workers |
| Cache hit | <1ms | Very fast |

---

## 🏗️ Architecture

### Go Components
```
cmd/ct/main.go              → CLI entry point
├── internal/cli/           → Command routing
├── internal/config/        → Configuration management
├── internal/scanner/       → Native port scanner
├── internal/plugins/       → Plugin embedding & resolution
├── internal/runner/        → Job queue & execution engine
├── internal/tui/           → Terminal UI dashboard
└── internal/storage/       → SQLite status formatting & output
```

### Python Plugins
```text
plugins/python/             → All 15 embedded plugins (flat structure)
```

### Key Technologies
- **Go**: Concurrency, performance, cross-platform
- **Charmbracelet**: Beautiful terminal UI
- **Python**: Plugin flexibility and ecosystem
- **Standard Library**: No external Go dependencies for core functionality

---

## 🚦 Current Development Status

### ✅ Completed & Stable
- [x] Native Go port scanner (no nmap dependency)
- [x] All 15 embedded plugins
- [x] CLI command routing
- [x] Configuration system
- [x] JSON output schema
- [x] Plugin caching
- [x] Persistent result database (SQLite)
- [x] Error handling & timeouts
- [x] Cross-directory execution
- [x] Performance optimization

### 🔄 In Development / Planned
- [ ] Web API interface (REST endpoints)
- [ ] Advanced filtering and searching
- [ ] Scheduled/recurring scans
- [ ] Multi-target batch processing
- [ ] Custom plugin marketplace
- [ ] Cloud integration (optional)
- [ ] Mobile companion app (future)

### 📝 Known Limitations
- Nominatim API has rate limiting (1 request/second)
- OpenSky API has free tier restrictions
- Some plugins require internet connectivity
- Cache stored in local user home directory
- Geographic accuracy depends on data sources

---

## 🔍 Use Cases

**Security Professionals:**
- Authorized penetration testing and vulnerability assessments
- Network reconnaissance during security audits
- OSINT gathering for threat intelligence

**System Administrators:**
- Network diagnostics and monitoring
- Service discovery and port analysis
- DNS troubleshooting

**Researchers:**
- Cybersecurity research and education
- Threat landscape analysis
- Network topology mapping

**Ethical Hackers & Bug Bounty:**
- Authorized target enumeration
- Service discovery during authorized testing
- Reconnaissance within scope of programs

---

## 🛠️ Troubleshooting

### Binary not found from other directories
```bash
# Rebuild to ensure static compilation
go build -o ct ./cmd/ct

# Binary should work from anywhere
cd /tmp && /path/to/ct --help
```

### Plugin extraction errors
```bash
# Clear cache if plugins fail to load
rm -rf ~/.cache/ct_plugins/

# Rebuild if cache issue persists
go build -o ct ./cmd/ct
```

### Low geolocation precision
```bash
# Use specific location names for better accuracy
./ct osint geo "Germany"                    # Low precision
./ct osint geo "Berlin, Germany"            # Medium precision
./ct osint geo "Potsdamer Platz, Berlin"   # High precision
```

### Rate limiting on API calls
```bash
# Wait before running scans again
# Nominatim limits to ~1 request/second
# Other APIs have their own throttling

# Use --json flag for batch processing
./ct osint geo "Tokyo" --json
```

---

## 📚 Additional Resources

- **Contributing**: See [CONTRIBUTING.md](./CONTRIBUTING.md) for development guidelines
- **License**: MIT - See [LICENSE](./LICENSE)
- **Issues**: Report bugs and suggest features on GitHub
- **Security**: If you discover a security vulnerability, please email responsibly

---

## 👨‍💻 Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

See [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed guidelines.

---

## 📄 License

MIT License - See [LICENSE](./LICENSE) for details.

This is free, open-source software. Use it responsibly and legally.

---

## 🙏 Acknowledgments

Built with Go, Python, and the following amazing libraries:
- [Charmbracelet/Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Charmbracelet/Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- Go Standard Library - Networking and concurrency

---

## 📞 Support

- 📖 Check the documentation above
- 🐛 Report issues on GitHub
- 💡 Suggest features as GitHub discussions
- 📧 For urgent matters, reach out responsibly

---

**Status**: 🚧 In Development (v0.6.0)  
**Last Updated**: March 7, 2026  
**License**: MIT  
**Made with ❤️ for the community**

---

### Remember: Use responsibly. Use legally. Use ethically.
