import pytest
from scan_nmap import validate_target, validate_ports

def test_validate_target_valid():
    assert validate_target("example.com") == "example.com"
    assert validate_target("192.168.1.1") == "192.168.1.1"
    assert validate_target("10.0.0.0/24") == "10.0.0.0/24"
    assert validate_target("2001:0db8:85a3:0000:0000:8a2e:0370:7334") == "2001:0db8:85a3:0000:0000:8a2e:0370:7334"
    assert validate_target("my-domain.net") == "my-domain.net"

def test_validate_target_invalid():
    with pytest.raises(ValueError):
        validate_target("example.com; rm -rf /")
    with pytest.raises(ValueError):
        validate_target("$(whoami).example.com")
    with pytest.raises(ValueError):
        validate_target("target|grep")
    with pytest.raises(ValueError):
        validate_target("param&eval()")

def test_validate_ports_valid():
    assert validate_ports("80") == "80"
    assert validate_ports("80,443") == "80,443"
    assert validate_ports("1000-2000") == "1000-2000"
    assert validate_ports("80,443,1000-2000") == "80,443,1000-2000"
    assert validate_ports("   80  , 443 ") == "   80  , 443 "
    assert validate_ports("") == ""

def test_validate_ports_invalid():
    with pytest.raises(ValueError):
        validate_ports("80,http")
    with pytest.raises(ValueError):
        validate_ports("80;443")
    with pytest.raises(ValueError):
        validate_ports("1000-abcd")
    with pytest.raises(ValueError):
        validate_ports("1000:abcd")
    with pytest.raises(ValueError):
        validate_ports("rm -rf /")
