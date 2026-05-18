package crawl

import (
	"net"
	"testing"
)

func TestParseCIDR(t *testing.T) {
	cidr, err := ParseCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cidr == nil {
		t.Fatal("expected non-nil CIDR")
	}
	ones, bits := cidr.Mask.Size()
	if ones != 24 || bits != 32 {
		t.Errorf("expected /24 (32 bits), got /%d (%d bits)", ones, bits)
	}

	_, err = ParseCIDR("not-cidr")
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}

	_, err = ParseCIDR("1.2.3.4/33")
	if err == nil {
		t.Error("expected error for /33")
	}
}

func TestValidateScope(t *testing.T) {
	if err := ValidateScope(nil); err == nil {
		t.Error("expected error for nil CIDR")
	}

	cidr0, _ := ParseCIDR("0.0.0.0/0")
	err := ValidateScope(cidr0)
	if err == nil {
		t.Error("expected error for /0")
	}

	cidr24, _ := ParseCIDR("192.168.1.0/24")
	if err := ValidateScope(cidr24); err != nil {
		t.Errorf("expected nil for /24, got %v", err)
	}

	cidr8, _ := ParseCIDR("10.0.0.0/8")
	if err := ValidateScope(cidr8); err != nil {
		t.Errorf("expected nil for /8, got %v", err)
	}
}

func TestIsPrivateRange(t *testing.T) {
	if IsPrivateRange(nil) {
		t.Error("expected false for nil CIDR")
	}

	private := []string{
		"192.168.1.0/24",
		"172.16.0.0/12",
		"169.254.0.0/16",
		"10.0.0.0/8",
		"127.0.0.0/8",
	}
	for _, p := range private {
		cidr, _ := ParseCIDR(p)
		if !IsPrivateRange(cidr) {
			t.Errorf("expected true for private range %s", p)
		}
	}

	public := []string{
		"8.8.8.0/24",
		"1.1.1.0/8",
		"93.184.216.0/24",
	}
	for _, p := range public {
		cidr, _ := ParseCIDR(p)
		if IsPrivateRange(cidr) {
			t.Errorf("expected false for public range %s", p)
		}
	}
}

func TestCIDRContainsIP(t *testing.T) {
	cidr, _ := ParseCIDR("192.168.1.0/24")

	tests := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"192.168.1.254", true},
		{"192.168.1.0", true},
		{"192.168.2.1", false},
		{"10.0.0.1", false},
		{"not-an-ip", false},
	}

	for _, tt := range tests {
		got := CIDRContainsIP(cidr, tt.ip)
		if got != tt.expected {
			t.Errorf("CIDRContainsIP(%q) = %v, want %v", tt.ip, got, tt.expected)
		}
	}
}

func TestParseExcludedCIDRs(t *testing.T) {
	excluded, err := ParseExcludedCIDRs("")
	if err != nil {
		t.Fatalf("unexpected error for empty: %v", err)
	}
	if len(excluded) != 0 {
		t.Errorf("expected 0 excluded, got %d", len(excluded))
	}

	excluded, err = ParseExcludedCIDRs("192.168.1.1/32,10.0.0.0/8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(excluded) != 2 {
		t.Errorf("expected 2 excluded, got %d", len(excluded))
	}

	_, err = ParseExcludedCIDRs("bad-cidr")
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestEnumerateHosts(t *testing.T) {
	// /30 has 4 IPs, 2 usable (excluding network and broadcast).
	cidr30, _ := ParseCIDR("10.0.0.0/30")
	hosts, err := EnumerateHosts(cidr30, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts for /30, got %d: %v", len(hosts), hosts)
	}
	if hosts[0] != "10.0.0.1" {
		t.Errorf("expected first host 10.0.0.1, got %s", hosts[0])
	}
	if hosts[1] != "10.0.0.2" {
		t.Errorf("expected second host 10.0.0.2, got %s", hosts[1])
	}

	// /31 has 2 IPs, both usable (no network/broadcast subtraction for point-to-point).
	cidr31, _ := ParseCIDR("10.0.0.0/31")
	hosts, err = EnumerateHosts(cidr31, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts for /31, got %d", len(hosts))
	}

	// /32 has 1 IP.
	cidr32, _ := ParseCIDR("10.0.0.1/32")
	hosts, err = EnumerateHosts(cidr32, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 1 {
		t.Errorf("expected 1 host for /32, got %d", len(hosts))
	}
	if hosts[0] != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", hosts[0])
	}

	// Max hosts cap.
	_, err = EnumerateHosts(cidr30, 1)
	if err == nil {
		t.Error("expected error when /30 exceeds maxHosts=1")
	}

	// Nil CIDR.
	_, err = EnumerateHosts(nil, 10)
	if err == nil {
		t.Error("expected error for nil CIDR")
	}
}

func TestDiscoverLocalCIDRs(t *testing.T) {
	cidrs, err := DiscoverLocalCIDRs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cidrs) == 0 {
		t.Log("no local private CIDRs found (expected in CI without private interfaces)")
	}
	for _, c := range cidrs {
		if !IsPrivateRange(c) {
			t.Errorf("DiscoverLocalCIDRs returned non-private range: %s", c.String())
		}
		if !c.IP.Equal(c.IP.Mask(c.Mask)) {
			t.Errorf("CIDR IP is not normalized to network address: %s", c.String())
		}
	}
}

func TestParseCIDRIPv6(t *testing.T) {
	_, err := ParseCIDR("::1/128")
	if err != nil {
		t.Fatalf("unexpected error for IPv6 loopback: %v", err)
	}
}

func TestIsPrivateRangeLinkLocal(t *testing.T) {
	cidr, _ := ParseCIDR("169.254.0.0/16")
	if !IsPrivateRange(cidr) {
		t.Error("169.254.0.0/16 should be private (link-local)")
	}
}

func TestIsPrivateRangeLoopback(t *testing.T) {
	cidr, _ := ParseCIDR("127.0.0.0/8")
	if !IsPrivateRange(cidr) {
		t.Error("127.0.0.0/8 should be private (loopback)")
	}
}

func TestValidateScopeIPv6(t *testing.T) {
	cidr, _ := ParseCIDR("::1/128")
	if err := ValidateScope(cidr); err != nil {
		t.Errorf("expected nil for ::1/128, got %v", err)
	}

	cidr0, _ := ParseCIDR("::/0")
	if err := ValidateScope(cidr0); err == nil {
		t.Error("expected error for ::/0")
	}
}

func TestParseExcludedCIDRsWithSpaces(t *testing.T) {
	excluded, err := ParseExcludedCIDRs(" 10.0.0.1/32 , 192.168.0.1/32 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(excluded) != 2 {
		t.Errorf("expected 2 excluded, got %d", len(excluded))
	}
}

func TestParseExcludedCIDRsEmptyParts(t *testing.T) {
	excluded, err := ParseExcludedCIDRs("10.0.0.1/32,,,192.168.0.1/32")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(excluded) != 2 {
		t.Errorf("expected 2 excluded (empty parts skipped), got %d", len(excluded))
	}
}

func TestEnumerateHostsMaxHostsZero(t *testing.T) {
	cidr24, _ := ParseCIDR("10.0.0.0/24")
	hosts, err := EnumerateHosts(cidr24, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 254 {
		t.Errorf("expected 254 hosts for /24, got %d", len(hosts))
	}
}

func TestEnumerateHostsWithCap(t *testing.T) {
	cidr24, _ := ParseCIDR("10.0.0.0/24")
	hosts, err := EnumerateHosts(cidr24, 256)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 254 {
		t.Errorf("expected 254 hosts (cap not hit), got %d", len(hosts))
	}

	_, err = EnumerateHosts(cidr24, 5)
	if err == nil {
		t.Error("expected error when /24 exceeds maxHosts=5")
	}
}

func TestCIDRContainsIPIPv6(t *testing.T) {
	cidr, _ := ParseCIDR("::1/128")
	if !CIDRContainsIP(cidr, "::1") {
		t.Error("::1 should be contained in ::1/128")
	}
	if CIDRContainsIP(cidr, "::2") {
		t.Error("::2 should not be contained in ::1/128")
	}
}

func TestIsPrivateRangeEdgeCases(t *testing.T) {
	cidr, _ := ParseCIDR("10.0.0.1/32")
	if !IsPrivateRange(cidr) {
		t.Error("10.0.0.1/32 should be private")
	}

	cidr2, _ := ParseCIDR("8.8.8.8/32")
	if IsPrivateRange(cidr2) {
		t.Error("8.8.8.8/32 should not be private")
	}
}

func TestEnumerateHostsDeduplication(t *testing.T) {
	cidr, _ := ParseCIDR("10.0.0.1/32")
	hosts, _ := EnumerateHosts(cidr, 10)
	if len(hosts) != 1 {
		t.Errorf("expected 1 host for /32, got %d: %v", len(hosts), hosts)
	}
}

func TestEnumerateHostsSlash24Boundaries(t *testing.T) {
	cidr, _ := ParseCIDR("10.0.0.0/24")
	networkIP := cidr.IP.String()
	hosts, err := EnumerateHosts(cidr, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not contain network address.
	for _, h := range hosts {
		if h == networkIP {
			t.Errorf("should not contain network address: %s", h)
		}
	}
	if hosts[0] != incIP(networkIP) {
		t.Errorf("first host should be %s, got %s", incIP(networkIP), hosts[0])
	}
	if len(hosts) != 254 {
		t.Errorf("expected 254 hosts, got %d", len(hosts))
	}
}

func incIP(s string) string {
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	ip = ip.To4()
	if ip == nil {
		return s
	}
	ip[3]++
	return ip.String()
}
