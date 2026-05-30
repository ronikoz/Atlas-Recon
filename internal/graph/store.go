package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS lan_scans (
	id TEXT PRIMARY KEY,
	cidrs TEXT NOT NULL,
	started_at TEXT NOT NULL,
	ended_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS lan_hosts (
	scan_id TEXT NOT NULL,
	ip TEXT NOT NULL,
	cidr TEXT NOT NULL,
	open_ports TEXT NOT NULL,
	PRIMARY KEY (scan_id, ip),
	FOREIGN KEY (scan_id) REFERENCES lan_scans(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS lan_services (
	scan_id TEXT NOT NULL,
	host_ip TEXT NOT NULL,
	port INTEGER NOT NULL,
	scheme TEXT NOT NULL,
	protocol TEXT NOT NULL,
	status_code INTEGER NOT NULL,
	title TEXT NOT NULL,
	tls_subject TEXT NOT NULL,
	tls_issuer TEXT NOT NULL,
	tls_not_before TEXT NOT NULL,
	tls_not_after TEXT NOT NULL,
	tls_fingerprint TEXT NOT NULL,
	error TEXT NOT NULL,
	PRIMARY KEY (scan_id, host_ip, port, scheme),
	FOREIGN KEY (scan_id, host_ip) REFERENCES lan_hosts(scan_id, ip) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS lan_pages (
	scan_id TEXT NOT NULL,
	url TEXT NOT NULL,
	service_key TEXT NOT NULL,
	status_code INTEGER NOT NULL,
	title TEXT NOT NULL,
	content_type TEXT NOT NULL,
	depth INTEGER NOT NULL,
	PRIMARY KEY (scan_id, url),
	FOREIGN KEY (scan_id) REFERENCES lan_scans(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS lan_links (
	scan_id TEXT NOT NULL,
	from_url TEXT NOT NULL,
	to_url TEXT NOT NULL,
	PRIMARY KEY (scan_id, from_url, to_url),
	FOREIGN KEY (scan_id) REFERENCES lan_scans(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS lan_certs (
	scan_id TEXT NOT NULL,
	fingerprint TEXT NOT NULL,
	subject TEXT NOT NULL,
	issuer TEXT NOT NULL,
	dns_names TEXT NOT NULL,
	not_before TEXT NOT NULL,
	not_after TEXT NOT NULL,
	PRIMARY KEY (scan_id, fingerprint),
	FOREIGN KEY (scan_id) REFERENCES lan_scans(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS lan_scans_started_idx ON lan_scans(started_at);
`

type Store struct {
	db *sql.DB
}

func DefaultDBPath() string {
	if env := os.Getenv("CT_GRAPH_DB"); env != "" {
		return env
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return "lan-graph.db"
	}
	return filepath.Join(cacheDir, "atlas-recon", "lan-graph.db")
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("graph db path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) SaveScan(scan *ScanNode) error {
	if scan == nil {
		return errors.New("scan is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	cidrs, err := json.Marshal(scan.CIDRs)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO lan_scans (id, cidrs, started_at, ended_at) VALUES (?, ?, ?, ?)`,
		scan.ID, string(cidrs), formatTime(scan.StartedAt), formatTime(scan.EndedAt),
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM lan_services WHERE scan_id = ?`, scan.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM lan_hosts WHERE scan_id = ?`, scan.ID); err != nil {
		return err
	}
	for _, host := range scan.Hosts {
		ports, err := json.Marshal(host.OpenPorts)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO lan_hosts (scan_id, ip, cidr, open_ports) VALUES (?, ?, ?, ?)`,
			scan.ID, host.IP, host.CIDR, string(ports),
		); err != nil {
			return err
		}
		for _, svc := range host.Services {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO lan_services
				(scan_id, host_ip, port, scheme, protocol, status_code, title, tls_subject, tls_issuer, tls_not_before, tls_not_after, tls_fingerprint, error)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				scan.ID, host.IP, svc.Port, svc.Scheme, svc.Protocol, svc.StatusCode, svc.Title,
				svc.TLSSubject, svc.TLSIssuer, formatTime(svc.TLSNotBefore), formatTime(svc.TLSNotAfter), svc.TLSFingerprint, svc.Error,
			); err != nil {
				return err
			}
		}
	}
	for _, page := range scan.Pages {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO lan_pages (scan_id, url, service_key, status_code, title, content_type, depth)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			scan.ID, page.URL, page.ServiceKey, page.StatusCode, page.Title, page.ContentType, page.Depth,
		); err != nil {
			return err
		}
	}
	for _, link := range scan.Links {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO lan_links (scan_id, from_url, to_url) VALUES (?, ?, ?)`,
			scan.ID, link.FromURL, link.ToURL,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LoadScan(scanID string) (*ScanNode, error) {
	if strings.TrimSpace(scanID) == "" {
		return s.LoadLatestScan()
	}
	return s.loadScan(`WHERE id = ?`, scanID)
}

func (s *Store) LoadLatestScan() (*ScanNode, error) {
	return s.loadScan(`ORDER BY started_at DESC LIMIT 1`)
}

func (s *Store) loadScan(suffix string, args ...any) (*ScanNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `SELECT id, cidrs, started_at, ended_at FROM lan_scans ` + suffix
	var scan ScanNode
	var cidrsJSON string
	var startedAt string
	var endedAt string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&scan.ID, &cidrsJSON, &startedAt, &endedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(cidrsJSON), &scan.CIDRs)
	scan.StartedAt = parseTime(startedAt)
	scan.EndedAt = parseTime(endedAt)
	hosts, err := s.loadHosts(ctx, scan.ID)
	if err != nil {
		return nil, err
	}
	scan.Hosts = hosts
	pages, err := s.loadPages(ctx, scan.ID)
	if err != nil {
		return nil, err
	}
	scan.Pages = pages
	links, err := s.loadLinks(ctx, scan.ID)
	if err != nil {
		return nil, err
	}
	scan.Links = links
	return &scan, nil
}

func (s *Store) loadHosts(ctx context.Context, scanID string) ([]HostNode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ip, cidr, open_ports FROM lan_hosts WHERE scan_id = ? ORDER BY ip`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []HostNode
	for rows.Next() {
		var host HostNode
		var portsJSON string
		if err := rows.Scan(&host.IP, &host.CIDR, &portsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(portsJSON), &host.OpenPorts)
		services, err := s.loadServices(ctx, scanID, host.IP)
		if err != nil {
			return nil, err
		}
		host.Services = services
		hosts = append(hosts, host)
	}
	return hosts, rows.Err()
}

func (s *Store) loadServices(ctx context.Context, scanID string, hostIP string) ([]ServiceNode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT host_ip, port, scheme, protocol, status_code, title, tls_subject, tls_issuer, tls_not_before, tls_not_after, tls_fingerprint, error
		FROM lan_services WHERE scan_id = ? AND host_ip = ? ORDER BY port, scheme`, scanID, hostIP)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var services []ServiceNode
	for rows.Next() {
		var svc ServiceNode
		var notBefore string
		var notAfter string
		if err := rows.Scan(&svc.HostIP, &svc.Port, &svc.Scheme, &svc.Protocol, &svc.StatusCode, &svc.Title, &svc.TLSSubject, &svc.TLSIssuer, &notBefore, &notAfter, &svc.TLSFingerprint, &svc.Error); err != nil {
			return nil, err
		}
		svc.TLSNotBefore = parseTime(notBefore)
		svc.TLSNotAfter = parseTime(notAfter)
		services = append(services, svc)
	}
	return services, rows.Err()
}

func (s *Store) loadPages(ctx context.Context, scanID string) ([]PageNode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT url, service_key, status_code, title, content_type, depth FROM lan_pages WHERE scan_id = ? ORDER BY url`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pages []PageNode
	for rows.Next() {
		var page PageNode
		if err := rows.Scan(&page.URL, &page.ServiceKey, &page.StatusCode, &page.Title, &page.ContentType, &page.Depth); err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, rows.Err()
}

func (s *Store) loadLinks(ctx context.Context, scanID string) ([]LinkEdge, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT from_url, to_url FROM lan_links WHERE scan_id = ? ORDER BY from_url, to_url`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []LinkEdge
	for rows.Next() {
		var link LinkEdge
		if err := rows.Scan(&link.FromURL, &link.ToURL); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
