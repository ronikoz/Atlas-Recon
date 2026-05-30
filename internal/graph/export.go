package graph

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ExportJSON(scan *ScanNode) ([]byte, error) {
	return json.MarshalIndent(scan, "", "  ")
}

func ExportMarkdown(scan *ScanNode) string {
	if scan == nil {
		return "No LAN graph found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# LAN Map %s\n\n", scan.ID)
	fmt.Fprintf(&b, "- CIDRs: %s\n", strings.Join(scan.CIDRs, ", "))
	fmt.Fprintf(&b, "- Hosts: %d\n", len(scan.Hosts))
	fmt.Fprintf(&b, "- Services: %d\n\n", countServices(scan))
	for _, host := range scan.Hosts {
		fmt.Fprintf(&b, "## Host %s\n\n", host.IP)
		if len(host.OpenPorts) == 0 {
			b.WriteString("No open ports recorded.\n\n")
			continue
		}
		b.WriteString("| Port | Service | State | HTTP(S) |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, port := range host.OpenPorts {
			fmt.Fprintf(&b, "| %d | %s | %s | %s |\n", port.Port, port.Service, port.State, serviceSummary(host.Services, port.Port))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func ExportDOT(scan *ScanNode) string {
	if scan == nil {
		return "digraph atlas_recon {}\n"
	}
	var b strings.Builder
	b.WriteString("digraph atlas_recon {\n")
	b.WriteString("  rankdir=LR;\n")
	for _, cidr := range scan.CIDRs {
		fmt.Fprintf(&b, "  %q [shape=box];\n", cidr)
	}
	for _, host := range scan.Hosts {
		fmt.Fprintf(&b, "  %q [shape=ellipse];\n", host.IP)
		if host.CIDR != "" {
			fmt.Fprintf(&b, "  %q -> %q [label=\"contains\"];\n", host.CIDR, host.IP)
		}
		for _, service := range host.Services {
			node := fmt.Sprintf("%s:%d/%s", host.IP, service.Port, service.Scheme)
			fmt.Fprintf(&b, "  %q [shape=component,label=%q];\n", node, node)
			fmt.Fprintf(&b, "  %q -> %q [label=\"exposes\"];\n", host.IP, node)
		}
	}
	for _, link := range scan.Links {
		fmt.Fprintf(&b, "  %q -> %q [label=\"links_to\"];\n", link.FromURL, link.ToURL)
	}
	b.WriteString("}\n")
	return b.String()
}

func countServices(scan *ScanNode) int {
	total := 0
	for _, host := range scan.Hosts {
		total += len(host.Services)
	}
	return total
}

func serviceSummary(services []ServiceNode, port int) string {
	for _, service := range services {
		if service.Port == port {
			if service.Error != "" {
				return service.Error
			}
			if service.Title != "" {
				return fmt.Sprintf("%s %d %s", strings.ToUpper(service.Scheme), service.StatusCode, service.Title)
			}
			return fmt.Sprintf("%s %d", strings.ToUpper(service.Scheme), service.StatusCode)
		}
	}
	return ""
}
