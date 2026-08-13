package report

import (
	"fmt"
	"sort"
	"strings"
)

// attributeLabels, connector'ların Context anahtarlarını okunabilir Türkçe
// etiketlere çevirir. Buradaki sıra aynı zamanda GÖSTERİM ÖNCELİĞİDİR:
// üstteki anahtarlar bir kişi hakkında daha çok şey söyler.
var attributeLabels = []struct {
	Key   string
	Label string
}{
	// Kimlik — bir araştırmacının önce görmek istediği şeyler
	{"name", "Ad"},
	{"full_name", "Tam ad"},
	{"display_name", "Görünen ad"},
	{"profile_username", "Profil adı"},
	{"bio", "Biyografi"},
	{"location", "Konum"},
	{"company", "Şirket"},
	{"email", "E-posta"},
	{"blog", "Web sitesi"},
	{"twitter_username", "X/Twitter"},
	{"avatar", "Avatar"},

	// Hesap sinyalleri — yaş ve büyüklük güvenilirlik göstergesidir
	{"created", "Hesap açılışı"},
	{"followers", "Takipçi"},
	{"following", "Takip edilen"},
	{"repos", "Repo sayısı"},
	{"platform", "Platform"},
	{"verification", "Doğrulama"},
	{"match", "Eşleşme"},
	{"independent_sources", "Bağımsız kaynak grubu"},
	{"confidence_basis", "Güven gerekçesi"},

	// Teknik varlıklar
	{"repo", "Repo"},
	{"language", "Dil"},
	{"org", "Organizasyon"},
	{"isp", "ISP"},
	{"os", "İşletim sistemi"},
	{"city", "Şehir"},
	{"country", "Ülke"},
	{"ports", "Açık portlar"},
	{"tags", "Etiketler"},
	{"vuln_count", "Zafiyet sayısı"},
	{"product", "Ürün"},
	{"version", "Sürüm"},
	{"ip", "IP"},
}

// displayNoiseKeys, GÖSTERİMDEN çıkarılan ama veride kalan anahtarlardır.
//
// Veriden silmiyoruz — API/JSON çıktısında dursunlar. Sadece insan okuyan
// tabloda yer kaplamasınlar:
//   - "found": raporlanmış her bulgu için zaten true, bilgi taşımıyor
//   - "username": aranan hedefin kendisi, her satırda tekrarlanıyor
//   - "source": zaten Entity.Sources sütununda gösteriliyor
//
// "source" resolution katmanında da eleniyor; burada tekrar elenmesi
// bilinçlidir — gösterim, yukarıdaki katmanın filtrelemesine güvenmemeli.
var displayNoiseKeys = map[string]bool{
	"found":    true,
	"username": true,
	"source":   true,
}

// identityKeys, "kişi özeti" bloğunda toplanacak anahtarlardır.
var identityKeys = map[string]bool{
	"name": true, "full_name": true, "display_name": true,
	"bio": true, "location": true, "company": true,
	"email": true, "blog": true, "twitter_username": true,
	"created": true, "followers": true,
	"profile_username": true,
}

// Attribute, gösterime hazır tek bir nitelik satırıdır.
type Attribute struct {
	Key   string
	Label string
	Value string

	// Source, bu iddianın HANGİ platformdan geldiğidir.
	//
	// Kimlik özetinde zorunludur: farklı platformlardaki aynı kullanıcı adı
	// aynı kişi DEĞİLDİR. Kaynak gösterilmezse birbiriyle ilgisiz insanların
	// bilgileri tek bir kimlik profiline karışır (bkz. IdentitySummary).
	Source string
}

// formatValue, JSON'dan gelen any değerini okunabilir tek satıra çevirir.
// json.Unmarshal tüm sayıları float64 yaptığı için tam sayılar ".00" olmadan
// basılır — "18.00 takipçi" saçma görünüyordu.
func formatValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		if t {
			return "evet"
		}
		return "hayır"
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := formatValue(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", t)
	}
}

// OrderedAttributes, bir entity'nin niteliklerini gösterim sırasına dizer.
// attributeLabels'ta tanımlı olanlar önce ve o sırayla; tanımsız anahtarlar
// sona alfabetik olarak eklenir (yeni connector'lar sessizce kaybolmasın).
func OrderedAttributes(attrs map[string]any) []Attribute {
	if len(attrs) == 0 {
		return nil
	}

	out := make([]Attribute, 0, len(attrs))
	seen := make(map[string]bool, len(attrs))

	for _, def := range attributeLabels {
		v, ok := attrs[def.Key]
		if !ok || displayNoiseKeys[def.Key] {
			continue
		}
		seen[def.Key] = true
		if s := formatValue(v); s != "" {
			out = append(out, Attribute{Key: def.Key, Label: def.Label, Value: s})
		}
	}

	rest := make([]string, 0)
	for k := range attrs {
		if !seen[k] && !displayNoiseKeys[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		if s := formatValue(attrs[k]); s != "" {
			out = append(out, Attribute{Key: k, Label: k, Value: s})
		}
	}

	return out
}

// IdentitySummary, kimlik iddialarını KAYNAĞIYLA BİRLİKTE listeler.
//
// Bu fonksiyon önce tüm platformlardan gelen değerleri tek bir profile
// birleştiriyordu. Bu YANLIŞTI ve canlı testte zararı görüldü: t.me/testuser
// gerçekten var ama BAŞKA birine ait; "Deniz Kaya" adı, araştırılan kişinin
// kimlik özetine sanki onunmuş gibi yazıldı. Aynı şekilde
// instagram.com/testuser bambaşka bir hesap — hedefin gerçek hesabı
// _testuser_ idi.
//
// Temel OSINT kuralı: FARKLI PLATFORMLARDAKİ AYNI KULLANICI ADI AYNI KİŞİ
// DEĞİLDİR. Bu yüzden artık birleştirme yapılmıyor; her iddia kendi kaynağıyla
// ayrı satır olarak gösteriliyor ve çelişkiler araştırmacıya görünür kalıyor.
//
// Aynı değer birden çok platformda geçiyorsa tek satırda toplanır — bu
// gerçek bir doğrulama sinyalidir.
func IdentitySummary(entities []EntityData) []Attribute {
	// key → değer → o değeri bildiren kaynaklar (sıra korunur)
	claims := make(map[string]map[string][]string)
	order := make(map[string][]string)

	for _, e := range entities {
		src := entitySourceLabel(e)
		for k, v := range e.Attributes {
			if !identityKeys[k] {
				continue
			}
			val := formatValue(v)
			if val == "" {
				continue
			}
			if claims[k] == nil {
				claims[k] = make(map[string][]string)
			}
			if _, seen := claims[k][val]; !seen {
				order[k] = append(order[k], val)
			}
			claims[k][val] = appendUniqueStr(claims[k][val], src)
		}
	}

	var out []Attribute
	for _, def := range attributeLabels {
		values, ok := claims[def.Key]
		if !ok {
			continue
		}
		for _, val := range order[def.Key] {
			out = append(out, Attribute{
				Key:    def.Key,
				Label:  def.Label,
				Value:  val,
				Source: strings.Join(values[val], ", "),
			})
		}
	}
	return out
}

// entitySourceLabel, bir iddianın hangi platformdan geldiğini adlandırır.
func entitySourceLabel(e EntityData) string {
	if p, ok := e.Attributes["platform"]; ok {
		if s := formatValue(p); s != "" {
			return s
		}
	}
	if len(e.Sources) > 0 {
		return e.Sources[0]
	}
	return "bilinmiyor"
}

func appendUniqueStr(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// SortByInformation, entity'leri BİLGİ YOĞUNLUĞUNA göre sıralar: gerçek
// nitelik taşıyanlar üstte, çıplak URL'ler altta.
//
// Önceden sıralama map iterasyon sırasına bağlıydı, yani rastgeleydi. Pratikte
// asıl değerli kayıt (ad, konum, hesap yaşı içeren profil) 25 satırlık bir
// URL listesinin ortasında kayboluyordu.
func SortByInformation(entities []EntityData) {
	sort.SliceStable(entities, func(i, j int) bool {
		ni := len(OrderedAttributes(entities[i].Attributes))
		nj := len(OrderedAttributes(entities[j].Attributes))
		if ni != nj {
			return ni > nj
		}
		if entities[i].Type != entities[j].Type {
			return entities[i].Type < entities[j].Type
		}
		return entities[i].Value < entities[j].Value
	})
}

// CompactAttributes, tablo hücresine sığacak kısa bir özet üretir.
func CompactAttributes(attrs map[string]any, max int) string {
	ordered := OrderedAttributes(attrs)
	if len(ordered) == 0 {
		return ""
	}
	if max > 0 && len(ordered) > max {
		ordered = ordered[:max]
	}

	parts := make([]string, 0, len(ordered))
	for _, a := range ordered {
		v := a.Value
		if len(v) > 40 {
			v = v[:37] + "..."
		}
		parts = append(parts, a.Label+": "+v)
	}
	return strings.Join(parts, " · ")
}
