package connectors

import "testing"

// TestExtractEntitiesFromText, gerçek OSINT zincirinin ilk halkasını doğrular:
// profil bul → biyografiyi oku → içindeki e-posta/site/@kullanıcı adını çıkar.
func TestExtractEntitiesFromText(t *testing.T) {
	bio := "Hi, I'm Ada. Yazılım geliştiricisi. " +
		"İletişim: ada@example.org veya https://example.org — " +
		"ayrıca @adadev hesabımdan yazabilirsiniz."

	got := ExtractEntitiesFromText(bio, "https://www.fiverr.com/testuser", "testuser")

	byType := map[string][]string{}
	for _, r := range got {
		byType[r.Type] = append(byType[r.Type], r.Value)
	}

	if len(byType["email"]) != 1 || byType["email"][0] != "ada@example.org" {
		t.Errorf("e-posta çıkarılmalıydı: %v", byType["email"])
	}
	if len(byType["url"]) != 1 || byType["url"][0] != "https://example.org" {
		t.Errorf("URL çıkarılmalıydı: %v", byType["url"])
	}
	if len(byType["username"]) != 1 || byType["username"][0] != "adadev" {
		t.Errorf("@kullanıcı adı çıkarılmalıydı: %v", byType["username"])
	}
}

// Profilin KENDİ platformuna ve kendi adına yapılan referanslar ipucu değildir.
func TestExtractEntitiesFromText_SkipsSelfReferences(t *testing.T) {
	bio := "Profilim: https://www.fiverr.com/demouser — ben @demouser"
	got := ExtractEntitiesFromText(bio, "https://www.fiverr.com/demouser", "demouser")
	if len(got) != 0 {
		t.Errorf("kendine yapılan referanslar elenmeliydi: %+v", got)
	}
}

// Jenerik posta kutuları ("support@", "info@") kişisel ipucu değildir.
func TestExtractEntitiesFromText_SkipsGenericMailboxes(t *testing.T) {
	bio := "Sorularınız için support@twitch.tv veya info@example.com"
	if got := ExtractEntitiesFromText(bio, "", ""); len(got) != 0 {
		t.Errorf("jenerik posta kutuları elenmeliydi: %+v", got)
	}
}

func TestExtractEntitiesFromText_EmptyInput(t *testing.T) {
	if got := ExtractEntitiesFromText("", "", ""); got != nil {
		t.Errorf("boş metin nil dönmeli: %+v", got)
	}
	if got := ExtractEntitiesFromText("   ", "", ""); got != nil {
		t.Errorf("boşluk nil dönmeli: %+v", got)
	}
}

// Aynı varlık birden çok kez geçse bile tek kayıt üretilmeli.
func TestExtractEntitiesFromText_Deduplicates(t *testing.T) {
	bio := "a@example.com yazın, tekrar: a@example.com"
	got := ExtractEntitiesFromText(bio, "", "")
	if len(got) != 1 {
		t.Errorf("tekilleştirme başarısız: %+v", got)
	}
}
