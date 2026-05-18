package crawl

import (
	"fmt"
	"net"
	"time"

	"github.com/ronikoz/atlas-recon/internal/scanner"
)

// DiscoveryResult is the output of a LAN discovery run.
type DiscoveryResult struct {
	CIDR      string       `json:"cidr"`
	Ports     []int        `json:"ports"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Hosts     []HostResult `json:"hosts"`
	Error     string       `json:"error,omitempty"`
}

// HostResult holds discovery data for a single host.
type HostResult struct {
	IP           string              `json:"ip"`
	OpenPorts    []scanner.PortResult `json:"open_ports"`
	ClosedCount  int                 `json:"closed_count"`
	FilteredCount int                `json:"filtered_count"`
}

// RunDiscovery scans all hosts in a CIDR range on the specified ports.
// Uses the project's scanner.Scanner for port probing.
// maxHosts caps enumeration (0 = let EnumerateHosts use its internal estimate).
func RunDiscovery(cidr *net.IPNet, ports []int, maxHosts int, concurrency int, timeout time.Duration) (*DiscoveryResult, error) {
	if cidr == nil {
		return nil, fmt.Errorf("cidr is nil")
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("no ports specified")
	}

	result := &DiscoveryResult{
		CIDR:      cidr.String(),
		Ports:     ports,
		StartTime: time.Now(),
	}

	hostIPs, err := EnumerateHosts(cidr, maxHosts)
	if err != nil {
		result.Error = err.Error()
		result.EndTime = time.Now()
		return result, err
	}

	if concurrency <= 0 {
		concurrency = 50 // conservative for host-level parallelism.
	}

	s := scanner.NewScanner(timeout, concurrency)
	for _, ip := range hostIPs {
		scanResult := s.ScanHost(ip, ports)
		hr := HostResult{IP: ip}

		if scanResult.Error != "" && scanResult.Error != "no ports specified" {
			// Host unreachable or resolution failed — count all as filtered.
			hr.FilteredCount = len(ports)
			result.Hosts = append(result.Hosts, hr)
			continue
		}

		for _, pr := range scanResult.Ports {
			switch pr.State {
			case "open":
				hr.OpenPorts = append(hr.OpenPorts, pr)
			case "closed":
				hr.ClosedCount++
			case "filtered":
				hr.FilteredCount++
			}
		}
		result.Hosts = append(result.Hosts, hr)
	}

	result.EndTime = time.Now()
	return result, nil
}
