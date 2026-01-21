#!/usr/bin/env python3
"""Enhanced DNS lookup utility using dnspython.

Features:
- Uses `dnspython` for robust queries
- JSON and CSV output modes
- Custom resolver servers, timeout and retries with exponential backoff
- Supports A, AAAA, MX, NS, TXT, CNAME, SOA, SRV, PTR, SPF
- Reverse lookups (PTR) and batch mode via `--file`
- Structured results (host, type, value, ttl, priority)
- Concurrency via ThreadPoolExecutor
- Verbose/quiet, colored output
"""

from __future__ import annotations

import argparse
import csv
import ipaddress
import json
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import Any, Dict, List, Optional, Set, Tuple

try:
    import dns.exception
    import dns.resolver
    import dns.reversename
    import dns.rdatatype
    import dns.flags
except ImportError as e:
    print("dnspython is required. Install with: pip install dnspython", file=sys.stderr)
    raise

try:
    from colorama import Fore, Style, init as colorama_init
    colorama_init()
    HAS_COLOR = True
except Exception:
    HAS_COLOR = False


DEFAULT_TYPES = ["A", "AAAA", "MX", "NS", "TXT"]


def normalize_ipv6(addr: str) -> str:
    try:
        ip = ipaddress.ip_address(addr)
        if ip.version == 6:
            return ip.exploded
    except Exception:
        pass
    return addr


def concat_txt(rdata) -> str:
    # rdata.strings may be bytes or str depending on dnspython version
    try:
        parts = [p.decode() if isinstance(p, bytes) else str(p) for p in rdata.strings]
        return "".join(parts)
    except Exception:
        return rdata.to_text().strip('"')


def build_record(domain: str, rdtype: str, answer) -> List[Dict[str, Any]]:
    results: List[Dict[str, Any]] = []
    ttl = getattr(answer.rrset, "ttl", None)
    for rdata in answer:
        rec: Dict[str, Any] = {"host": domain, "type": rdtype, "ttl": ttl, "value": None}
        if rdtype in ("A", "AAAA"):
            rec["value"] = normalize_ipv6(getattr(rdata, "address", rdata.to_text()))
        elif rdtype == "MX":
            rec["priority"] = getattr(rdata, "preference", None)
            rec["value"] = str(getattr(rdata, "exchange", rdata.to_text())).rstrip('.')
        elif rdtype == "CNAME":
            rec["value"] = str(getattr(rdata, "target", rdata.to_text())).rstrip('.')
        elif rdtype == "NS":
            rec["value"] = str(getattr(rdata, "target", rdata.to_text())).rstrip('.')
        elif rdtype == "SOA":
            rec["value"] = {
                "mname": str(rdata.mname).rstrip('.'),
                "rname": str(rdata.rname).rstrip('.'),
                "serial": rdata.serial,
            }
        elif rdtype == "SRV":
            rec.update({"priority": rdata.priority, "weight": rdata.weight, "port": rdata.port})
            rec["value"] = str(rdata.target).rstrip('.')
        elif rdtype == "TXT":
            rec["value"] = concat_txt(rdata)
        else:
            rec["value"] = rdata.to_text()
        results.append(rec)
    return results


def query_with_retries(resolver: dns.resolver.Resolver, name: str, rdtype: str, timeout: float, retries: int, backoff: float, dnssec: bool) -> Tuple[List[Dict[str, Any]], Optional[str]]:
    last_err = None
    
    # Validate PTR queries require IP addresses
    if rdtype == "PTR":
        try:
            ipaddress.ip_address(name)
        except ValueError:
            return [], f"PTR requires IP address, got: {name}"
    
    for attempt in range(1, retries + 1):
        try:
            if rdtype == "PTR":
                rev = dns.reversename.from_address(name)
                answer = resolver.resolve(rev, "PTR", lifetime=timeout)
                recs = build_record(name, "PTR", answer)
            else:
                # set DO bit for DNSSEC if requested
                if dnssec:
                    resolver.use_edns(edns=0, ednsflags=dns.flags.DO)
                answer = resolver.resolve(name, rdtype, lifetime=timeout)
                recs = build_record(name, rdtype, answer)
            return recs, None
        except Exception as e:
            last_err = str(e)
            if attempt < retries:
                sleep = backoff * (2 ** (attempt - 1))
                time.sleep(sleep)
            else:
                break
    return [], last_err


def query_domain_types(domain: str, types: List[str], resolver: dns.resolver.Resolver, timeout: float, retries: int, backoff: float, dnssec: bool, verbose: bool) -> Tuple[List[Dict[str, Any]], List[str]]:
    results: List[Dict[str, Any]] = []
    errors: List[str] = []
    dedup: Set[Tuple] = set()

    def clone_resolver() -> dns.resolver.Resolver:
        new_resolver = dns.resolver.Resolver()
        new_resolver.nameservers = resolver.nameservers
        new_resolver.timeout = resolver.timeout
        new_resolver.lifetime = resolver.lifetime
        return new_resolver

    with ThreadPoolExecutor(max_workers=min(10, max(1, len(types)))) as exc:
        futures = {exc.submit(query_with_retries, clone_resolver(), domain, t, timeout, retries, backoff, dnssec): t for t in types}
        for fut in as_completed(futures):
            t = futures[fut]
            try:
                recs, err = fut.result()
                if err:
                    errors.append(f"{t}: {err}")
                    if verbose:
                        print(f"{t}: error: {err}", file=sys.stderr)
                    continue
                for r in recs:
                    key = (r.get("host"), r.get("type"), json.dumps(r.get("value"), sort_keys=True), r.get("priority"))
                    if key in dedup:
                        continue
                    dedup.add(key)
                    results.append(r)
            except Exception as e:
                errors.append(f"{t}: {e}")
    return results, errors


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
    parser = argparse.ArgumentParser(description="DNS lookup utility (dnspython)")
    parser.add_argument("domains", nargs="*", help="domain names or IPs (for PTR)")
    parser.add_argument("--file", help="file with domains, one per line")
    parser.add_argument("--types", default=','.join(DEFAULT_TYPES), help="comma-separated record types")
    parser.add_argument("--server", help="comma-separated DNS servers to use (IP addresses)")
    parser.add_argument("--timeout", type=float, default=5.0, help="per-query timeout in seconds")
    parser.add_argument("--retries", type=int, default=3, help="number of retries")
    parser.add_argument("--backoff", type=float, default=0.5, help="base backoff seconds for retries")
    parser.add_argument("--json", dest="out_json", action="store_true", help="output results as JSON")
    parser.add_argument("--csv", dest="out_csv", action="store_true", help="output results as CSV")
    parser.add_argument("--dnssec", action="store_true", help="set DO bit / request DNSSEC records where available")
    parser.add_argument("--verbose", action="store_true")
    parser.add_argument("--quiet", action="store_true")
    parser.add_argument("--no-color", action="store_true")
    args = parser.parse_args(argv)

    types = [t.strip().upper() for t in args.types.split(',') if t.strip()]
    domains = load_domains(args)

    resolver = dns.resolver.Resolver()
    if args.server:
        resolver.nameservers = [s.strip() for s in args.server.split(',') if s.strip()]

    all_results: List[Dict[str, Any]] = []
    all_errors: List[str] = []

    for domain in domains:
        recs, errors = query_domain_types(domain, types, resolver, args.timeout, args.retries, args.backoff, args.dnssec, args.verbose)
        all_results.extend(recs)
        all_errors.extend(errors)

    # Output
    use_color = HAS_COLOR and not args.no_color
    if args.out_json:
        print(json.dumps(all_results, indent=2, sort_keys=True))
    elif args.out_csv:
        writer = csv.writer(sys.stdout)
        writer.writerow(["host", "type", "value", "ttl", "priority", "extra"])
        for r in all_results:
            extra = json.dumps({k: v for k, v in r.items() if k not in ("host", "type", "value", "ttl", "priority")})
            writer.writerow([r.get("host"), r.get("type"), r.get("value"), r.get("ttl"), r.get("priority"), extra])
    else:
        for r in all_results:
            t = r.get("type")
            host = r.get("host")
            line = f"{host} {t} {r.get('value')}"
            if r.get("priority") is not None:
                line += f" priority={r.get('priority')}"
            if use_color and args.verbose:
                print(Fore.GREEN + line + Style.RESET_ALL)
            else:
                print(line)

    if all_errors and not args.quiet:
        for e in all_errors:
            print(f"error: {e}", file=sys.stderr)

    return 0 if not all_errors else 2


if __name__ == "__main__":
    sys.exit(main())
