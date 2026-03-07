package scanner

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

// PortResult represents the result of scanning a single port
type PortResult struct {
	Port    int    `json:"port"`
	State   string `json:"state"` // "open", "closed", "filtered"
	Service string `json:"service"`
	Error   string `json:"error,omitempty"`
}

// ScanResult represents the result of a network scan
type ScanResult struct {
	Host      string       `json:"host"`
	Ports     []PortResult `json:"ports"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Error     string       `json:"error,omitempty"`
}

// Scanner performs network port scans
type Scanner struct {
	timeout     time.Duration
	scanTimeout time.Duration // per-port TCP dial timeout
	workers     int
	verbosity   int
}

// NewScanner creates a new scanner with default settings
func NewScanner(timeout time.Duration, workers int) *Scanner {
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	if workers == 0 {
		workers = 100
	}
	if workers > 1000 {
		workers = 1000
	}
	return &Scanner{
		timeout:     timeout,
		scanTimeout: 2 * time.Second,
		workers:     workers,
		verbosity:   0,
	}
}

// ScanHost scans a single host for open ports
func (s *Scanner) ScanHost(host string, ports []int) *ScanResult {
	result := &ScanResult{
		Host:      host,
		Ports:     make([]PortResult, 0),
		StartTime: time.Now(),
	}

	if len(ports) == 0 {
		result.Error = "no ports specified"
		result.EndTime = time.Now()
		return result
	}

	// Resolve host if it's a domain
	lookupHost := host
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		lookupHost = splitHost
	}
	resolvedHost, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(lookupHost, "80"))
	if err != nil {
		// Try just the host
		ips, err := net.LookupIP(lookupHost)
		if err != nil {
			result.Error = fmt.Sprintf("failed to resolve host: %v", err)
			result.EndTime = time.Now()
			return result
		}
		if len(ips) == 0 {
			result.Error = "host not resolved"
			result.EndTime = time.Now()
			return result
		}
		host = ips[0].String()
	} else {
		host = resolvedHost.IP.String()
	}

	// Use worker pool to scan ports concurrently
	portsChan := make(chan int, len(ports))
	resultsChan := make(chan PortResult, len(ports))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < s.workers && i < len(ports); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for port := range portsChan {
				result := s.scanPort(host, port)
				resultsChan <- result
			}
		}()
	}

	// Send ports to workers
	for _, port := range ports {
		portsChan <- port
	}
	close(portsChan)

	// Wait for all workers to finish
	wg.Wait()
	close(resultsChan)

	// Collect results
	for portResult := range resultsChan {
		result.Ports = append(result.Ports, portResult)
	}

	// Sort results by port number
	sort.Slice(result.Ports, func(i, j int) bool {
		return result.Ports[i].Port < result.Ports[j].Port
	})

	result.EndTime = time.Now()
	return result
}

// scanPort scans a single port
func (s *Scanner) scanPort(host string, port int) PortResult {
	result := PortResult{
		Port:    port,
		State:   "closed",
		Service: getServiceName(port),
	}

	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, s.scanTimeout)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			result.State = "filtered"
		} else {
			result.State = "closed"
		}
		return result
	}
	defer conn.Close()

	result.State = "open"
	return result
}

// ParsePorts parses port specification into a list of ports
// Supports: "80", "80,443", "80-100", "80,443,8000-9000"
func ParsePorts(portSpec string) ([]int, error) {
	if portSpec == "" {
		// Default common ports
		return []int{21, 22, 23, 25, 53, 80, 110, 143, 443, 465, 587, 993, 995, 3306, 5432, 8080, 8443}, nil
	}

	portsMap := make(map[int]bool)
	parts := split(portSpec, ",")

	for _, part := range parts {
		part = trim(part)
		if len(part) == 0 {
			continue
		}

		// Check if it's a range
		if contains(part, "-") {
			rangeParts := split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range: %s", part)
			}

			start, err := parsePort(trim(rangeParts[0]))
			if err != nil {
				return nil, err
			}

			end, err := parsePort(trim(rangeParts[1]))
			if err != nil {
				return nil, err
			}

			if start > end {
				start, end = end, start
			}

			// Limit range to prevent DoS
			if end-start > 10000 {
				return nil, fmt.Errorf("port range too large: %d-%d (max 10000)", start, end)
			}

			for p := start; p <= end; p++ {
				portsMap[p] = true
			}
		} else {
			// Single port
			port, err := parsePort(part)
			if err != nil {
				return nil, err
			}
			portsMap[port] = true
		}
	}

	// Convert map to sorted slice
	ports := make([]int, 0, len(portsMap))
	for port := range portsMap {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	return ports, nil
}

// Helper functions
func parsePort(s string) (int, error) {
	var port int
	_, err := fmt.Sscanf(s, "%d", &port)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port: %s", s)
	}
	return port, nil
}

func split(s, sep string) []string {
	var result []string
	var current string
	for _, ch := range s {
		if string(ch) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	result = append(result, current)
	return result
}

func trim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// getServiceName returns common service names for ports
func getServiceName(port int) string {
	services := map[int]string{
		21:    "FTP",
		22:    "SSH",
		23:    "Telnet",
		25:    "SMTP",
		53:    "DNS",
		80:    "HTTP",
		110:   "POP3",
		143:   "IMAP",
		443:   "HTTPS",
		465:   "SMTPS",
		587:   "SMTP",
		993:   "IMAPS",
		995:   "POP3S",
		3306:  "MySQL",
		3389:  "RDP",
		5432:  "PostgreSQL",
		5900:  "VNC",
		8080:  "HTTP-ALT",
		8443:  "HTTPS-ALT",
		27017: "MongoDB",
		6379:  "Redis",
	}
	if service, ok := services[port]; ok {
		return service
	}
	return "Unknown"
}
