package crawl

import (
	"net"
	"testing"
)

func TestFrontierDepthLimit(t *testing.T) {
	f := NewFrontier(1, 10)
	if !f.Add("http://example.com", 0) {
		t.Fatal("expected first add")
	}
	if f.Add("http://example.com/deep", 2) {
		t.Fatal("expected depth-limited URL to be rejected")
	}
}

func TestFrontierPageLimit(t *testing.T) {
	f := NewFrontier(3, 1)
	if !f.Add("http://example.com/a", 0) {
		t.Fatal("expected first add")
	}
	if f.Add("http://example.com/b", 0) {
		t.Fatal("expected page-limited URL to be rejected")
	}
}

func TestFrontierVisitedDedup(t *testing.T) {
	f := NewFrontier(3, 10)
	if !f.Add("http://example.com/a#frag", 0) {
		t.Fatal("expected add")
	}
	if f.Add("http://example.com/a", 0) {
		t.Fatal("expected duplicate normalized URL to be rejected")
	}
	item, ok := f.Next()
	if !ok || item.URL != "http://example.com/a" {
		t.Fatalf("unexpected next item: %+v ok=%v", item, ok)
	}
	if !f.Visited("http://example.com/a#other") {
		t.Fatal("expected normalized URL to be marked visited")
	}
}

func TestNormalizeURL(t *testing.T) {
	got, err := NormalizeURL("HTTP://Example.COM:80/a#fragment")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.com/a" {
		t.Fatalf("unexpected normalized URL: %s", got)
	}
	got, err = ResolveURL("http://example.com/base/index.html", "../next?q=1#x")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.com/next?q=1" {
		t.Fatalf("unexpected resolved URL: %s", got)
	}
}

func TestIsInScope(t *testing.T) {
	_, local, _ := net.ParseCIDR("10.0.0.0/24")
	seeds := map[string]bool{"example.com": true}
	if !IsInScope("http://example.com/a", seeds, nil, false) {
		t.Fatal("expected same host in scope")
	}
	if !IsInScope("http://10.0.0.9/a", seeds, []*net.IPNet{local}, false) {
		t.Fatal("expected CIDR host in scope")
	}
	if IsInScope("http://other.example/a", seeds, []*net.IPNet{local}, false) {
		t.Fatal("expected external host out of scope")
	}
	if !IsInScope("http://other.example/a", seeds, nil, true) {
		t.Fatal("expected allowExternal to permit external URL")
	}
}
