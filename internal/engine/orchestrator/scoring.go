package orchestrator

import (
	"encoding/json"
	"strings"

	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/intel/evidence"
	"github.com/iamfurkann/osint-engine/internal/intel/resolution"
)

// scoreEntities, her varlık için kanıt motoruyla güven puanı hesaplar ve
// gerekçesini Attributes'a yazar.
//
// Önceden Entity.Confidence hiçbir yerde atanmıyordu; resolveOne onu sıfır
// bırakıyor, graf düğümüne sıfır kopyalanıyor ve rapor "0%" basıyordu.
// Yazılmış bir skorlama paketi vardı ama hiçbir yerden çağrılmıyordu.
func scoreEntities(entities []*resolution.Entity, findings []*domain.Finding) {
	if len(entities) == 0 {
		return
	}

	// Bulguları ID ile indeksle — entity yalnızca FindingID listesi tutuyor.
	byID := make(map[string]*domain.Finding, len(findings))
	for _, f := range findings {
		byID[f.ID] = f
	}

	engine := evidence.NewEngine()

	for _, ent := range entities {
		observations := make([]evidence.Observation, 0, len(ent.FindingIDs))
		for _, fid := range ent.FindingIDs {
			f, ok := byID[fid]
			if !ok {
				continue
			}
			observations = append(observations, observationFromFinding(f))
		}

		score := engine.Score(observations)
		ent.Confidence = score.Value

		if ent.Attributes == nil {
			ent.Attributes = make(map[string]any)
		}
		ent.Attributes["confidence_basis"] = score.Explanation
		ent.Attributes["independent_sources"] = score.IndependentGroups

		// Kalibrasyon harness'ı için ham birikim ve katkı veren gruplar.
		// Puanın kendisi sigmoid'den geçmiş hâlidir; Platt ölçekleme ham
		// log-odds üzerinde çalışmak zorundadır.
		ent.Attributes["confidence_logodds"] = score.LogOdds
		groups := make([]string, 0, len(score.Groups))
		for _, g := range score.Groups {
			groups = append(groups, string(g.Group))
		}
		ent.Attributes["confidence_groups"] = groups
	}
}

// observationFromFinding, bir bulguyu kanıt motorunun anlayacağı gözleme çevirir.
func observationFromFinding(f *domain.Finding) evidence.Observation {
	o := evidence.Observation{
		FindingID: f.ID,
		Source:    f.FoundBy,
		Group:     evidence.GroupOf(f.FoundBy),
		Kind:      string(f.Type),
	}

	// Context içindeki niteliklerden kanıt gücü sinyallerini oku.
	var ctx map[string]any
	if f.Context != "" {
		if err := json.Unmarshal([]byte(f.Context), &ctx); err != nil {
			return o // bozuk JSON → yalnızca temel ağırlık
		}
	}

	// Profil verisi taşıyor mu? "HTTP 200" ile "sayfada gerçek kimlik var"
	// arasındaki fark budur.
	for _, k := range []string{"display_name", "bio", "avatar", "name", "profile_username"} {
		if s, ok := ctx[k].(string); ok && strings.TrimSpace(s) != "" {
			o.HasProfileEvidence = true
			break
		}
	}

	// Connector sonucu şüpheli işaretlemiş mi?
	if v, ok := ctx["verification"].(string); ok && strings.TrimSpace(v) != "" {
		o.Suspect = true
	}

	// Aranan değerin tam eşleşmesi mi, varyantı mı?
	if v, ok := ctx["match"].(string); ok && strings.Contains(v, "varyant") {
		o.VariantMatch = true
	}

	// Biyografiden çıkarılanlar sistemin kendi türetmesidir, gözlem değil.
	if v, ok := ctx["source"].(string); ok && v == "bio-extraction" {
		o.Group = evidence.GroupDerived
	}

	return o
}
