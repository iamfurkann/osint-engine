package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// VirusTotal, VirusTotal API v3 üzerinden domain/hash sorgusu yapar.
// API anahtarı gerektirir.
type VirusTotal struct {
	client *http.Client
	apiKey string
}

func NewVirusTotal(apiKey string) *VirusTotal {
	return &VirusTotal{
		client: &http.Client{Timeout: 30 * time.Second},
		apiKey: apiKey,
	}
}

func (v *VirusTotal) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "virustotal",
		Name:        "virustotal",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"domain", "hash"},
		Description: "VirusTotal domain/hash analizi",
		RateLimit:   4, // Ücretsiz plan: 4 req/min
		Confidence:  90,
	}
}

func (v *VirusTotal) Timeout() time.Duration { return 30 * time.Second }

func (v *VirusTotal) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	if v.apiKey == "" {
		return nil, fmt.Errorf("virustotal: API anahtarı gerekli — 'osint keys set virustotal <key>' ile ekleyin")
	}

	// Domain sorgusu
	url := fmt.Sprintf("https://www.virustotal.com/api/v3/domains/%s", target)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("virustotal: request creation failed: %w", err)
	}
	req.Header.Set("x-apikey", v.apiKey)
	req.Header.Set("User-Agent", "OSINT-Engine/1.0")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("virustotal: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("virustotal: geçersiz API anahtarı (401)")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("virustotal: rate limit aşıldı (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("virustotal: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("virustotal: failed to read response: %w", err)
	}

	// Yanıtı parse et
	var vtResp struct {
		Data struct {
			Attributes struct {
				LastAnalysisStats struct {
					Malicious  int `json:"malicious"`
					Suspicious int `json:"suspicious"`
					Harmless   int `json:"harmless"`
					Undetected int `json:"undetected"`
				} `json:"last_analysis_stats"`
				Reputation     int               `json:"reputation"`
				Categories     map[string]string `json:"categories"`
				LastDNSRecords []struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"last_dns_records"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &vtResp); err != nil {
		return nil, fmt.Errorf("virustotal: failed to parse response: %w", err)
	}

	attrs := vtResp.Data.Attributes
	var results []plugin.Result

	// Analiz sonucu
	results = append(results, plugin.Result{
		Type:  "vt_analysis",
		Value: target,
		Context: fmt.Sprintf(`{"malicious":%d,"suspicious":%d,"harmless":%d,"undetected":%d,"reputation":%d}`,
			attrs.LastAnalysisStats.Malicious,
			attrs.LastAnalysisStats.Suspicious,
			attrs.LastAnalysisStats.Harmless,
			attrs.LastAnalysisStats.Undetected,
			attrs.Reputation),
	})

	// DNS kayıtları
	for _, dns := range attrs.LastDNSRecords {
		results = append(results, plugin.Result{
			Type:    "dns_record",
			Value:   dns.Value,
			Context: fmt.Sprintf(`{"type":"%s","source":"virustotal"}`, dns.Type),
		})
	}

	return results, nil
}
