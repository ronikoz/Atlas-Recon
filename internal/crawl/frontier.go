package crawl

import (
	"net"
	"net/url"
	"strings"
)

type FrontierItem struct {
	URL   string
	Depth int
}

type Frontier struct {
	maxDepth int
	maxPages int
	queue    []FrontierItem
	visited  map[string]bool
	enqueued map[string]bool
}

func NewFrontier(maxDepth int, maxPages int) *Frontier {
	if maxDepth < 0 {
		maxDepth = 0
	}
	if maxPages <= 0 {
		maxPages = 500
	}
	return &Frontier{
		maxDepth: maxDepth,
		maxPages: maxPages,
		visited:  make(map[string]bool),
		enqueued: make(map[string]bool),
	}
}

func (f *Frontier) Add(rawURL string, depth int) bool {
	normalized, err := NormalizeURL(rawURL)
	if err != nil || depth > f.maxDepth || len(f.visited)+len(f.queue) >= f.maxPages {
		return false
	}
	if f.visited[normalized] || f.enqueued[normalized] {
		return false
	}
	f.queue = append(f.queue, FrontierItem{URL: normalized, Depth: depth})
	f.enqueued[normalized] = true
	return true
}

func (f *Frontier) Next() (FrontierItem, bool) {
	if len(f.queue) == 0 || len(f.visited) >= f.maxPages {
		return FrontierItem{}, false
	}
	item := f.queue[0]
	f.queue = f.queue[1:]
	delete(f.enqueued, item.URL)
	f.visited[item.URL] = true
	return item, true
}

func (f *Frontier) Visited(rawURL string) bool {
	normalized, err := NormalizeURL(rawURL)
	if err != nil {
		return false
	}
	return f.visited[normalized]
}

func (f *Frontier) Remaining() int {
	return len(f.queue)
}

func NormalizeURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	if (parsed.Scheme == "http" && strings.HasSuffix(parsed.Host, ":80")) ||
		(parsed.Scheme == "https" && strings.HasSuffix(parsed.Host, ":443")) {
		host, _, err := net.SplitHostPort(parsed.Host)
		if err == nil {
			parsed.Host = host
		}
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}

func ResolveURL(baseURL string, href string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return "", err
	}
	return NormalizeURL(base.ResolveReference(ref).String())
}

func IsInScope(candidate string, seedHosts map[string]bool, allowedCIDRs []*net.IPNet, allowExternal bool) bool {
	if allowExternal {
		return true
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if seedHosts[host] {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range allowedCIDRs {
		if cidr != nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}
