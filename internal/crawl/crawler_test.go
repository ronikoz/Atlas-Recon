package crawl

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractLinks(t *testing.T) {
	links := ExtractLinks("http://example.com/a/", `<a href="../b#x">B</a><link href="/style.css"><a href="mailto:a@example.com">Mail</a>`)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d: %+v", len(links), links)
	}
	if links[0] != "http://example.com/b" || links[1] != "http://example.com/style.css" {
		t.Fatalf("unexpected links: %+v", links)
	}
}

func TestCrawlSinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<title>Home</title>"))
	}))
	defer srv.Close()

	result := RunCrawl([]string{srv.URL}, CrawlOptions{MaxDepth: 1, MaxPages: 5, Timeout: time.Second})
	if len(result.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(result.Pages))
	}
	if result.Pages[0].Title != "Home" {
		t.Fatalf("expected Home title, got %q", result.Pages[0].Title)
	}
}

func TestCrawlDepth1(t *testing.T) {
	srv := httptest.NewServer(nil)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<a href="/one">one</a>`))
		case "/one":
			_, _ = w.Write([]byte(`<a href="/two">two</a>`))
		default:
			http.NotFound(w, r)
		}
	})
	defer srv.Close()

	result := RunCrawl([]string{srv.URL}, CrawlOptions{MaxDepth: 1, MaxPages: 10, Timeout: time.Second})
	if len(result.Pages) != 2 {
		t.Fatalf("expected root and depth-1 page, got %d", len(result.Pages))
	}
}

func TestCrawlMaxPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="/a">a</a><a href="/b">b</a>`))
	}))
	defer srv.Close()

	result := RunCrawl([]string{srv.URL}, CrawlOptions{MaxDepth: 2, MaxPages: 1, Timeout: time.Second})
	if len(result.Pages) != 1 {
		t.Fatalf("expected max-pages to limit crawl to 1 page, got %d", len(result.Pages))
	}
}

func TestCrawlExternalLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="http://external.example/">external</a>`))
	}))
	defer srv.Close()

	result := RunCrawl([]string{srv.URL}, CrawlOptions{MaxDepth: 1, MaxPages: 10, Timeout: time.Second})
	if len(result.Pages) != 1 {
		t.Fatalf("expected external link to be skipped, got pages=%d", len(result.Pages))
	}
	if result.Pages[0].ExternalLinksSkipped != 1 {
		t.Fatalf("expected 1 skipped external link, got %d", result.Pages[0].ExternalLinksSkipped)
	}
}

func TestCrawlTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	result := RunCrawl([]string{srv.URL}, CrawlOptions{MaxDepth: 0, MaxPages: 1, Timeout: time.Millisecond})
	if len(result.Pages) != 1 {
		t.Fatalf("expected 1 timeout page, got %d", len(result.Pages))
	}
	if result.Pages[0].Error == "" {
		t.Fatal("expected timeout error")
	}
}
