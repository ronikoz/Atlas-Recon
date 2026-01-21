#!/usr/bin/env python3
import argparse
import sys
import json
import urllib.parse
from datetime import datetime

try:
    from geopy.geocoders import Nominatim
except ImportError:
    print("geopy is required. Install with: pip install geopy", file=sys.stderr)
    sys.exit(1)

def get_links(lat, lon):
    return {
        "Google Earth Web": f"https://earth.google.com/web/@{lat},{lon},1000a,35y,0h,0t,0r",
        "Google Maps": f"https://www.google.com/maps?q={lat},{lon}",
        "Sentinel Hub Playground": f"https://apps.sentinel-hub.com/sentinel-playground/?source=S2&lat={lat}&lng={lon}&zoom=12",
        "OpenStreetMap": f"https://www.openstreetmap.org/#map=15/{lat}/{lon}",
        "Wikimapia": f"http://wikimapia.org/#lat={lat}&lon={lon}&z=15&l=0&m=b",
        "SunCalc": f"https://www.suncalc.org/#/{lat},{lon},12/2023.01.01/12:00/1/3" # useful for shadow analysis
    }

def get_nasa_image(lat, lon, api_key="DEMO_KEY", date=None):
    # NASA Earth Imagery API
    # https://api.nasa.gov/planetary/earth/imagery
    if not date:
        date = datetime.now().strftime("%Y-%m-%d")
    
    base_url = "https://api.nasa.gov/planetary/earth/imagery"
    params = {
        "lon": lon,
        "lat": lat,
        "date": date,
        "dim": 0.1, # degrees width
        "api_key": api_key
    }
    encoded = urllib.parse.urlencode(params)
    return f"{base_url}?{encoded}"

def resolve_location(query):
    geolocator = Nominatim(user_agent="ct-cli-tools-recon")
    try:
        location = geolocator.geocode(query)
        if location:
            return {
                "found": True,
                "address": location.address,
                "lat": location.latitude,
                "lon": location.longitude,
                "raw": location.raw
            }
        return {"found": False, "error": "Location not found"}
    except Exception as e:
        return {"found": False, "error": str(e)}

def main():
    parser = argparse.ArgumentParser(description="Geospatial Recon")
    parser.add_argument("query", help="Address, City, or Coordinates (lat,lon) to resolve")
    parser.add_argument("--nasa-key", help="NASA API Key (optional)", default="DEMO_KEY")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    args = parser.parse_args()

    # check if input is lat,lon manually
    if "," in args.query and any(c.isdigit() for c in args.query):
        try:
            parts = args.query.split(",")
            lat = float(parts[0].strip())
            lon = float(parts[1].strip())
            data = {
                "found": True,
                "address": "Coordinates Input",
                "lat": lat,
                "lon": lon
            }
        except ValueError:
             data = resolve_location(args.query)
    else:
        data = resolve_location(args.query)

    if data.get("found"):
        lat = data["lat"]
        lon = data["lon"]
        data["links"] = get_links(lat, lon)
        data["nasa_image_url"] = get_nasa_image(lat, lon, args.nasa_key)

    if args.json:
        print(json.dumps(data, indent=2))
    else:
        if not data.get("found"):
            print(f"Error: {data.get('error')}", file=sys.stderr)
            sys.exit(1)

        print(f"Recon Results for: {args.query}")
        print(f"  Resolved: {data.get('address')}")
        print(f"  Coords:   {data.get('lat')}, {data.get('lon')}")
        print(f"  NASA Img: {data.get('nasa_image_url')}")
        print("\nRecon Links:")
        for name, link in data.get("links", {}).items():
            print(f"  [{name}]")
            print(f"  {link}")

if __name__ == "__main__":
    main()
