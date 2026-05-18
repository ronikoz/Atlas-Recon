// Package web provides HTTP/HTTPS probing and metadata extraction.
package web

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProbeOptions controls HTTP/HTTPS probe behavior.
type ProbeOptions struct {
	Timeout         time.Duration
	FollowRedirects bool
	Insecure        bool
}

// TLSInfo holds extracted TLS certificate metadata.
type TLSInfo struct {
	Subject           string    `json:"subject"`
	Issuer            string    `json:"issuer"`
	DNSNames          []string  `json:"dns_names"`
	NotBefore         time.Time `json:"not_before"`
	NotAfter          time.Time `json:"not_after"`
	FingerprintSHA256 string    `json:"fingerprint_sha256"`
	SelfSigned        bool      `json:"self_signed"`
}

// ProbeResult contains the result of an HTTP/HTTPS probe.
type ProbeResult struct {
	URL           string            `json:"url"`
	StatusCode    int               `json:"status_code"`
	FinalURL      string            `json:"final_url"`
	Title         string            `json:"title"`
	ContentType   string            `json:"content_type"`
	Headers       map[string]string `json:"headers"`
	TLS           *TLSInfo          `json:"tls,omitempty"`
	RedirectChain []string          `json:"redirect_chain"`
	DurationMs    int64             `json:"duration_ms"`
	Error         string            `json:"error,omitempty"`
}

// Probe performs an HTTP/HTTPS probe against a target URL or domain.
// If target has no scheme, https:// is tried first, then http:// as fallback.
func Probe(target string, opts ProbeOptions) (*ProbeResult, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	// Ensure target has a scheme.
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme == "" {
		// Try https first, then http.
		httpsResult, httpsErr := probeURL("https://"+target, opts)
		if httpsErr == nil && httpsResult.Error == "" {
			return httpsResult, nil
		}
		return probeURL("http://"+target, opts)
	}

	return probeURL(target, opts)
}

func probeURL(rawURL string, opts ProbeOptions) (*ProbeResult, error) {
	result := &ProbeResult{URL: rawURL}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.Insecure,
		},
	}

	client := &http.Client{
		Timeout:   opts.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			result.RedirectChain = append(result.RedirectChain, req.URL.String())
			if !opts.FollowRedirects && len(via) >= 1 {
				return http.ErrUseLastResponse
			}
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	req.Header.Set("User-Agent", "Atlas-Recon/dev")

	started := time.Now()
	resp, err := client.Do(req)
	result.DurationMs = time.Since(started).Milliseconds()

	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.FinalURL = resp.Request.URL.String()
	result.ContentType = resp.Header.Get("Content-Type")

	result.Headers = make(map[string]string)
	for k := range resp.Header {
		result.Headers[k] = resp.Header.Get(k)
	}

	// Extract title from body.
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<19)) // 512KB max
	if err == nil {
		result.Title = extractTitle(string(bodyBytes))
	}

	// Extract TLS info.
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		fp := sha256.Sum256(cert.Raw)
		result.TLS = &TLSInfo{
			Subject:           cert.Subject.CommonName,
			Issuer:            cert.Issuer.CommonName,
			DNSNames:          cert.DNSNames,
			NotBefore:         cert.NotBefore,
			NotAfter:          cert.NotAfter,
			FingerprintSHA256: fmt.Sprintf("%x", fp),
			SelfSigned:        cert.Issuer.CommonName == cert.Subject.CommonName,
		}
	}

	return result, nil
}

func extractTitle(html string) string {
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

// FormatProbe returns a human-readable summary of the probe result.
func FormatProbe(result *ProbeResult) string {
	if result == nil {
		return "No probe result."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("URL: %s\n", result.URL))
	if result.FinalURL != "" && result.FinalURL != result.URL {
		b.WriteString(fmt.Sprintf("Final URL: %s\n", result.FinalURL))
	}
	b.WriteString(fmt.Sprintf("Status: %d", result.StatusCode))
	if statusText := http.StatusText(result.StatusCode); statusText != "" {
		b.WriteString(" " + statusText)
	}
	b.WriteString("\n")

	if result.Error != "" {
		b.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
	}
	if result.Title != "" {
		b.WriteString(fmt.Sprintf("Title: %s\n", result.Title))
	}
	if result.ContentType != "" {
		b.WriteString(fmt.Sprintf("Content-Type: %s\n", result.ContentType))
	}
	if result.TLS != nil {
		b.WriteString(fmt.Sprintf("TLS Subject: %s\n", result.TLS.Subject))
		b.WriteString(fmt.Sprintf("TLS Issuer: %s\n", result.TLS.Issuer))
		b.WriteString(fmt.Sprintf("TLS Valid: %s to %s\n",
			result.TLS.NotBefore.Format("2006-01-02"), result.TLS.NotAfter.Format("2006-01-02")))
		if result.TLS.SelfSigned {
			b.WriteString("TLS: self-signed\n")
		}
	}
	if len(result.RedirectChain) > 0 {
		b.WriteString(fmt.Sprintf("Redirects: %s\n", strings.Join(result.RedirectChain, " -> ")))
	}
	b.WriteString(fmt.Sprintf("Duration: %dms", result.DurationMs))
	return b.String()
}

// ProbeToJSON marshals a probe result to indented JSON.
func ProbeToJSON(result *ProbeResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}
