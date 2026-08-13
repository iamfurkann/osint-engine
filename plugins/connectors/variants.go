package connectors

import "strings"

// maxVariants, üretilecek azami varyant sayısıdır.
//
// Varyantlar yalnızca büyük kimlik platformlarında denendiği için toplam
// istek sayısı = maxVariants × len(variantCheckPlatforms). Bu çarpımın
// kontrolden çıkmaması gerekiyor; hedef siteleri kızdırmadan gerçek hesabı
// yakalamak istiyoruz.
const maxVariants = 10

// variantCheckPlatforms, varyantların DENENECEĞİ platformlardır.
//
// Neden hepsi değil: 76 platform × 10 varyant = 760 istek, hem yavaş hem
// kaba olur. Bu liste, insanların kullanıcı adı varyantı tutmaya en meyilli
// olduğu büyük kimlik platformlarıdır.
var variantCheckPlatforms = map[string]bool{
	"Instagram": true,
	"Twitter/X": true,
	"TikTok":    true,
	"Telegram":  true,
	"GitHub":    true,
	"YouTube":   true,
	"Facebook":  true,
	"Reddit":    true,
	"Threads":   true,
	"Bluesky":   true,
	"Twitch":    true,
	"Snapchat":  true,
}

// commonPrefixes, kullanıcı adlarına sıkça eklenen öneklerdir.
var commonPrefixes = []string{"iam", "real", "the", "official", "its", "im"}

// UsernameVariants, bir kullanıcı adının yaygın biçim varyantlarını üretir.
//
// Bunun sebebi doğrudan kullanıcı geri bildirimi: "testuser" arandığında
// Instagram'da BAŞKA birinin hesabı bulundu; hedefin gerçek hesabı
// "_testuser_" idi. Alt çizgi ve nokta ile çevrelenmiş varyantlar, istenen
// kullanıcı adı alınmış olduğunda insanların en sık başvurduğu çözümdür.
//
// Üretilen varyantlar TAHMİNDİR. Bulgu olarak raporlanmazlar — yalnızca
// kontrol edilir ve YALNIZCA gerçekten var olanlar, varyant oldukları
// açıkça işaretlenerek sonuçlara girer.
func UsernameVariants(base string) []string {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil
	}

	// Nokta ile başlayan/biten varyantlar ("testuser.", ".testuser")
	// bilerek YOK: canlı testte hiçbir gerçek hesap bulmadılar, yalnızca
	// yanlış pozitif ürettiler. Nokta gerçek kullanımda adın ORTASINDA
	// geçiyor ("ada.yilmaz"), kenarında değil.
	candidates := []string{
		"_" + base,
		base + "_",
		"_" + base + "_",
		base + "official",
		"official" + base,
	}

	// Kullanıcı adı yaygın bir önekle başlıyorsa çekirdek adı da dene:
	// "iamdemo" → "demo" → ve onun alt çizgili hâlleri.
	lower := strings.ToLower(base)
	for _, p := range commonPrefixes {
		if len(lower) > len(p)+2 && strings.HasPrefix(lower, p) {
			core := base[len(p):]
			candidates = append(candidates, core, "_"+core+"_")
			break
		}
	}

	// Önek TAŞIMIYORSA yaygın önekleri ekleyerek dene.
	if !hasCommonPrefix(lower) {
		candidates = append(candidates, "iam"+base, "real"+base)
	}

	// Tekilleştir, orijinali ve geçersizleri ele.
	seen := map[string]bool{strings.ToLower(base): true}
	out := make([]string, 0, maxVariants)
	for _, c := range candidates {
		key := strings.ToLower(c)
		if seen[key] || !isPlausibleUsername(c) {
			continue
		}
		seen[key] = true
		out = append(out, c)
		if len(out) >= maxVariants {
			break
		}
	}
	return out
}

func hasCommonPrefix(lower string) bool {
	for _, p := range commonPrefixes {
		if len(lower) > len(p)+2 && strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// isPlausibleUsername, üretilen varyantın hiçbir platformda geçerli
// olamayacak kadar bozuk olup olmadığını denetler. Boşa istek atmamak için.
func isPlausibleUsername(s string) bool {
	if len(s) < 3 || len(s) > 30 {
		return false
	}
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
		if !ok {
			return false
		}
	}
	return true
}
