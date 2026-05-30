package graph

import (
	"os"
	"testing"
	"time"
)

func tmpGraphStore(t *testing.T) *Store {
	t.Helper()
	f, err := os.CreateTemp("", "atlas-recon-graph-*.db")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	_ = f.Close()
	t.Cleanup(func() { _ = os.Remove(path) })
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func sampleScan() *ScanNode {
	return &ScanNode{
		ID:        "scan-1",
		CIDRs:     []string{"127.0.0.1/32"},
		StartedAt: time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 5, 30, 10, 0, 1, 0, time.UTC),
		Hosts: []HostNode{
			{
				IP:   "127.0.0.1",
				CIDR: "127.0.0.1/32",
				OpenPorts: []PortNode{
					{Port: 8080, State: "open", Service: "HTTP-ALT"},
				},
				Services: []ServiceNode{
					{HostIP: "127.0.0.1", Port: 8080, Scheme: "http", Protocol: "http", StatusCode: 200, Title: "Local"},
				},
			},
		},
		Pages: []PageNode{
			{URL: "http://127.0.0.1:8080/", ServiceKey: "127.0.0.1:8080/http", StatusCode: 200, Title: "Local", ContentType: "text/html", Depth: 0},
		},
		Links: []LinkEdge{
			{FromURL: "http://127.0.0.1:8080/", ToURL: "http://127.0.0.1:8080/about"},
		},
	}
}

func TestSaveAndRetrieveScan(t *testing.T) {
	store := tmpGraphStore(t)
	if err := store.SaveScan(sampleScan()); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadScan("scan-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "scan-1" {
		t.Fatalf("expected scan-1, got %s", loaded.ID)
	}
	if len(loaded.Hosts) != 1 || loaded.Hosts[0].IP != "127.0.0.1" {
		t.Fatalf("unexpected hosts: %+v", loaded.Hosts)
	}
}

func TestSaveHostDedup(t *testing.T) {
	store := tmpGraphStore(t)
	scan := sampleScan()
	if err := store.SaveScan(scan); err != nil {
		t.Fatal(err)
	}
	scan.Hosts[0].OpenPorts = append(scan.Hosts[0].OpenPorts, PortNode{Port: 9090, State: "open", Service: "Unknown"})
	if err := store.SaveScan(scan); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadScan("scan-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Hosts) != 1 {
		t.Fatalf("expected deduped host, got %d", len(loaded.Hosts))
	}
	if len(loaded.Hosts[0].OpenPorts) != 2 {
		t.Fatalf("expected updated open ports, got %+v", loaded.Hosts[0].OpenPorts)
	}
}

func TestHostServiceRelation(t *testing.T) {
	store := tmpGraphStore(t)
	if err := store.SaveScan(sampleScan()); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadLatestScan()
	if err != nil {
		t.Fatal(err)
	}
	services := loaded.Hosts[0].Services
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].HostIP != loaded.Hosts[0].IP || services[0].Port != 8080 {
		t.Fatalf("service relation mismatch: %+v", services[0])
	}
}

func TestPageLinkGraph(t *testing.T) {
	store := tmpGraphStore(t)
	if err := store.SaveScan(sampleScan()); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadScan("scan-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(loaded.Pages))
	}
	if len(loaded.Links) != 1 || loaded.Links[0].ToURL == "" {
		t.Fatalf("expected link edge, got %+v", loaded.Links)
	}
}
