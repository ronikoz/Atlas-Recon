import pytest
import os
import sys

# Add python plugins directory to the path so we can import the module directly
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../plugins/python')))

from dns_lookup import normalize_ipv6, concat_txt

def test_normalize_ipv6():
    # Valid IPv6
    addr = "2001:db8::1"
    assert normalize_ipv6(addr) == "2001:0db8:0000:0000:0000:0000:0000:0001"
    
    # Not an IPv6, should return as-is
    addr = "192.168.1.1"
    assert normalize_ipv6(addr) == "192.168.1.1"
    
    # Invalid IP, should return as-is
    addr = "not_an_ip"
    assert normalize_ipv6(addr) == "not_an_ip"

class MockRData:
    def __init__(self, strings):
        self.strings = strings

def test_concat_txt_bytes():
    rdata = MockRData([b"v=spf1 ", b"include:_spf.google.com ", b"~all"])
    assert concat_txt(rdata) == "v=spf1 include:_spf.google.com ~all"

def test_concat_txt_strings():
    rdata = MockRData(["hello ", "world"])
    assert concat_txt(rdata) == "hello world"
