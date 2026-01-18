#!/usr/bin/env python3
import argparse
import sys
import json
import urllib.parse
from datetime import datetime

try:
    import requests
except ImportError:
    print("requests is required. Install with: pip install requests", file=sys.stderr)
    sys.exit(1)

# GDELT 2.0 Doc API
# https://blog.gdeltproject.org/gdelt-doc-2-0-api-debuts/
GDELT_API_URL = "https://api.gdeltproject.org/api/v2/doc/doc"

def fetch_conflicts(query, max_records=10):
    # Construct a GDELT query
    # mode=artlist gives us a list of articles
    # format=json
    # sort=DateDesc
    params = {
        "query": query,
        "mode": "artlist",
        "maxrecords": max_records,
        "format": "json",
        "sort": "DateDesc"
    }
    
    try:
        resp = requests.get(GDELT_API_URL, params=params, timeout=10)
        resp.raise_for_status()
        data = resp.json()
        return data
    except Exception as e:
        return {"error": str(e)}

def fetch_timeline(query):
    # graphical timeline of volume
    params = {
        "query": query,
        "mode": "timelinevol",
        "format": "json",
    }
    try:
        resp = requests.get(GDELT_API_URL, params=params, timeout=10)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        return {"error": str(e)}

def main():
    parser = argparse.ArgumentParser(description="Conflict & Geopolitical Data (GDELT)")
    parser.add_argument("query", help="Search query (e.g. 'Ukraine', 'Protest', 'Cyberattack')")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    parser.add_argument("--limit", type=int, default=10, help="Max records")
    args = parser.parse_args()

    # Get Articles
    articles = fetch_conflicts(args.query, args.limit)
    
    # Get Volume Timeline (optional, maybe just for JSON output or summary)
    timeline = fetch_timeline(args.query)

    results = {
        "query": args.query,
        "timestamp": datetime.now().isoformat(),
        "articles": articles.get("articles", []),
        "timeline": timeline.get("timeline", [])
    }

    if args.json:
        print(json.dumps(results, indent=2))
    else:
        print(f"Conflict Report: {args.query}")
        print(f"Source: GDELT Project v2")
        print("--------------------------------------------------")
        
        if "error" in articles:
             print(f"Error: {articles['error']}")
             sys.exit(1)

        count = 0
        for art in results["articles"]:
            count += 1
            if count > args.limit: break
            
            title = art.get("title", "No Title")
            source = art.get("source name", "Unknown Source")
            url = art.get("url")
            seen_date = art.get("seendate")
            
            print(f"[{seen_date}] {title}")
            print(f"  Source: {source}")
            print(f"  URL: {url}")
            print("")

if __name__ == "__main__":
    main()
