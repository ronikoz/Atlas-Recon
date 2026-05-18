package crawl

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ronikoz/atlas-recon/internal/scanner"
)

func serveHTML(title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><head><title>%s</title></head><body>page</body></html>", title)
	}
}

func TestInspectHTTPService(t *testing.T) {
	srv := httptest.NewServer(serveHTML("HelloHTTP"))
	defer srv.Close()

	addr := srv.Listener.Addr().(*net.TCPAddr)
	host := addr.IP.String()
	port := addr.Port

	svc := InspectService(host, port, InspectOptions{Timeout: 5 * time.Second})

	if svc.Scheme != "http" {
		t.Errorf("expected scheme http, got %s (error=%s)", svc.Scheme, svc.Error)
	}
	if svc.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", svc.StatusCode)
	}
	if svc.Title != "HelloHTTP" {
		t.Errorf("expected title 'HelloHTTP', got %q", svc.Title)
	}
	if svc.TLS != nil {
		t.Errorf("expected nil TLS for plain HTTP")
	}
}

func TestInspectHTTPSService(t *testing.T) {
	srv := httptest.NewTLSServer(serveHTML("SecurePage"))
	defer srv.Close()

	addr := srv.Listener.Addr().(*net.TCPAddr)
	host := addr.IP.String()
	port := addr.Port

	svc := InspectService(host, port, InspectOptions{
		Timeout:  5 * time.Second,
		Insecure: true,
	})

	if svc.Scheme != "https" {
		t.Errorf("expected scheme https, got %s (error=%s)", svc.Scheme, svc.Error)
	}
	if svc.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", svc.StatusCode)
	}
	if svc.Title != "SecurePage" {
		t.Errorf("expected title 'SecurePage', got %q", svc.Title)
	}
	if svc.TLS == nil {
		t.Fatal("expected non-nil TLS for HTTPS")
	}
	if svc.TLS.FingerprintSHA256 == "" {
		t.Errorf("expected non-empty TLS fingerprint")
	}
}

func TestInspectClosedPort(t *testing.T) {
	// Port 1 is unlikely to be open; probe will timeout or be refused.
	svc := InspectService("[IP_ADDRESS]", 1, InspectOptions{Timeout: 500 * time.Millisecond})
	if svc.Error == "" {
		t.Errorf("expected non-empty error for closed port")
	}
}

func TestInspectRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/destination", http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><head><title>Dest</title></head></html>")
	}))
	defer srv.Close()

	addr := srv.Listener.Addr().(*net.TCPAddr)
	host := addr.IP.String()
	port := addr.Port
	svc := InspectService(host, port, InspectOptions{Timeout: 5 * time.Second})

	if svc.StatusCode != 301 {
		t.Errorf("expected status 301 for redirect, got %d", svc.StatusCode)
	}
	// Redirect responses typically have no title in body.
}

func TestFormatServiceInfo(t *testing.T) {
	svc := &ServiceInfo{
		Host:       "[IP_ADDRESS]",
		Port:       443,
		Scheme:     "https",
		StatusCode: 200,
		Title:      "Admin Dashboard",
	}
	out := FormatServiceInfo(svc)
	if out == "" {
		t.Fatal("expected non-empty formatted output")
	}
	if !containsStr(out, "Port:") || !containsStr(out, "Status:") {
		t.Errorf("formatted output missing expected fields: %s", out)
	}
}

func TestServiceInfoToJSON(t *testing.T) {
	svc := &ServiceInfo{
		Host:       "[IP_ADDRESS]",
		Port:       443,
		Scheme:     "https",
		StatusCode: 200,
		Title:      "Secure",
		Headers:    map[string]string{"Content-Type": "text/html"},
		TLS: &ServiceTLSInfo{
			Subject:           "example.com",
			Issuer:            "CA",
			DNSNames:          []string{"example.com"},
			NotBefore:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			NotAfter:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			FingerprintSHA256: "abcdef",
			SelfSigned:        false,
		},
	}

	data, err := ServiceInfoToJSON(svc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var roundtrip ServiceInfo
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if roundtrip.Host != "[IP_ADDRESS]" {
		t.Errorf("host mismatch: %s", roundtrip.Host)
	}
	if roundtrip.Port != 443 {
		t.Errorf("port mismatch: %d", roundtrip.Port)
	}
	if roundtrip.Scheme != "https" {
		t.Errorf("scheme mismatch: %s", roundtrip.Scheme)
	}
	if roundtrip.TLS == nil {
		t.Fatal("TLS lost in roundtrip")
	}
	if roundtrip.TLS.Subject != "example.com" {
		t.Errorf("TLS subject mismatch: %s", roundtrip.TLS.Subject)
	}
}

func TestInspectHostServices(t *testing.T) {
	// Start an HTTP server on a known port.
	srv := httptest.NewServer(serveHTML("HostSvc"))
	defer srv.Close()

	addr := srv.Listener.Addr().(*net.TCPAddr)
	host := addr.IP.String()
	port := addr.Port

	hr := HostResult{
		IP: host,
		OpenPorts: []scanner.PortResult{
			{Port: port, State: "open", Service: "HTTP"},
			{Port: 9999, State: "open", Service: "Unknown"}, // nothing listening here
		},
	}

	services := InspectHostServices(hr, InspectOptions{Timeout: 2 * time.Second})
	if len(services) < 1 {
		t.Fatal("expected at least 1 service result")
	}
	// The HTTP server on the first port should succeed.
	found := false
	for _, svc := range services {
		if svc.Port == port && svc.Scheme == "http" && svc.StatusCode == 200 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("did not find successful HTTP probe for port %d among %d results", port, len(services))
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
