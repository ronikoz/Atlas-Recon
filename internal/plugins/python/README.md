# DNS Lookup Plugin

This plugin provides DNS lookups using `dnspython` and replaces the older `nslookup`-based script.

Installation

Install Python deps (recommended in a virtualenv):

```bash
pip install -r requirements.txt
```

Usage

```bash
python plugins/python/dns_lookup.py example.com --types A,MX,TXT --json
python plugins/python/dns_lookup.py --file domains.txt --types A,AAAA,NS --csv
python plugins/python/dns_lookup.py 8.8.8.8 --types PTR
```

Options include `--server`, `--timeout`, `--retries`, `--backoff`, `--dnssec`, `--verbose`, `--quiet`.
