#!/usr/bin/env python3
import argparse
import sys
import json
import ssl
import socket
from datetime import datetime
from urllib.parse import urlparse

import warnings
# Suppress urllib3 NotOpenSSLWarning which happens at import time on some systems
with warnings.catch_warnings():
    warnings.simplefilter("ignore")
    try:
        import requests
        import urllib3
        urllib3.disable_warnings()
    except ImportError:
        pass

if 'requests' not in sys.modules:
     print("requests is required. Install with: pip install requests", file=sys.stderr)
     sys.exit(1)

def check_ssl(hostname, port=443):
    context = ssl.create_default_context()
    conn = context.wrap_socket(socket.socket(socket.AF_INET), server_hostname=hostname)
    conn.settimeout(5.0)
    try:
        conn.connect((hostname, port))
        cert = conn.getpeercert()
        return {
            "valid": True,
            "subject": dict(x[0] for x in cert['subject']),
            "issuer": dict(x[0] for x in cert['issuer']),
            "version": cert['version'],
            "notBefore": cert['notBefore'],
            "notAfter": cert['notAfter']
        }
    except Exception as e:
        return {"valid": False, "error": str(e)}
    finally:
        conn.close()

def check_headers(url, timeout):
    try:
        resp = requests.get(url, timeout=timeout, allow_redirects=True)
        return {
            "url": url,
            "status_code": resp.status_code,
            "headers": dict(resp.headers),
            "elapsed_ms": resp.elapsed.microseconds / 1000
        }
    except Exception as e:
        return {"error": str(e)}

def main():
    parser = argparse.ArgumentParser(description="Basic web checks")
    parser.add_argument("url", help="URL to check")
    parser.add_argument("--timeout", type=float, default=10.0)
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()

    url = args.url
    if not url.startswith("http"):
        url = "https://" + url

    parsed = urlparse(url)
    hostname = parsed.hostname or parsed.netloc
    port = parsed.port

    results = {
        "target": url,
        "timestamp": datetime.now().isoformat(),
        "http": check_headers(url, args.timeout),
    }

    if url.startswith("https"):
        results["ssl"] = check_ssl(hostname, port or 443)

    if args.json:
        print(json.dumps(results, indent=2))
    else:
        print(f"Target: {results['target']}")
        
        http = results.get("http", {})
        if "error" in http:
            print(f"HTTP Error: {http['error']}")
        else:
            print(f"Status: {http.get('status_code')} ({http.get('elapsed_ms')}ms)")
            print("Interesting Headers:")
            for k, v in http.get("headers", {}).items():
                if k.lower() in ['server', 'x-powered-by', 'strict-transport-security', 'content-security-policy']:
                    print(f"  {k}: {v}")

        ssl_info = results.get("ssl", {})
        if ssl_info:
            print("\nSSL Certificate:")
            if ssl_info.get("valid"):
                subject = ssl_info.get("subject", {})
                print(f"  Subject: {subject.get('commonName')} ({subject.get('organizationName', 'N/A')})")
                print(f"  Issuer:  {ssl_info.get('issuer', {}).get('commonName')}")
                print(f"  Expires: {ssl_info.get('notAfter')}")
            else:
                print(f"  Error: {ssl_info.get('error')}")

if __name__ == "__main__":
    main()
