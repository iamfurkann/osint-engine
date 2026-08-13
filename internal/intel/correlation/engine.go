package correlation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iamfurkann/osint-engine/internal/intel/evidence"
	"github.com/iamfurkann/osint-engine/internal/intel/resolution"
)

// CorrelationType, korelasyon stratejisini belirtir.
type CorrelationType string

const (
	TypeDeterministic CorrelationType = "deterministic" // Aynı benzersiz değer
	TypeProbabilistic CorrelationType = "probabilistic" // Fuzzy benzerlik
	TypeRuleBased     CorrelationType = "rule_based"    // Yapılandırılabilir kural
)

// Correlation, iki entity arasında tespit edilen ilişkidir.
type Correlation struct {
	SourceEntityID string          `json:"source_entity_id"`
	TargetEntityID string          `json:"target_entity_id"`
	Type           CorrelationType `json:"type"`
	Confidence     int             `json:"confidence"` // 0-100
	Evidence       string          `json:"evidence"`   // İlişki kanıtı açıklaması
	Rule           string          `json:"rule"`       // Hangi kural tetikledi
}

// Rule, kural tabanlı korelasyon için yapılandırılabilir kuraldır.
type Rule struct {
	Name        string
	Description string
	SourceType  resolution.EntityType // Kaynak entity tipi
	TargetType  resolution.EntityType // Hedef entity tipi
	MatchFunc   func(source, target *resolution.Entity) (bool, int, string)
}

// Engine, entity'ler arasında korelasyon bulan motordir.
type Engine struct {
	rules []Rule
}

// NewEngine, varsayılan kurallarla korelasyon motoru oluşturur.
func NewEngine() *Engine {
	return &Engine{
		rules: defaultRules(),
	}
}

// AddRule, özel bir korelasyon kuralı ekler.
func (e *Engine) AddRule(rule Rule) {
	e.rules = append(e.rules, rule)
}

// Correlate, entity listesi üzerinde tüm korelasyon stratejilerini çalıştırır.
func (e *Engine) Correlate(entities []*resolution.Entity) []Correlation {
	var correlations []Correlation
	seen := make(map[string]bool)

	for i := 0; i < len(entities); i++ {
		for j := i + 1; j < len(entities); j++ {
			src := entities[i]
			tgt := entities[j]

			// Deterministic korelasyon
			if c, ok := e.deterministicMatch(src, tgt); ok {
				key := pairKey(src.ID, tgt.ID, string(c.Type))
				if !seen[key] {
					correlations = append(correlations, c)
					seen[key] = true
				}
			}

			// Kural tabanlı korelasyon
			for _, rule := range e.rules {
				if c, ok := e.ruleMatch(rule, src, tgt); ok {
					key := pairKey(src.ID, tgt.ID, rule.Name)
					if !seen[key] {
						correlations = append(correlations, c)
						seen[key] = true
					}
				}
			}
		}
	}

	return correlations
}

// deterministicMatch: birden çok BAĞIMSIZ kaynak grubunun aynı iki varlığı
// birlikte gördüğü durumlar.
//
// Bu fonksiyon eskiden "aynı connector buldu" diye her varlığı her varlığa
// bağlıyordu ve 70 güven puanı veriyordu. Sonucu canlı olarak ölçüldü:
// tek bir HTTP isteğinden 13 varlık çıkaran bir araştırma 78 "ilişki"
// raporladı — tam bir K₁₃ grafı, sıfır bilgi. Kenar sayısı ayrıca API
// katmanında çift sayılıp 156 olarak gösteriliyordu.
//
// Ortak köken bir İLİŞKİ DEĞİLDİR. İki port numarasının "ilişkili" olması,
// ikisini de aynı taramanın bulmuş olmasından çıkarılamaz. Bu bilgi zaten
// Entity.Sources alanında duruyor.
//
// Artık kenar yalnızca paylaşılan kaynaklar FARKLI bağımsızlık sınıflarına
// yayıldığında kuruluyor — o zaman gerçek bir çapraz doğrulama vardır.
func (e *Engine) deterministicMatch(src, tgt *resolution.Entity) (Correlation, bool) {
	sharedSources := sharedStrings(src.Sources, tgt.Sources)
	if len(sharedSources) == 0 {
		return Correlation{}, false
	}

	groups := make(map[evidence.CueGroup]bool, len(sharedSources))
	for _, s := range sharedSources {
		groups[evidence.GroupOf(s)] = true
	}

	// Tek bağımsızlık sınıfı → ortak köken, ilişki değil. Kenar kurulmaz.
	if len(groups) < 2 {
		return Correlation{}, false
	}

	// Güven, bağımsız grup sayısıyla artar — kaynak sayısıyla değil.
	confidence := 40 + len(groups)*15
	if confidence > 90 {
		confidence = 90
	}

	labels := make([]string, 0, len(groups))
	for g := range groups {
		labels = append(labels, g.Label())
	}
	sort.Strings(labels)

	return Correlation{
		SourceEntityID: src.ID,
		TargetEntityID: tgt.ID,
		Type:           TypeDeterministic,
		Confidence:     confidence,
		Evidence: fmt.Sprintf("%d bağımsız kaynak grubu birlikte gördü (%s): %s ↔ %s",
			len(groups), strings.Join(labels, ", "), src.PrimaryValue, tgt.PrimaryValue),
		Rule: "independent_corroboration",
	}, true
}

// ruleMatch: Kural tabanlı eşleşme.
func (e *Engine) ruleMatch(rule Rule, src, tgt *resolution.Entity) (Correlation, bool) {
	// Tip kontrolü (iki yönlü)
	srcMatch := (rule.SourceType == "" || src.Type == rule.SourceType) && (rule.TargetType == "" || tgt.Type == rule.TargetType)
	tgtMatch := (rule.SourceType == "" || tgt.Type == rule.SourceType) && (rule.TargetType == "" || src.Type == rule.TargetType)

	if !srcMatch && !tgtMatch {
		return Correlation{}, false
	}

	// Doğru yöndeki entity'leri seç
	s, t := src, tgt
	if tgtMatch && !srcMatch {
		s, t = tgt, src
	}

	matched, confidence, evidence := rule.MatchFunc(s, t)
	if !matched {
		return Correlation{}, false
	}

	return Correlation{
		SourceEntityID: s.ID,
		TargetEntityID: t.ID,
		Type:           TypeRuleBased,
		Confidence:     confidence,
		Evidence:       evidence,
		Rule:           rule.Name,
	}, true
}

// defaultRules, varsayılan korelasyon kurallarını döndürür.
func defaultRules() []Rule {
	return []Rule{
		{
			Name:        "email_domain",
			Description: "E-posta adresi domain ile eşleşir",
			// Tip filtresi bilerek BOŞ.
			//
			// Kurallar EntityEmail/EntityDomain tiplerine bağlıydı, ama
			// connector'lar serbest biçimli tipler üretiyor: "hostname",
			// "dns_record", "certificate", "shodan_host"... Sonuç olarak
			// kurallar gerçek veride neredeyse HİÇ ateşlenmiyordu.
			// Karar artık MatchFunc'ta, DEĞERİN kendisine bakılarak veriliyor.
			MatchFunc: func(src, tgt *resolution.Entity) (bool, int, string) {
				// admin@example.com → example.com
				parts := strings.SplitN(src.PrimaryValue, "@", 2)
				if len(parts) != 2 {
					return false, 0, ""
				}
				emailDomain := strings.ToLower(parts[1])
				targetDomain := strings.ToLower(tgt.PrimaryValue)

				if emailDomain == targetDomain {
					return true, 90, fmt.Sprintf("E-posta domaini eşleşiyor: %s → %s", src.PrimaryValue, tgt.PrimaryValue)
				}
				return false, 0, ""
			},
		},
		{
			Name:        "subdomain_parent",
			Description: "Alt domain üst domain ile eşleşir",
			// Tip filtresi bilerek boş — bkz. email_domain kuralı.
			MatchFunc: func(src, tgt *resolution.Entity) (bool, int, string) {
				s := strings.ToLower(src.PrimaryValue)
				t := strings.ToLower(tgt.PrimaryValue)

				// Tip filtresi kalktığı için burada eleme yapılmalı:
				// port numaraları, CPE dizeleri ve URL'ler alan adı değildir.
				if !looksLikeDomain(s) || !looksLikeDomain(t) {
					return false, 0, ""
				}
				if s == t {
					return false, 0, ""
				}
				if strings.HasSuffix(s, "."+t) {
					return true, 85, fmt.Sprintf("Alt domain ilişkisi: %s → %s", s, t)
				}
				if strings.HasSuffix(t, "."+s) {
					return true, 85, fmt.Sprintf("Alt domain ilişkisi: %s → %s", t, s)
				}
				return false, 0, ""
			},
		},
	}
}

// --- Yardımcılar ---

func sharedStrings(a, b []string) []string {
	set := make(map[string]bool)
	for _, s := range a {
		set[s] = true
	}
	var shared []string
	for _, s := range b {
		if set[s] {
			shared = append(shared, s)
		}
	}
	return shared
}

func pairKey(a, b, rule string) string {
	if a > b {
		a, b = b, a
	}
	return a + ":" + b + ":" + rule
}

// looksLikeDomain, bir değerin alan adı biçiminde olup olmadığını söyler.
// Kural tipe değil değere baktığı için gereklidir.
func looksLikeDomain(v string) bool {
	if v == "" || len(v) > 253 {
		return false
	}
	if strings.ContainsAny(v, " /@:") || !strings.Contains(v, ".") {
		return false
	}
	// Sadece rakam ve noktadan oluşuyorsa IP'dir, alan adı değil.
	onlyDigits := true
	for _, r := range v {
		if r != '.' && (r < '0' || r > '9') {
			onlyDigits = false
			break
		}
	}
	return !onlyDigits
}
