#!/usr/bin/env python3
import argparse
import sys
import json
import xml.etree.ElementTree as ET
import requests

# ISW RSS Feed
ISW_RSS_URL = "https://www.understandingwar.org/feeds.xml"

def get_isw_updates(limit=5):
    try:
        headers = {'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36'}
        resp = requests.get(ISW_RSS_URL, headers=headers, timeout=15)
        resp.raise_for_status()
        
        # Simple XML parsing
        root = ET.fromstring(resp.content)
        
        items = []
        count = 0
        ns = {'dc': 'http://purl.org/dc/elements/1.1/'} # ISW uses DC for creator
        
        # RSS 2.0 usually has channel/item
        # But ISW might be standard RSS
        
        for item in root.findall('.//item'):
            if count >= limit: break
            
            title = item.findtext('title') or "Untitled"
            link = item.findtext('link') or ""
            desc = item.findtext('description') or ""
            pub_date = item.findtext('pubDate') or ""
            
            # Simple content clean up (remove heavy html if needed, but summary usually ok)
            
            items.append({
                "title": title,
                "link": link,
                "pubDate": pub_date,
                "description": desc[:200] + "..." if desc else ""
            })
            count += 1
            
        return items
    except Exception as e:
        return {"error": str(e)}

def deepstate_link(lat, lon):
    return f"https://deepstatemap.live/en#10/{lat}/{lon}"

def main():
    parser = argparse.ArgumentParser(description="War Intelligence Edge")
    parser.add_argument("target", help="Focus Region (e.g. 'Ukraine', 'Gaza') or lat,lon for map links")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    args = parser.parse_args()

    # If target resolves to coords, provide map links
    lat, lon = None, None
    map_links = {}
    
    # Try geopy for coords
    try:
        from geopy.geocoders import Nominatim
        geolocator = Nominatim(user_agent="ct-cli-tools-war")
        loc = geolocator.geocode(args.target)
        if loc:
            lat, lon = loc.latitude, loc.longitude
            map_links["DeepStateMap"] = deepstate_link(lat, lon)
            map_links["LiveUAMap"] = f"https://{args.target.lower().replace(' ', '')}.liveuamap.com/"
            map_links["Google Maps"] = f"https://www.google.com/maps/@{lat},{lon},11z"
    except:
        pass

    # Get ISW Feeds
    isw_reports = get_isw_updates(limit=3)

    results = {
        "target": args.target,
        "map_links": map_links,
        "intelligence_feed": isw_reports
    }

    if args.json:
        print(json.dumps(results, indent=2))
    else:
        print(f"War Intelligence Edge: {args.target}")
        print("--------------------------------------------------")
        
        if map_links:
            print("Real-time Conflict Maps:")
            for name, link in map_links.items():
                print(f"* {name}: {link}")
            print("")
        
        print(f"Latest ISW Assessment Reports:")
        if isinstance(isw_reports, dict) and "error" in isw_reports:
             print(f"[-] Could not auto-fetch ISW Feed (Protection active).")
             print(f"[*] Access Daily Reports here: https://www.understandingwar.org/backgrounder/ukraine-conflict-updates")
        else:
             for report in isw_reports:
                 print(f"[{report['pubDate'][:16]}] {report['title']}")
                 print(f"  Link: {report['link']}")
                 print("")

if __name__ == "__main__":
    main()
