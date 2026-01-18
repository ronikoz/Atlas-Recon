#!/usr/bin/env python3
"""OSINT suite runner with two sources per domain category.

This plugin is a thin wrapper around open-source tools and web sources. It
validates inputs, builds commands, and runs tools if installed.
"""

from __future__ import annotations

import argparse
import json
import shlex
import shutil
import subprocess
import sys
from typing import Any, Dict, List, Optional

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


def run_cli(cmd: List[str], timeout: float) -> Dict[str, Any]:
    if not cmd:
        return {"status": "failed", "error": "empty command"}
    binary = cmd[0]
    if not shutil.which(binary):
        return {
            "status": "failed",
            "error": f"missing dependency: {binary}",
            "install_hint": f"install {binary} and re-run",
            "command": cmd,
        }
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return {
            "status": "success" if proc.returncode == 0 else "failed",
            "command": cmd,
            "stdout": proc.stdout,
            "stderr": proc.stderr,
            "exit_code": proc.returncode,
        }
    except subprocess.TimeoutExpired:
        return {"status": "failed", "command": cmd, "error": "timeout"}
    except Exception as e:
        return {"status": "failed", "command": cmd, "error": str(e)}


def fetch_text(url: str, timeout: float) -> str:
    resp = requests.get(url, timeout=timeout)
    resp.raise_for_status()
    return resp.text


def filter_lines(text: str, query: Optional[str], limit: int) -> List[str]:
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    if query:
        q = query.lower()
        lines = [line for line in lines if q in line.lower()]
    return lines[:limit]


def run_awesome_osint(args: argparse.Namespace) -> Dict[str, Any]:
    url = "https://raw.githubusercontent.com/jivoi/awesome-osint/master/README.md"
    try:
        text = fetch_text(url, args.timeout)
        return {"status": "success", "data": filter_lines(text, args.query, args.limit)}
    except Exception as e:
        return {"status": "failed", "error": str(e), "url": url}


def run_osint_stuff(args: argparse.Namespace) -> Dict[str, Any]:
    url = "https://raw.githubusercontent.com/cipher387/osint_stuff_tool_collection/main/README.md"
    try:
        text = fetch_text(url, args.timeout)
        return {"status": "success", "data": filter_lines(text, args.query, args.limit)}
    except Exception as e:
        return {"status": "failed", "error": str(e), "url": url}


def run_misp(args: argparse.Namespace) -> Dict[str, Any]:
    if not args.misp_url or not args.misp_key:
        return {"status": "failed", "error": "misp requires --misp-url and --misp-key"}
    base = args.misp_url.rstrip("/")
    headers = {
        "Authorization": args.misp_key,
        "Accept": "application/json",
        "Content-Type": "application/json",
    }
    try:
        if args.indicator:
            url = f"{base}/attributes/restSearch"
            payload = {"value": args.indicator}
            resp = requests.post(url, headers=headers, json=payload, timeout=args.timeout)
        else:
            url = f"{base}/servers/getVersion"
            resp = requests.get(url, headers=headers, timeout=args.timeout)
        resp.raise_for_status()
        return {"status": "success", "data": resp.json(), "url": url}
    except Exception as e:
        return {"status": "failed", "error": str(e)}


def run_opencti(args: argparse.Namespace) -> Dict[str, Any]:
    if not args.opencti_url or not args.opencti_token:
        return {"status": "failed", "error": "opencti requires --opencti-url and --opencti-token"}
    url = f"{args.opencti_url.rstrip('/')}/graphql"
    headers = {"Authorization": f"Bearer {args.opencti_token}"}
    query = "query { platformStatus { status version } }"
    if args.indicator:
        query = (
            "query { indicators(filters: {key: \"pattern\", values: [\""
            + args.indicator.replace('"', '\\"')
            + "\"]}) { edges { node { id name pattern } } } }"
        )
    try:
        resp = requests.post(url, headers=headers, json={"query": query}, timeout=args.timeout)
        resp.raise_for_status()
        return {"status": "success", "data": resp.json(), "url": url}
    except Exception as e:
        return {"status": "failed", "error": str(e)}


def build_trufflehog(args: argparse.Namespace) -> List[str]:
    if args.repo:
        return ["trufflehog", "git", "--repo", args.repo]
    if args.path:
        return ["trufflehog", "filesystem", args.path]
    return []


def build_gitleaks(args: argparse.Namespace) -> List[str]:
    if not args.path:
        return []
    return ["gitleaks", "detect", "--source", args.path]


def build_snscrape(args: argparse.Namespace) -> List[str]:
    target = args.username or args.query
    if not target:
        return []
    return ["snscrape", "--jsonl", args.sns_service, target]


def build_masscan(args: argparse.Namespace) -> List[str]:
    target = args.target or args.ip or args.domain
    if not target:
        return []
    return ["masscan", target, "-p", args.ports]


def build_naabu(args: argparse.Namespace) -> List[str]:
    target = args.target or args.ip or args.domain
    if not target:
        return []
    return ["naabu", "-host", target]


def build_waybackurls(args: argparse.Namespace) -> List[str]:
    target = args.target or args.domain or args.url
    if not target:
        return []
    return ["waybackurls", target]


def build_katana(args: argparse.Namespace) -> List[str]:
    if not args.url:
        return []
    return ["katana", "-u", args.url]


def build_metagoofil(args: argparse.Namespace) -> List[str]:
    if not args.domain:
        return []
    cmd = ["metagoofil", "-d", args.domain, "-t", args.filetypes, "-l", str(args.limit)]
    if args.out_dir:
        cmd.extend(["-o", args.out_dir])
    return cmd


def build_exiftool(args: argparse.Namespace) -> List[str]:
    if not args.path:
        return []
    return ["exiftool", args.path]


def build_sherlock(args: argparse.Namespace) -> List[str]:
    if not args.username:
        return []
    return ["sherlock", args.username]


def build_maigret(args: argparse.Namespace) -> List[str]:
    if not args.username:
        return []
    return ["maigret", args.username]


def build_subfinder(args: argparse.Namespace) -> List[str]:
    if not args.domain:
        return []
    return ["subfinder", "-d", args.domain, "-silent"]


def build_amass(args: argparse.Namespace) -> List[str]:
    if not args.domain:
        return []
    return ["amass", "enum", "-d", args.domain]


SOURCES: Dict[str, Dict[str, Any]] = {
    # Core OSINT frameworks
    "sherlock": {"category": "core", "kind": "cli", "build": build_sherlock},
    "maigret": {"category": "core", "kind": "cli", "build": build_maigret},
    # Domain & DNS intelligence
    "subfinder": {"category": "domain_dns", "kind": "cli", "build": build_subfinder},
    "amass": {"category": "domain_dns", "kind": "cli", "build": build_amass},
    # IP/Infrastructure & network
    "naabu": {"category": "ip_infra", "kind": "cli", "build": build_naabu},
    "masscan": {"category": "ip_infra", "kind": "cli", "build": build_masscan},
    # Metadata & file analysis
    "metagoofil": {"category": "metadata", "kind": "cli", "build": build_metagoofil},
    "exiftool": {"category": "metadata", "kind": "cli", "build": build_exiftool},
    # Leaks, credentials, and code
    "trufflehog": {"category": "leaks", "kind": "cli", "build": build_trufflehog},
    "gitleaks": {"category": "leaks", "kind": "cli", "build": build_gitleaks},
    # Social & people OSINT
    "holehe": {"category": "social", "kind": "cli", "build": lambda args: ["holehe", args.email] if args.email else []},
    "snscrape": {"category": "social", "kind": "cli", "build": build_snscrape},
    # Web archive & historical
    "waybackurls": {"category": "archives", "kind": "cli", "build": build_waybackurls},
    "katana": {"category": "archives", "kind": "cli", "build": build_katana},
    # Search & indexing (curated lists)
    "awesome-osint": {"category": "search", "kind": "http", "run": run_awesome_osint},
    "osint-stuff": {"category": "search", "kind": "http", "run": run_osint_stuff},
    # Threat intel / indicators
    "misp": {"category": "threat", "kind": "http", "run": run_misp},
    "opencti": {"category": "threat", "kind": "http", "run": run_opencti},
}


def list_sources() -> str:
    categories: Dict[str, List[str]] = {}
    for name, meta in SOURCES.items():
        categories.setdefault(meta["category"], []).append(name)
    lines = []
    for cat in sorted(categories.keys()):
        lines.append(f"{cat}: {', '.join(sorted(categories[cat]))}")
    return "\n".join(lines)


def parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="OSINT suite runner (2 sources per domain category)")
    parser.add_argument("--list", action="store_true", help="list available categories and sources")
    parser.add_argument("--category", choices=sorted({s['category'] for s in SOURCES.values()}))
    parser.add_argument("--source", choices=sorted(SOURCES.keys()))
    parser.add_argument("--timeout", type=float, default=30.0)
    parser.add_argument("--limit", type=int, default=200)
    parser.add_argument("--ports", default="1-1024")
    parser.add_argument("--filetypes", default="pdf,doc,docx,xls,xlsx,ppt,pptx")
    parser.add_argument("--out-dir")
    parser.add_argument("--domain")
    parser.add_argument("--target")
    parser.add_argument("--ip")
    parser.add_argument("--username")
    parser.add_argument("--email")
    parser.add_argument("--path")
    parser.add_argument("--repo")
    parser.add_argument("--url")
    parser.add_argument("--query")
    parser.add_argument("--indicator")
    parser.add_argument("--sns-service", default="twitter-user")
    parser.add_argument("--misp-url")
    parser.add_argument("--misp-key")
    parser.add_argument("--opencti-url")
    parser.add_argument("--opencti-token")
    parser.add_argument("--extra-args", help="extra args appended to CLI tools")
    parser.add_argument("--json", action="store_true", help="output JSON")
    return parser.parse_args(argv)


def run_source(name: str, args: argparse.Namespace) -> Dict[str, Any]:
    meta = SOURCES[name]
    result: Dict[str, Any] = {"source": name, "category": meta["category"]}

    if meta["kind"] == "http":
        payload = meta["run"](args)
        result.update(payload)
        return result

    cmd = meta["build"](args)
    if not cmd:
        result["status"] = "failed"
        result["error"] = "missing required input for source"
        return result
    if args.extra_args:
        cmd.extend(shlex.split(args.extra_args))
    payload = run_cli(cmd, args.timeout)
    result.update(payload)
    return result


def main(argv: Optional[List[str]] = None) -> int:
    args = parse_args(argv)
    if args.list:
        print(list_sources())
        return 0

    if not args.source and not args.category:
        print("provide --source or --category (use --list to view options)", file=sys.stderr)
        return 2

    if args.source:
        sources = [args.source]
    else:
        sources = [name for name, meta in SOURCES.items() if meta["category"] == args.category]

    results = [run_source(name, args) for name in sources]
    if args.json:
        print(json.dumps(results, indent=2))
    else:
        for r in results:
            print(f"Source: {r['source']} ({r['category']})")
            if r.get("status") == "success":
                if "data" in r:
                    for line in r["data"]:
                        print(line)
                else:
                    stdout = (r.get("stdout") or "").strip()
                    if stdout:
                        print(stdout)
                print("")
            else:
                print(f"Error: {r.get('error')}")
                print("")
    return 0


if __name__ == "__main__":
    sys.exit(main())
