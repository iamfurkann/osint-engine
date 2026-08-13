package connectors

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// DNSWhois, domain için DNS kayıtlarını ve temel WHOIS bilgilerini sorgular.
// Harici API anahtarı gerektirmez — Go standart kütüphanesi kullanır.
type DNSWhois struct{}

func NewDNSWhois() *DNSWhois { return &DNSWhois{} }

func (d *DNSWhois) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "dns-whois",
		Name:        "dns-whois",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"domain"},
		Description: "DNS kayıtları ve IP çözümleme",
		Confidence:  95,
	}
}

func (d *DNSWhois) Timeout() time.Duration { return 30 * time.Second }

func (d *DNSWhois) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	var results []plugin.Result
	resolver := &net.Resolver{}

	// A kayıtları (IPv4)
	ips, err := resolver.LookupHost(ctx, target)
	if err == nil {
		for _, ip := range ips {
			results = append(results, plugin.Result{
				Type:    "ip",
				Value:   ip,
				Context: fmt.Sprintf(`{"record_type":"A/AAAA","domain":"%s"}`, target),
			})
		}
	}

	// MX kayıtları
	mxRecords, err := resolver.LookupMX(ctx, target)
	if err == nil {
		for _, mx := range mxRecords {
			results = append(results, plugin.Result{
				Type:    "mx_record",
				Value:   strings.TrimSuffix(mx.Host, "."),
				Context: fmt.Sprintf(`{"priority":%d,"domain":"%s"}`, mx.Pref, target),
			})
		}
	}

	// NS kayıtları
	nsRecords, err := resolver.LookupNS(ctx, target)
	if err == nil {
		for _, ns := range nsRecords {
			results = append(results, plugin.Result{
				Type:    "ns_record",
				Value:   strings.TrimSuffix(ns.Host, "."),
				Context: fmt.Sprintf(`{"domain":"%s"}`, target),
			})
		}
	}

	// TXT kayıtları
	txtRecords, err := resolver.LookupTXT(ctx, target)
	if err == nil {
		for _, txt := range txtRecords {
			results = append(results, plugin.Result{
				Type:    "txt_record",
				Value:   txt,
				Context: fmt.Sprintf(`{"domain":"%s"}`, target),
			})
		}
	}

	// CNAME
	cname, err := resolver.LookupCNAME(ctx, target)
	if err == nil && cname != target+"." {
		results = append(results, plugin.Result{
			Type:    "cname_record",
			Value:   strings.TrimSuffix(cname, "."),
			Context: fmt.Sprintf(`{"domain":"%s"}`, target),
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("dns-whois: no DNS records found for %s", target)
	}

	return results, nil
}
