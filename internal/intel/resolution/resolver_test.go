package resolution

import (
	"testing"
)

func TestResolver_DeterministicMerge(t *testing.T) {
	r := NewResolver()

	// Aynı e-posta iki farklı connector'dan
	findings := []FindingInput{
		{ID: "f1", Type: "email", Value: "admin@example.com", Source: "hibp"},
		{ID: "f2", Type: "email", Value: "admin@example.com", Source: "hunter"},
		{ID: "f3", Type: "email", Value: "user@other.com", Source: "hibp"},
	}

	entities := r.Resolve(findings)

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities (1 merged email + 1 separate), got %d", len(entities))
	}

	// admin@example.com entity'si 2 finding ve 2 source içermeli
	adminEntity, ok := r.GetByValue("admin@example.com", "email")
	if !ok {
		t.Fatal("admin@example.com entity not found")
	}
	if len(adminEntity.FindingIDs) != 2 {
		t.Errorf("expected 2 findings, got %d", len(adminEntity.FindingIDs))
	}
	if len(adminEntity.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(adminEntity.Sources))
	}
}

func TestResolver_CaseInsensitive(t *testing.T) {
	r := NewResolver()

	findings := []FindingInput{
		{ID: "f1", Type: "email", Value: "Admin@Example.COM", Source: "connector-1"},
		{ID: "f2", Type: "email", Value: "admin@example.com", Source: "connector-2"},
	}

	entities := r.Resolve(findings)

	if len(entities) != 1 {
		t.Errorf("expected 1 entity (case-insensitive merge), got %d", len(entities))
	}
}

func TestResolver_DifferentTypes(t *testing.T) {
	r := NewResolver()

	findings := []FindingInput{
		{ID: "f1", Type: "email", Value: "test@example.com", Source: "src1"},
		{ID: "f2", Type: "domain", Value: "example.com", Source: "src2"},
		{ID: "f3", Type: "ip", Value: "93.184.216.34", Source: "src3"},
	}

	entities := r.Resolve(findings)

	if len(entities) != 3 {
		t.Errorf("expected 3 entities (different types), got %d", len(entities))
	}
}

func TestResolver_ManualMerge(t *testing.T) {
	r := NewResolver()

	r.Resolve([]FindingInput{
		{ID: "f1", Type: "email", Value: "a@x.com", Source: "src1"},
		{ID: "f2", Type: "email", Value: "b@x.com", Source: "src2"},
	})

	entities := r.AllEntities()
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}

	err := r.MergeEntities(entities[0].ID, entities[1].ID)
	if err != nil {
		t.Fatalf("MergeEntities failed: %v", err)
	}

	merged := r.AllEntities()
	if len(merged) != 1 {
		t.Errorf("expected 1 entity after merge, got %d", len(merged))
	}
	if len(merged[0].Aliases) < 1 {
		t.Error("expected at least 1 alias after merge")
	}

	// Merge geçmişi
	history := r.MergeHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 merge event, got %d", len(history))
	}
}

func TestResolver_MergeNonexistent(t *testing.T) {
	r := NewResolver()
	err := r.MergeEntities("fake-1", "fake-2")
	if err == nil {
		t.Fatal("expected error for nonexistent entities")
	}
}

func TestResolver_GetByValue(t *testing.T) {
	r := NewResolver()
	r.Resolve([]FindingInput{
		{ID: "f1", Type: "ip", Value: "8.8.8.8", Source: "dns"},
	})

	e, ok := r.GetByValue("8.8.8.8", "ip")
	if !ok {
		t.Fatal("expected to find entity by value")
	}
	if e.PrimaryValue != "8.8.8.8" {
		t.Errorf("expected '8.8.8.8', got %q", e.PrimaryValue)
	}

	_, ok = r.GetByValue("nonexistent", "ip")
	if ok {
		t.Error("expected no entity for nonexistent value")
	}
}
