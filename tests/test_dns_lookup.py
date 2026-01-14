import os
import sys

# Ensure plugin path is importable
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
PLUGIN_PATH = os.path.join(ROOT, 'plugins', 'python')
sys.path.insert(0, PLUGIN_PATH)

import dns_lookup as dl


def test_normalize_ipv6():
    v6 = '2001:db8::1'
    out = dl.normalize_ipv6(v6)
    assert ':' in out


class DummyTXT:
    def __init__(self, strings):
        self.strings = strings


def test_concat_txt():
    d = DummyTXT([b'part1', b'part2'])
    assert dl.concat_txt(d) == 'part1part2'
