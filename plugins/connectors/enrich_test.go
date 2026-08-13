package connectors

import (
	"strings"
	"testing"
)

func TestExtractProfileMeta_OpenGraph(t *testing.T) {
	page := `<!DOCTYPE html><html><head>
		<title>Generic Site Title</title>
		<meta property="og:title" content="Deniz Kaya">
		<meta property="og:description" content="You can contact @testuser right away.">
		<meta property="og:image" content="https://example.com/avatar.png">
		<meta property="profile:username" content="testuser">
		<meta name="description" content="fallback açıklama">
	</head><body>gövde</body></html>`

	got := extractProfileMeta(strings.NewReader(page))

	if got["display_name"] != "Deniz Kaya" {
		t.Errorf("display_name: %q", got["display_name"])
	}
	if got["bio"] != "You can contact @testuser right away." {
		t.Errorf("bio: %q", got["bio"])
	}
	if got["avatar"] != "https://example.com/avatar.png" {
		t.Errorf("avatar: %q", got["avatar"])
	}
	if got["profile_username"] != "testuser" {
		t.Errorf("profile_username: %q", got["profile_username"])
	}
}

// og:title yoksa <title>'a düşülmeli, ama og:title varsa ONA öncelik verilmeli.
func TestExtractProfileMeta_TitleFallback(t *testing.T) {
	withOG := `<html><head><title>Site</title>
		<meta property="og:title" content="Gerçek Ad"></head></html>`
	if got := extractProfileMeta(strings.NewReader(withOG)); got["display_name"] != "Gerçek Ad" {
		t.Errorf("og:title tercih edilmeliydi: %q", got["display_name"])
	}

	onlyTitle := `<html><head><title>Sadece Başlık</title></head></html>`
	if got := extractProfileMeta(strings.NewReader(onlyTitle)); got["display_name"] != "Sadece Başlık" {
		t.Errorf("<title> yedeği çalışmadı: %q", got["display_name"])
	}
}

func TestExtractProfileMeta_IgnoresShortNoise(t *testing.T) {
	// Çok kısa açıklamalar gürültüdür, saklanmamalı.
	page := `<html><head><meta property="og:description" content="-"></head></html>`
	if got := extractProfileMeta(strings.NewReader(page)); got["bio"] != nil {
		t.Errorf("çok kısa bio elenmeliydi: %q", got["bio"])
	}
}

func TestCleanMetaValue_NormalisesAndTruncates(t *testing.T) {
	if got := cleanMetaValue("  çok\n  boşluklu   metin "); got != "çok boşluklu metin" {
		t.Errorf("boşluk normalizasyonu: %q", got)
	}
	if got := cleanMetaValue("&amp; kaçış"); got != "& kaçış" {
		t.Errorf("HTML kaçışı çözülmeli: %q", got)
	}

	long := strings.Repeat("ş", 400)
	got := cleanMetaValue(long)
	if len([]rune(got)) > maxMetaValueLen+1 { // +1 = "…"
		t.Errorf("kırpma başarısız: %d rune", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Error("kırpılan değer '…' ile bitmeli")
	}
}

// TestCleanDisplayName, canlı çalıştırmada gözlemlenen GERÇEK başlıkları
// kullanır. Bunlar temizlenmeden "Görünen ad" olarak gösterildiğinde
// araştırmacıya hiçbir şey söylemiyordu.
func TestCleanDisplayName(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		platform string
		username string
		want     string
	}{
		{"GitHub overview", "testuser - Overview", "GitHub", "testuser", ""},
		{"Fiverr profil", "Ada | Profile | Fiverr", "Fiverr", "testuser", "Ada"},
		{"Chess profil", "testuser - Chess Profile", "Chess.com", "testuser", ""},
		{"Telegram temiz ad", "Deniz Kaya", "Telegram", "testuser", "Deniz Kaya"},
		{"X noktalı ad", ". (@testuser) on X", "Twitter/X", "testuser", ""},
		{"X gerçek ad", "Ada Yilmaz (@testuser) on X", "Twitter/X", "testuser", "Ada Yilmaz"},
		{"boş girdi", "", "GitHub", "testuser", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cleanDisplayName(c.raw, c.platform, c.username)
			if got != c.want {
				t.Errorf("cleanDisplayName(%q) = %q, beklenen %q", c.raw, got, c.want)
			}
		})
	}
}

// Türkçe karakterli adlar bozulmamalı.
func TestCleanDisplayName_PreservesTurkish(t *testing.T) {
	got := cleanDisplayName("Şeyma Çağlar | Profile | Fiverr", "Fiverr", "seymac")
	if got != "Şeyma Çağlar" {
		t.Errorf("Türkçe ad bozuldu: %q", got)
	}
}

func TestExtractProfileMeta_EmptyAndBrokenInput(t *testing.T) {
	if got := extractProfileMetaFromBytes(nil); got != nil {
		t.Errorf("boş gövde nil dönmeli, alınan %+v", got)
	}
	// Bozuk HTML panik atmamalı; parser hata toleranslıdır.
	if got := extractProfileMetaFromBytes([]byte("<html><head><meta")); got != nil && len(got) > 0 {
		t.Logf("bozuk HTML'den çıkan (kabul edilebilir): %+v", got)
	}
}

// TestExtractProfileMeta_ChallengePage, bot koruma ekranlarının profil
// sanılmamasını doğrular. Canlı çalıştırmada PyPI için "Client Challenge"
// görünen ad olarak raporlanmıştı.
func TestExtractProfileMeta_ChallengePage(t *testing.T) {
	page := `<html><head><title>Client Challenge</title>
		<meta property="og:description" content="Bir şeyler doğrulanıyor"></head></html>`

	got := extractProfileMeta(strings.NewReader(page))

	if got["display_name"] != nil {
		t.Errorf("bot koruma sayfasından görünen ad üretilmemeli: %q", got["display_name"])
	}
	if got["verification"] == nil {
		t.Error("doğrulanamadığı işaretlenmeliydi")
	}
}

func TestIsChallengePage(t *testing.T) {
	blocked := []string{"Client Challenge", "Just a moment...", "Attention Required! | Cloudflare", "CAPTCHA"}
	for _, s := range blocked {
		if !isChallengePage(s) {
			t.Errorf("%q bot koruması olarak tanınmalıydı", s)
		}
	}
	ok := []string{"Deniz Kaya", "testuser - Overview", ""}
	for _, s := range ok {
		if isChallengePage(s) {
			t.Errorf("%q yanlışlıkla bot koruması sayıldı", s)
		}
	}
}

// TestHasProfileEvidence, yanlış pozitiflere karşı en ucuz savunmayı doğrular.
// Canlı testte Facebook, Bluesky, Twitch ve TikTok var olmayan HER kullanıcı
// adı için 200 döndü — ama o sayfalarda profil metadata'sı yoktu.
func TestHasProfileEvidence(t *testing.T) {
	// Gerçek profil: metadata var
	real := map[string]any{"platform": "Instagram", "display_name": "Ada"}
	if !hasProfileEvidence(real) {
		t.Error("metadata taşıyan sonuç kanıtlı sayılmalı")
	}

	// Yanlış pozitif: yalnızca HTTP 200, hiçbir profil verisi yok
	fake := map[string]any{"platform": "Facebook", "username": "zqxjkl", "found": true}
	if hasProfileEvidence(fake) {
		t.Error("metadata taşımayan sonuç kanıtsız sayılmalı")
	}

	// Boş string kanıt değildir
	empty := map[string]any{"display_name": "   "}
	if hasProfileEvidence(empty) {
		t.Error("boş display_name kanıt sayılmamalı")
	}

	// bio veya avatar tek başına yeterli
	if !hasProfileEvidence(map[string]any{"bio": "Merhaba, ben Ada"}) {
		t.Error("bio kanıt sayılmalı")
	}
	if !hasProfileEvidence(map[string]any{"avatar": "https://x/y.png"}) {
		t.Error("avatar kanıt sayılmalı")
	}
}
