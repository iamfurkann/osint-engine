package connectors

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

type WebScraper struct {
	client *http.Client
}

func NewWebScraper() *WebScraper {
	return &WebScraper{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (w *WebScraper) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "web-scraper",
		Name:        "web-scraper",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"person", "username", "email"},
		Description: "DuckDuckGo ile web araması yaparak kişi hakkında bilgi toplar",
		RateLimit:   2,
		Confidence:  60,
	}
}

func (w *WebScraper) Timeout() time.Duration { return 30 * time.Second }

func (w *WebScraper) Initialize(ctx context.Context, config map[string]string) error {
	return nil
}

func (w *WebScraper) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	// Build search queries
	queries := []string{
		fmt.Sprintf("\"%s\"", target),
		fmt.Sprintf("\"%s\" site:linkedin.com", target),
		fmt.Sprintf("\"%s\" site:github.com", target),
	}

	var results []plugin.Result
	seen := make(map[string]bool)

	for _, query := range queries {
		select {
		case <-ctx.Done():
			return results, nil
		default:
		}

		// DuckDuckGo HTML search
		searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := w.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		if err != nil {
			continue
		}

		// Extract URLs and snippets from DuckDuckGo HTML results
		// DuckDuckGo HTML format: <a rel="nofollow" class="result__a" href="URL">Title</a>
		// and <a class="result__snippet" href="URL">snippet text</a>
		urlPattern := regexp.MustCompile(`class="result__a" href="(https?://[^"]+)"[^>]*>([^<]+)`)
		snippetPattern := regexp.MustCompile(`class="result__snippet"[^>]*>([^<]+)`)

		urlMatches := urlPattern.FindAllStringSubmatch(string(body), 10)
		snippetMatches := snippetPattern.FindAllStringSubmatch(string(body), 10)

		for i, match := range urlMatches {
			if len(match) < 3 || seen[match[1]] {
				continue
			}
			seen[match[1]] = true

			title := strings.TrimSpace(match[2])
			resultURL := match[1]
			snippet := ""
			if i < len(snippetMatches) && len(snippetMatches[i]) > 1 {
				snippet = strings.TrimSpace(snippetMatches[i][1])
			}

			resultType := "web_result"
			if strings.Contains(resultURL, "linkedin.com") {
				resultType = "social_profile"
			} else if strings.Contains(resultURL, "github.com") {
				resultType = "social_profile"
			} else if strings.Contains(resultURL, "twitter.com") || strings.Contains(resultURL, "x.com") {
				resultType = "social_profile"
			} else if strings.Contains(resultURL, "instagram.com") {
				resultType = "social_profile"
			} else if strings.Contains(resultURL, "facebook.com") {
				resultType = "social_profile"
			}

			ctxJSON := fmt.Sprintf(`{"source":"web-scraper","title":"%s","snippet":"%s","query":"%s"}`,
				strings.ReplaceAll(title, `"`, `'`),
				strings.ReplaceAll(snippet, `"`, `'`),
				strings.ReplaceAll(query, `"`, `'`))

			results = append(results, plugin.Result{
				Type:    resultType,
				Value:   resultURL,
				Context: ctxJSON,
			})
		}

		// Small delay between queries to be polite
		time.Sleep(500 * time.Millisecond)
	}

	return results, nil
}
