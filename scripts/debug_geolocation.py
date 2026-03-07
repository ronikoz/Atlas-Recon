#!/usr/bin/env python3
"""
Geolocation Accuracy Debug Tool
Helps identify where geolocation inaccuracies are coming from
"""

import sys
import json
import argparse
from datetime import datetime

try:
    from geopy.geocoders import Nominatim
except ImportError:
    print("geopy is required. Install with: pip install geopy", file=sys.stderr)
    sys.exit(1)

def debug_geocoding(query, verbose=True):
    """Debug geocoding accuracy"""
    print(f"\n{'='*70}")
    print(f"DEBUG: Geocoding Query: {query}")
    print(f"{'='*70}\n")
    
    geolocator = Nominatim(user_agent="atlas-recon-debug")
    
    if verbose:
        print("[1] Attempting to geocode...")
    
    try:
        location = geolocator.geocode(query)
        if location:
            print(f"[✓] Geocoding successful\n")
            print(f"Address:      {location.address}")
            print(f"Coordinates:  {location.latitude}, {location.longitude}")
            print(f"Latitude:     {location.latitude}")
            print(f"Longitude:    {location.longitude}")
            print(f"\nRaw Data:")
            print(json.dumps(location.raw, indent=2))
            
            # Check for accuracy issues
            print(f"\n[2] Accuracy Analysis:")
            raw = location.raw
            
            # OpenStreetMap/Nominatim provides these fields
            if 'class' in raw:
                print(f"  OSM Class:    {raw.get('class')}")
            if 'type' in raw:
                print(f"  OSM Type:     {raw.get('type')}")
            if 'importance' in raw:
                print(f"  Importance:   {raw.get('importance')}")
            
            # Boundingbox can indicate resolution
            if 'boundingbox' in raw:
                bbox = raw['boundingbox']
                lat_range = float(bbox[1]) - float(bbox[0])
                lon_range = float(bbox[3]) - float(bbox[2])
                print(f"  Bounding Box: [{bbox[0]}, {bbox[1]}, {bbox[2]}, {bbox[3]}]")
                print(f"  Lat Range:    {lat_range:.4f}° (~{lat_range*111:.1f}km)")
                print(f"  Lon Range:    {lon_range:.4f}° (~{lon_range*111:.1f}km)")
                
                if lat_range > 0.5 or lon_range > 0.5:
                    print(f"  ⚠  WARNING: Large bounding box indicates low precision!")
                    print(f"      Result is likely at country/region level, not city level")
            
            return location
        else:
            print(f"[✗] Location not found")
            return None
            
    except Exception as e:
        print(f"[✗] Error during geocoding: {e}")
        return None

def compare_locations(location1, location2):
    """Compare two locations and calculate distance"""
    from geopy.distance import geodesic
    
    lat1, lon1 = location1.latitude, location1.longitude
    lat2, lon2 = location2.latitude, location2.longitude
    
    distance_km = geodesic((lat1, lon1), (lat2, lon2)).kilometers
    
    print(f"\n{'='*70}")
    print(f"COMPARISON")
    print(f"{'='*70}")
    print(f"Location 1: {location1.address}")
    print(f"  Coords: {lat1}, {lon1}")
    print(f"\nLocation 2: {location2.address}")
    print(f"  Coords: {lat2}, {lon2}")
    print(f"\nDistance: {distance_km:.2f} km")
    
    if distance_km > 150:
        print(f"⚠  WARNING: Locations are {distance_km:.0f}km apart!")
        print(f"  This indicates a major accuracy issue.")
    
    return distance_km

def main():
    parser = argparse.ArgumentParser(description="Geolocation Accuracy Debug")
    parser.add_argument("location", help="Location to debug")
    parser.add_argument("--compare", help="Compare with another location")
    parser.add_argument("--json", action="store_true", help="Output as JSON")
    args = parser.parse_args()
    
    loc1 = debug_geocoding(args.location, verbose=not args.json)
    
    if loc1 and args.compare:
        loc2 = debug_geocoding(args.compare, verbose=not args.json)
        if loc2:
            distance = compare_locations(loc1, loc2)
            
            if args.json:
                print(json.dumps({
                    "location1": {
                        "query": args.location,
                        "address": loc1.address,
                        "lat": loc1.latitude,
                        "lon": loc1.longitude
                    },
                    "location2": {
                        "query": args.compare,
                        "address": loc2.address,
                        "lat": loc2.latitude,
                        "lon": loc2.longitude
                    },
                    "distance_km": distance
                }, indent=2))

if __name__ == "__main__":
    main()
