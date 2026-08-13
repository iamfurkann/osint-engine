package orchestrator

import (
	"context"
	"fmt"

	"github.com/iamfurkann/osint-engine/internal/intel/correlation"
	"github.com/iamfurkann/osint-engine/internal/intel/graph"
	"github.com/iamfurkann/osint-engine/internal/intel/resolution"
)

// ActiveInvestigations, tüm araştırmaların güncel durumunu döndürür.
func (o *Orchestrator) ActiveInvestigations() []Progress {
	o.trackerMu.RLock()
	defer o.trackerMu.RUnlock()

	var list []Progress
	for id, tracker := range o.trackers {
		completed := tracker.completed + tracker.failed
		percent := 0.0
		if tracker.total > 0 {
			percent = float64(completed) / float64(tracker.total) * 100
		}
		list = append(list, Progress{
			InvestigationID: id,
			Total:           tracker.total,
			Completed:       tracker.completed,
			Failed:          tracker.failed,
			Percent:         percent,
		})
	}
	return list
}

// BuildGraph, bir araştırma için elde edilen bulguları çeker,
// entity'lere çözümler, korelasyon yapar ve bir Graph döner.
func (o *Orchestrator) BuildGraph(ctx context.Context, investigationID string) (*graph.Graph, []*resolution.Entity, []correlation.Correlation, error) {
	// 1. Veritabanından (veya repo'dan) bulguları al
	findings, err := o.findingRepo.GetByInvestigationID(ctx, investigationID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("bulgular alınamadı: %w", err)
	}

	// 2. Bulguları Entity'lere çözümle
	resolver := resolution.NewResolver()
	var inputs []resolution.FindingInput

	for _, f := range findings {
		inputs = append(inputs, resolution.FindingInput{
			ID:     f.ID,
			Type:   string(f.Type),
			Value:  f.Value,
			Source: f.FoundBy,
			// Context, connector'ın topladığı gerçek istihbarat verisini
			// taşır (ad, konum, bio, takipçi, hesap yaşı). Önceden burada
			// aktarılmıyordu ve tüm bu veri kayboluyordu.
			Context: f.Context,
		})
	}

	// Toplu çözümle
	entities := resolver.Resolve(inputs)

	// Kanıt motoruyla güven puanı hesapla.
	//
	// Bu adım olmadan Entity.Confidence hiçbir yerde atanmıyordu ve üretilen
	// HER raporda güven "0%" yazıyordu.
	scoreEntities(entities, findings)

	// 3. Entity'ler arası korelasyon kur
	corrEngine := correlation.NewEngine()
	correlations := corrEngine.Correlate(entities)

	// 4. Grafı oluştur
	g := graph.NewGraph()
	for _, ent := range entities {
		g.AddNode(graph.Node{
			ID:         ent.ID,
			Type:       string(ent.Type),
			Value:      ent.PrimaryValue,
			Confidence: ent.Confidence,
		})
	}

	for _, corr := range correlations {
		g.AddEdge(graph.Edge{
			Source:     corr.SourceEntityID,
			Target:     corr.TargetEntityID,
			Type:       string(corr.Type),
			Confidence: corr.Confidence,
			Evidence:   corr.Evidence,
		})
	}

	return g, entities, correlations, nil
}
