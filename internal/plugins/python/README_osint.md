# OSINT Domain Plugin

This plugin gathers domain OSINT using `crt.sh`, `whois` and optional DNS enrichment.

Features:

- Batch mode: `--file` with concurrency via `--workers`
- Output: `--json`, `--csv`, `--out-file`
- Filters: `--since`, `--until`, `--limit`
- Caching: local cache in `~/.cache/ct_osint` (disabled with `--no-cache`)
- DNS enrichment: `--enrich-dns` resolves discovered subdomains and clusters by IP

Example:

```bash
python plugins/python/osint_domain.py example.com --enrich-dns --json
python plugins/python/osint_domain.py --file domains.txt --workers 8 --out-file results.json
```

# OSINT Suite Plugin

This plugin runs a broader OSINT suite with two sources per domain category.

Categories:
- core: sherlock, maigret
- domain_dns: subfinder, amass
- ip_infra: naabu, masscan
- metadata: metagoofil, exiftool
- leaks: trufflehog, gitleaks
- social: holehe, snscrape
- archives: waybackurls, katana
- search: awesome-osint, osint-stuff
- threat: misp, opencti

Examples:

```bash
python plugins/python/osint_suite.py --list
python plugins/python/osint_suite.py --category core --username alice
python plugins/python/osint_suite.py --category domain_dns --domain example.com
python plugins/python/osint_suite.py --source holehe --email alice@example.com
python plugins/python/osint_suite.py --source waybackurls --domain example.com
python plugins/python/osint_suite.py --source misp --misp-url https://misp.local --misp-key KEY --indicator 1.2.3.4
```
