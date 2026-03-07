import pytest
import os
import sys

# Add python plugins directory to the path so we can import the module directly
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../plugins/python')))

from geo_recon import assess_accuracy, get_links, get_nasa_image

def test_assess_accuracy_exact():
    # If there is no boundingbox, we return default "low" right now. Let's test the graceful fallback
    accuracy = assess_accuracy({})
    assert accuracy["confidence"] == "low"
    assert accuracy["precision_km"] == 0

def test_assess_accuracy_high():
    raw = {
        "boundingbox": ["40.68", "40.69", "-74.05", "-74.04"]
    }
    accuracy = assess_accuracy(raw)
    assert accuracy["confidence"] == "high"
    assert accuracy["precision_km"] <= 2.0

def test_assess_accuracy_very_low():
    raw = {
        "boundingbox": ["30.0", "40.0", "-100.0", "-90.0"] # 10 degrees wide
    }
    accuracy = assess_accuracy(raw)
    assert accuracy["confidence"] == "very_low"
    assert accuracy["precision_km"] > 1000

def test_get_nasa_image():
    url = get_nasa_image(40.6892, -74.0445, "TEST_KEY")
    assert url.startswith("https://api.nasa.gov/planetary/earth/imagery")
    assert "lat=40.6892" in url
    assert "lon=-74.0445" in url
    assert "api_key=TEST_KEY" in url

def test_get_links():
    links = get_links(10.0, 20.0)
    assert "Google Maps" in links
    assert "10.0,20.0" in links["Google Maps"]
    assert "OpenStreetMap" in links
    assert "15/10.0/20.0" in links["OpenStreetMap"]
