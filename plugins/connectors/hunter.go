package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// Hunter, Hunter.io API ile e-posta doğrulama ve domain e-posta keşfi yapar.
type Hunter struct {
	client *http.Client
	apiKey string
}

func NewHunter(apiKey string) *Hunter {
	return &Hunter{
		client: &http.Client{Timeout: 30 * time.Second},
		apiKey: apiKey,
	}
}

func (h *Hunter) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "hunter",
		Name:        "hunter",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"domain", "email"},
		Description: "Hunter.io — domain e-posta keşfi ve e-posta doğrulama",
		RateLimit:   2,
		Confidence:  85,
	}
}

func (h *Hunter) Timeout() time.Duration { return 30 * time.Second }

func (h *Hunter) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	if h.apiKey == "" {
		return nil, fmt.Errorf("hunter: API anahtarı gerekli — 'osint keys set hunter <key>' ile ekleyin")
	}

	if strings.Contains(target, "@") {
		parts := strings.SplitN(target, "@", 2)
		if len(parts) == 2 {
			target = parts[1]
		}
	}

	// Domain sorgusu — o domain'deki e-postaları keşfet.
	// API anahtarı bilerek query string'de DEĞİL: URL'ler log'lara, proxy
	// kayıtlarına ve Referer başlığına sızar. Hunter v2 'Authorization: Bearer'
	// destekliyor.
	url := fmt.Sprintf("https://api.hunter.io/v2/domain-search?domain=%s", target)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OSINT-Engine/1.0")
	req.Header.Set("Authorization", "Bearer "+h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hunter: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hunter: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}

	var hunterResp struct {
		Data struct {
			Domain       string `json:"domain"`
			Organization string `json:"organization"`
			Emails       []struct {
				Value      string `json:"value"`
				Type       string `json:"type"`
				Confidence int    `json:"confidence"`
				FirstName  string `json:"first_name"`
				LastName   string `json:"last_name"`
				Position   string `json:"position"`
			} `json:"emails"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &hunterResp); err != nil {
		return nil, fmt.Errorf("hunter: parse failed: %w", err)
	}

	var results []plugin.Result
	for _, email := range hunterResp.Data.Emails {
		results = append(results, plugin.Result{
			Type:  "email",
			Value: email.Value,
			Context: fmt.Sprintf(`{"type":"%s","confidence":%d,"first_name":"%s","last_name":"%s","position":"%s","org":"%s","source":"hunter.io"}`,
				email.Type, email.Confidence, email.FirstName, email.LastName, email.Position, hunterResp.Data.Organization),
		})
	}

	return results, nil
}
