#!/usr/bin/env python3
"""
Improved Geospatial Reconnaissance
Features:
- Multiple geocoding sources (Nominatim/OSM primary)
- Accuracy assessment and warnings
- Verification through reverse geocoding
- High-precision coordinate support
- NASA Earth imagery links
"""

import argparse
import sys
import json
import urllib.parse
from datetime import datetime

try:
    from geopy.geocoders import Nominatim
    from geopy.distance import geodesic
except ImportError:
    print("geopy is required. Install with: pip install geopy", file=sys.stderr)
    sys.exit(1)


def assess_accuracy(location_raw):
    """Assess accuracy of geocoding result"""
    accuracy = {
        "confidence": "low",
        "precision_km": 0,
        "issues": []
    }
    
    if 'boundingbox' in location_raw:
        bbox = location_raw['boundingbox']
        try:
            lat_min, lat_max = float(bbox[0]), float(bbox[1])
            lon_min, lon_max = float(bbox[2]), float(bbox[3])
            
            lat_range = lat_max - lat_min
            lon_range = lon_max - lon_min
            max_range = max(lat_range, lon_range)
            
            precision_km = max_range * 111
            accuracy["precision_km"] = precision_km
            
            # Assess confidence level
            if max_range > 0.5:
                accuracy["confidence"] = "very_low"
                accuracy["issues"].append("Country/region-level result only")
            elif max_range > 0.1:
                accuracy["confidence"] = "low"
                accuracy["issues"].append("City/province-level precision")
            elif max_range > 0.01:
                accuracy["confidence"] = "medium"
                accuracy["issues"].append("Neighborhood-level precision")
            else:
                accuracy["confidence"] = "high"
                importance = location_raw.get('importance', 0)
                if importance > 0.7:
                    accuracy["confidence"] = "very_high"
        except (ValueError, IndexError):
            accuracy["issues"].append("Could not parse bounding box")
    
    # Check OSM class/type
    osm_class = location_raw.get('class', '')
    osm_type = location_raw.get('type', '')
    
    if osm_class == 'boundary':
        accuracy["issues"].append("Result is an administrative boundary")
    if osm_type in ['country', 'state', 'county']:
        accuracy["issues"].append(f"Result is {osm_type}-level, try with city name for precision")
    
    return accuracy


def verify_location(lat, lon, address, geolocator):
    """Verify accuracy by reverse geocoding"""
    try:
        reverse = geolocator.reverse(f"{lat}, {lon}", timeout=5)
        if reverse:
            distance_km = geodesic((lat, lon), (reverse.latitude, reverse.longitude)).kilometers
            return {
                "verified": True,
                "reverse_address": reverse.address,
                "distance_km": distance_km,
                "accurate": distance_km < 1.0  # Within 1km is good
            }
    except Exception:
        pass
    return {"verified": False}


def get_links(lat, lon):
    return {
        "Google Earth Web": f"https://earth.google.com/web/@{lat},{lon},1000a,35y,0h,0t,0r",
        "Google Maps": f"https://www.google.com/maps?q={lat},{lon}",
        "Sentinel Hub Playground": f"https://apps.sentinel-hub.com/sentinel-playground/?source=S2&lat={lat}&lng={lon}&zoom=12",
        "OpenStreetMap": f"https://www.openstreetmap.org/#map=15/{lat}/{lon}",
        "Wikimapia": f"http://wikimapia.org/#lat={lat}&lon={lon}&z=15&l=0&m=b",
        "SunCalc": f"https://www.suncalc.org/#/{lat},{lon},12/2023.01.01/12:00/1/3"
    }


def get_nasa_image(lat, lon, api_key="DEMO_KEY", date=None):
    """Generate NASA Earth imagery URL"""
    if not date:
        date = datetime.now().strftime("%Y-%m-%d")
    
    base_url = "https://api.nasa.gov/planetary/earth/imagery"
    params = {
        "lon": lon,
        "lat": lat,
        "date": date,
        "dim": 0.1,
        "api_key": api_key
    }
    encoded = urllib.parse.urlencode(params)
    return f"{base_url}?{encoded}"


def resolve_location(query):
    """Resolve location with improved accuracy assessment"""
    geolocator = Nominatim(user_agent="atlas-recon-geo-recon")
    try:
        location = geolocator.geocode(query, timeout=10)
        if location:
            accuracy = assess_accuracy(location.raw)
            verification = verify_location(
                location.latitude, 
                location.longitude, 
                location.address,
                geolocator
            )
            
            return {
                "found": True,
                "address": location.address,
                "lat": location.latitude,
                "lon": location.longitude,
                "accuracy": accuracy,
                "verification": verification,
                "raw": location.raw
            }
        return {"found": False, "error": "Location not found"}
    except Exception as e:
        return {"found": False, "error": str(e)}


def main():
    parser = argparse.ArgumentParser(description="Geospatial Reconnaissance (Improved)")
    parser.add_argument("query", help="Address, City, or Coordinates (lat,lon)")
    parser.add_argument("--nasa-key", help="NASA API Key", default="DEMO_KEY")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    parser.add_argument("--verify", action="store_true", help="Verify with reverse geocoding", default=True)
    args = parser.parse_args()

    # Parse coordinates if provided as lat,lon
    if "," in args.query and any(c.isdigit() for c in args.query):
        try:
            parts = args.query.split(",")
            lat = float(parts[0].strip())
            lon = float(parts[1].strip())
            data = {
                "found": True,
                "address": "Coordinates Input",
                "lat": lat,
                "lon": lon,
                "accuracy": {"confidence": "exact", "precision_km": 0, "issues": []}
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

        print(f"Geospatial Recon: {args.query}")
        print(f"  Resolved Address: {data.get('address')}")
        print(f"  Coordinates:      {data.get('lat'):.6f}, {data.get('lon'):.6f}")
        
        # Show accuracy information
        accuracy = data.get("accuracy", {})
        print(f"\n  Accuracy Assessment:")
        print(f"    Confidence:  {accuracy.get('confidence', 'unknown')}")
        print(f"    Precision:   ±{accuracy.get('precision_km', 0):.1f} km")
        
        if accuracy.get('issues'):
            print(f"    ⚠ Issues:")
            for issue in accuracy['issues']:
                print(f"      - {issue}")
        
        # Show verification if available
        verification = data.get("verification", {})
        if verification.get('verified'):
            print(f"\n  Reverse Geocoding Verification:")
            print(f"    Address: {verification.get('reverse_address')}")
            print(f"    Distance: {verification.get('distance_km'):.2f} km")
            if verification.get('accurate'):
                print(f"    ✓ Result is accurate")
            else:
                print(f"    ⚠ WARNING: Significant distance between forward and reverse geocoding")
        
        print(f"\n  NASA Imagery: {data.get('nasa_image_url')}")
        print(f"\n  Recon Links:")
        for name, link in data.get("links", {}).items():
            print(f"    [{name}]")
            print(f"    {link}")


if __name__ == "__main__":
    main()
