package connectors

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// maxMetaValueLen, tek bir metadata değerinin saklanacak azami uzunluğudur.
// og:description bazı sitelerde sayfanın tamamını içerebiliyor.
const maxMetaValueLen = 300

// metaMapping, HTML meta anahtarını Entity.Attributes anahtarına eşler.
// Sıra ÖNEMLİDİR: aynı hedefe yazan birden çok kaynak varsa ilki kazanır,
// yani og:* değerleri jenerik <title>'a tercih edilir.
var metaMapping = []struct {
	MetaKey   string // HTML'deki property/name değeri
	AttrKey   string // Entity.Attributes anahtarı
	MinLength int    // Bu uzunluğun altındaki değerler gürültü sayılır
}{
	{"og:title", "display_name", 1},
	{"twitter:title", "display_name", 1},
	{"profile:username", "profile_username", 1},
	{"og:description", "bio", 8},
	{"twitter:description", "bio", 8},
	{"description", "bio", 8},
	{"og:image", "avatar", 8},
	{"og:site_name", "site", 1},
}

// extractProfileMeta, bir profil sayfasının HTML'inden görünen ad, biyografi
// ve avatar gibi bilgileri çıkarır.
//
// Neden bu var: sistem bir profili bulduğunda yalnızca "şu URL'de bir hesap
// var" diyordu. Sayfanın kendisi Open Graph etiketlerinde görünen adı ve
// bio'yu zaten yayınlıyor; bunları okumak ek API, anahtar veya ücret
// gerektirmiyor. Üstelik varlık kontrolü sayfayı ZATEN indiriyor, yani
// ilave bir HTTP isteği de yok.
func extractProfileMeta(r io.Reader) map[string]any {
	doc, err := html.Parse(r)
	if err != nil {
		return nil
	}

	raw := make(map[string]string)
	var title string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				var key, content string
				for _, a := range n.Attr {
					switch strings.ToLower(a.Key) {
					case "property", "name":
						key = strings.ToLower(a.Val)
					case "content":
						content = a.Val
					}
				}
				if key != "" && content != "" {
					if _, exists := raw[key]; !exists {
						raw[key] = content
					}
				}
			case "title":
				if title == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					title = n.FirstChild.Data
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Bot koruma ekranı: içerik profil değil. Uydurma "görünen ad"
	// üretmektense hiçbir nitelik döndürmüyoruz ve durumu işaretliyoruz;
	// bu, varlık kontrolünün de doğrulanamadığı anlamına gelir.
	if isChallengePage(raw["og:title"]) || isChallengePage(title) {
		return map[string]any{"verification": "engellendi (bot koruması)"}
	}

	out := make(map[string]any)
	for _, m := range metaMapping {
		v := cleanMetaValue(raw[m.MetaKey])
		if len(v) < m.MinLength {
			continue
		}
		if _, exists := out[m.AttrKey]; !exists {
			out[m.AttrKey] = v
		}
	}

	// <title> yalnızca og:title yoksa kullanılır — çoğu sitede jeneriktir
	// ("GitHub", "Instagram") ve kimlik bilgisi taşımaz.
	if _, exists := out["display_name"]; !exists {
		if v := cleanMetaValue(title); v != "" {
			out["display_name"] = v
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// cleanMetaValue, HTML metadata değerini normalize eder ve kırpar.
func cleanMetaValue(s string) string {
	s = strings.TrimSpace(html.UnescapeString(s))
	s = strings.Join(strings.Fields(s), " ") // satır sonları/çoklu boşluk tek boşluğa
	if len(s) > maxMetaValueLen {
		// Rune sınırında kes — Türkçe karakterleri ortadan bölmemek için.
		runes := []rune(s)
		if len(runes) > maxMetaValueLen {
			runes = runes[:maxMetaValueLen]
		}
		s = strings.TrimSpace(string(runes)) + "…"
	}
	return s
}

// extractProfileMetaFromBytes, extractProfileMeta'nın byte dilimi alan hâlidir.
func extractProfileMetaFromBytes(body []byte) map[string]any {
	if len(body) == 0 {
		return nil
	}
	return extractProfileMeta(bytes.NewReader(body))
}

// titleSeparators, sayfa başlıklarında bölüm ayıracı olarak kullanılan
// karakterlerdir: "Ada | Profile | Fiverr".
var titleSeparators = []string{"|", "·", "•", "—", "–", " - "}

// trailingBoilerplate, başlığın SONUNA ayıraçsız yapışan site kalıplarıdır.
// Bölümleme bunları yakalayamaz çünkü araya "|" veya "-" girmez.
var trailingBoilerplate = []string{
	" on X", " on Twitter", " on Instagram", " on TikTok",
	" photos and videos", " - Chess Profile", " - Overview",
}

// boilerplateSegments, sayfa başlığında geçen ama kimlik bilgisi TAŞIMAYAN
// bölümlerdir. Küçük harfe çevrilip karşılaştırılır.
var boilerplateSegments = map[string]bool{
	"overview": true, "profile": true, "chess profile": true,
	"home": true, "user profile": true, "public profile": true,
	"repositories": true, "photos and videos": true,
	"on x": true, "on twitter": true, "github": true,
}

// challengeTitles, bot korumasının döndürdüğü sayfa başlıklarıdır.
//
// Bunlar profil DEĞİLDİR: canlı çalıştırmada PyPI için "Client Challenge"
// görünen ad olarak raporlanmıştı. Böyle bir sayfa aynı zamanda varlık
// kontrolünün güvenilmez olduğunun kanıtıdır — sunucu bize gerçek içeriği
// hiç göstermemiştir.
var challengeTitles = []string{
	"client challenge", "just a moment", "attention required",
	"access denied", "bot verification", "are you a robot",
	"security check", "checking your browser", "403 forbidden",
	"captcha",
}

// isChallengePage, sayfa başlığının bir bot koruma ekranı olup olmadığını söyler.
func isChallengePage(title string) bool {
	low := strings.ToLower(strings.TrimSpace(title))
	if low == "" {
		return false
	}
	for _, c := range challengeTitles {
		if strings.Contains(low, c) {
			return true
		}
	}
	return false
}

// cleanDisplayName, og:title'dan gelen ham sayfa başlığını insan adına indirger.
//
// Ham değerler pratikte şöyle geliyor:
//
//	"testuser - Overview"          (GitHub)
//	". (@testuser) on X"           (X/Twitter)
//	"Ada | Profile | Fiverr"      (Fiverr)
//	"testuser - Chess Profile"     (Chess.com)
//
// Bunlar kimlik özetinde "Görünen ad" olarak gösterildiğinde araştırmacıya
// hiçbir şey söylemiyor. Başlık bölümlere ayrılıp platform adı, kullanıcı adı
// ve jenerik kalıplar elenince geriye gerçek ad kalıyor ("Ada").
//
// Hiçbir bölüm ayakta kalmazsa boş string döner — uydurma bir değer
// göstermektense hiç göstermemek daha doğru.
func cleanDisplayName(raw, platform, username string) string {
	s := cleanMetaValue(raw)
	if s == "" {
		return ""
	}

	// "(@handle)" kalıbını at — X başlıklarında ada yapışık geliyor.
	if username != "" {
		s = strings.ReplaceAll(s, "(@"+username+")", " ")
	}

	// Ayıraçsız yapışan sonekleri at. Bunlar bölümleme ile yakalanamıyor
	// çünkü araya "|" veya "-" girmiyor: ". (@testuser) on X"
	for _, suffix := range trailingBoilerplate {
		for {
			trimmed := strings.TrimSpace(s)
			if len(trimmed) > len(suffix) &&
				strings.EqualFold(trimmed[len(trimmed)-len(suffix):], suffix) {
				s = trimmed[:len(trimmed)-len(suffix)]
				continue
			}
			break
		}
	}
	s = strings.Join(strings.Fields(s), " ")

	segments := []string{s}
	for _, sep := range titleSeparators {
		var next []string
		for _, seg := range segments {
			next = append(next, strings.Split(seg, sep)...)
		}
		segments = next
	}

	lowerUser := strings.ToLower(username)
	lowerPlatform := strings.ToLower(platform)

	best := ""
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		low := strings.ToLower(seg)

		if boilerplateSegments[low] || low == lowerUser || low == lowerPlatform {
			continue
		}
		// Yalnızca noktalama içeren bölümler (X'te ad yerine "." koyanlar)
		if strings.TrimFunc(seg, func(r rune) bool {
			return !isLetterOrDigit(r)
		}) == "" {
			continue
		}
		// İlk anlamlı bölüm genellikle addır; sonrakiler site/bölüm adıdır.
		if best == "" {
			best = seg
		}
	}

	return best
}

func isLetterOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r > 127 // Türkçe/Unicode harfler
}

// profileEvidenceKeys, bir sayfanın GERÇEKTEN profil olduğunu gösteren
// niteliklerdir. Bunlardan biri varsa sunucu bize gerçek içerik göstermiştir.
var profileEvidenceKeys = []string{"display_name", "bio", "avatar", "profile_username"}

// hasProfileEvidence, çıkarılan niteliklerin bir profil sayfasına işaret edip
// etmediğini söyler.
//
// Bu, yanlış pozitiflere karşı en ucuz savunmadır: Facebook, Bluesky, Twitch
// ve TikTok var olmayan kullanıcı adları için de HTTP 200 dönüyor, ama o
// sayfalarda profil metadata'sı bulunmuyor.
func hasProfileEvidence(attrs map[string]any) bool {
	for _, k := range profileEvidenceKeys {
		if v, ok := attrs[k]; ok {
			if s, isStr := v.(string); isStr && strings.TrimSpace(s) != "" {
				return true
			}
		}
	}
	return false
}
