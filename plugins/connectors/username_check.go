package connectors

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// UsernameCheck, kullanıcı adının çeşitli platformlardaki varlığını kontrol eder.
// HTTP durum kodu kontrolü ile çalışır — API anahtarı gerektirmez.
type UsernameCheck struct {
	client *http.Client
}

func NewUsernameCheck() *UsernameCheck {
	return &UsernameCheck{
		client: &http.Client{
			Timeout: 10 * time.Second,
			// Redirect'leri takip etme — bazı platformlar 404'ü redirect ile yakalar
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (u *UsernameCheck) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "username-check",
		Name:        "username-check",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"username"},
		Description: "Kullanıcı adı varlık kontrolü — çoklu platform tarama",
		RateLimit:   5,
		Confidence:  75,
	}
}

func (u *UsernameCheck) Timeout() time.Duration { return 120 * time.Second }

// platformCheck, kontrol edilecek platform bilgisini tanımlar.
type platformCheck struct {
	Name         string
	URL          string // {username} placeholder'ı target ile değiştirilir
	Method       string // GET veya HEAD
	OkCode       int    // Bu kod dönerse hesap var
	NotFoundText string // If body contains this text, user does NOT exist (even if 200)
}

var platforms = []platformCheck{
	// === SOSYAL MEDYA ===
	{Name: "GitHub", URL: "https://github.com/%s", Method: http.MethodGet, OkCode: 200},
	{Name: "Twitter/X", URL: "https://x.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "This account doesn\u2019t exist"},
	{Name: "Instagram", URL: "https://www.instagram.com/%s/", Method: http.MethodGet, OkCode: 200, NotFoundText: "Sorry, this page isn't available"},
	{Name: "Reddit", URL: "https://www.reddit.com/user/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Sorry, nobody on Reddit goes by that name"},
	{Name: "LinkedIn", URL: "https://www.linkedin.com/in/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page not found"},
	{Name: "Facebook", URL: "https://www.facebook.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page Not Found"},
	{Name: "TikTok", URL: "https://www.tiktok.com/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Couldn't find this account"},
	{Name: "Snapchat", URL: "https://www.snapchat.com/add/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "could not be found"},
	{Name: "Pinterest", URL: "https://www.pinterest.com/%s/", Method: http.MethodGet, OkCode: 200, NotFoundText: "Sorry! We couldn't find that page"},
	{Name: "Tumblr", URL: "https://%s.tumblr.com", Method: http.MethodGet, OkCode: 200, NotFoundText: "There's nothing here"},
	{Name: "Flickr", URL: "https://www.flickr.com/people/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "couldn't find"},
	{Name: "VK", URL: "https://vk.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page not found"},
	{Name: "Mastodon.social", URL: "https://mastodon.social/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "is not available"},
	{Name: "Threads", URL: "https://www.threads.net/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Sorry, this page isn't available"},
	{Name: "Bluesky", URL: "https://bsky.app/profile/%s.bsky.social", Method: http.MethodGet, OkCode: 200, NotFoundText: "profile not found"},

	// === VIDEO & STREAMING ===
	{Name: "YouTube", URL: "https://www.youtube.com/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "404 Not Found"},
	{Name: "Twitch", URL: "https://www.twitch.tv/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Sorry. Unless you've got a time machine"},
	{Name: "Vimeo", URL: "https://vimeo.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Sorry, we couldn't find that page"},
	{Name: "Dailymotion", URL: "https://www.dailymotion.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page not found"},
	{Name: "Kick", URL: "https://kick.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "This channel doesn't exist"},

	// === MESAJLAŞMA ===
	{Name: "Telegram", URL: "https://t.me/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "If you have Telegram"},
	{Name: "Discord (ID)", URL: "https://discord.com/users/%s", Method: http.MethodGet, OkCode: 200},

	// === KOD & GELİŞTİRİCİ ===
	{Name: "GitLab", URL: "https://gitlab.com/%s", Method: http.MethodGet, OkCode: 200},
	{Name: "Bitbucket", URL: "https://bitbucket.org/%s/", Method: http.MethodGet, OkCode: 200, NotFoundText: "We can't find that page"},
	{Name: "StackOverflow", URL: "https://stackoverflow.com/users/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page Not Found"},
	{Name: "HackerRank", URL: "https://www.hackerrank.com/profile/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page Not Found"},
	{Name: "LeetCode", URL: "https://leetcode.com/u/%s/", Method: http.MethodGet, OkCode: 200, NotFoundText: "The page you're looking for"},
	{Name: "CodePen", URL: "https://codepen.io/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "404"},
	{Name: "Replit", URL: "https://replit.com/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "CodeWars", URL: "https://www.codewars.com/users/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "DockerHub", URL: "https://hub.docker.com/u/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "HttpError"},
	{Name: "npm", URL: "https://www.npmjs.com/~%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "404"},
	{Name: "PyPI", URL: "https://pypi.org/user/%s/", Method: http.MethodGet, OkCode: 200, NotFoundText: "Not Found"},
	{Name: "RubyGems", URL: "https://rubygems.org/profiles/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "HackerOne", URL: "https://hackerone.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page not found"},
	{Name: "BugCrowd", URL: "https://bugcrowd.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "whoa, that's a 404"},
	{Name: "Dev.to", URL: "https://dev.to/%s", Method: http.MethodGet, OkCode: 200},
	{Name: "Hashnode", URL: "https://hashnode.com/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},

	// === MÜZİK & SES ===
	{Name: "SoundCloud", URL: "https://soundcloud.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "We can't find that user"},
	{Name: "Spotify", URL: "https://open.spotify.com/user/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "page-not-found"},
	{Name: "Last.fm", URL: "https://www.last.fm/user/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page Not Found"},
	{Name: "Bandcamp", URL: "https://bandcamp.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Sorry, that something isn't here"},

	// === TASARIM & PORTFOLYO ===
	{Name: "Behance", URL: "https://www.behance.net/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Oops! We can't find"},
	{Name: "Dribbble", URL: "https://dribbble.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "That page doesn't exist"},
	{Name: "DeviantArt", URL: "https://www.deviantart.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "does not exist"},
	{Name: "ArtStation", URL: "https://www.artstation.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page Not Found"},
	{Name: "Figma", URL: "https://www.figma.com/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "404"},
	{Name: "Gravatar", URL: "https://en.gravatar.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Profile not found"},

	// === OYUN ===
	{Name: "Steam", URL: "https://steamcommunity.com/id/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "The specified profile could not be found"},
	{Name: "Xbox Gamertag", URL: "https://xboxgamertag.com/search/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "Chess.com", URL: "https://www.chess.com/member/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "Lichess", URL: "https://lichess.org/@/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},

	// === BLOG & İÇERİK ===
	{Name: "Medium", URL: "https://medium.com/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Out of nothing, something"},
	{Name: "Substack", URL: "https://%s.substack.com", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "WordPress", URL: "https://%s.wordpress.com", Method: http.MethodGet, OkCode: 200, NotFoundText: "doesn't exist"},
	{Name: "Blogger", URL: "https://%s.blogspot.com", Method: http.MethodGet, OkCode: 200, NotFoundText: "Blog not found"},

	// === FİNANS & BAĞIŞ ===
	{Name: "Patreon", URL: "https://www.patreon.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not exist"},
	{Name: "BuyMeACoffee", URL: "https://www.buymeacoffee.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "404"},
	{Name: "Ko-fi", URL: "https://ko-fi.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "doesn't exist"},

	// === DİĞER ===
	{Name: "About.me", URL: "https://about.me/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "doesn't have a page"},
	{Name: "Linktree", URL: "https://linktr.ee/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "Keybase", URL: "https://keybase.io/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "Trello", URL: "https://trello.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "model not found"},
	{Name: "Kaggle", URL: "https://www.kaggle.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "404"},
	{Name: "Hugging Face", URL: "https://huggingface.co/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "does not exist"},
	{Name: "Product Hunt", URL: "https://www.producthunt.com/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page not found"},
	{Name: "Slideshare", URL: "https://www.slideshare.net/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "404"},
	{Name: "Quora", URL: "https://www.quora.com/profile/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page Not Found"},
	{Name: "Goodreads", URL: "https://www.goodreads.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page not found"},
	{Name: "Disqus", URL: "https://disqus.com/by/%s/", Method: http.MethodGet, OkCode: 200, NotFoundText: "not found"},
	{Name: "Fiverr", URL: "https://www.fiverr.com/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "doesn't seem to exist"},
	{Name: "Freelancer", URL: "https://www.freelancer.com/u/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "does not exist"},
	{Name: "Letterboxd", URL: "https://letterboxd.com/%s/", Method: http.MethodGet, OkCode: 200, NotFoundText: "Sorry, we can't find the page"},
	{Name: "Unsplash", URL: "https://unsplash.com/@%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Page not found"},
	{Name: "500px", URL: "https://500px.com/p/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "Oops! Something went wrong"},
	{Name: "MyAnimeList", URL: "https://myanimelist.net/profile/%s", Method: http.MethodGet, OkCode: 200, NotFoundText: "404 Not Found"},
}

// probe, kontrol edilecek tek bir (platform, kullanıcı adı) çiftidir.
type probe struct {
	platform  platformCheck
	username  string
	isVariant bool
}

func (u *UsernameCheck) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	var (
		results []plugin.Result
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	// Faz 1: tam eşleşme, TÜM platformlarda.
	probes := make([]probe, 0, len(platforms))
	for _, p := range platforms {
		probes = append(probes, probe{platform: p, username: target})
	}

	// Faz 0: varyant denenecek platformları kalibre et.
	//
	// Bu platformların bir kısmı var olmayan kullanıcı adları için de
	// "bulundu" döndürüyor. Varyantlar zaten tahmin olduğu için, kalibre
	// edilmemiş bir platformda üretilecek sonuçların tamamı çöp olur.
	var variantPlatforms []platformCheck
	for _, p := range platforms {
		if variantCheckPlatforms[p.Name] {
			variantPlatforms = append(variantPlatforms, p)
		}
	}
	negatives := u.calibratePlatforms(ctx, variantPlatforms)

	// Faz 2: varyantlar, YALNIZCA büyük kimlik platformlarında.
	//
	// Gerçek vaka: "testuser" arandığında Instagram'da başka birinin hesabı
	// bulunuyor, hedefin asıl hesabı "_testuser_" olduğu için kaçırılıyordu.
	for _, v := range UsernameVariants(target) {
		for _, p := range variantPlatforms {
			probes = append(probes, probe{platform: p, username: v, isVariant: true})
		}
	}

	// Paralel kontrol (semaphore ile max 15 eşzamanlı)
	sem := make(chan struct{}, 15)

	for _, pr := range probes {
		wg.Add(1)
		go func(pr probe) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			p := pr.platform
			found, body := u.checkPlatform(ctx, p, pr.username)
			if !found {
				return
			}

			// Sayfa varlık kontrolü için ZATEN indirildi; aynı gövdeden
			// görünen ad, biyografi ve avatar çıkarılıyor. Ek HTTP isteği yok.
			attrs := map[string]any{
				"platform": p.Name,
				"username": pr.username,
				"found":    true,
			}
			if pr.isVariant {
				// Varyantla bulunan hesap, aranan kullanıcı adının TAM
				// eşleşmesi değildir. Araştırmacı bunu bilmeli: farklı bir
				// kullanıcı adı, farklı bir kişi olma ihtimalini artırır.
				attrs["match"] = "varyant (aranan: " + target + ")"
			}
			for k, v := range extractProfileMetaFromBytes(body) {
				// Ham og:title sayfa başlığıdır ("testuser - Overview");
				// insan adına indirgenmeden gösterilmesi anlamsız.
				if k == "display_name" {
					if s, ok := v.(string); ok {
						cleaned := cleanDisplayName(s, p.Name, pr.username)
						if cleaned == "" {
							continue
						}
						v = cleaned
					}
				}
				attrs[k] = v
			}

			// Zenginleştirmeyi DOĞRULAMA olarak kullan.
			//
			// Canlı testte görüldü: Facebook, Bluesky, Twitch ve TikTok
			// var olmayan her kullanıcı adı için 200 dönüyor. Ama gerçek bir
			// profil sayfası her zaman og:title/og:description yayınlar;
			// sahte "bulundu" sonuçlarında hiçbir metadata çıkmıyor.
			//
			// Varyantlar zaten TAHMİN olduğu için burada katı davranıyoruz:
			// kanıt yoksa raporlamıyoruz. Tam eşleşmeler ise raporlanıyor ama
			// doğrulanmadıkları işaretleniyor — kullanıcı aradığı adı
			// sonuçlarda hiç görmemektense şüpheli olarak görmeli.
			// Kalibrasyonla karşılaştır: sonuç, o platformun "kullanıcı yok"
			// sayfasıyla aynıysa bu bir yanlış pozitiftir.
			if negatives[p.Name].matches(attrs) {
				if pr.isVariant {
					return // varyant + kanıtsız → hiç raporlama
				}
				attrs["verification"] = "şüpheli (platform var olmayan adlara da 'bulundu' diyor)"
			} else if !hasProfileEvidence(attrs) {
				if pr.isVariant {
					return
				}
				attrs["verification"] = "doğrulanamadı (profil verisi yok)"
			}

			profileURL := fmt.Sprintf(p.URL, pr.username)

			// Biyografiden pivot edilebilir varlıklar çıkar: e-posta,
			// kişisel site, çapraz platform kullanıcı adı. Zincirin asıl
			// değeri burada — "profil bul → bio oku → e-postayı çıkar".
			var derived []plugin.Result
			if bio, ok := attrs["bio"].(string); ok {
				derived = ExtractEntitiesFromText(bio, profileURL, pr.username)
			}

			mu.Lock()
			results = append(results, plugin.Result{
				Type:    "username_presence",
				Value:   profileURL,
				Context: mustJSONContext(attrs),
			})
			results = append(results, derived...)
			mu.Unlock()
		}(pr)
	}

	wg.Wait()
	return results, nil
}

// maxProfileBody, profil sayfasından okunacak azami bayt sayısıdır.
// Meta etiketleri <head> içinde olduğu için 256 KB fazlasıyla yeterli,
// ve bu sınır bir sitenin devasa yanıtla belleği doldurmasını engeller.
const maxProfileBody = 256 * 1024

// checkPlatform, bir platformda kullanıcı adının var olup olmadığını kontrol
// eder ve sayfa gövdesini geri döndürür.
//
// Gövde artık atılmıyor: aynı indirmeden profil metadata'sı (görünen ad,
// biyografi, avatar) çıkarılıyor. Önceden gövde yalnızca NotFoundText
// kontrolü için okunuyor, sonra çöpe gidiyordu.
func (u *UsernameCheck) checkPlatform(ctx context.Context, p platformCheck, username string) (bool, []byte) {
	url := fmt.Sprintf(p.URL, username)

	req, err := http.NewRequestWithContext(ctx, p.Method, url, nil)
	if err != nil {
		return false, nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := u.client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != p.OkCode {
		return false, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProfileBody))
	if err != nil {
		// Gövde okunamadıysa varlık kararı durum koduna dayanır; ancak
		// NotFoundText tanımlıysa doğrulayamayacağımız için reddediyoruz.
		return p.NotFoundText == "", nil
	}

	if p.NotFoundText != "" && strings.Contains(string(body), p.NotFoundText) {
		return false, nil
	}

	return true, body
}
