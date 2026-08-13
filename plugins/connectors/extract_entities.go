package connectors

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

var (
	// bioEmailRegex, serbest metin içindeki e-posta adreslerini yakalar.
	// input.Detect'teki desenden farklı: orası tüm dizeyi doğrular (^...$),
	// burada metnin İÇİNDEN çıkarma yapılıyor.
	bioEmailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	// bioURLRegex, http(s) bağlantılarını yakalar.
	bioURLRegex = regexp.MustCompile(`https?://[^\s<>"'\)\]]+`)

	// bioHandleRegex, "@kullaniciadi" biçimindeki çapraz platform
	// referanslarını yakalar. Biyografilerde en sık görülen ipucu budur.
	bioHandleRegex = regexp.MustCompile(`(?:^|[\s(\[])@([a-zA-Z0-9_.]{3,30})`)
)

// genericMailboxes, kişisel olmayan, her sitede bulunan posta kutularıdır.
// Bunları bulgu olarak raporlamak gürültüdür.
var genericMailboxes = map[string]bool{
	"support": true, "info": true, "contact": true, "help": true,
	"admin": true, "noreply": true, "no-reply": true, "hello": true,
	"privacy": true, "legal": true, "abuse": true, "press": true,
	"sales": true, "team": true, "hi": true,
}

// ExtractEntitiesFromText, bir profil biyografisinden pivot edilebilir
// varlıklar çıkarır: e-posta, kişisel site ve çapraz platform kullanıcı adları.
//
// Gerçek OSINT zinciri budur: profil bul → biyografiyi oku → içindeki
// e-postayı çıkar → o e-postayla yeni arama başlat. Sistem daha önce bu
// zincirin ilk halkasında duruyordu; biyografi metni toplanıyor ama hiç
// ayrıştırılmıyordu.
//
// selfHost ve selfHandle, kaydın kendi platformunu/kullanıcı adını eleyerek
// "profil kendini işaret ediyor" türü gürültüyü engeller.
func ExtractEntitiesFromText(text, selfHost, selfHandle string) []plugin.Result {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// selfHost tam URL olarak da gelebiliyor (profil bağlantısı); host'a indir.
	selfHost = hostOf(selfHost)

	var out []plugin.Result
	seen := make(map[string]bool)

	add := func(typ, value, note string) {
		key := typ + ":" + strings.ToLower(value)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, plugin.Result{
			Type:  typ,
			Value: value,
			Context: mustJSONContext(map[string]any{
				"source":    "bio-extraction",
				"extracted": note,
			}),
		})
	}

	for _, m := range bioEmailRegex.FindAllString(text, -1) {
		m = strings.Trim(m, ".,;:")
		local := strings.ToLower(m)
		if i := strings.Index(local, "@"); i > 0 {
			if genericMailboxes[local[:i]] {
				continue
			}
		}
		add("email", m, "biyografiden çıkarıldı")
	}

	for _, raw := range bioURLRegex.FindAllString(text, -1) {
		raw = strings.Trim(raw, ".,;:")
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		// Kaydın kendi platformuna giden bağlantı ipucu değildir.
		if selfHost != "" && strings.EqualFold(hostOf(u.Host), selfHost) {
			continue
		}
		add("url", raw, "biyografiden çıkarıldı")
	}

	for _, m := range bioHandleRegex.FindAllStringSubmatch(text, -1) {
		handle := m[1]
		if selfHandle != "" && strings.EqualFold(handle, selfHandle) {
			continue // profilin kendi kullanıcı adı
		}
		add("username", handle, "biyografide geçen @kullanıcı adı")
	}

	return out
}

// hostOf, host karşılaştırmasını normalize eder. Girdi tam URL de olabilir
// ("https://www.fiverr.com/testuser") çıplak host da ("www.fiverr.com").
func hostOf(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			raw = u.Host
		}
	}

	// Yol veya port varsa at.
	if i := strings.IndexAny(raw, "/:"); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimPrefix(raw, "www.")
}
