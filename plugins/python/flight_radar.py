#!/usr/bin/env python3
import argparse
import sys
import json
import requests
import datetime

# OpenSky Network REST API (Anonymous access has limits)
# https://openskynetwork.github.io/opensky-api/rest.html
OPENSKY_API_URL = "https://opensky-network.org/api/states/all"

def get_flights(lat, lon, radius_km=50):
    # Convert radius to bbox strings
    # 1 deg lat ~ 111 km
    deg_delta = radius_km / 111.0
    
    lamin = lat - deg_delta
    lamax = lat + deg_delta
    lomin = lon - deg_delta
    lomax = lon + deg_delta
    
    params = {
        "lamin": lamin,
        "lomin": lomin,
        "lamax": lamax,
        "lomax": lomax
    }
    
    try:
        resp = requests.get(OPENSKY_API_URL, params=params, timeout=10)
        resp.raise_for_status()
        data = resp.json()
        return data
    except Exception as e:
        return {"error": str(e)}

def resolve_location(query):
    # Reuse Geopy if installed, or failover.
    # We'll just assume the user might have geopy from geo_recon.py 
    # OR we can assume `geo_recon.py` logic.
    # For simplicity, let's try to import geopy here.
    try:
        from geopy.geocoders import Nominatim
        geolocator = Nominatim(user_agent="ct-cli-tools-flight")
        location = geolocator.geocode(query)
        if location:
            return location.latitude, location.longitude, location.address
    except ImportError:
        pass
    return None, None, None

def main():
    parser = argparse.ArgumentParser(description="Flight Radar (OpenSky)")
    parser.add_argument("target", help="Location name (e.g. 'Kyiv') or lat,lon")
    parser.add_argument("--radius", type=int, default=100, help="Search radius in km")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    args = parser.parse_args()

    # Parse Target
    lat, lon = None, None
    address = args.target
    
    if "," in args.target:
        try:
            parts = args.target.split(",")
            lat = float(parts[0])
            lon = float(parts[1])
            address = f"Coordinates {lat},{lon}"
        except:
            pass
            
    if lat is None:
        lat, lon, found_addr = resolve_location(args.target)
        if found_addr: 
            address = found_addr

    if lat is None:
        print(f"Error: Could not resolve location '{args.target}'. Install geopy or use lat,lon.", file=sys.stderr)
        sys.exit(1)

    data = get_flights(lat, lon, args.radius)
    
    # OpenSky returns 'states' array: [transponder, callsign, origin_country, time_pos, last_contact, long, lat, baro_altitude, on_ground, velocity, true_track, vertical_rate, sensors, geo_altitude, squawk, spi, position_source]
    
    flights = []
    raw_states = data.get("states", [])
    
    if raw_states:
        for s in raw_states:
            # Basic cleanup
            callsign = s[1].strip()
            country = s[2]
            f_lat = s[6]
            f_lon = s[5]
            alt = s[7]
            speed = s[9]
            on_ground = s[8]
            
            flights.append({
                "callsign": callsign,
                "country": country,
                "lat": f_lat,
                "lon": f_lon,
                "altitude_m": alt,
                "velocity_ms": speed,
                "on_ground": on_ground
            })

    results = {
        "location": address,
        "center": {"lat": lat, "lon": lon},
        "radius_km": args.radius,
        "count": len(flights),
        "flights": flights
    }

    if args.json:
        print(json.dumps(results, indent=2))
    else:
        if "error" in data:
             print(f"Error: {data['error']}", file=sys.stderr)
             sys.exit(1)

        print(f"Flight Radar: {address}")
        print(f"Radius: {args.radius}km | Source: OpenSky Network")
        print("--------------------------------------------------")
        
        if not flights:
            print("No aircraft detected in this sector.")
        
        for f in flights:
            status = "GROUND" if f['on_ground'] else f"AIR ({f.get('altitude_m', 0)}m)"
            print(f"✈  {f['callsign'] or 'N/A'} [{f['country']}]")
            print(f"   Status: {status} | Speed: {f.get('velocity_ms', 0)} m/s")
            print(f"   Pos: {f['lat']}, {f['lon']}")
            print("")

if __name__ == "__main__":
    main()
