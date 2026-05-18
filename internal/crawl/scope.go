// Package crawl provides authorized LAN discovery, service inspection, and bounded crawling.
package crawl

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
)

// ParseCIDR parses a CIDR string like "192.168.1.0/24". Returns the parsed network or an error.
func ParseCIDR(s string) (*net.IPNet, error) {
	_, cidr, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	return cidr, nil
}

// ValidateScope checks a CIDR for safety. Rejects /0. Allows everything else.
func ValidateScope(cidr *net.IPNet) error {
	if cidr == nil {
		return fmt.Errorf("cidr is nil")
	}
	ones, _ := cidr.Mask.Size()
	if ones == 0 {
		return fmt.Errorf("CIDR too broad: /0 is not allowed")
	}
	if ones < 8 {
		fmt.Fprintf(os.Stderr, "warning: very broad CIDR /%d requested\n", ones)
	}
	return nil
}

// IsPrivateRange returns true if the CIDR falls within RFC 1918, loopback, or link-local ranges.
func IsPrivateRange(cidr *net.IPNet) bool {
	if cidr == nil {
		return false
	}
	// Check the network address against known private ranges.
	privateCIDRs := []string{
		"127.0.0.0/8",     // loopback + reserved
		"10.0.0.0/8",     // RFC 1918 class A
		"172.16.0.0/12",   // RFC 1918 class B
		"192.168.0.0/16",    // RFC 1918 class C
		"169.254.0.0/16",  // link-local
	}
	for _, private := range privateCIDRs {
		_, pnet, err := net.ParseCIDR(private)
		if err != nil {
			continue
		}
		if pnet.Contains(cidr.IP) || cidr.Contains(pnet.IP) {
			return true
		}
	}
	return false
}

// CIDRContainsIP checks whether an IP string is within a CIDR range.
func CIDRContainsIP(cidr *net.IPNet, ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return cidr.Contains(parsed)
}

// ParseExcludedCIDRs parses a comma-separated list of CIDR strings.
// Returns an error on the first invalid entry. Empty input returns nil.
func ParseExcludedCIDRs(s string) ([]*net.IPNet, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	excluded := make([]*net.IPNet, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cidr, err := ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("invalid excluded CIDR %q: %w", part, err)
		}
		excluded = append(excluded, cidr)
	}
	return excluded, nil
}

// DiscoverLocalCIDRs returns all private (RFC 1918) CIDRs from local network interfaces.
// Filters out loopback and zero-value addresses. Deduplicates results.
func DiscoverLocalCIDRs() ([]*net.IPNet, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("listing interface addresses: %w", err)
	}
	seen := make(map[string]bool)
	var cidrs []*net.IPNet
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		if !IsPrivateRange(ipNet) {
			continue
		}
		key := ipNet.String()
		if seen[key] {
			continue
		}
		seen[key] = true

		// Normalize to network address.
		mask := ipNet.Mask
		network := ip.Mask(mask)
		normalized := &net.IPNet{IP: network, Mask: mask}
		cidrs = append(cidrs, normalized)
	}
	return cidrs, nil
}

// EnumerateHosts generates all usable host IP addresses within a CIDR, capped at maxHosts.
// For IPv4, excludes network and broadcast addresses when prefix is /30 or wider.
// Returns an error if the range exceeds maxHosts (when maxHosts > 0).
func EnumerateHosts(cidr *net.IPNet, maxHosts int) ([]string, error) {
	if cidr == nil {
		return nil, fmt.Errorf("cidr is nil")
	}

	prefix, err := netip.ParsePrefix(cidr.String())
	if err != nil {
		return nil, fmt.Errorf("internal: failed to convert CIDR to prefix: %w", err)
	}

	addr := prefix.Addr()
	if !addr.IsValid() {
		return nil, fmt.Errorf("unsupported address: %s", cidr.String())
	}

	// Count estimated hosts.
	estimated := prefixHostEstimate(prefix)
	if maxHosts > 0 && estimated > maxHosts {
		return nil, fmt.Errorf("CIDR has ~%d hosts, exceeds max of %d", estimated, maxHosts)
	}

	var hosts []string
	// For IPv4 /31 and /32, start at network address (no broadcast subtraction needed).
	// For wider IPv4 prefixes, skip the network address (.0).
	skipFirst := addr.Is4() && prefix.Bits() <= 30
	ip := addr
	if skipFirst {
		ip = ip.Next()
		if !ip.IsValid() {
			return nil, fmt.Errorf("cannot advance past network address")
		}
	}

	for ip.IsValid() && prefix.Contains(ip) {
		hosts = append(hosts, ip.String())
		if maxHosts > 0 && len(hosts) >= maxHosts {
			break
		}

		next := ip.Next()
		if !next.IsValid() {
			break
		}
		// For IPv4 wider than /31: stop before broadcast.
		if addr.Is4() && prefix.Bits() <= 30 && !prefix.Contains(next) {
			break
		}
		// Also stop if next would be the broadcast address (all-ones in host portion).
		if addr.Is4() && prefix.Bits() <= 30 && isIPv4Broadcast(next, prefix) {
			break
		}
		ip = next
	}

	return hosts, nil
}

// prefixHostEstimate returns a conservative estimate of usable hosts in a prefix.
func prefixHostEstimate(p netip.Prefix) int {
	if p.Addr().Is4() {
		bits := p.Bits()
		if bits > 32 {
			return 0
		}
		total := 1 << (32 - bits)
		if bits <= 30 {
			return total - 2
		}
		return total
	}
	// IPv6: cap estimate for safety.
	bits := p.Bits()
	if bits < 120 {
		// Too many hosts; maxHosts should cap.
		return 1 << 40 // large sentinel
	}
	return 1 << (128 - bits)
}

// isIPv4Broadcast checks if the given address is the broadcast address for the prefix.
func isIPv4Broadcast(addr netip.Addr, p netip.Prefix) bool {
	if !addr.Is4() {
		return false
	}
	a := addr.As4()
	bits := p.Bits()
	// Check if all host bits are 1s.
	for i := bits; i < 32; i++ {
		byteIdx := i / 8
		bitIdx := 7 - (i % 8)
		if a[byteIdx]&(1<<bitIdx) == 0 {
			return false
		}
	}
	return true
}
