package crawl

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// InspectOptions configures an HTTP(S) service probe.
type InspectOptions struct {
	Timeout  time.Duration
	Insecure bool
}

// ServiceInfo holds the result of probing a single service endpoint.
type ServiceInfo struct {
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	Scheme     string            `json:"scheme"` // "http", "https", or "" if undetermined
	StatusCode int               `json:"status_code"`
	Title      string            `json:"title"`
	Headers    map[string]string `json:"headers"`
	TLS        *ServiceTLSInfo   `json:"tls,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// ServiceTLSInfo holds extracted TLS certificate metadata.
type ServiceTLSInfo struct {
	Subject           string    `json:"subject"`
	Issuer            string    `json:"issuer"`
	DNSNames          []string  `json:"dns_names"`
	NotBefore         time.Time `json:"not_before"`
	NotAfter          time.Time `json:"not_after"`
	FingerprintSHA256 string    `json:"fingerprint_sha256"`
	SelfSigned        bool      `json:"self_signed"`
}

// InspectService probes a host:port to determine if it serves HTTP or HTTPS,
// and extracts metadata from the response.
func InspectService(host string, port int, opts InspectOptions) *ServiceInfo {
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}

	// Determine probe order based on port heuristics.
	primary := "https"
	fallback := "http"
	if port == 80 || port == 8080 {
		primary, fallback = fallback, primary
	}

	svc := tryScheme(host, port, primary, opts)
	if svc.Error != "" {
		alt := tryScheme(host, port, fallback, opts)
		if alt.Error != "" {
			// Both failed — keep the primary attempt's error.
			return svc
		}
		return alt
	}
	return svc
}

func tryScheme(host string, port int, scheme string, opts InspectOptions) *ServiceInfo {
	svc := &ServiceInfo{
		Host:   host,
		Port:   port,
		Scheme: scheme,
	}

	urlStr := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.Insecure,
		},
	}

	client := &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		svc.Error = err.Error()
		return svc
	}
	req.Header.Set("User-Agent", "Atlas-Recon/dev")

	resp, err := client.Do(req)
	if err != nil {
		svc.Error = err.Error()
		return svc
	}
	defer resp.Body.Close()

	// Success — we got a response.
	svc.StatusCode = resp.StatusCode
	svc.Headers = make(map[string]string)
	for k := range resp.Header {
		svc.Headers[k] = resp.Header.Get(k)
	}

	// Extract title from body.
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16)) // 64KB max
	if err == nil {
		svc.Title = extractHTMLTitle(string(bodyBytes))
	}

	// Extract TLS info.
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		svc.TLS = extractTLSInfo(resp.TLS.PeerCertificates[0])
	}

	return svc
}

func extractTLSInfo(cert *x509.Certificate) *ServiceTLSInfo {
	fp := sha256.Sum256(cert.Raw)
	info := &ServiceTLSInfo{
		Subject:           cert.Subject.CommonName,
		Issuer:            cert.Issuer.CommonName,
		DNSNames:          cert.DNSNames,
		NotBefore:         cert.NotBefore,
		NotAfter:          cert.NotAfter,
		FingerprintSHA256: fmt.Sprintf("%x", fp),
		SelfSigned:        cert.Issuer.CommonName == cert.Subject.CommonName,
	}
	// Ensure DNSNames is never nil in JSON output.
	if info.DNSNames == nil {
		info.DNSNames = []string{}
	}
	return info
}

// extractHTMLTitle finds the content between <title> and </title> tags (case-insensitive).
func extractHTMLTitle(html string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title>")
	if start == -1 {
		return ""
	}
	start += len("<title>")
	end := strings.Index(lower[start:], "</title>")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(html[start : start+end])
}

// InspectHostServices probes all open ports on a HostResult.
// Returns a slice of ServiceInfo, one per port that responded.
func InspectHostServices(host HostResult, opts InspectOptions) []ServiceInfo {
	var services []ServiceInfo
	for _, pr := range host.OpenPorts {
		svc := InspectService(host.IP, pr.Port, opts)
		services = append(services, *svc)
	}
	return services
}

// FormatServiceInfo returns a human-readable string for a single service.
func FormatServiceInfo(svc *ServiceInfo) string {
	if svc == nil {
		return "No service info."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Host: %s\n", svc.Host))
	b.WriteString(fmt.Sprintf("Port: %d/tcp", svc.Port))
	if svc.Scheme != "" {
		b.WriteString(fmt.Sprintf(" (%s)", strings.ToUpper(svc.Scheme)))
	}
	b.WriteString("\n")

	if svc.Error != "" {
		b.WriteString(fmt.Sprintf("Error: %s\n", svc.Error))
		return b.String()
	}

	b.WriteString(fmt.Sprintf("Status: %d", svc.StatusCode))
	if statusText := http.StatusText(svc.StatusCode); statusText != "" {
		b.WriteString(" " + statusText)
	}
	b.WriteString("\n")

	if svc.Title != "" {
		b.WriteString(fmt.Sprintf("Title: %s\n", svc.Title))
	}
	if svc.TLS != nil {
		b.WriteString(fmt.Sprintf("TLS Subject: %s\n", svc.TLS.Subject))
		b.WriteString(fmt.Sprintf("TLS Issuer: %s\n", svc.TLS.Issuer))
		if svc.TLS.SelfSigned {
			b.WriteString(" (self-signed)")
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("TLS Valid: %s to %s\n",
			svc.TLS.NotBefore.Format("2006-01-02"),
			svc.TLS.NotAfter.Format("2006-01-02")))
	}
	return b.String()
}

// ServiceInfoToJSON marshals a ServiceInfo to indented JSON.
func ServiceInfoToJSON(svc *ServiceInfo) ([]byte, error) {
	return json.MarshalIndent(svc, "", "  ")
}
