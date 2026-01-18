#!/usr/bin/env python3
import argparse
import sys
import json
import urllib.parse

try:
    import phonenumbers
    from phonenumbers import geocoder, carrier, timezone
except ImportError:
    print("phonenumbers is required. Install with: pip install phonenumbers", file=sys.stderr)
    sys.exit(1)

def generate_dorks(number):
    """Generate Google Dork links for the number."""
    # Common dork patterns
    # "number"
    # site:facebook.com "number"
    # site:linkedin.com "number"
    # ext:txt "number"
    
    q_base = f'"{number}"'
    dorks = {
        "General Search": f"https://www.google.com/search?q={urllib.parse.quote(q_base)}",
        "Social Media (Facebook)": f"https://www.google.com/search?q={urllib.parse.quote('site:facebook.com ' + q_base)}",
        "Social Media (LinkedIn)": f"https://www.google.com/search?q={urllib.parse.quote('site:linkedin.com ' + q_base)}",
        "Social Media (Twitter/X)": f"https://www.google.com/search?q={urllib.parse.quote('site:twitter.com ' + q_base)}",
        "Social Media (Instagram)": f"https://www.google.com/search?q={urllib.parse.quote('site:instagram.com ' + q_base)}",
        "Text Files/Pastebins": f"https://www.google.com/search?q={urllib.parse.quote('ext:txt OR site:pastebin.com ' + q_base)}",
        "Directory Listings": f"https://www.google.com/search?q={urllib.parse.quote('intitle:index.of ' + q_base)}",
        "Locanto/Classifieds": f"https://www.google.com/search?q={urllib.parse.quote('site:locanto.com OR site:craigslist.org ' + q_base)}",
    }
    return dorks

def analyze_number(number_str):
    try:
        # Parse the number. Default region to US if not specified, 
        # but the "+" format is preferred.
        parsed = phonenumbers.parse(number_str, None)
    except phonenumbers.NumberParseException as e:
        return {"error": str(e)}

    if not phonenumbers.is_valid_number(parsed):
        return {"valid": False, "possible": phonenumbers.is_possible_number(parsed)}

    # Get data
    region_code = phonenumbers.region_code_for_number(parsed)
    location = geocoder.description_for_number(parsed, "en")
    carrier_name = carrier.name_for_number(parsed, "en")
    time_zones = timezone.time_zones_for_number(parsed)
    formatted_e164 = phonenumbers.format_number(parsed, phonenumbers.PhoneNumberFormat.E164)
    formatted_intl = phonenumbers.format_number(parsed, phonenumbers.PhoneNumberFormat.INTERNATIONAL)
    formatted_national = phonenumbers.format_number(parsed, phonenumbers.PhoneNumberFormat.NATIONAL)

    return {
        "valid": True,
        "input": number_str,
        "e164": formatted_e164,
        "international": formatted_intl,
        "national": formatted_national,
        "region_code": region_code,
        "location": location,
        "carrier": carrier_name,
        "timezones": list(time_zones),
        "type": phonenumbers.number_type(parsed), # 0=Fixed, 1=Mobile, 2=Fixed/Mobile ...
        "possible": True
    }

def main():
    parser = argparse.ArgumentParser(description="Phone Number OSINT")
    parser.add_argument("number", help="Phone number to analyze (e.g. +14155552671)")
    parser.add_argument("--json", action="store_true", help="Output JSON")
    args = parser.parse_args()

    data = analyze_number(args.number)
    
    # If valid, generate dorks
    if data.get("valid"):
        data["dorks"] = generate_dorks(data["e164"])
        # Also generate dorks for national format without spaces if useful
        # broad_dorks = generate_dorks(data["national"]) 

    if args.json:
        print(json.dumps(data, indent=2))
    else:
        if "error" in data:
            print(f"Error parsing number: {data['error']}")
            sys.exit(1)
        
        print(f"Analysis for: {args.number}")
        print(f"  Valid: {data.get('valid')}")
        
        if data.get("valid"):
            print(f"  Format (E164): {data.get('e164')}")
            print(f"  Format (Intl): {data.get('international')}")
            print(f"  Location: {data.get('location')}")
            print(f"  Region:   {data.get('region_code')}")
            print(f"  Carrier:  {data.get('carrier')}")
            print(f"  Timezones: {', '.join(data.get('timezones', []))}")
            
            print("\nOSINT Search Links (CMD+Click to open):")
            for name, link in data.get("dorks", {}).items():
                print(f"  [{name}]")
                print(f"  {link}")
                print()

if __name__ == "__main__":
    main()
