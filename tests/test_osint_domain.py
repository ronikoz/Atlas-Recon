import pytest
import os
import sys

# Add python plugins directory to the path so we can import the module directly
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../plugins/python')))

from osint_domain import extract_subdomains, cluster_by_ip

def test_extract_subdomains():
    entries = [
        {"name_value": "www.example.com"},
        {"name_value": "api.example.com\n*.example.com"},
        {"common_name": "example.com"},
        {"name_value": "otherdomain.com"} # Should be ignored
    ]
    
    subs = extract_subdomains(entries, "example.com")
    assert len(subs) == 3
    assert "www.example.com" in subs
    assert "api.example.com" in subs
    assert "example.com" in subs
    assert "otherdomain.com" not in subs

def test_cluster_by_ip():
    dnsmap = {
        "www.example.com": ["1.2.3.4", "5.6.7.8"],
        "api.example.com": ["1.2.3.4"],
        "mail.example.com": ["9.10.11.12"]
    }
    
    clusters = cluster_by_ip(dnsmap)
    assert len(clusters) == 3
    
    assert "www.example.com" in clusters["1.2.3.4"]
    assert "api.example.com" in clusters["1.2.3.4"]
    assert len(clusters["1.2.3.4"]) == 2
    
    assert "www.example.com" in clusters["5.6.7.8"]
    assert len(clusters["5.6.7.8"]) == 1
    
    assert "mail.example.com" in clusters["9.10.11.12"]
    assert len(clusters["9.10.11.12"]) == 1
