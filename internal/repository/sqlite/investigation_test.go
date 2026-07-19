package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/internal/db"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/errors"
	"github.com/iamfurkann/osint-engine/internal/testutil"
)

// setupTestDB, test bitiminde kendini yok eden izole bir SQLite veritabanı kurar.
func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	dbPath := testutil.TempSQLiteDBPath(t)
	ctx := context.Background()

	database, err := db.Connect(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// Tabloları oluşturmak için migrasyonu çalıştır
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}

func TestInvestigationRepository(t *testing.T) {
	database := setupTestDB(t)
	repo := NewInvestigationRepository(database)
	ctx := context.Background()

	// 1. Create Testi
	now := time.Now().UTC()
	inv := &domain.Investigation{
		ID:        "inv-test-1",
		Name:      "Test Target",
		Status:    domain.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.Create(ctx, inv); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 2. GetByID Testi
	fetched, err := repo.GetByID(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Name != inv.Name {
		t.Errorf("Expected name %s, got %s", inv.Name, fetched.Name)
	}

	// 3. Update Testi
	fetched.Name = "Updated Target"
	fetched.Status = domain.StatusCompleted
	if err := repo.Update(ctx, fetched); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := repo.GetByID(ctx, inv.ID)
	if updated.Status != domain.StatusCompleted {
		t.Errorf("Expected status %s, got %s", domain.StatusCompleted, updated.Status)
	}

	// 4. List Testi
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected 1 investigation, got %d", len(list))
	}

	// 5. Delete Testi
	if err := repo.Delete(ctx, inv.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Silindikten sonra bulunamaması (TypeNotFound) gerektiğini doğrula
	_, err = repo.GetByID(ctx, inv.ID)
	if !errors.IsType(err, errors.TypeNotFound) {
		t.Errorf("Expected TypeNotFound after deletion, got: %v", err)
	}
}
