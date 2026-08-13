package report

import (
	"strings"
	"testing"
)

// githubProfileContext, social-profile connector'ının GitHub için ürettiği
// gerçek Context verisidir (canlı çalıştırmadan alınmıştır).
func githubProfileEntity() EntityData {
	return EntityData{
		Type:  "social_profile",
		Value: "https://github.com/testuser",
		Attributes: map[string]any{
			"source":    "social-profile",
			"platform":  "GitHub",
			"name":      "Ada",
			"bio":       "",
			"location":  "",
			"company":   "",
			"followers": float64(18),
			"following": float64(17),
			"repos":     float64(25),
			"created":   "2022-11-09T17:56:58Z",
		},
	}
}

func TestOrderedAttributes_PrioritisesIdentity(t *testing.T) {
	got := OrderedAttributes(githubProfileEntity().Attributes)
	if len(got) == 0 {
		t.Fatal("nitelik bekleniyordu, hiç dönmedi")
	}

	// "Ad" en başta olmalı — bir araştırmacının ilk aradığı şey.
	if got[0].Key != "name" {
		t.Errorf("ilk nitelik 'name' olmalı, alınan %q", got[0].Key)
	}

	keys := map[string]string{}
	for _, a := range got {
		keys[a.Key] = a.Value
	}

	// Boş değerler HİÇ gösterilmemeli ("Biyografi: " gibi boş satırlar
	// raporu kirletiyordu).
	for _, empty := range []string{"bio", "location", "company"} {
		if _, exists := keys[empty]; exists {
			t.Errorf("boş değer %q gösterilmemeli", empty)
		}
	}

	// Sayılar tam sayı olarak basılmalı, "18.00" değil.
	if keys["followers"] != "18" {
		t.Errorf("followers '18' olmalı, alınan %q", keys["followers"])
	}

	// "source" gürültü olarak elenmeli — zaten Entity.Sources'ta var.
	if _, exists := keys["source"]; exists {
		t.Error("'source' anahtarı gösterilmemeli")
	}
}

func TestOrderedAttributes_HidesDisplayNoise(t *testing.T) {
	got := OrderedAttributes(map[string]any{
		"platform": "Trello",
		"found":    true,
		"username": "testuser",
	})

	for _, a := range got {
		if a.Key == "found" || a.Key == "username" {
			t.Errorf("%q her satırda tekrarlanan gürültü, gizlenmeliydi", a.Key)
		}
	}
	if len(got) != 1 || got[0].Key != "platform" {
		t.Errorf("yalnızca 'platform' beklenmişti, alınan %+v", got)
	}
}

func TestOrderedAttributes_UnknownKeysSurvive(t *testing.T) {
	// Yeni bir connector tanımsız bir anahtar üretirse sessizce kaybolmamalı.
	got := OrderedAttributes(map[string]any{"brand_new_field": "değer"})
	if len(got) != 1 || got[0].Value != "değer" {
		t.Errorf("tanımsız anahtar korunmalıydı, alınan %+v", got)
	}
}

func TestIdentitySummary_MergesAcrossEntities(t *testing.T) {
	entities := []EntityData{
		{Type: "username_presence", Value: "https://x.com/a",
			Attributes: map[string]any{"platform": "Twitter/X"}},
		githubProfileEntity(),
		{Type: "social_profile", Value: "https://example.com/b",
			Attributes: map[string]any{"location": "İstanbul"}},
	}

	got := IdentitySummary(entities)

	found := map[string]string{}
	for _, a := range got {
		found[a.Key] = a.Value
	}

	if found["name"] != "Ada" {
		t.Errorf("özet 'name' içermeli, alınan %+v", found)
	}
	if found["location"] != "İstanbul" {
		t.Errorf("özet farklı entity'den 'location' toplamalı, alınan %+v", found)
	}
	// "platform" kimlik bilgisi değil, özete girmemeli.
	if _, exists := found["platform"]; exists {
		t.Error("'platform' kimlik özetine girmemeli")
	}
}

// TestSortByInformation, asıl kullanıcı şikâyetini hedefler: bilgi dolu kayıt
// 25 satırlık URL listesinin ortasında kayboluyordu.
func TestSortByInformation_RichEntitiesFirst(t *testing.T) {
	entities := []EntityData{
		{Type: "username_presence", Value: "https://500px.com/p/x",
			Attributes: map[string]any{"platform": "500px"}},
		{Type: "username_presence", Value: "https://trello.com/x"},
		githubProfileEntity(),
	}

	SortByInformation(entities)

	if entities[0].Value != "https://github.com/testuser" {
		t.Errorf("en bilgi dolu kayıt başta olmalı, alınan %q", entities[0].Value)
	}
	if entities[len(entities)-1].Value != "https://trello.com/x" {
		t.Errorf("niteliksiz kayıt sonda olmalı, alınan %q", entities[len(entities)-1].Value)
	}
}

func TestCompactAttributes_TruncatesAndLimits(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := CompactAttributes(map[string]any{
		"name": "Ada", "location": "İstanbul",
		"company": "ACME", "bio": long, "blog": "https://example.com",
	}, 3)

	if strings.Count(got, "·") != 2 {
		t.Errorf("3 nitelik (2 ayraç) beklenmişti: %q", got)
	}
	if len(got) > 200 {
		t.Errorf("çıktı tablo hücresi için fazla uzun: %d karakter", len(got))
	}
}

func TestFormatValue_Types(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{float64(18), "18"},
		{float64(1.5), "1.5"},
		{"  boşluklu  ", "boşluklu"},
		{true, "evet"},
		{false, "hayır"},
		{[]any{float64(53), float64(443)}, "53, 443"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := formatValue(c.in); got != c.want {
			t.Errorf("formatValue(%v) = %q, beklenen %q", c.in, got, c.want)
		}
	}
}

// TestIdentitySummary_DoesNotMergeDifferentPeople, canlı testte ortaya çıkan
// GERÇEK hatayı hedefler: t.me/testuser var ama BAŞKA birine ait. Eski
// birleştirmeli özet "Deniz Kaya" adını araştırılan kişininmiş gibi
// gösteriyordu. Aynı kullanıcı adı ≠ aynı kişi.
func TestIdentitySummary_DoesNotMergeDifferentPeople(t *testing.T) {
	entities := []EntityData{
		{Type: "username_presence", Value: "https://t.me/testuser",
			Attributes: map[string]any{"platform": "Telegram", "display_name": "Deniz Kaya"}},
		{Type: "username_presence", Value: "https://instagram.com/testuser",
			Attributes: map[string]any{"platform": "Instagram", "display_name": "AFK"}},
		{Type: "username_presence", Value: "https://huggingface.co/testuser",
			Attributes: map[string]any{"platform": "Hugging Face", "display_name": "Ada Nova YILMAZ"}},
	}

	got := IdentitySummary(entities)

	// Üç çelişkili iddia da AYRI AYRI görünmeli; biri diğerini bastırmamalı.
	names := map[string]string{}
	for _, a := range got {
		if a.Key == "display_name" {
			names[a.Value] = a.Source
		}
	}
	if len(names) != 3 {
		t.Fatalf("3 ayrı kimlik iddiası bekleniyordu, alınan %d: %+v", len(names), names)
	}

	// Her iddia KAYNAĞIYLA gelmeli — araştırmacı hangisinin kime ait
	// olduğunu ancak böyle ayırt edebilir.
	if names["Deniz Kaya"] != "Telegram" {
		t.Errorf("'Deniz Kaya' kaynağı Telegram olmalı, alınan %q", names["Deniz Kaya"])
	}
	if names["AFK"] != "Instagram" {
		t.Errorf("'AFK' kaynağı Instagram olmalı, alınan %q", names["AFK"])
	}
}

// Aynı değer birden çok platformda geçiyorsa TEK satırda toplanmalı —
// bu gerçek bir çapraz doğrulama sinyalidir.
func TestIdentitySummary_MergesMatchingClaims(t *testing.T) {
	entities := []EntityData{
		{Attributes: map[string]any{"platform": "GitHub", "profile_username": "testuser"}},
		{Attributes: map[string]any{"platform": "Twitter/X", "profile_username": "testuser"}},
	}

	got := IdentitySummary(entities)
	if len(got) != 1 {
		t.Fatalf("aynı değer tek satırda toplanmalıydı, alınan %d", len(got))
	}
	if !strings.Contains(got[0].Source, "GitHub") || !strings.Contains(got[0].Source, "Twitter/X") {
		t.Errorf("her iki kaynak da listelenmeli, alınan %q", got[0].Source)
	}
}
