// Package dns provides native DNS lookups using the Go standard library resolver.
package dns

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

var (
	lookupIP       = net.LookupIP
	lookupMXNet    = net.LookupMX
	lookupNSNet    = net.LookupNS
	lookupTXTNet   = net.LookupTXT
	lookupCNAMENet = net.LookupCNAME
)

// Record represents a single DNS resource record.
type Record struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   uint32 `json:"ttl"`
}

// Lookup resolves a domain for the requested record types.
// Supported types: A, AAAA, MX, NS, TXT, CNAME.
func Lookup(domain string, types []string) ([]Record, error) {
	var all []Record
	var errs []string

	for _, t := range types {
		recs, err := lookupType(domain, t)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", t, err))
			continue
		}
		all = append(all, recs...)
	}

	if len(all) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("lookup failed: %s", strings.Join(errs, "; "))
	}
	return all, nil
}

func lookupType(domain, t string) ([]Record, error) {
	switch t {
	case "A":
		return lookupA(domain)
	case "AAAA":
		return lookupAAAA(domain)
	case "MX":
		return lookupMX(domain)
	case "NS":
		return lookupNS(domain)
	case "TXT":
		return lookupTXT(domain)
	case "CNAME":
		return lookupCNAME(domain)
	default:
		return nil, fmt.Errorf("unsupported record type: %s", t)
	}
}

func lookupA(domain string) ([]Record, error) {
	ips, err := lookupIP(domain)
	if err != nil {
		return nil, err
	}
	var recs []Record
	for _, ip := range ips {
		if ip.To4() != nil {
			recs = append(recs, Record{Name: domain, Type: "A", Value: ip.String()})
		}
	}
	return recs, nil
}

func lookupAAAA(domain string) ([]Record, error) {
	ips, err := lookupIP(domain)
	if err != nil {
		return nil, err
	}
	var recs []Record
	for _, ip := range ips {
		if ip.To4() == nil {
			recs = append(recs, Record{Name: domain, Type: "AAAA", Value: ip.String()})
		}
	}
	return recs, nil
}

func lookupMX(domain string) ([]Record, error) {
	mxs, err := lookupMXNet(domain)
	if err != nil {
		return nil, err
	}
	var recs []Record
	for _, mx := range mxs {
		recs = append(recs, Record{
			Name:  domain,
			Type:  "MX",
			Value: fmt.Sprintf("%d %s", mx.Pref, mx.Host),
		})
	}
	return recs, nil
}

func lookupNS(domain string) ([]Record, error) {
	nss, err := lookupNSNet(domain)
	if err != nil {
		return nil, err
	}
	var recs []Record
	for _, ns := range nss {
		recs = append(recs, Record{Name: domain, Type: "NS", Value: ns.Host})
	}
	return recs, nil
}

func lookupTXT(domain string) ([]Record, error) {
	txts, err := lookupTXTNet(domain)
	if err != nil {
		return nil, err
	}
	var recs []Record
	for _, txt := range txts {
		recs = append(recs, Record{Name: domain, Type: "TXT", Value: txt})
	}
	return recs, nil
}

func lookupCNAME(domain string) ([]Record, error) {
	cname, err := lookupCNAMENet(domain)
	if err != nil {
		return nil, err
	}
	// LookupCNAME returns the input if no CNAME exists. Treat that as empty.
	if cname == domain || strings.TrimSuffix(cname, ".") == strings.TrimSuffix(domain, ".") {
		return nil, nil
	}
	return []Record{{Name: domain, Type: "CNAME", Value: cname}}, nil
}

// FormatRecords returns a human-readable table of DNS records.
func FormatRecords(records []Record) string {
	if len(records) == 0 {
		return "No records found."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-8s %-40s %-50s %s\n", "TYPE", "NAME", "VALUE", "TTL"))
	b.WriteString(strings.Repeat("-", 110) + "\n")
	for _, r := range records {
		ttl := ""
		if r.TTL > 0 {
			ttl = fmt.Sprintf("%d", r.TTL)
		}
		b.WriteString(fmt.Sprintf("%-8s %-40s %-50s %s\n", r.Type, r.Name, r.Value, ttl))
	}
	return b.String()
}

// RecordsToJSON marshals records to indented JSON.
func RecordsToJSON(records []Record) ([]byte, error) {
	out := struct {
		Records []Record `json:"records"`
	}{Records: records}
	if records == nil {
		out.Records = []Record{}
	}
	return json.MarshalIndent(out, "", "  ")
}
