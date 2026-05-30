package crawl

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type CrawlOptions struct {
	MaxDepth      int
	MaxPages      int
	Timeout       time.Duration
	AllowExternal bool
	AllowedCIDRs  []*net.IPNet
	Progress      func(ProgressEvent)
}

type ProgressEvent struct {
	URL     string
	Depth   int
	Visited int
	Queued  int
}

type PageResult struct {
	URL                  string   `json:"url"`
	StatusCode           int      `json:"status_code"`
	Title                string   `json:"title"`
	ContentType          string   `json:"content_type"`
	Depth                int      `json:"depth"`
	LinksFound           []string `json:"links_found"`
	ExternalLinksSkipped int      `json:"external_links_skipped"`
	Error                string   `json:"error,omitempty"`
}

type LinkResult struct {
	FromURL string `json:"from_url"`
	ToURL   string `json:"to_url"`
}

type CrawlResult struct {
	Pages []PageResult `json:"pages"`
	Links []LinkResult `json:"links"`
}

var linkPattern = regexp.MustCompile(`(?i)\b(?:href|src)\s*=\s*["']([^"']+)["']`)

func RunCrawl(seedURLs []string, opts CrawlOptions) CrawlResult {
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}
	frontier := NewFrontier(opts.MaxDepth, opts.MaxPages)
	seedHosts := make(map[string]bool)
	for _, seed := range seedURLs {
		normalized, err := NormalizeURL(seed)
		if err != nil {
			continue
		}
		if parsed, err := url.Parse(normalized); err == nil {
			seedHosts[parsed.Hostname()] = true
		}
		frontier.Add(normalized, 0)
	}

	client := &http.Client{
		Timeout: opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	var result CrawlResult
	visited := 0
	for {
		item, ok := frontier.Next()
		if !ok {
			break
		}
		visited++
		page := CrawlPage(context.Background(), client, item.URL, item.Depth)
		for _, link := range page.LinksFound {
			if IsInScope(link, seedHosts, opts.AllowedCIDRs, opts.AllowExternal) {
				result.Links = append(result.Links, LinkResult{FromURL: item.URL, ToURL: link})
				frontier.Add(link, item.Depth+1)
			} else {
				page.ExternalLinksSkipped++
			}
		}
		result.Pages = append(result.Pages, page)
		if opts.Progress != nil {
			opts.Progress(ProgressEvent{URL: item.URL, Depth: item.Depth, Visited: visited, Queued: frontier.Remaining()})
		}
	}
	return result
}

func CrawlPage(ctx context.Context, client *http.Client, pageURL string, depth int) PageResult {
	page := PageResult{URL: pageURL, Depth: depth}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		page.Error = err.Error()
		return page
	}
	req.Header.Set("User-Agent", "Atlas-Recon/dev")
	resp, err := client.Do(req)
	if err != nil {
		page.Error = err.Error()
		return page
	}
	defer resp.Body.Close()
	page.StatusCode = resp.StatusCode
	page.ContentType = resp.Header.Get("Content-Type")
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		page.Error = err.Error()
		return page
	}
	text := string(body)
	page.Title = extractHTMLTitle(text)
	page.LinksFound = ExtractLinks(pageURL, text)
	return page
}

func ExtractLinks(baseURL string, html string) []string {
	matches := linkPattern.FindAllStringSubmatch(html, -1)
	seen := make(map[string]bool)
	var links []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		href := strings.TrimSpace(match[1])
		if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") || strings.HasPrefix(strings.ToLower(href), "mailto:") {
			continue
		}
		resolved, err := ResolveURL(baseURL, href)
		if err != nil || seen[resolved] {
			continue
		}
		seen[resolved] = true
		links = append(links, resolved)
	}
	return links
}
