// Package graph stores and exports LAN discovery/crawl graph data.
package graph

import "time"

type ScanNode struct {
	ID        string         `json:"id"`
	CIDRs     []string       `json:"cidrs"`
	StartedAt time.Time      `json:"started_at"`
	EndedAt   time.Time      `json:"ended_at"`
	Hosts     []HostNode     `json:"hosts"`
	Pages     []PageNode     `json:"pages,omitempty"`
	Links     []LinkEdge     `json:"links,omitempty"`
	Redirects []RedirectEdge `json:"redirects,omitempty"`
}

type HostNode struct {
	IP        string        `json:"ip"`
	CIDR      string        `json:"cidr"`
	OpenPorts []PortNode    `json:"open_ports"`
	Services  []ServiceNode `json:"services,omitempty"`
}

type PortNode struct {
	Port    int    `json:"port"`
	State   string `json:"state"`
	Service string `json:"service"`
}

type ServiceNode struct {
	HostIP         string    `json:"host_ip"`
	Port           int       `json:"port"`
	Scheme         string    `json:"scheme"`
	Protocol       string    `json:"protocol"`
	StatusCode     int       `json:"status_code"`
	Title          string    `json:"title"`
	TLSSubject     string    `json:"tls_subject,omitempty"`
	TLSIssuer      string    `json:"tls_issuer,omitempty"`
	TLSNotBefore   time.Time `json:"tls_not_before,omitempty"`
	TLSNotAfter    time.Time `json:"tls_not_after,omitempty"`
	TLSFingerprint string    `json:"tls_fingerprint,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type PageNode struct {
	URL         string `json:"url"`
	ServiceKey  string `json:"service_key,omitempty"`
	StatusCode  int    `json:"status_code"`
	Title       string `json:"title"`
	ContentType string `json:"content_type"`
	Depth       int    `json:"depth"`
}

type CertNode struct {
	Fingerprint string    `json:"fingerprint"`
	Subject     string    `json:"subject"`
	Issuer      string    `json:"issuer"`
	DNSNames    []string  `json:"dns_names"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
}

type LinkEdge struct {
	FromURL string `json:"from_url"`
	ToURL   string `json:"to_url"`
}

type RedirectEdge struct {
	FromURL string `json:"from_url"`
	ToURL   string `json:"to_url"`
}
