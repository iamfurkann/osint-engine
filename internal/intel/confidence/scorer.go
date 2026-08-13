package confidence

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Score, bir bulgunun güven puanını ve puan bileşenlerini temsil eder.
type Score struct {
	Value       int      `json:"value"`       // 0-100 arası güven puanı
	Factors     []Factor `json:"factors"`     // Puanı oluşturan faktörler
	Explanation string   `json:"explanation"` // İnsan-okunur açıklama
}

// Factor, güven puanına katkıda bulunan tek bir faktördür.
type Factor struct {
	Name   string  `json:"name"`   // Faktör adı
	Weight float64 `json:"weight"` // Ağırlık (0-1)
	Score  int     `json:"score"`  // Bu faktörün puanı (0-100)
}

// ScoringMetadata, güven puanı hesaplaması için gereken ek bilgilerdir.
type ScoringMetadata struct {
	SourceCount       int       // Aynı bulguyu doğrulayan bağımsız kaynak sayısı
	SourceReliability float64   // Kaynağın güvenilirlik oranı (0-1)
	FindingAge        time.Time // Bulgunun oluşturulma zamanı
	CrossMatches      int       // Diğer bulgularla çapraz eşleşme sayısı
	TotalSources      int       // Araştırmadaki toplam kaynak sayısı
}

// Weights, faktör ağırlıklarını tanımlar.
type Weights struct {
	SourceCount       float64
	SourceReliability float64
	DataFreshness     float64
	CrossConsistency  float64
}

// DefaultWeights, varsayılan faktör ağırlıklarını döndürür.
func DefaultWeights() Weights {
	return Weights{
		SourceCount:       0.30, // Bağımsız kaynak sayısı en önemli
		SourceReliability: 0.25, // Kaynağın güvenilirliği
		DataFreshness:     0.20, // Verinin tazeliği
		CrossConsistency:  0.25, // Çapraz tutarlılık
	}
}

// Scorer, güven puanı hesaplayan motordir.
type Scorer struct {
	weights Weights
}

// NewScorer, varsayılan ağırlıklarla scorer oluşturur.
func NewScorer() *Scorer {
	return &Scorer{weights: DefaultWeights()}
}

// NewScorerWithWeights, özel ağırlıklarla scorer oluşturur.
func NewScorerWithWeights(w Weights) *Scorer {
	return &Scorer{weights: w}
}

// Calculate, verilen metadata'ya göre güven puanı hesaplar.
func (s *Scorer) Calculate(meta ScoringMetadata) Score {
	factors := make([]Factor, 0, 4)

	// 1. Kaynak Sayısı Faktörü
	sourceScore := s.calcSourceCountScore(meta.SourceCount)
	factors = append(factors, Factor{
		Name:   "source_count",
		Weight: s.weights.SourceCount,
		Score:  sourceScore,
	})

	// 2. Kaynak Güvenilirliği Faktörü
	reliabilityScore := s.calcReliabilityScore(meta.SourceReliability)
	factors = append(factors, Factor{
		Name:   "source_reliability",
		Weight: s.weights.SourceReliability,
		Score:  reliabilityScore,
	})

	// 3. Veri Tazeliği Faktörü
	freshnessScore := s.calcFreshnessScore(meta.FindingAge)
	factors = append(factors, Factor{
		Name:   "data_freshness",
		Weight: s.weights.DataFreshness,
		Score:  freshnessScore,
	})

	// 4. Çapraz Tutarlılık Faktörü
	crossScore := s.calcCrossConsistencyScore(meta.CrossMatches)
	factors = append(factors, Factor{
		Name:   "cross_consistency",
		Weight: s.weights.CrossConsistency,
		Score:  crossScore,
	})

	// Ağırlıklı toplam
	total := 0.0
	for _, f := range factors {
		total += float64(f.Score) * f.Weight
	}

	value := int(math.Round(total))
	if value > 100 {
		value = 100
	}
	if value < 0 {
		value = 0
	}

	return Score{
		Value:       value,
		Factors:     factors,
		Explanation: s.explain(value, factors),
	}
}

// calcSourceCountScore: Kaç bağımsız kaynak doğruladı?
// 1 kaynak=40, 2=65, 3+=85, 5+=100
//
// Not: Eskiden ikinci bir 'total' parametresi alıyor, onu normalize ediyor ve
// SONRA HİÇ KULLANMIYORDU (ineffassign). Ölü parametre kaldırıldı.
// ScoringMetadata.TotalSources alanı şimdilik yerinde bırakıldı; oransal
// skorlama Evidence Engine tasarımında yeniden ele alınacak.
func (s *Scorer) calcSourceCountScore(count int) int {
	if count <= 0 {
		return 20
	}

	switch {
	case count >= 5:
		return 100
	case count >= 3:
		return 85
	case count >= 2:
		return 65
	default:
		return 40
	}
}

// calcReliabilityScore: Kaynak ne kadar güvenilir? (0-1 → 0-100)
func (s *Scorer) calcReliabilityScore(reliability float64) int {
	if reliability <= 0 {
		return 50 // Bilinmeyen kaynak → nötr
	}
	if reliability > 1 {
		reliability = 1
	}
	return int(reliability * 100)
}

// calcFreshnessScore: Veri ne kadar taze?
// <1 saat=100, <1 gün=85, <7 gün=70, <30 gün=50, <90 gün=30, >90 gün=15
func (s *Scorer) calcFreshnessScore(findingTime time.Time) int {
	if findingTime.IsZero() {
		return 50 // Bilinmeyen zaman → nötr
	}

	age := time.Since(findingTime)
	switch {
	case age < 1*time.Hour:
		return 100
	case age < 24*time.Hour:
		return 85
	case age < 7*24*time.Hour:
		return 70
	case age < 30*24*time.Hour:
		return 50
	case age < 90*24*time.Hour:
		return 30
	default:
		return 15
	}
}

// calcCrossConsistencyScore: Diğer bulgularla ne kadar tutarlı?
// 0 eşleşme=30, 1=50, 2=70, 3+=90
func (s *Scorer) calcCrossConsistencyScore(matches int) int {
	switch {
	case matches >= 3:
		return 90
	case matches >= 2:
		return 70
	case matches >= 1:
		return 50
	default:
		return 30
	}
}

// explain, puanı insan-okunur formata dönüştürür.
func (s *Scorer) explain(value int, factors []Factor) string {
	var level string
	switch {
	case value >= 80:
		level = "Yüksek güven"
	case value >= 60:
		level = "Orta güven"
	case value >= 40:
		level = "Düşük güven"
	default:
		level = "Çok düşük güven"
	}

	parts := make([]string, 0, len(factors))
	for _, f := range factors {
		parts = append(parts, fmt.Sprintf("%s=%d", f.Name, f.Score))
	}

	return fmt.Sprintf("%s (%d/100) [%s]", level, value, strings.Join(parts, ", "))
}
