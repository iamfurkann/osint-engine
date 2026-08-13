package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

type WaybackPlugin struct {
	client *http.Client
}

func NewWaybackPlugin() *WaybackPlugin {
	return &WaybackPlugin{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (w *WaybackPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "wayback",
		Name:        "wayback",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"domain", "username"},
		Description: "Wayback Machine (archive.org) ile hedefin geçmiş web sayfalarını kontrol eder",
		RateLimit:   3,
		Confidence:  70,
	}
}

func (w *WaybackPlugin) Timeout() time.Duration { return 20 * time.Second }

func (w *WaybackPlugin) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	var results []plugin.Result

	// Check multiple possible URLs for the target
	urls := []string{
		fmt.Sprintf("https://web.archive.org/web/2024*/https://github.com/%s", target),
		fmt.Sprintf("https://web.archive.org/web/2024*/https://twitter.com/%s", target),
	}

	// Use CDX API to check for archived snapshots
	cdxURL := fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=*%s*&output=json&limit=5&fl=timestamp,original,statuscode&filter=statuscode:200", target)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cdxURL, nil)
	if err != nil {
		return nil, fmt.Errorf("wayback: istek oluşturulamadı: %w", redactURLError(err))
	}
	req.Header.Set("User-Agent", "OSINT-Engine/1.0")

	// Hatalar yutulmuyor: arşivde kayıt olmaması ile arşive ulaşamamak
	// birbirinden ayırt edilebilmeli.
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wayback: istek başarısız: %w", redactURLError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // arşivde kayıt yok — hata değil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wayback: beklenmeyen durum kodu %d", resp.StatusCode)
	}

	var rows [][]string
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("wayback: yanıt çözümlenemedi: %w", err)
	}

	seen := make(map[string]bool)
	// Skip header row (first element)
	for i, row := range rows {
		if i == 0 || len(row) < 3 {
			continue
		}
		timestamp := row[0]
		originalURL := row[1]

		if seen[originalURL] {
			continue
		}
		seen[originalURL] = true

		archiveURL := fmt.Sprintf("https://web.archive.org/web/%s/%s", timestamp, originalURL)

		result := plugin.Result{
			Type:    "web_result",
			Value:   archiveURL,
			Context: fmt.Sprintf(`{"source":"wayback","original_url":"%s","timestamp":"%s","title":"Arsiv Kaydi: %s","snippet":"Wayback Machine arsivinden %s tarihli sayfa"}`, originalURL, timestamp, originalURL, timestamp[:8]),
		}
		results = append(results, result)
	}

	_ = urls // suppress unused warning

	return results, nil
}
