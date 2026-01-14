import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
PLUGIN_PATH = os.path.join(ROOT, 'plugins', 'python')
sys.path.insert(0, PLUGIN_PATH)

import osint_domain as od


def test_extract_subdomains_basic():
    entries = [
        {"name_value": "example.com\nwww.example.com\n*.dev.example.com"},
    ]
    subs = od.extract_subdomains(entries, 'example.com')
    assert 'www.example.com' in subs
    assert 'dev.example.com' in subs or 'dev.example.com' in [s.lstrip('*.') for s in subs]


def test_cache_roundtrip(tmp_path):
    domain = 'unit-test.example'
    p = od.cache_path_for(domain)
    # ensure using tmp cache dir by pointing CACHE_DIR
    od.CACHE_DIR = tmp_path
    data = [{"name_value": "a.unit-test.example"}]
    od.save_cache(domain, data)
    loaded = od.load_cache(domain)
    assert loaded is not None
    assert loaded[0]['name_value'] == 'a.unit-test.example'
