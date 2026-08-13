package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

type SocialProfile struct {
	client *http.Client
}

func NewSocialProfile() *SocialProfile {
	return &SocialProfile{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SocialProfile) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "social-profile",
		Name:        "social-profile",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"username"},
		Description: "GitHub API üzerinden profil detayları, repo listesi ve bağlantılı kullanıcılar",
		RateLimit:   5,
		Confidence:  85,
	}
}

func (s *SocialProfile) Timeout() time.Duration { return 20 * time.Second }

func (s *SocialProfile) Initialize(ctx context.Context, config map[string]string) error {
	return nil
}

func (s *SocialProfile) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	var results []plugin.Result

	// 1. Fetch GitHub user profile
	userURL := fmt.Sprintf("https://api.github.com/users/%s", target)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("social-profile: istek oluşturulamadı: %w", redactURLError(err))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "OSINT-Engine/1.0")

	// Hatalar artık YUTULMUYOR.
	//
	// Önceden ağ hatası, rate limit ve "kullanıcı yok" durumlarının hepsi
	// (nil, nil) dönüyordu. Sonuç: GitHub geçici olarak 403 verdiğinde
	// connector sessizce sıfır bulgu döndürüyor ve görev BAŞARILI sayılıyordu.
	// Canlı testte tam olarak bu yaşandı ve sonucun neden boş olduğu
	// anlaşılamadı.
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("social-profile: istek başarısız: %w", redactURLError(err))
	}
	defer resp.Body.Close()

	// 404 = kullanıcı yok. Bu gerçekten hata değil.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf(
			"social-profile: GitHub API limiti aşıldı (%d) — kimliksiz erişim saatte 60 istekle sınırlı",
			resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("social-profile: beklenmeyen durum kodu %d", resp.StatusCode)
	}

	var ghUser struct {
		Login       string `json:"login"`
		Name        string `json:"name"`
		Bio         string `json:"bio"`
		Location    string `json:"location"`
		Email       string `json:"email"`
		Blog        string `json:"blog"`
		Company     string `json:"company"`
		Followers   int    `json:"followers"`
		Following   int    `json:"following"`
		PublicRepos int    `json:"public_repos"`
		AvatarURL   string `json:"avatar_url"`
		HTMLURL     string `json:"html_url"`
		Twitter     string `json:"twitter_username"`
		CreatedAt   string `json:"created_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("social-profile: yanıt çözümlenemedi: %w", err)
	}

	// Profile result
	profileCtx := fmt.Sprintf(`{"source":"social-profile","platform":"GitHub","name":"%s","bio":"%s","location":"%s","company":"%s","followers":%d,"following":%d,"repos":%d,"created":"%s"}`,
		esc(ghUser.Name), esc(ghUser.Bio), esc(ghUser.Location), esc(ghUser.Company),
		ghUser.Followers, ghUser.Following, ghUser.PublicRepos, ghUser.CreatedAt)

	results = append(results, plugin.Result{
		Type:    "social_profile",
		Value:   ghUser.HTMLURL,
		Context: profileCtx,
	})

	// If email is public
	if ghUser.Email != "" {
		results = append(results, plugin.Result{
			Type:    "email",
			Value:   ghUser.Email,
			Context: `{"source":"social-profile","platform":"GitHub"}`,
		})
	}

	// If blog/website exists
	if ghUser.Blog != "" {
		results = append(results, plugin.Result{
			Type:    "domain",
			Value:   ghUser.Blog,
			Context: `{"source":"social-profile","platform":"GitHub"}`,
		})
	}

	// If twitter username exists
	if ghUser.Twitter != "" {
		results = append(results, plugin.Result{
			Type:    "username",
			Value:   ghUser.Twitter,
			Context: `{"source":"social-profile","platform":"GitHub-Twitter-Link"}`,
		})
	}

	// 2. Fetch recent repos for potential secrets/info
	repoURL := fmt.Sprintf("https://api.github.com/users/%s/repos?sort=updated&per_page=5", target)
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, repoURL, nil)
	req2.Header.Set("Accept", "application/vnd.github.v3+json")
	req2.Header.Set("User-Agent", "OSINT-Engine/1.0")

	resp2, err := s.client.Do(req2)
	if err == nil && resp2.StatusCode == 200 {
		defer resp2.Body.Close()
		var repos []struct {
			Name     string `json:"name"`
			HTMLURL  string `json:"html_url"`
			Language string `json:"language"`
			Fork     bool   `json:"fork"`
		}
		if err := json.NewDecoder(resp2.Body).Decode(&repos); err == nil {
			for _, repo := range repos {
				if repo.Fork {
					continue
				}
				lang := repo.Language
				if lang == "" {
					lang = "unknown"
				}
				repoCtx := fmt.Sprintf(`{"source":"social-profile","platform":"GitHub","repo":"%s","language":"%s"}`, esc(repo.Name), lang)
				results = append(results, plugin.Result{
					Type:    "web_result",
					Value:   repo.HTMLURL,
					Context: repoCtx,
				})
			}
		}
	}

	return results, nil
}

// esc escapes quotes for JSON embedding
func esc(s string) string {
	s = strings.ReplaceAll(s, `"`, `'`)
	s = strings.ReplaceAll(s, `\`, ``)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
