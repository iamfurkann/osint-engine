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

// CrtSh, crt.sh üzerinden sertifika şeffaflığı sorgusu yapar.
// Açık API — API anahtarı gerektirmez.
type CrtSh struct {
	client *http.Client
}

func NewCrtSh() *CrtSh {
	return &CrtSh{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CrtSh) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "crtsh",
		Name:        "crtsh",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"domain"},
		Description: "Sertifika Şeffaflığı (Certificate Transparency) sorgusu — crt.sh",
		RateLimit:   2, // crt.sh yavaş olabilir
		Confidence:  90,
	}
}

func (c *CrtSh) Timeout() time.Duration { return 60 * time.Second }

// crtshEntry, crt.sh API yanıtındaki tek bir sertifika kaydı.
type crtshEntry struct {
	IssuerCAID   int    `json:"issuer_ca_id"`
	IssuerName   string `json:"issuer_name"`
	CommonName   string `json:"common_name"`
	NameValue    string `json:"name_value"`
	SerialNumber string `json:"serial_number"`
	NotBefore    string `json:"not_before"`
	NotAfter     string `json:"not_after"`
}

func (c *CrtSh) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	url := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", target)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("crtsh: request creation failed: %w", err)
	}
	req.Header.Set("User-Agent", "OSINT-Engine/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crtsh: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crtsh: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // Max 5MB
	if err != nil {
		return nil, fmt.Errorf("crtsh: failed to read response: %w", err)
	}

	var entries []crtshEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("crtsh: failed to parse response: %w", err)
	}

	// Tekrar eden domain'leri filtrele
	seen := make(map[string]bool)
	var results []plugin.Result

	for _, entry := range entries {
		if seen[entry.CommonName] {
			continue
		}
		seen[entry.CommonName] = true

		results = append(results, plugin.Result{
			Type:  "certificate",
			Value: entry.CommonName,
			Context: fmt.Sprintf(`{"issuer":"%s","not_before":"%s","not_after":"%s","serial":"%s"}`,
				entry.IssuerName, entry.NotBefore, entry.NotAfter, entry.SerialNumber),
		})
	}

	return results, nil
}
