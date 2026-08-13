// Package evidence, bulguları kanıta çeviren ve güven puanı üreten motordur.
//
// Var olma sebebi iki somut arıza:
//
//  1. Üretilen her raporda güven "0%" yazıyordu. Entity.Confidence hiçbir
//     yerde atanmıyordu ve onu dolduracak skorlama paketi hiçbir yerden
//     çağrılmıyordu.
//
//  2. Korelasyon motoru "aynı connector buldu" diye her şeyi her şeye
//     bağlıyordu. Tek bir HTTP isteğinden 13 varlık çıkan bir araştırma
//     78 "ilişki" raporluyordu — sıfır bilgi, tam bir K₁₃ grafı.
//
// İkisinin de kökü aynı: sistemde KANIT BAĞIMSIZLIĞI kavramı yoktu.
package evidence

import "strings"

// CueGroup, bir kanıtın hangi BAĞIMSIZ bilgi kaynağından türediğini belirtir.
//
// Bu, motorun en önemli tek fikridir. Aynı gruptaki gözlemler birbirini
// çarpımsal olarak güçlendiremez, çünkü aynı temel sinyali paylaşırlar:
// aynı HTTP taramasının 40 sonucu 40 bağımsız kanıt değil, 1 taramadır.
type CueGroup string

const (
	// GroupPresence: bir kullanıcı adının platformda var olup olmadığı.
	// Tek bir tarama turundan gelen tüm sonuçlar buraya düşer.
	GroupPresence CueGroup = "platform.presence"

	// GroupProfile: profil sayfasından okunan içerik (ad, bio, avatar).
	// Presence'tan AYRI bir gruptur: varlık kontrolü "sayfa 200 döndü" der,
	// profil verisi "sayfada gerçek bir kimlik var" der. İkisi farklı iddia.
	GroupProfile CueGroup = "platform.profile"

	GroupDNS         CueGroup = "dns"         // DNS kayıtları
	GroupCertificate CueGroup = "certificate" // Sertifika şeffaflığı
	GroupWebSearch   CueGroup = "web.search"  // Arama motoru sonuçları
	GroupWebArchive  CueGroup = "web.archive" // Arşiv kayıtları
	GroupInfraScan   CueGroup = "infra.scan"  // Port/servis taraması
	GroupReputation  CueGroup = "reputation"  // İtibar servisleri
	GroupEmailIntel  CueGroup = "email"       // E-posta keşif servisleri

	// GroupDerived: SİSTEMİN KENDİ ÜRETTİĞİ tahminler.
	//
	// name-generator kullanıcı adı permütasyonları üretir, bio-extraction
	// metinden desen çıkarır. Bunlar dış dünyadan gelen gözlem değil,
	// hipotezdir; kanıt olarak neredeyse hiç ağırlık taşımamalıdırlar.
	GroupDerived CueGroup = "derived"

	GroupUnknown CueGroup = "unknown"
)

// sourceGroups, connector adını bağımsızlık sınıfına eşler.
//
// Yeni bir connector eklendiğinde buraya da eklenmelidir; aksi hâlde
// GroupUnknown'a düşer ve düşük ağırlıkla değerlendirilir (güvenli varsayılan).
var sourceGroups = map[string]CueGroup{
	"username-check":    GroupPresence,
	"social-profile":    GroupProfile,
	"dns-whois":         GroupDNS,
	"crtsh":             GroupCertificate,
	"web-scraper":       GroupWebSearch,
	"wayback":           GroupWebArchive,
	"shodan":            GroupInfraScan,
	"shodan-internetdb": GroupInfraScan,
	"virustotal":        GroupReputation,
	"hunter":            GroupEmailIntel,
	"gravatar":          GroupEmailIntel,
	"hibp":              GroupEmailIntel,
	"name-generator":    GroupDerived,
	"bio-extraction":    GroupDerived,
}

// GroupOf, bir kaynağın bağımsızlık sınıfını döndürür.
func GroupOf(source string) CueGroup {
	if g, ok := sourceGroups[strings.ToLower(strings.TrimSpace(source))]; ok {
		return g
	}
	return GroupUnknown
}

// groupLabels, raporda gösterilecek Türkçe adlardır.
var groupLabels = map[CueGroup]string{
	GroupPresence:    "platform varlığı",
	GroupProfile:     "profil içeriği",
	GroupDNS:         "DNS kayıtları",
	GroupCertificate: "sertifika şeffaflığı",
	GroupWebSearch:   "web araması",
	GroupWebArchive:  "web arşivi",
	GroupInfraScan:   "altyapı taraması",
	GroupReputation:  "itibar servisi",
	GroupEmailIntel:  "e-posta istihbaratı",
	GroupDerived:     "sistem tahmini",
	GroupUnknown:     "bilinmeyen kaynak",
}

// Label, grubun insan-okunur adını döndürür.
func (g CueGroup) Label() string {
	if l, ok := groupLabels[g]; ok {
		return l
	}
	return string(g)
}

// groupWeights, bir gruptan gelen TEK bir doğrulamanın log-odds katkısıdır.
//
// Bu değerler ÖLÇÜLMÜŞ DEĞİL, mühendislik tahminidir. Kalibrasyon için
// etiketli bir doğrulama seti gerekir (bkz. Score.Calibrated). Bu yüzden
// çıktı yüzdesi her yerde "kalibre edilmemiş" etiketiyle sunulur.
//
// Sıralamanın mantığı:
//   - profil içeriği en güçlü: sayfa gerçek bir kimlik yayınlıyor
//   - DNS/sertifika güçlü: kriptografik veya yetkili kayıtlar
//   - platform varlığı zayıf: yalnızca "sayfa 200 döndü"
//   - sistem tahmini neredeyse sıfır: bu bir gözlem değil, hipotez
var groupWeights = map[CueGroup]float64{
	GroupProfile:     1.30,
	GroupDNS:         1.20,
	GroupCertificate: 1.20,
	GroupInfraScan:   1.00,
	GroupEmailIntel:  0.90,
	GroupReputation:  0.80,
	GroupWebArchive:  0.70,
	GroupWebSearch:   0.50,
	GroupPresence:    0.45,
	GroupUnknown:     0.30,
	GroupDerived:     0.05,
}

// Weight, gruptan gelen tek bir doğrulamanın temel ağırlığıdır.
func (g CueGroup) Weight() float64 {
	if w, ok := groupWeights[g]; ok {
		return w
	}
	return groupWeights[GroupUnknown]
}
