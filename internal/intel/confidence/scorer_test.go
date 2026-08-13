package confidence

import (
	"testing"
	"time"
)

func TestScorer_HighConfidence(t *testing.T) {
	s := NewScorer()
	score := s.Calculate(ScoringMetadata{
		SourceCount:       5,
		SourceReliability: 0.95,
		FindingAge:        time.Now().Add(-30 * time.Minute),
		CrossMatches:      3,
		TotalSources:      5,
	})

	if score.Value < 80 {
		t.Errorf("expected high confidence (≥80), got %d", score.Value)
	}
	if len(score.Factors) != 4 {
		t.Errorf("expected 4 factors, got %d", len(score.Factors))
	}
}

func TestScorer_LowConfidence(t *testing.T) {
	s := NewScorer()
	score := s.Calculate(ScoringMetadata{
		SourceCount:       1,
		SourceReliability: 0.3,
		FindingAge:        time.Now().Add(-120 * 24 * time.Hour),
		CrossMatches:      0,
		TotalSources:      1,
	})

	if score.Value > 40 {
		t.Errorf("expected low confidence (≤40), got %d", score.Value)
	}
}

func TestScorer_MediumConfidence(t *testing.T) {
	s := NewScorer()
	score := s.Calculate(ScoringMetadata{
		SourceCount:       2,
		SourceReliability: 0.7,
		FindingAge:        time.Now().Add(-5 * 24 * time.Hour),
		CrossMatches:      1,
		TotalSources:      3,
	})

	if score.Value < 40 || score.Value > 80 {
		t.Errorf("expected medium confidence (40-80), got %d", score.Value)
	}
}

func TestScorer_Explanation(t *testing.T) {
	s := NewScorer()
	score := s.Calculate(ScoringMetadata{
		SourceCount:       3,
		SourceReliability: 0.8,
		FindingAge:        time.Now(),
		CrossMatches:      2,
	})

	if score.Explanation == "" {
		t.Error("expected non-empty explanation")
	}
}

func TestScorer_BoundaryValues(t *testing.T) {
	s := NewScorer()

	// Tüm minimum
	score := s.Calculate(ScoringMetadata{})
	if score.Value < 0 || score.Value > 100 {
		t.Errorf("score out of range: %d", score.Value)
	}

	// Tüm maximum
	score = s.Calculate(ScoringMetadata{
		SourceCount:       10,
		SourceReliability: 1.5, // >1 → clamp
		FindingAge:        time.Now(),
		CrossMatches:      10,
		TotalSources:      10,
	})
	if score.Value < 0 || score.Value > 100 {
		t.Errorf("score out of range: %d", score.Value)
	}
}

func TestScorer_CustomWeights(t *testing.T) {
	w := Weights{
		SourceCount:       0.5,
		SourceReliability: 0.5,
		DataFreshness:     0,
		CrossConsistency:  0,
	}
	s := NewScorerWithWeights(w)
	score := s.Calculate(ScoringMetadata{
		SourceCount:       5,
		SourceReliability: 1.0,
	})

	if score.Value != 100 {
		t.Errorf("expected 100 with perfect source factors, got %d", score.Value)
	}
}

func TestDefaultWeights_SumToOne(t *testing.T) {
	w := DefaultWeights()
	sum := w.SourceCount + w.SourceReliability + w.DataFreshness + w.CrossConsistency
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("weights should sum to ~1.0, got %f", sum)
	}
}
