package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/testutil"
)

func TestFindingRepository(t *testing.T) {
	database := testutil.SetupTestDB(t) // investigation_test.go içindeki yardımcı fonksiyon

	invRepo := NewInvestigationRepository(database)
	findingRepo := NewFindingRepository(database)
	ctx := context.Background()

	// 1. Önce ebeveyn (Investigation) oluştur
	invID := "inv-test-2"
	inv := &domain.Investigation{
		ID:        invID,
		Name:      "Target Corp",
		Status:    domain.StatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := invRepo.Create(ctx, inv); err != nil {
		t.Fatalf("Failed to create parent investigation: %v", err)
	}

	// 2. Create Finding Testi
	fID := "find-1"
	finding := &domain.Finding{
		ID:              fID,
		InvestigationID: invID,
		Type:            domain.TypeEmail,
		Value:           "admin@target.corp",
		Context:         `{"confidence": 90}`,
		FoundBy:         "hunter2-module",
		CreatedAt:       time.Now().UTC(),
	}

	if err := findingRepo.Create(ctx, finding); err != nil {
		t.Fatalf("Create finding failed: %v", err)
	}

	// 3. GetByInvestigationID Testi
	findings, err := findingRepo.GetByInvestigationID(ctx, invID)
	if err != nil {
		t.Fatalf("GetByInvestigationID failed: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding, got %d", len(findings))
	}
	if findings[0].Value != "admin@target.corp" {
		t.Errorf("Expected email admin@target.corp, got %s", findings[0].Value)
	}

	// 4. Delete & Cascade Testi
	// Araştırmayı sil, CASCADE tetiklensin
	if err := invRepo.Delete(ctx, invID); err != nil {
		t.Fatalf("Failed to delete investigation: %v", err)
	}

	// Bulguları tekrar kontrol et, silinmiş (0) olmalılar
	findingsAfterCascade, err := findingRepo.GetByInvestigationID(ctx, invID)
	if err != nil {
		t.Fatalf("GetByInvestigationID failed after cascade: %v", err)
	}
	if len(findingsAfterCascade) != 0 {
		t.Errorf("Expected 0 findings after cascade deletion, got %d", len(findingsAfterCascade))
	}
}
