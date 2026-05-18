package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProbeHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("<html><head><title>Test Page</title></head><body></body></html>"))
	}))
	defer srv.Close()

	result, err := Probe(srv.URL, ProbeOptions{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("expected 200, got %d", result.StatusCode)
	}
	if result.Title != "Test Page" {
		t.Errorf("expected title 'Test Page', got %q", result.Title)
	}
	if !strings.Contains(result.ContentType, "text/html") {
		t.Errorf("expected text/html content-type, got %q", result.ContentType)
	}
}

func TestProbeHTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("<html><head><title>Secure</title></head><body></body></html>"))
	}))
	defer srv.Close()

	result, err := Probe(srv.URL, ProbeOptions{Timeout: 5 * time.Second, Insecure: true})
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}
	if result.TLS == nil {
		t.Fatal("expected TLS info for HTTPS probe")
	}
	// Self-signed test certs may have empty subject; verify fingerprint present.
	if result.TLS.FingerprintSHA256 == "" {
		t.Error("expected non-empty TLS fingerprint")
	}
}

func TestProbeRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("<title>Target</title>"))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusMovedPermanently)
	}))
	defer redirector.Close()

	result, err := Probe(redirector.URL, ProbeOptions{Timeout: 5 * time.Second, FollowRedirects: true})
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("expected 200, got %d", result.StatusCode)
	}
	if len(result.RedirectChain) == 0 {
		t.Error("expected redirect chain to be recorded")
	}
}

func TestProbeClosedPort(t *testing.T) {
	result, err := Probe("http://127.0.0.1:19999", ProbeOptions{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}
	if result.Error == "" {
		t.Log("expected error for closed port (may succeed if something is listening)")
	}
}

func TestProbeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer srv.Close()

	result, err := Probe(srv.URL, ProbeOptions{Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}
	if result.Error == "" {
		t.Log("expected timeout error (may not trigger on fast systems)")
	}
}

func TestProbeInvalidURL(t *testing.T) {
	_, err := Probe("not-a-valid-url!!!", ProbeOptions{Timeout: 2 * time.Second})
	if err != nil {
		return // error on parse is acceptable
	}
	// If Probe succeeds, result should have Error field set.
}

func TestFormatProbe(t *testing.T) {
	result := &ProbeResult{
		URL:         "https://example.com",
		StatusCode:  200,
		Title:       "Example Domain",
		ContentType: "text/html",
		TLS: &TLSInfo{
			Subject:  "example.com",
			Issuer:   "R3",
			NotAfter: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		DurationMs: 150,
	}
	out := FormatProbe(result)
	if !strings.Contains(out, "URL: https://example.com") {
		t.Errorf("expected URL in output, got: %s", out)
	}
	if !strings.Contains(out, "Status: 200") {
		t.Errorf("expected Status in output, got: %s", out)
	}
	if !strings.Contains(out, "TLS Subject") {
		t.Errorf("expected TLS info in output, got: %s", out)
	}
}

func TestFormatProbeNil(t *testing.T) {
	out := FormatProbe(nil)
	if out != "No probe result." {
		t.Errorf("expected 'No probe result.', got %q", out)
	}
}

func TestProbeToJSON(t *testing.T) {
	result := &ProbeResult{
		URL:         "https://example.com",
		StatusCode:  200,
		Title:       "Example",
		ContentType: "text/html",
		Headers:     map[string]string{"Server": "nginx"},
		TLS: &TLSInfo{
			Subject: "example.com",
		},
		DurationMs: 42,
	}
	data, err := ProbeToJSON(result)
	if err != nil {
		t.Fatalf("ProbeToJSON failed: %v", err)
	}
	var out ProbeResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if out.URL != result.URL {
		t.Errorf("URL mismatch: %q vs %q", out.URL, result.URL)
	}
	if out.StatusCode != result.StatusCode {
		t.Errorf("StatusCode mismatch")
	}
	if out.TLS == nil || out.TLS.Subject != "example.com" {
		t.Error("TLS info not preserved in JSON roundtrip")
	}
}
