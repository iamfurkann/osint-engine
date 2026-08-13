package evidence

import "testing"

func obs(source string, profileEvidence bool) Observation {
	return Observation{
		Source:             source,
		Group:              GroupOf(source),
		HasProfileEvidence: profileEvidence,
	}
}

// TestScore_SameGroupDoesNotMultiply, motorun var oluş sebebini doğrular:
// tek bir taramanın 40 sonucu 40 bağımsız kanıt değildir.
//
// Canlı gözlem: 1.1.1.1 araştırması tek bir HTTP isteğinden 13 varlık çıkardı
// ve sistem bunu 78 "ilişki" olarak raporladı.
func TestScore_SameGroupDoesNotMultiply(t *testing.T) {
	e := NewEngine()

	one := e.Score([]Observation{obs("username-check", false)})

	many := make([]Observation, 40)
	for i := range many {
		many[i] = obs("username-check", false)
	}
	forty := e.Score(many)

	if forty.Value <= one.Value {
		t.Fatalf("daha çok gözlem puanı biraz artırmalı: 1→%d, 40→%d", one.Value, forty.Value)
	}

	// Ama 40 kat DEĞİL. Tek bir tarama turu, kaç sonuç üretirse üretsin
	// skoru domine edemez — grup katkısı tavanlıdır.
	single := GroupPresence.Weight()
	groupContribution := forty.LogOdds - one.LogOdds + single
	if groupContribution > single*groupCeiling+0.001 {
		t.Errorf("grup katkısı tavanı (%.2f) aştı: %.2f",
			single*groupCeiling, groupContribution)
	}

	// Somut beklenti: tek gruptan gelen 40 gözlem yüksek güven ÜRETMEMELİ.
	if forty.Value >= 60 {
		t.Errorf("tek taramadan gelen 40 sonuç %d%% güven üretti — "+
			"bu, motorun önlemesi gereken sahte kesinliğin ta kendisi", forty.Value)
	}

	if forty.IndependentGroups != 1 {
		t.Errorf("hepsi tek gruptan, bağımsız grup sayısı 1 olmalı: %d",
			forty.IndependentGroups)
	}
}

// Farklı gruplardan gelen kanıt GERÇEKTEN birikmeli — asıl istediğimiz bu.
func TestScore_DifferentGroupsAccumulate(t *testing.T) {
	e := NewEngine()

	sameGroup := e.Score([]Observation{
		obs("username-check", false),
		obs("username-check", false),
		obs("username-check", false),
	})

	crossGroup := e.Score([]Observation{
		obs("username-check", false),
		obs("dns-whois", false),
		obs("crtsh", false),
	})

	if crossGroup.Value <= sameGroup.Value {
		t.Errorf("3 farklı grup, 3 aynı gruptan daha güçlü olmalı: %d vs %d",
			crossGroup.Value, sameGroup.Value)
	}
	if crossGroup.IndependentGroups != 3 {
		t.Errorf("3 bağımsız grup bekleniyordu: %d", crossGroup.IndependentGroups)
	}
}

// Sistemin kendi ürettiği tahminler kanıt sayılmamalı.
func TestScore_DerivedGuessesCarryAlmostNoWeight(t *testing.T) {
	e := NewEngine()

	guesses := make([]Observation, 10)
	for i := range guesses {
		guesses[i] = obs("name-generator", false)
	}
	guessScore := e.Score(guesses)

	single := e.Score([]Observation{obs("dns-whois", false)})

	if guessScore.Value >= single.Value {
		t.Errorf("10 tahmin, 1 gerçek gözlemden güçlü olmamalı: %d vs %d",
			guessScore.Value, single.Value)
	}
	if guessScore.Value > 35 {
		t.Errorf("yalnızca tahminlere dayanan puan düşük olmalı: %d", guessScore.Value)
	}
}

// Profil verisi taşıyan gözlem, yalnızca "HTTP 200"den güçlü olmalı.
func TestScore_ProfileEvidenceStrengthens(t *testing.T) {
	e := NewEngine()

	bare := e.Score([]Observation{obs("username-check", false)})
	rich := e.Score([]Observation{obs("username-check", true)})

	if rich.Value <= bare.Value {
		t.Errorf("profil verisi puanı artırmalı: çıplak=%d zengin=%d",
			bare.Value, rich.Value)
	}
}

// Şüpheli işaretli sonuçlar KARŞI kanıttır, puanı düşürmelidir.
func TestScore_SuspectResultsLowerConfidence(t *testing.T) {
	e := NewEngine()

	clean := e.Score([]Observation{obs("dns-whois", false)})

	withSuspect := e.Score([]Observation{
		obs("dns-whois", false),
		{Source: "username-check", Group: GroupPresence, Suspect: true},
		{Source: "username-check", Group: GroupPresence, Suspect: true},
	})

	if withSuspect.Value >= clean.Value {
		t.Errorf("şüpheli sonuçlar puanı düşürmeli: temiz=%d şüpheli=%d",
			clean.Value, withSuspect.Value)
	}
}

// Varyant eşleşmesi tam eşleşmeden zayıf sayılmalı.
func TestScore_VariantMatchIsWeaker(t *testing.T) {
	e := NewEngine()

	exact := e.Score([]Observation{
		{Source: "username-check", Group: GroupPresence, HasProfileEvidence: true},
	})
	variant := e.Score([]Observation{
		{Source: "username-check", Group: GroupPresence, HasProfileEvidence: true, VariantMatch: true},
	})

	if variant.Value >= exact.Value {
		t.Errorf("varyant eşleşme daha zayıf olmalı: tam=%d varyant=%d",
			exact.Value, variant.Value)
	}
}

func TestScore_EmptyAndBounds(t *testing.T) {
	e := NewEngine()

	if got := e.Score(nil); got.Value != 0 {
		t.Errorf("kanıtsız puan 0 olmalı: %d", got.Value)
	}

	// Çok sayıda güçlü, çeşitli kanıt bile 100'ü aşmamalı.
	var lots []Observation
	for _, src := range []string{"social-profile", "dns-whois", "crtsh", "shodan-internetdb",
		"virustotal", "wayback", "web-scraper", "hunter"} {
		for i := 0; i < 5; i++ {
			lots = append(lots, obs(src, true))
		}
	}
	got := e.Score(lots)
	if got.Value < 0 || got.Value > 100 {
		t.Errorf("puan 0-100 aralığında olmalı: %d", got.Value)
	}
}

// Puan ASLA kalibre edilmiş gibi sunulmamalı — ağırlıklar mühendislik tahmini.
func TestScore_NeverClaimsCalibration(t *testing.T) {
	got := NewEngine().Score([]Observation{obs("dns-whois", true)})
	if got.Calibrated {
		t.Error("ölçülmüş doğrulama seti yokken Calibrated true olamaz")
	}
	if !contains(got.Explanation, "kalibre edilmemiş") {
		t.Errorf("açıklama kalibrasyon durumunu belirtmeli: %q", got.Explanation)
	}
}

func TestGroupOf(t *testing.T) {
	cases := map[string]CueGroup{
		"username-check":    GroupPresence,
		"social-profile":    GroupProfile,
		"dns-whois":         GroupDNS,
		"shodan-internetdb": GroupInfraScan,
		"name-generator":    GroupDerived,
		"bio-extraction":    GroupDerived,
		"bilinmeyen-şey":    GroupUnknown,
		"":                  GroupUnknown,
	}
	for src, want := range cases {
		if got := GroupOf(src); got != want {
			t.Errorf("GroupOf(%q) = %q, beklenen %q", src, got, want)
		}
	}
}

// Sönümleme matematiğinin kendisi.
func TestDampGroup(t *testing.T) {
	// Tek katkı: olduğu gibi
	if got := dampGroup([]float64{1.0}, 0.2); got != 1.0 {
		t.Errorf("tek katkı sönümlenmemeli: %.2f", got)
	}

	// max + α·kalan = 1.0 + 0.2*(1.0+1.0) = 1.4 (tavanın altında)
	if got := dampGroup([]float64{1.0, 1.0, 1.0}, 0.2); !approx(got, 1.4) {
		t.Errorf("beklenen 1.40, alınan %.2f", got)
	}

	// Çok sayıda gözlem tavana takılmalı: 1.0 * groupCeiling = 2.0
	many := make([]float64, 50)
	for i := range many {
		many[i] = 1.0
	}
	if got := dampGroup(many, 0.2); !approx(got, groupCeiling) {
		t.Errorf("tavan uygulanmalıydı, beklenen %.2f, alınan %.2f", groupCeiling, got)
	}

	// Negatif katkılar sönümlenmeden tam uygulanır.
	if got := dampGroup([]float64{1.0, -0.25, -0.25}, 0.2); !approx(got, 0.5) {
		t.Errorf("beklenen 0.50, alınan %.2f", got)
	}

	// Yalnızca negatifler
	if got := dampGroup([]float64{-0.25, -0.25}, 0.2); !approx(got, -0.5) {
		t.Errorf("beklenen -0.50, alınan %.2f", got)
	}

	if got := dampGroup(nil, 0.2); got != 0 {
		t.Errorf("boş girdi 0 dönmeli: %.2f", got)
	}
}

func approx(a, b float64) bool { return a-b < 0.001 && b-a < 0.001 }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
