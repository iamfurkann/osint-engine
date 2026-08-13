package connectors

import (
	"strings"
	"testing"
)

// TestUsernameVariants_UnderscoreForms, alan geri bildiriminden gelen gerçek
// vakayı hedefler: aranan kullanıcı adı alınmış olduğunda insanlar en sık alt
// çizgi ekliyor ve hedefin asıl hesabı "_ad_" biçiminde oluyor.
func TestUsernameVariants_UnderscoreForms(t *testing.T) {
	got := UsernameVariants("testuser")

	want := []string{"_testuser", "testuser_", "_testuser_"}
	for _, w := range want {
		if !contains(got, w) {
			t.Errorf("%q varyantı üretilmeliydi, üretilenler: %v", w, got)
		}
	}
}

// Yaygın önek taşıyan adlarda çekirdek ad da denenmeli:
// "iamdemo" → "demo"
func TestUsernameVariants_StripsCommonPrefix(t *testing.T) {
	got := UsernameVariants("iamdemo")
	if !contains(got, "demo") {
		t.Errorf("'demo' çekirdek adı denenmeliydi: %v", got)
	}
}

// Önek taşımayan adlara önek EKLENEREK denenmeli.
func TestUsernameVariants_AddsPrefixWhenAbsent(t *testing.T) {
	got := UsernameVariants("demo")
	if !contains(got, "iamdemo") {
		t.Errorf("'iamdemo' varyantı üretilmeliydi: %v", got)
	}
}

func TestUsernameVariants_NeverReturnsOriginal(t *testing.T) {
	for _, base := range []string{"iamdemo", "demo", "AbC123"} {
		for _, v := range UsernameVariants(base) {
			if strings.EqualFold(v, base) {
				t.Errorf("%q için orijinal ad varyant olarak dönmemeli", base)
			}
		}
	}
}

func TestUsernameVariants_RespectsCap(t *testing.T) {
	if got := UsernameVariants("demo"); len(got) > maxVariants {
		t.Errorf("varyant sayısı %d, üst sınır %d", len(got), maxVariants)
	}
}

func TestUsernameVariants_EdgeCases(t *testing.T) {
	if got := UsernameVariants(""); got != nil {
		t.Errorf("boş girdi nil dönmeli, alınan %v", got)
	}
	if got := UsernameVariants("   "); got != nil {
		t.Errorf("boşluk girdisi nil dönmeli, alınan %v", got)
	}
	// Çok kısa adlarda "_x_" gibi varyantlar hâlâ geçerli olabilir ama
	// hiçbiri 3 karakterin altına düşmemeli.
	for _, v := range UsernameVariants("ab") {
		if len(v) < 3 {
			t.Errorf("çok kısa varyant üretildi: %q", v)
		}
	}
}

func TestIsPlausibleUsername(t *testing.T) {
	valid := []string{"_testuser_", "ada.yilmaz", "user-123"}
	for _, v := range valid {
		if !isPlausibleUsername(v) {
			t.Errorf("%q geçerli sayılmalıydı", v)
		}
	}
	invalid := []string{"ab", strings.Repeat("x", 31), "boşluk var", "emoji🙂"}
	for _, v := range invalid {
		if isPlausibleUsername(v) {
			t.Errorf("%q geçersiz sayılmalıydı", v)
		}
	}
}

// Varyantlar yalnızca büyük platformlarda denenmeli; aksi hâlde istek
// sayısı (76 platform × 10 varyant) kontrolden çıkar.
func TestVariantCheckPlatforms_IsNarrow(t *testing.T) {
	if len(variantCheckPlatforms) > 20 {
		t.Errorf("varyant platform listesi fazla geniş: %d", len(variantCheckPlatforms))
	}
	for _, must := range []string{"Instagram", "Twitter/X", "Telegram"} {
		if !variantCheckPlatforms[must] {
			t.Errorf("%q varyant listesinde olmalı", must)
		}
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// TestNegativeSignature_StripsUsername, Telegram gibi sitelerin "kullanıcı yok"
// sayfasında adı metne gömmesini ele alır: "Telegram: Contact @X". Ham
// karşılaştırma hiçbir zaman eşleşmez çünkü X her seferinde değişir.
func TestNegativeSignature_StripsUsername(t *testing.T) {
	sig := negativeSignature{
		unreliable:  true,
		control:     "zqxabc123",
		displayName: "telegram: contact @zqxabc123",
	}

	// Aynı kalıp, farklı kullanıcı adı → yanlış pozitif olarak yakalanmalı.
	fake := map[string]any{
		"username":     "testuserofficial",
		"display_name": "Telegram: Contact @testuserofficial",
	}
	if !sig.matches(fake) {
		t.Error("aynı kalıptaki 'kullanıcı yok' sayfası yakalanmalıydı")
	}

	// Gerçek bir profil adı → elenmemeli.
	real := map[string]any{"username": "_testuser_", "display_name": "Ada"}
	if sig.matches(real) {
		t.Error("gerçek profil elenmemeliydi")
	}
}

func TestNegativeSignature_ReliablePlatformNeverFilters(t *testing.T) {
	sig := negativeSignature{unreliable: false}
	// Güvenilir platformda kanıtsız sonuç bile elenmemeli — 404 zaten ayırıyor.
	if sig.matches(map[string]any{"platform": "GitHub"}) {
		t.Error("güvenilir platformda filtreleme yapılmamalı")
	}
}

func TestRandomControlUsername_IsPlausibleAndUnique(t *testing.T) {
	a, b := randomControlUsername(), randomControlUsername()
	if a == b {
		t.Error("kontrol adı rastgele olmalı")
	}
	if !isPlausibleUsername(a) {
		t.Errorf("kontrol adı geçerli formatta olmalı: %q", a)
	}
}
