#!/usr/bin/env python3
import argparse
import sys
import json
import urllib.parse
import requests

try:
    import phonenumbers
    from phonenumbers import geocoder, carrier, timezone
    from geopy.geocoders import Nominatim
except ImportError:
    print("Dependencies missing. Install with: pip install phonenumbers geopy", file=sys.stderr)
    sys.exit(1)

def generate_social_links(e164_number):
    """Generate direct deep links for messaging apps."""
    clean_num = e164_number.replace("+", "")
    return {
        "WhatsApp": f"https://wa.me/{clean_num}",
        "Telegram": f"https://t.me/{clean_num}",
        "Viber": f"viber://chat?number={clean_num}",
        "Skype": f"skype:{clean_num}?call",
        "FaceTime": f"facetime://{clean_num}"
    }

def generate_dorks(number, national_fmt):
    """Generate extensive Google Dorks (PhoneInfoga style)."""
    q_base = f'"{number}"'
    q_nat = f'"{national_fmt}"'
    
    dorks = {
        "General Search": f"https://www.google.com/search?q={urllib.parse.quote(q_base)}",
        "National Format": f"https://www.google.com/search?q={urllib.parse.quote(q_nat)}",
        "Facebook": f"https://www.google.com/search?q={urllib.parse.quote('site:facebook.com ' + q_base)}",
        "Twitter": f"https://www.google.com/search?q={urllib.parse.quote('site:twitter.com ' + q_base)}",
        "LinkedIn": f"https://www.google.com/search?q={urllib.parse.quote('site:linkedin.com ' + q_base)}",
        "Instagram": f"https://www.google.com/search?q={urllib.parse.quote('site:instagram.com ' + q_base)}",
        "Documents (PDF/DOC)": f"https://www.google.com/search?q={urllib.parse.quote('filetype:pdf OR filetype:doc ' + q_base)}",
        "Pastebin/Dumps": f"https://www.google.com/search?q={urllib.parse.quote('site:pastebin.com OR site:nomorobo.com ' + q_base)}",
        "TrueCaller/Sync": f"https://www.google.com/search?q={urllib.parse.quote('site:truecaller.com OR site:sync.me ' + q_base)}",
        "Disposable Numbers": f"https://www.google.com/search?q={urllib.parse.quote('site:receive-smss.com OR site:receive-sms-online.info ' + q_base)}"
    }
    return dorks

def get_approx_coords(location_name):
    """
    'Digital Triangulation': converts the registered location (Region/City) 
    into Lat/Lon coordinates to visualize the number's origin.
    """
    if not location_name:
        return None
    try:
        geolocator = Nominatim(user_agent="atlas-recon-phone")
        loc = geolocator.geocode(location_name)
        if loc:
            return {
                "lat": loc.latitude,
                "lon": loc.longitude,
                "address": loc.address,
                "map": f"https://www.google.com/maps?q={loc.latitude},{loc.longitude}"
            }
    except:
        pass
    return None

def analyze_number(number_str):
    try:
        parsed = phonenumbers.parse(number_str, None)
    except phonenumbers.NumberParseException as e:
        return {"error": str(e)}

    if not phonenumbers.is_valid_number(parsed):
        return {"valid": False, "possible": phonenumbers.is_possible_number(parsed)}

    # Get data
    region_code = phonenumbers.region_code_for_number(parsed)
    location_desc = geocoder.description_for_number(parsed, "en")
    carrier_name = carrier.name_for_number(parsed, "en")
    time_zones = list(timezone.time_zones_for_number(parsed))
    
    formatted_e164 = phonenumbers.format_number(parsed, phonenumbers.PhoneNumberFormat.E164)
    formatted_intl = phonenumbers.format_number(parsed, phonenumbers.PhoneNumberFormat.INTERNATIONAL)
    formatted_national = phonenumbers.format_number(parsed, phonenumbers.PhoneNumberFormat.NATIONAL)
    
    # "Digital Triangulation" (Geocoding the area code/region)
    geo_data = get_approx_coords(location_desc or region_code)
    
    num_type = phonenumbers.number_type(parsed)
    type_str = "Mobile" if num_type == phonenumbers.PhoneNumberType.MOBILE else \
               "Fixed Line" if num_type == phonenumbers.PhoneNumberType.FIXED_LINE else \
               "VoIP" if num_type == phonenumbers.PhoneNumberType.VOIP else "Unknown"

    return {
        "valid": True,
        "input": number_str,
        "e164": formatted_e164,
        "international": formatted_intl,
        "national": formatted_national,
        "type": type_str,
        "region_code": region_code,
        "location": location_desc,
        "carrier": carrier_name,
        "timezones": time_zones,
        "geoloc": geo_data,
        "possible": True
    }

def main():
    parser = argparse.ArgumentParser(description="Phone Number OSINT (Enhanced)")
    parser.add_argument("number", help="Phone number (e.g. +14155552671)")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    args = parser.parse_args()

    data = analyze_number(args.number)
    
    if data.get("valid"):
        data["dorks"] = generate_dorks(data["e164"], data["national"])
        data["social_links"] = generate_social_links(data["e164"])

    if args.json:
        print(json.dumps(data, indent=2))
    else:
        if "error" in data:
            print(f"Error parsing number: {data['error']}")
            sys.exit(1)
        
        print(f"Phone Intelligence: {args.number}")
        print("--------------------------------------------------")
        print(f"  Valid:      {data.get('valid')}")
        print(f"  Type:       {data.get('type')}")
        print(f"  Format:     {data.get('international')} / {data.get('e164')}")
        print(f"  Carrier:    {data.get('carrier') or 'Unknown'}")
        print(f"  Location:   {data.get('location') or 'Unknown'}")
        print(f"  Region:     {data.get('region_code')}")
        print(f"  Timezones:  {', '.join(data.get('timezones', []))}")
        
        if data.get("geoloc"):
            print("\n[Digital Triangulation]")
            print(f"  Inferred Area: {data['geoloc']['address']}")
            print(f"  Coordinates:   {data['geoloc']['lat']}, {data['geoloc']['lon']}")
            print(f"  Map View:      {data['geoloc']['map']}")
        else:
             print("\n[Triangulation]: No specific region data available for this number type.")

        print("\n[Direct Social Links]")
        for platform, link in data.get("social_links", {}).items():
            print(f"  {platform}: {link}")

        print("\n[Reconnaissance Dorks] (CMD+Click)")
        for name, link in data.get("dorks", {}).items():
            print(f"  * {name}")
            print(f"    {link}")

if __name__ == "__main__":
    main()
