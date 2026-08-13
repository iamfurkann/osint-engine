package engine

import (
	"context"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/repository/sqlite"
	"github.com/iamfurkann/osint-engine/internal/testutil"
)

func TestEngine_ModuleExecutionAndFindings(t *testing.T) {
	// Artık merkezi yardımcımızı kullanıyoruz
	database := testutil.SetupTestDB(t)

	// İlişkisel veritabanı kısıtlarına (FOREIGN KEY) takılmamak için
	// önce sahte bir araştırma (Investigation) kaydı oluşturalım.
	invRepo := sqlite.NewInvestigationRepository(database)
	invID := "test-inv-1"
	err := invRepo.Create(context.Background(), &domain.Investigation{
		ID:        invID,
		Name:      "Test Target",
		Status:    domain.StatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Failed to create parent investigation: %v", err)
	}

	// Motoru 2 işçiyle başlatıyoruz — bağımlılıklar dışarıdan enjekte ediliyor (DIP)
	findingRepo := sqlite.NewFindingRepository(database)
	registry := NewRegistry()
	lifecycle := NewLifecycleManager(registry)
	registry.SetLifecycle(lifecycle)
	eng := NewEngine(findingRepo, registry, lifecycle, 2)

	// Test modülümüzü registry'ye kaydediyoruz
	dummy := NewDummyModule()
	if err := registry.Register(dummy); err != nil {
		t.Fatalf("failed to register dummy module: %v", err)
	}

	eng.Start()

	// 3 adet görev yolluyoruz
	for i := 0; i < 3; i++ {
		task := Task{
			InvestigationID: invID,
			Target:          "example.com",
			PluginName:      dummy.Manifest().Name, // Artık Manifest üzerinden ismini alıyoruz
		}
		if err := eng.SubmitTask(task); err != nil {
			t.Fatalf("failed to submit task: %v", err)
		}
	}

	// Kuyruğun erimesi için motoru güvenle kapatıyoruz
	eng.Stop()

	// Motor durdurulduktan sonra yeni görev eklemeyi denersek hata almalıyız
	err = eng.SubmitTask(Task{})
	if err == nil {
		t.Errorf("expected error when submitting task to stopped engine, got nil")
	}

	// Veritabanına gidip 3 adet bulgu (Finding) kaydedilmiş mi diye kontrol ediyoruz
	findings, err := findingRepo.GetByInvestigationID(context.Background(), invID)
	if err != nil {
		t.Fatalf("failed to get findings: %v", err)
	}

	if len(findings) != 3 {
		t.Errorf("expected 3 findings, got %d", len(findings))
	}

	// Örnek veriyi doğrula
	if len(findings) > 0 && findings[0].Value != "admin@example.com" {
		t.Errorf("expected finding value 'admin@example.com', got '%s'", findings[0].Value)
	}
}
