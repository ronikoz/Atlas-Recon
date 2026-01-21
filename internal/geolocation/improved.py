#!/usr/bin/env python3
"""
Improved Geolocation Module
Provides multiple geocoding sources with accuracy verification
"""

import sys
import json
from typing import Optional, Dict, Tuple, List

try:
    from geopy.geocoders import Nominatim, GoogleV3
    from geopy.exc import GeocoderTimedOut
    from geopy.distance import geodesic
except ImportError:
    print("geopy is required. Install with: pip install geopy", file=sys.stderr)
    sys.exit(1)

class GeoLocation:
    """Improved geolocation with multiple sources and accuracy checks"""
    
    # Max bounding box size acceptable (in degrees)
    # 0.5 deg ~ 55 km, 0.1 deg ~ 11 km, 0.01 deg ~ 1.1 km
    MAX_BBOX_SIZE = 0.1  # ~11 km acceptable range
    
    def __init__(self):
        self.nominatim = Nominatim(user_agent="ct-cli-tools-geoloc")
        self.results = []
    
    def geocode_nominatim(self, query: str) -> Optional[Dict]:
        """Geocode using Nominatim (OSM) - free, no API key needed"""
        try:
            location = self.nominatim.geocode(query, timeout=10)
            if location:
                return {
                    "provider": "Nominatim (OSM)",
                    "address": location.address,
                    "lat": location.latitude,
                    "lon": location.longitude,
                    "raw": location.raw,
                    "accuracy": self._assess_accuracy(location.raw)
                }
        except GeocoderTimedOut:
            print("Nominatim geocoder timed out", file=sys.stderr)
        except Exception as e:
            print(f"Nominatim error: {e}", file=sys.stderr)
        return None
    
    def _assess_accuracy(self, raw_data: Dict) -> Dict:
        """Assess accuracy based on bounding box and other factors"""
        accuracy = {
            "confidence": "low",
            "precision_km": 0,
            "issues": []
        }
        
        # Check importance (0-1, higher = more important/precise)
        importance = raw_data.get('importance', 0)
        
        # Check bounding box size
        if 'boundingbox' in raw_data:
            bbox = raw_data['boundingbox']
            try:
                lat_min, lat_max = float(bbox[0]), float(bbox[1])
                lon_min, lon_max = float(bbox[2]), float(bbox[3])
                
                lat_range = lat_max - lat_min
                lon_range = lon_max - lon_min
                max_range = max(lat_range, lon_range)
                
                # Estimate precision in km
                precision_km = max_range * 111  # 1 degree ~ 111 km
                accuracy["precision_km"] = precision_km
                
                # Assess confidence
                if max_range > 0.5:
                    accuracy["confidence"] = "very_low"
                    accuracy["issues"].append("Country/region-level precision only")
                elif max_range > 0.1:
                    accuracy["confidence"] = "low"
                    accuracy["issues"].append("City/province-level precision")
                elif max_range > 0.01:
                    accuracy["confidence"] = "medium"
                    accuracy["issues"].append("Neighborhood-level precision")
                else:
                    accuracy["confidence"] = "high"
                    if importance > 0.7:
                        accuracy["confidence"] = "very_high"
                        
            except (ValueError, IndexError):
                accuracy["issues"].append("Could not parse bounding box")
        
        # Check for known precision issues
        osm_class = raw_data.get('class', '')
        osm_type = raw_data.get('type', '')
        
        if osm_class == 'boundary':
            accuracy["issues"].append("Administrative boundary - may be imprecise")
        if osm_type in ['country', 'state', 'county']:
            accuracy["issues"].append(f"Result is {osm_type}-level - recommend adding city name")
        
        return accuracy
    
    def reverse_geocode(self, lat: float, lon: float) -> Optional[Dict]:
        """Reverse geocode (coordinates to address)"""
        try:
            location = self.nominatim.reverse(f"{lat}, {lon}", timeout=10)
            return {
                "provider": "Nominatim (OSM)",
                "address": location.address,
                "lat": location.latitude,
                "lon": location.longitude,
                "raw": location.raw,
                "accuracy": self._assess_accuracy(location.raw)
            }
        except Exception as e:
            print(f"Reverse geocode error: {e}", file=sys.stderr)
            return None
    
    def verify_accuracy(self, lat: float, lon: float, address: str) -> Dict:
        """Verify accuracy by geocoding address and comparing with coordinates"""
        forward = self.geocode_nominatim(address)
        if not forward:
            return {"verified": False, "error": "Could not geocode address"}
        
        # Calculate distance
        calc_lat = forward['lat']
        calc_lon = forward['lon']
        distance_km = geodesic((lat, lon), (calc_lat, calc_lon)).kilometers
        
        return {
            "verified": True,
            "original_coords": (lat, lon),
            "geocoded_coords": (calc_lat, calc_lon),
            "distance_km": distance_km,
            "accuracy_assessment": forward['accuracy'],
            "accurate": distance_km < 5  # Within 5 km is acceptable
        }
    
    def geocode(self, query: str, verify: bool = False) -> Optional[Dict]:
        """
        Geocode with accuracy assessment and optional verification
        """
        result = self.geocode_nominatim(query)
        
        if result and verify:
            # Reverse geocode to verify
            reverse = self.reverse_geocode(result['lat'], result['lon'])
            if reverse:
                result['verification'] = {
                    'reverse_address': reverse['address'],
                    'distance_km': geodesic(
                        (result['lat'], result['lon']),
                        (reverse['lat'], reverse['lon'])
                    ).kilometers
                }
        
        return result

def format_result(result: Dict, detailed: bool = False) -> str:
    """Format result for display"""
    output = []
    
    if not result:
        return "No result"
    
    output.append(f"Address:      {result['address']}")
    output.append(f"Coordinates:  {result['lat']:.6f}, {result['lon']:.6f}")
    output.append(f"Provider:     {result['provider']}")
    
    accuracy = result.get('accuracy', {})
    output.append(f"\nAccuracy:")
    output.append(f"  Confidence:  {accuracy.get('confidence', 'unknown')}")
    output.append(f"  Precision:   ±{accuracy.get('precision_km', 0):.1f} km")
    
    if accuracy.get('issues'):
        output.append(f"  Issues:")
        for issue in accuracy['issues']:
            output.append(f"    - {issue}")
    
    if detailed and result.get('raw'):
        output.append(f"\nRaw Data:")
        output.append(json.dumps(result['raw'], indent=2))
    
    if result.get('verification'):
        v = result['verification']
        output.append(f"\nVerification:")
        output.append(f"  Reverse Address: {v['reverse_address']}")
        output.append(f"  Verification Distance: {v['distance_km']:.2f} km")
    
    return "\n".join(output)

if __name__ == "__main__":
    # Test the module
    geo = GeoLocation()
    
    result = geo.geocode("Tel Aviv, Israel", verify=True)
    print(format_result(result, detailed=True))
