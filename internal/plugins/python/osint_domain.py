#!/usr/bin/env python3
"""OSINT enrichment for domains using crt.sh, whois and DNS enrichment.

Features:
- crt.sh queries with retries, backoff, and simple caching
- whois lookup via `python-whois` with fallback to system `whois`
- batch mode via `--file` and concurrency
- JSON/CSV output and file output
- Filters: `--since`, `--until`, `--limit`
- DNS enrichment: resolve A/AAAA records and cluster by IP
- Verbose/quiet, colored output
"""

from __future__ import annotations

import argparse
import concurrent.futures
import hashlib
import json
import os
import sys
import time
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Set

import warnings
with warnings.catch_warnings():
    warnings.simplefilter("ignore")
    try:
        import requests
        import urllib3
        urllib3.disable_warnings()
    except Exception:
        pass

if 'requests' not in sys.modules:
    print("requests is required. Install with: pip install requests", file=sys.stderr)
    raise ImportError("missing requests")

try:
    import whois as pywhois
except Exception:
    pywhois = None

try:
    import dns.resolver
except Exception:
    dns = None

try:
    from dateutil import parser as dateparser
except Exception:
    dateparser = None

try:
    from colorama import Fore, Style, init as colorama_init
    colorama_init()
    HAS_COLOR = True
except Exception:
    HAS_COLOR = False


CACHE_DIR = Path.home() / ".cache" / "ct_osint"
CACHE_DIR.mkdir(parents=True, exist_ok=True)


def cache_path_for(domain: str) -> Path:
    key = hashlib.sha1(domain.encode()).hexdigest()
    return CACHE_DIR / f"crtsh_{key}.json"


def load_cache(domain: str, max_age: int = 86400) -> Optional[List[Dict[str, Any]]]:
    p = cache_path_for(domain)
    if not p.exists():
        return None
    try:
        mtime = p.stat().st_mtime
        if time.time() - mtime > max_age:
            return None
        with p.open("r") as fh:
            return json.load(fh)
    except Exception:
        return None


def save_cache(domain: str, data: List[Dict[str, Any]]):
    p = cache_path_for(domain)
    try:
        with p.open("w") as fh:
            json.dump(data, fh)
    except Exception:
        pass


def fetch_crtsh(domain: str, timeout: float = 20.0, retries: int = 3, backoff: float = 0.5, use_cache: bool = True) -> List[Dict[str, Any]]:
    if use_cache:
        cached = load_cache(domain)
        if cached is not None:
            return cached

    url = f"https://crt.sh/?q=%25.{domain}&output=json"
    headers = {"User-Agent": "ct-osint"}
    last_err = None
    for attempt in range(1, retries + 1):
        try:
            resp = requests.get(url, headers=headers, timeout=timeout)
            resp.raise_for_status()
            data = resp.json()
            save_cache(domain, data)
            return data
        except Exception as e:
            last_err = e
            if attempt < retries:
                time.sleep(backoff * (2 ** (attempt - 1)))
            else:
                raise


def extract_subdomains(entries: Iterable[Dict[str, Any]], domain: str) -> List[str]:
    subdomains: Set[str] = set()
    suffix = f".{domain}".lower()
    for entry in entries:
        name_value = entry.get("name_value") or entry.get("common_name") or ""
        for name in str(name_value).splitlines():
            n = name.strip().lower()
            if n.endswith(suffix) or n == domain.lower():
                # strip possible leading wildcards
                n = n.lstrip("*.")
                subdomains.add(n)
    return sorted(subdomains)


def parse_entry_times(entry: Dict[str, Any]) -> Dict[str, Optional[datetime]]:
    # crt.sh entries sometimes include not_before / not_after or entry_timestamp
    times = {"not_before": None, "not_after": None, "entry_timestamp": None}
    for k in list(times.keys()):
        v = entry.get(k) or entry.get(k.replace('_', ' '))
        if v and dateparser:
            try:
                times[k] = dateparser.parse(v)
            except Exception:
                times[k] = None
    return times


def run_whois(domain: str, timeout: float = 20.0) -> Dict[str, Any]:
    if pywhois:
        try:
            return pywhois.whois(domain)
        except Exception:
            pass
    # fallback to system whois
    try:
        import subprocess
        proc = subprocess.run(["whois", domain], capture_output=True, text=True, timeout=timeout)
        if proc.returncode == 0:
            return {"raw": proc.stdout}
        return {"error": proc.stderr}
    except FileNotFoundError:
        return {"error": "whois not found"}
    except Exception as e:
        return {"error": str(e)}


def enrich_dns(names: Iterable[str], timeout: float = 5.0, workers: int = 10) -> Dict[str, List[str]]:
    results: Dict[str, List[str]] = {}
    if dns is None:
        return results
    resolver = dns.resolver.Resolver()

    def resolve(name: str) -> List[str]:
        ips: List[str] = []
        for t in ("A", "AAAA"):
            try:
                ans = resolver.resolve(name, t, lifetime=timeout)
                for r in ans:
                    ips.append(r.to_text())
            except Exception:
                continue
        return ips

    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, workers)) as exc:
        future_map = {exc.submit(resolve, n): n for n in names}
        for fut in concurrent.futures.as_completed(future_map):
            name = future_map[fut]
            try:
                results[name] = fut.result()
            except Exception:
                results[name] = []
    return results


def cluster_by_ip(mapping: Dict[str, List[str]]) -> Dict[str, List[str]]:
    ip_map: Dict[str, List[str]] = {}
    for name, ips in mapping.items():
        for ip in ips:
            ip_map.setdefault(ip, []).append(name)
    return ip_map


def process_domain(domain: str, args) -> Dict[str, Any]:
    result: Dict[str, Any] = {"domain": domain, "whois": None, "subdomains": [], "dns": {}, "clusters": {}}

    who = run_whois(domain)
    result["whois"] = who

    try:
        entries = fetch_crtsh(domain, timeout=args.timeout, retries=args.retries, backoff=args.backoff, use_cache=not args.no_cache)
    except Exception as e:
        result["crt_error"] = str(e)
        entries = []

    subdomains = extract_subdomains(entries, domain)

    # apply since/until filters if provided
    if args.since or args.until:
        filtered = []
        for entry in entries:
            times = parse_entry_times(entry)
            keep = True
            if args.since and times.get("entry_timestamp"):
                keep = keep and times["entry_timestamp"] >= args.since
            if args.until and times.get("entry_timestamp"):
                keep = keep and times["entry_timestamp"] <= args.until
            if keep:
                filtered.append(entry)
        subdomains = extract_subdomains(filtered, domain)

    if args.limit:
        subdomains = subdomains[: args.limit]

    result["subdomains"] = subdomains

    if args.enrich_dns and subdomains:
        dnsmap = enrich_dns(subdomains, timeout=args.dns_timeout, workers=args.workers)
        result["dns"] = dnsmap
        result["clusters"] = cluster_by_ip(dnsmap)

    return result


def load_domains(args) -> List[str]:
    domains: List[str] = []
    if args.file:
        try:
            with open(args.file, "r") as fh:
                for line in fh:
                    s = line.strip()
                    if s:
                        domains.append(s)
        except Exception as e:
            print(f"failed to read {args.file}: {e}", file=sys.stderr)
            sys.exit(2)
    domains.extend([d for d in (args.domains or []) if d])
    if not domains:
        print("no domains provided", file=sys.stderr)
        sys.exit(2)
    return domains


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description="OSINT enrichment for a domain (crt.sh + whois + DNS)")
    parser.add_argument("domains", nargs="*", help="domain names")
    parser.add_argument("--file", help="file with domains, one per line")
    parser.add_argument("--json", dest="out_json", action="store_true", help="output JSON")
    parser.add_argument("--csv", dest="out_csv", action="store_true", help="output CSV")
    parser.add_argument("--out-file", help="write output to file")
    parser.add_argument("--timeout", type=float, default=20.0, help="HTTP query timeout seconds")
    parser.add_argument("--retries", type=int, default=3, help="number of retries for HTTP calls")
    parser.add_argument("--backoff", type=float, default=0.5, help="base backoff seconds")
    parser.add_argument("--no-cache", action="store_true", help="disable crt.sh caching")
    parser.add_argument("--since", type=str, help="filter certs since ISO date")
    parser.add_argument("--until", type=str, help="filter certs until ISO date")
    parser.add_argument("--limit", type=int, help="limit number of returned subdomains")
    parser.add_argument("--enrich-dns", action="store_true", help="resolve discovered subdomains to A/AAAA")
    parser.add_argument("--dns-timeout", type=float, default=5.0, help="DNS lookup timeout")
    parser.add_argument("--workers", type=int, default=4, help="concurrency for batch runs")
    parser.add_argument("--verbose", action="store_true")
    parser.add_argument("--quiet", action="store_true")
    parser.add_argument("--no-color", action="store_true")
    args = parser.parse_args(argv)

    if (args.since or args.until) and not dateparser:
        print("python-dateutil is required for --since/--until. Install with: pip install python-dateutil", file=sys.stderr)
        return 2

    if args.since:
        args.since = dateparser.parse(args.since) if dateparser else None
    else:
        args.since = None
    if args.until:
        args.until = dateparser.parse(args.until) if dateparser else None
    else:
        args.until = None

    domains = load_domains(args)

    results: List[Dict[str, Any]] = []

    with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as exc:
        futures = {exc.submit(process_domain, d, args): d for d in domains}
        for fut in concurrent.futures.as_completed(futures):
            d = futures[fut]
            try:
                results.append(fut.result())
            except Exception as e:
                results.append({"domain": d, "error": str(e)})

    # Output
    use_color = HAS_COLOR and not args.no_color
    if args.out_json or args.out_file:
        out = json.dumps(results, indent=2, default=str)
        if args.out_file:
            with open(args.out_file, "w") as fh:
                fh.write(out)
        else:
            print(out)
    elif args.out_csv:
        # simple CSV: domain,subdomain,ips
        import csv

        writer = csv.writer(sys.stdout)
        writer.writerow(["domain", "subdomain", "ips"])
        for r in results:
            domain = r.get("domain")
            dnsmap = r.get("dns", {})
            if dnsmap:
                for sub, ips in dnsmap.items():
                    writer.writerow([domain, sub, ";".join(ips)])
            else:
                for sub in r.get("subdomains", []):
                    writer.writerow([domain, sub, ""])
    else:
        for r in results:
            domain = r.get("domain")
            if use_color and args.verbose:
                print(Fore.CYAN + f"Domain: {domain}" + Style.RESET_ALL)
            else:
                print(f"Domain: {domain}")
            who = r.get("whois")
            if who:
                print("WHOIS:")
                if isinstance(who, dict) and who.get("raw"):
                    print(who.get("raw"))
                else:
                    print(json.dumps(who, indent=2, default=str))
            subs = r.get("subdomains", [])
            if subs:
                print("SUBDOMAINS:")
                for s in subs:
                    print(" - ", s)
            clusters = r.get("clusters", {})
            if clusters:
                print("CLUSTERS:")
                for ip, hosts in clusters.items():
                    print(f" {ip}: {', '.join(hosts)}")
            print("")

    return 0


if __name__ == "__main__":
    sys.exit(main())
