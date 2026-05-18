package crawl

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ronikoz/atlas-recon/internal/scanner"
)

func TestRunDiscoveryLoopback(t *testing.T) {
	cidr, _ := ParseCIDR("127.0.0.1/32")
	result, err := RunDiscovery(cidr, []int{0}, 1, 1, 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(result.Hosts))
	}
	host := result.Hosts[0]
	if host.IP == "" {
		t.Error("host IP should not be empty")
	}
	if len(host.OpenPorts) != 0 {
		t.Errorf("expected 0 open ports for port 0, got %d", len(host.OpenPorts))
	}
	if result.CIDR != "127.0.0.1/32" {
		t.Errorf("expected 127.0.0.1/32, got %s", result.CIDR)
	}
	if result.EndTime.Before(result.StartTime) {
		t.Error("end time before start time")
	}
}

func TestRunDiscoveryWithOpenPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	cidr, _ := ParseCIDR("127.0.0.1/32")
	result, err := RunDiscovery(cidr, []int{port}, 1, 1, 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(result.Hosts))
	}
	host := result.Hosts[0]
	if len(host.OpenPorts) != 1 {
		t.Fatalf("expected 1 open port, got %d", len(host.OpenPorts))
	}
	openPort := host.OpenPorts[0]
	if openPort.Port != port {
		t.Errorf("expected port %d, got %d", port, openPort.Port)
	}
	if openPort.State != "open" {
		t.Errorf("expected state open, got %s", openPort.State)
	}
}

func TestDiscoveryResultJSON(t *testing.T) {
	dr := DiscoveryResult{
		CIDR:      "192.168.1.0/24",
		Ports:     []int{80, 443},
		StartTime: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 5, 19, 12, 0, 5, 0, time.UTC),
		Hosts: []HostResult{
			{
				IP: "10.0.0.1",
				OpenPorts: []scanner.PortResult{
					{Port: 80, State: "open", Service: "HTTP"},
				},
				ClosedCount:  1,
				FilteredCount: 0,
			},
		},
	}

	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded DiscoveryResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.CIDR != dr.CIDR {
		t.Errorf("CIDR mismatch: %s != %s", decoded.CIDR, dr.CIDR)
	}
	if len(decoded.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(decoded.Hosts))
	}
	if decoded.Hosts[0].IP != "10.0.0.1" {
		t.Errorf("IP mismatch: %s", decoded.Hosts[0].IP)
	}
	if decoded.Ports[0] != 80 || decoded.Ports[1] != 443 {
		t.Errorf("ports mismatch: %v", decoded.Ports)
	}
}

func TestRunDiscoveryMaxHosts(t *testing.T) {
	cidr, _ := ParseCIDR("192.168.1.0/24")
	_, err := RunDiscovery(cidr, []int{80}, 5, 1, 2*time.Second)
	if err == nil {
		t.Error("expected error when /24 exceeds maxHosts=5")
	}
}

func TestRunDiscoveryNilCIDR(t *testing.T) {
	_, err := RunDiscovery(nil, []int{80}, 10, 1, 2*time.Second)
	if err == nil {
		t.Error("expected error for nil CIDR")
	}
}

func TestRunDiscoveryNoPorts(t *testing.T) {
	cidr, _ := ParseCIDR("127.0.0.1/32")
	_, err := RunDiscovery(cidr, nil, 1, 1, 2*time.Second)
	if err == nil {
		t.Error("expected error for no ports")
	}
	_, err = RunDiscovery(cidr, []int{}, 1, 1, 2*time.Second)
	if err == nil {
		t.Error("expected error for empty ports")
	}
}

func TestDiscoveryResultJSONOmitEmptyError(t *testing.T) {
	dr := DiscoveryResult{
		CIDR:  "127.0.0.1/32",
		Ports: []int{80},
	}
	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if contains(string(data), `"error":""`) {
		t.Error("empty error should be omitted from JSON")
	}
}

func TestDiscoveryResultJSONWithError(t *testing.T) {
	dr := DiscoveryResult{
		CIDR:  "127.0.0.1/32",
		Ports: []int{80},
		Error: "something went wrong",
	}
	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !contains(string(data), `"error":"something went wrong"`) {
		t.Error("error field should be present when non-empty")
	}
}

func TestRunDiscoveryScannedAllHosts(t *testing.T) {
	cidr, _ := ParseCIDR("10.0.0.0/30")
	result, err := RunDiscovery(cidr, []int{1}, 10, 1, 3*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hosts) != 2 {
		t.Errorf("expected 2 hosts for /30, got %d", len(result.Hosts))
	}
	seen := make(map[string]bool)
	for _, h := range result.Hosts {
		if h.IP == "" {
			t.Error("host IP should not be empty")
		}
		seen[h.IP] = true
	}
	if len(seen) != 2 {
		t.Errorf("expected 2 distinct host IPs, got %d: %v", len(seen), seen)
	}
}

func TestRunDiscoveryPortCounts(t *testing.T) {
	cidr, _ := ParseCIDR("127.0.0.1/32")
	result, err := RunDiscovery(cidr, []int{1, 2}, 10, 1, 3*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	host := result.Hosts[0]
	if host.ClosedCount+host.FilteredCount != 2 {
		t.Errorf("expected 2 non-open ports, got closed=%d filtered=%d", host.ClosedCount, host.FilteredCount)
	}
	if len(host.OpenPorts) != 0 {
		t.Errorf("expected 0 open ports, got %d", len(host.OpenPorts))
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
