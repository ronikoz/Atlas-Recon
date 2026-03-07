#!/usr/bin/env python3
import argparse
import re
import subprocess
import sys


def validate_target(target: str) -> str:
    """Validate target is a valid IP, domain, or CIDR."""
    # Allow alphanumeric, dots, hyphens, colons (IPv6), forward slashes (CIDR)
    if not re.match(r'^[a-zA-Z0-9\-.\/:]+$', target):
        raise ValueError(f"Invalid target format: {target}")
    return target


def validate_ports(ports: str) -> str:
    """Validate port specification."""
    if not ports:
        return ports
    for part in ports.split(','):
        part = part.strip()
        if not part:
            continue
        sep = ":" if ":" in part else "-" if "-" in part else None
        if sep:
            try:
                start, end = part.split(sep)
                if not (start.isdigit() and end.isdigit()):
                    raise ValueError()
            except (ValueError, TypeError):
                raise ValueError(f"Invalid port range: {part}")
        elif not part.isdigit():
            raise ValueError(f"Invalid port: {part}")
    return ports


def main():
    parser = argparse.ArgumentParser(description="Run an nmap scan")
    parser.add_argument("target", help="target host, IP, or CIDR")
    parser.add_argument("--ports", default="", help="comma-separated ports or ranges")
    args, extra = parser.parse_known_args()

    try:
        target = validate_target(args.target)
        ports = validate_ports(args.ports)
    except ValueError as e:
        print(f"validation error: {e}", file=sys.stderr)
        return 2

    cmd = ["nmap", "-sV", target]
    if ports:
        cmd.extend(["-p", ports])
    cmd.extend(extra)

    try:
        print(" ".join(cmd))
        result = subprocess.run(cmd)
        return result.returncode
    except FileNotFoundError:
        print("nmap not found in PATH", file=sys.stderr)
        return 127


if __name__ == "__main__":
    sys.exit(main())


# Signed-off-by: ronikoz
