#!/usr/bin/env python3
import argparse
import sys
import json
import requests

# Polymarket Gamma API
# https://docs.polymarket.com/
POLY_API_URL = "https://gamma-api.polymarket.com/events"

def fetch_markets(query, limit=10):
    params = {
        "q": query,
        "limit": limit,
        "closed": "false", # fetch only open markets
        "sort": "volume",  # sort by volume to get most relevant
    }
    
    try:
        resp = requests.get(POLY_API_URL, params=params, timeout=10)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        return {"error": str(e)}

def main():
    parser = argparse.ArgumentParser(description="Prediction Market Sentiment (Polymarket)")
    parser.add_argument("query", help="Search term (e.g. 'Election', 'Trump', 'Ukraine')")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    parser.add_argument("--limit", type=int, default=5, help="Max markets")
    args = parser.parse_args()

    data = fetch_markets(args.query, args.limit)

    if args.json:
        print(json.dumps(data, indent=2))
    else:
        if isinstance(data, dict) and "error" in data:
            print(f"Error: {data['error']}", file=sys.stderr)
            sys.exit(1)

        print(f"Market Sentiment: {args.query}")
        print(f"Source: Polymarket")
        print("--------------------------------------------------")
        
        # Polymarket API returns a list of events usually
        if not data:
            print("No markets found.")
            return

        query_lower = args.query.lower()
        
        filtered_count = 0
        for event in data:
            title = event.get("title", "")
            markets = event.get("markets", [])
            
            # Filter: Check if query matches Title or any Market question
            # Polymarket's search API is sometimes fuzzy/broad.
            
            relevant_markets = []
            for m in markets:
                q_text = m.get("question", "")
                if query_lower in title.lower() or query_lower in q_text.lower():
                    relevant_markets.append(m)

            # If no specific market matched but the Event Title matched, show all Top 3 markets
            if not relevant_markets and query_lower in title.lower():
                 relevant_markets = markets[:3]

            if not relevant_markets:
                continue
                
            filtered_count += 1
            print(f"Event: {title}")
            
            for m in relevant_markets:
                question = m.get("question")
                print(f"  - {question}")
                try:
                    prices = json.loads(m.get("outcomePrices", "[]"))
                    outcomes = json.loads(m.get("outcomes", "[]"))
                    
                    if len(prices) == len(outcomes):
                        for i, name in enumerate(outcomes):
                            prob = float(prices[i]) * 100
                            # Highlight the likely winner
                            marker = "*" if prob > 50 else " "
                            print(f"    {marker} {name}: {prob:.1f}%")
                except:
                    pass
            print("")
        
        if filtered_count == 0:
             print(f"No relevant markets found for '{args.query}' (API returned unrelated results).")

if __name__ == "__main__":
    main()
