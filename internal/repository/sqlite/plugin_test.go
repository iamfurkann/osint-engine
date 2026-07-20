package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/testutil"
)

func TestPluginRepository(t *testing.T) {
	database := testutil.SetupTestDB(t) // isolation sağlayan testutil yardımcısı
	repo := NewPluginRepository(database)
	ctx := context.Background()

	// 1. Yeni Kayıt (INSERT) Testi
	now := time.Now().UTC()
	plugin := &domain.Plugin{
		Name:        "github-scraper",
		Description: "Scrapes GitHub for public emails",
		Version:     "v1.0.0",
		Status:      domain.PluginStatusActive,
		Language:    "go",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := repo.Upsert(ctx, plugin); err != nil {
		t.Fatalf("Upsert (insert) failed: %v", err)
	}

	// 2. GetByName Testi
	fetched, err := repo.GetByName(ctx, plugin.Name)
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}
	if fetched.Version != "v1.0.0" {
		t.Errorf("Expected version v1.0.0, got %s", fetched.Version)
	}

	// 3. Güncelleme (ON CONFLICT UPDATE) Testi
	// Aynı isme (github-scraper) sahip yeni bir versiyon yüklüyoruz
	plugin.Version = "v1.1.0"
	if err := repo.Upsert(ctx, plugin); err != nil {
		t.Fatalf("Upsert (update) failed: %v", err)
	}

	fetchedUpdated, _ := repo.GetByName(ctx, plugin.Name)
	if fetchedUpdated.Version != "v1.1.0" {
		t.Errorf("Expected updated version v1.1.0, got %s", fetchedUpdated.Version)
	}

	// 4. UpdateStatus Testi
	if err := repo.UpdateStatus(ctx, plugin.Name, domain.PluginStatusInactive); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	inactivePlugin, _ := repo.GetByName(ctx, plugin.Name)
	if inactivePlugin.Status != domain.PluginStatusInactive {
		t.Errorf("Expected status inactive, got %s", inactivePlugin.Status)
	}

	// 5. List Testi
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected exactly 1 plugin in list, got %d", len(list))
	}
}
