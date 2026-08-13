package connectors

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// Gravatar, e-posta adresine bağlı Gravatar profil bilgilerini sorgular.
// API anahtarı gerektirmez — e-posta hash'i ile açık profil sorgusu.
type Gravatar struct {
	client *http.Client
}

func NewGravatar() *Gravatar {
	return &Gravatar{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (g *Gravatar) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "gravatar",
		Name:        "gravatar",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"email"},
		Description: "Gravatar profil sorgusu — avatar ve profil bilgileri",
		RateLimit:   5,
		Confidence:  80,
	}
}

func (g *Gravatar) Timeout() time.Duration { return 15 * time.Second }

func (g *Gravatar) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	// E-posta hash'i
	email := strings.ToLower(strings.TrimSpace(target))
	hash := fmt.Sprintf("%x", md5.Sum([]byte(email)))

	// JSON profil endpoint'i
	url := fmt.Sprintf("https://en.gravatar.com/%s.json", hash)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OSINT-Engine/1.0")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gravatar: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []plugin.Result{{
			Type:    "gravatar_status",
			Value:   "no_profile",
			Context: fmt.Sprintf(`{"email":"%s","message":"Gravatar profili bulunamadı"}`, target),
		}}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gravatar: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, err
	}

	var gravatarResp struct {
		Entry []struct {
			Hash        string `json:"hash"`
			DisplayName string `json:"displayName"`
			AboutMe     string `json:"aboutMe"`
			CurrentLoc  string `json:"currentLocation"`
			PhotoURL    string `json:"thumbnailUrl"`
			ProfileURL  string `json:"profileUrl"`
			URLs        []struct {
				Title string `json:"title"`
				Value string `json:"value"`
			} `json:"urls"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(body, &gravatarResp); err != nil {
		return nil, fmt.Errorf("gravatar: parse failed: %w", err)
	}

	var results []plugin.Result
	for _, entry := range gravatarResp.Entry {
		results = append(results, plugin.Result{
			Type:  "gravatar_profile",
			Value: entry.DisplayName,
			Context: fmt.Sprintf(`{"email":"%s","about":"%s","location":"%s","photo":"%s","profile_url":"%s"}`,
				target, entry.AboutMe, entry.CurrentLoc, entry.PhotoURL, entry.ProfileURL),
		})

		// Profildeki URL'ler
		for _, u := range entry.URLs {
			results = append(results, plugin.Result{
				Type:    "profile_url",
				Value:   u.Value,
				Context: fmt.Sprintf(`{"title":"%s","email":"%s","source":"gravatar"}`, u.Title, target),
			})
		}
	}

	return results, nil
}
