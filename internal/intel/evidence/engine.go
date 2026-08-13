package evidence

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// dampingFactor, AYNI gruptaki ek kanıtların katkı oranıdır.
//
// Bir gruptan gelen n gözlemin toplam katkısı:
//
//	max + α · Σ(kalanlar)      (α = dampingFactor)
//
// Böylece tek bir taramanın 40 sonucu 40 kat kanıt sayılmaz. α tam sıfır
// değil, çünkü aynı grup içindeki tekrar da bir miktar bilgi taşır — ama
// bağımsız bir kaynağın getirdiği kadar değil.
const dampingFactor = 0.20

// priorLogOdds, hiçbir kanıt yokken başlangıç noktasıdır.
// Negatif: varsayılan olarak bir iddiaya inanmayız.
const priorLogOdds = -1.2

// Observation, tek bir bulgunun kanıt olarak temsilidir.
type Observation struct {
	FindingID string
	Source    string   // Üreten connector
	Group     CueGroup // Bağımsızlık sınıfı
	Kind      string   // Bulgu tipi

	// HasProfileEvidence, sayfadan gerçek profil verisi (ad/bio/avatar)
	// okunabildiğini gösterir. Yalnızca "HTTP 200" çok daha zayıf bir iddiadır.
	HasProfileEvidence bool

	// Suspect, connector'ın sonucu şüpheli işaretlediğini gösterir
	// (platform var olmayan adlara da "bulundu" diyor, bot koruması vb.).
	Suspect bool

	// VariantMatch, sonucun aranan değerin TAM eşleşmesi olmadığını gösterir.
	VariantMatch bool
}

// contribution, tek bir gözlemin sönümleme öncesi log-odds katkısıdır.
func (o Observation) contribution() float64 {
	w := o.Group.Weight()

	if o.HasProfileEvidence {
		w *= 1.6 // sayfa gerçek kimlik yayınlıyor
	}
	if o.VariantMatch {
		w *= 0.5 // aranan adın kendisi değil, varyantı
	}
	if o.Suspect {
		// Şüpheli sonuç kanıt DEĞİL, hafif karşı kanıttır: connector bize
		// açıkça "bu platform ayırt edemiyor" diyor.
		return -0.25
	}
	return w
}

// GroupBreakdown, bir grubun toplam katkısını ve ayrıntısını taşır.
type GroupBreakdown struct {
	Group        CueGroup
	Label        string
	Observations int
	Raw          float64 // sönümleme öncesi toplam
	Damped       float64 // sönümleme sonrası (skora giren)
	Sources      []string
}

// Score, hesaplanmış güven puanı ve gerekçesidir.
type Score struct {
	// Value, 0-100 aralığında güven puanıdır.
	Value int

	// LogOdds, ham birikim. Hata ayıklama ve ileride kalibrasyon için.
	LogOdds float64

	// Groups, katkıların grup bazında dökümü — raporun "kanıt" bölümü.
	Groups []GroupBreakdown

	// IndependentGroups, katkı sağlayan farklı bağımsızlık sınıfı sayısı.
	// Tek gruptan gelen yüksek puan, çok gruptan gelenle aynı şey değildir.
	IndependentGroups int

	// Calibrated, puanın ÖLÇÜLMÜŞ bir doğrulama setine dayanıp dayanmadığı.
	//
	// Şu an her zaman false: ağırlıklar mühendislik tahminidir. Bu alan
	// bilerek var — kalibre edilmemiş bir yüzdeyi ölçülmüş gibi sunmak
	// araştırmacıyı yanıltır ve sistemin verebileceği en büyük zarardır.
	Calibrated bool

	Explanation string
}

// Engine, gözlemleri güven puanına çeviren motordur.
type Engine struct {
	damping float64
	prior   float64
}

// NewEngine, varsayılan parametrelerle motor oluşturur.
func NewEngine() *Engine {
	return &Engine{damping: dampingFactor, prior: priorLogOdds}
}

// Score, bir varlığa ait gözlemlerden güven puanı hesaplar.
func (e *Engine) Score(observations []Observation) Score {
	if len(observations) == 0 {
		return Score{Value: 0, Explanation: "kanıt yok"}
	}

	// 1. Gözlemleri bağımsızlık sınıfına göre topla.
	byGroup := make(map[CueGroup][]Observation)
	for _, o := range observations {
		byGroup[o.Group] = append(byGroup[o.Group], o)
	}

	// 2. Her grup içinde SÖNÜMLE, sonra gruplar arası TOPLA.
	logOdds := e.prior
	breakdowns := make([]GroupBreakdown, 0, len(byGroup))
	independent := 0

	for group, obs := range byGroup {
		contributions := make([]float64, 0, len(obs))
		sourceSet := make(map[string]bool)
		raw := 0.0

		for _, o := range obs {
			c := o.contribution()
			contributions = append(contributions, c)
			raw += c
			if o.Source != "" {
				sourceSet[o.Source] = true
			}
		}

		damped := dampGroup(contributions, e.damping)
		logOdds += damped

		if damped > 0 {
			independent++
		}

		sources := make([]string, 0, len(sourceSet))
		for s := range sourceSet {
			sources = append(sources, s)
		}
		sort.Strings(sources)

		breakdowns = append(breakdowns, GroupBreakdown{
			Group:        group,
			Label:        group.Label(),
			Observations: len(obs),
			Raw:          raw,
			Damped:       damped,
			Sources:      sources,
		})
	}

	// En güçlü katkı üstte.
	sort.SliceStable(breakdowns, func(i, j int) bool {
		return breakdowns[i].Damped > breakdowns[j].Damped
	})

	value := int(math.Round(sigmoid(logOdds) * 100))
	value = clamp(value, 0, 100)

	return Score{
		Value:             value,
		LogOdds:           logOdds,
		Groups:            breakdowns,
		IndependentGroups: independent,
		Calibrated:        false,
		Explanation:       explain(value, independent, breakdowns),
	}
}

// groupCeiling, bir grubun katkısının en güçlü tek kanıtının kaç katına
// kadar çıkabileceğidir.
//
// Bu tavan olmadan sönümleme YETMİYOR: α·Σ(kalan) ifadesi gözlem sayısıyla
// DOĞRUSAL büyüyor. Ölçüldü — tek bir tarama turundan gelen 40 sonuç,
// α=0.20 ile bile %94 güven üretiyordu. Oysa 40 platformda aynı kullanıcı
// adının bulunması tek bir taramadır ve tek bir iddiadır.
//
// Tavanla birlikte bir bağımsızlık sınıfı, kaç gözlem içerirse içersin,
// skoru tek başına domine edemez. Yüksek güven ancak FARKLI gruplardan
// gelen kanıtla mümkün olur — motorun bütün amacı da budur.
const groupCeiling = 2.0

// dampGroup, bir grubun katkılarını sönümleyerek tek bir değere indirir.
//
// En güçlü kanıt tam sayılır, kalanlar α oranında katkı yapar ve grubun
// toplamı groupCeiling ile sınırlanır.
//
// Negatif katkılar (şüpheli sonuçlar) sönümlenmeden ve tavansız uygulanır —
// karşı kanıtı yumuşatmak, sahte kesinlik üretmenin bir başka yoludur.
func dampGroup(contributions []float64, alpha float64) float64 {
	if len(contributions) == 0 {
		return 0
	}

	best := math.Inf(-1)
	restSum := 0.0
	negatives := 0.0

	for _, c := range contributions {
		if c < 0 {
			negatives += c
			continue
		}
		if c > best {
			if !math.IsInf(best, -1) {
				restSum += best
			}
			best = c
		} else {
			restSum += c
		}
	}

	if math.IsInf(best, -1) {
		return negatives // yalnızca negatif katkılar vardı
	}

	positive := math.Min(best+alpha*restSum, best*groupCeiling)
	return positive + negatives
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func explain(value, independent int, groups []GroupBreakdown) string {
	var level string
	switch {
	case value >= 80:
		level = "Yüksek"
	case value >= 60:
		level = "Orta"
	case value >= 35:
		level = "Düşük"
	default:
		level = "Çok düşük"
	}

	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		if g.Observations > 1 {
			parts = append(parts, fmt.Sprintf("%s (%d gözlem → %+.2f)",
				g.Label, g.Observations, g.Damped))
		} else {
			parts = append(parts, fmt.Sprintf("%s (%+.2f)", g.Label, g.Damped))
		}
	}

	return fmt.Sprintf("%s güven — %d bağımsız kaynak grubu · %s · kalibre edilmemiş",
		level, independent, strings.Join(parts, ", "))
}
