package dns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLookupA(t *testing.T) {
	recs, err := Lookup("cloudflare.com", []string{"A"})
	if err != nil {
		t.Fatalf("Lookup A failed: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected at least one A record")
	}
	for _, r := range recs {
		if r.Type != "A" {
			t.Errorf("expected type A, got %s", r.Type)
		}
		if r.Value == "" {
			t.Error("expected non-empty A value")
		}
	}
}

func TestLookupMX(t *testing.T) {
	recs, err := Lookup("gmail.com", []string{"MX"})
	if err != nil {
		t.Fatalf("Lookup MX failed: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected at least one MX record")
	}
	for _, r := range recs {
		if r.Type != "MX" {
			t.Errorf("expected type MX, got %s", r.Type)
		}
		if !strings.Contains(r.Value, " ") {
			t.Errorf("expected 'pref host' format, got %q", r.Value)
		}
	}
}

func TestLookupInvalidDomain(t *testing.T) {
	_, err := Lookup("this-domain-definitely-does-not-exist-12345.com", []string{"A"})
	if err == nil {
		t.Fatal("expected error for nonexistent domain")
	}
}

func TestFormatRecords(t *testing.T) {
	recs := []Record{
		{Name: "example.com.", Type: "A", Value: "93.184.216.34"},
	}
	out := FormatRecords(recs)
	if out == "" {
		t.Error("expected non-empty formatted output")
	}
	if !strings.Contains(out, "A") || !strings.Contains(out, "example.com") {
		t.Errorf("expected output to contain record info, got: %s", out)
	}
}

func TestFormatRecordsEmpty(t *testing.T) {
	out := FormatRecords(nil)
	if out != "No records found." {
		t.Errorf("expected 'No records found.', got %q", out)
	}
}

func TestRecordsToJSON(t *testing.T) {
	recs := []Record{
		{Name: "example.com.", Type: "A", Value: "93.184.216.34"},
	}
	data, err := RecordsToJSON(recs)
	if err != nil {
		t.Fatalf("RecordsToJSON failed: %v", err)
	}
	var out struct {
		Records []Record `json:"records"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}
	if len(out.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(out.Records))
	}
}

func TestRecordsToJSONEmpty(t *testing.T) {
	data, err := RecordsToJSON(nil)
	if err != nil {
		t.Fatalf("RecordsToJSON nil failed: %v", err)
	}
	if !strings.Contains(string(data), `"records"`) {
		t.Errorf("expected records key, got %s", string(data))
	}
}

func TestLookupUnsupportedType(t *testing.T) {
	_, err := Lookup("cloudflare.com", []string{"SOA"})
	if err == nil {
		t.Fatal("expected error for unsupported type SOA")
	}
	if !strings.Contains(err.Error(), "unsupported record type") {
		t.Errorf("expected 'unsupported record type', got: %v", err)
	}
}
