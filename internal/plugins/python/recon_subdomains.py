#!/usr/bin/env python3
import argparse
import json
import sys
import urllib.error
import urllib.request


def fetch_crtsh(domain):
    url = f"https://crt.sh/?q=%25.{domain}&output=json"
    req = urllib.request.Request(url, headers={"User-Agent": "ct-recon"})
    with urllib.request.urlopen(req, timeout=20) as resp:
        data = resp.read().decode("utf-8")
    return json.loads(data)


def extract_subdomains(entries, domain):
    subdomains = set()
    suffix = f".{domain}"
    for entry in entries:
        name_value = entry.get("name_value", "")
        for name in name_value.splitlines():
            name = name.strip().lower()
            if name.endswith(suffix) or name == domain:
                subdomains.add(name)
    return sorted(subdomains)


def main():
    parser = argparse.ArgumentParser(description="Subdomain recon via crt.sh")
    parser.add_argument("domain", help="domain name")
    args = parser.parse_args()

    try:
        entries = fetch_crtsh(args.domain)
    except (urllib.error.URLError, json.JSONDecodeError) as exc:
        print(f"crt.sh query failed: {exc}", file=sys.stderr)
        return 1

    subdomains = extract_subdomains(entries, args.domain)
    for name in subdomains:
        print(name)
    return 0


if __name__ == "__main__":
    sys.exit(main())


# Signed-off-by: ronikoz
