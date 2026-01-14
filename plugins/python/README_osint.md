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
