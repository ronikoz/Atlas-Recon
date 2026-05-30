package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExportJSON(t *testing.T) {
	data, err := ExportJSON(sampleScan())
	if err != nil {
		t.Fatal(err)
	}
	var decoded ScanNode
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "scan-1" {
		t.Fatalf("expected scan-1, got %s", decoded.ID)
	}
}

func TestExportMarkdown(t *testing.T) {
	out := ExportMarkdown(sampleScan())
	for _, want := range []string{"# LAN Map scan-1", "## Host 127.0.0.1", "HTTP-ALT", "Local"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", want, out)
		}
	}
}

func TestExportDOT(t *testing.T) {
	out := ExportDOT(sampleScan())
	for _, want := range []string{"digraph atlas_recon", "127.0.0.1/32", "contains", "exposes"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected dot to contain %q, got:\n%s", want, out)
		}
	}
}
