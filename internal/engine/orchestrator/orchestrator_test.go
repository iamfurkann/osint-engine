package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/engine"
	"github.com/iamfurkann/osint-engine/internal/engine/cache"
	"github.com/iamfurkann/osint-engine/internal/engine/retry"
	"github.com/iamfurkann/osint-engine/internal/repository/sqlite"
	"github.com/iamfurkann/osint-engine/internal/testutil"
	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// --- Mock Plugin'ler ---

type mockConnector struct {
	name   string
	inputs []string
	delay  time.Duration
	fail   bool
}

func (m *mockConnector) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:      m.name,
		Name:    m.name,
		Version: "v1.0.0",
		Type:    plugin.TypeConnector,
		Inputs:  m.inputs,
	}
}

func (m *mockConnector) Timeout() time.Duration { return 5 * time.Second }

func (m *mockConnector) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.fail {
		return nil, fmt.Errorf("mock error from %s", m.name)
	}
	return []plugin.Result{
		{Type: "email", Value: "found@" + target, Context: fmt.Sprintf(`{"source":"%s"}`, m.name)},
	}, nil
}

// --- Test Yardımcıları ---

func setupOrchestrator(t *testing.T, plugins []plugin.Plugin, useCache bool) (*Orchestrator, domain.FindingRepository) {
	t.Helper()

	database := testutil.SetupTestDB(t)

	// Investigation oluştur (FK kısıtı)
	invRepo := sqlite.NewInvestigationRepository(database)
	_ = invRepo.Create(context.Background(), &domain.Investigation{
		ID:        "test-inv",
		Name:      "Test",
		Status:    domain.StatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	findingRepo := sqlite.NewFindingRepository(database)

	registry := engine.NewRegistry()
	lifecycle := engine.NewLifecycleManager(registry)
	registry.SetLifecycle(lifecycle)

	for _, p := range plugins {
		if err := registry.Register(p); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}
	lifecycle.ActivateAll()

	deps := Deps{
		Registry:    registry,
		Lifecycle:   lifecycle,
		FindingRepo: findingRepo,
		RetryConfig: retry.Config{MaxAttempts: 2, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond, Multiplier: 2},
		MaxWorkers:  3,
		DefaultRate: 100,
	}

	if useCache {
		cachePath := filepath.Join(t.TempDir(), "test_cache.db")
		c, err := cache.NewCache(cachePath, 1*time.Hour)
		if err != nil {
			t.Fatalf("NewCache failed: %v", err)
		}
		t.Cleanup(func() { c.Close() })
		deps.Cache = c
	}

	o := NewOrchestrator(deps)
	return o, findingRepo
}

// --- Testler ---

func TestOrchestrator_EndToEnd(t *testing.T) {
	plugins := []plugin.Plugin{
		&mockConnector{name: "mock-shodan", inputs: []string{"domain"}},
	}

	o, findingRepo := setupOrchestrator(t, plugins, false)
	o.Start()

	err := o.StartInvestigation(context.Background(), "test-inv", "example.com", "domain", false)
	if err != nil {
		t.Fatalf("StartInvestigation failed: %v", err)
	}

	// Sonuçların gelmesini bekle
	time.Sleep(500 * time.Millisecond)
	o.Stop()

	// DB'de finding olmalı
	findings, err := findingRepo.GetByInvestigationID(context.Background(), "test-inv")
	if err != nil {
		t.Fatalf("GetByInvestigationID failed: %v", err)
	}
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
	if len(findings) > 0 && findings[0].Value != "found@example.com" {
		t.Errorf("expected 'found@example.com', got %q", findings[0].Value)
	}
}

func TestOrchestrator_ParallelConnectors(t *testing.T) {
	plugins := []plugin.Plugin{
		&mockConnector{name: "connector-1", inputs: []string{"domain"}, delay: 50 * time.Millisecond},
		&mockConnector{name: "connector-2", inputs: []string{"domain"}, delay: 50 * time.Millisecond},
		&mockConnector{name: "connector-3", inputs: []string{"domain"}, delay: 50 * time.Millisecond},
	}

	o, findingRepo := setupOrchestrator(t, plugins, false)
	o.Start()

	err := o.StartInvestigation(context.Background(), "test-inv", "example.com", "domain", false)
	if err != nil {
		t.Fatalf("StartInvestigation failed: %v", err)
	}

	time.Sleep(1 * time.Second)
	o.Stop()

	findings, _ := findingRepo.GetByInvestigationID(context.Background(), "test-inv")
	if len(findings) != 3 {
		t.Errorf("expected 3 findings from 3 connectors, got %d", len(findings))
	}

	// İlerleme kontrolü
	progress, err := o.GetProgress("test-inv")
	if err != nil {
		t.Fatalf("GetProgress failed: %v", err)
	}
	if progress.Total != 3 {
		t.Errorf("expected total 3, got %d", progress.Total)
	}
	if progress.Completed != 3 {
		t.Errorf("expected completed 3, got %d", progress.Completed)
	}
	if progress.Percent != 100 {
		t.Errorf("expected 100%%, got %.1f%%", progress.Percent)
	}
}

func TestOrchestrator_FailedConnectorIsolation(t *testing.T) {
	plugins := []plugin.Plugin{
		&mockConnector{name: "good-connector", inputs: []string{"domain"}},
		&mockConnector{name: "bad-connector", inputs: []string{"domain"}, fail: true},
	}

	o, findingRepo := setupOrchestrator(t, plugins, false)
	o.Start()

	_ = o.StartInvestigation(context.Background(), "test-inv", "example.com", "domain", false)

	time.Sleep(1 * time.Second)
	o.Stop()

	// İyi connector'ın sonucu kaydedilmeli
	findings, _ := findingRepo.GetByInvestigationID(context.Background(), "test-inv")
	if len(findings) != 1 {
		t.Errorf("expected 1 finding (good connector only), got %d", len(findings))
	}

	// İlerleme: 1 completed, 1 failed
	progress, _ := o.GetProgress("test-inv")
	if progress.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", progress.Completed)
	}
	if progress.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", progress.Failed)
	}
}

func TestOrchestrator_CacheHit(t *testing.T) {
	plugins := []plugin.Plugin{
		&mockConnector{name: "cached-connector", inputs: []string{"domain"}},
	}

	o, findingRepo := setupOrchestrator(t, plugins, true)
	o.Start()

	// İlk çalıştırma — cache miss, plugin çalışır
	_ = o.StartInvestigation(context.Background(), "test-inv", "example.com", "domain", false)
	time.Sleep(500 * time.Millisecond)
	o.Stop()

	findings1, _ := findingRepo.GetByInvestigationID(context.Background(), "test-inv")
	if len(findings1) != 1 {
		t.Fatalf("expected 1 finding from first run, got %d", len(findings1))
	}

	// Cache istatistikleri
	stats := o.cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 cache miss, got %d", stats.Misses)
	}
}

func TestOrchestrator_NoMatchingPlugins(t *testing.T) {
	plugins := []plugin.Plugin{
		&mockConnector{name: "email-only", inputs: []string{"email"}},
	}

	o, _ := setupOrchestrator(t, plugins, false)

	err := o.StartInvestigation(context.Background(), "test-inv", "example.com", "domain", false)
	if err == nil {
		t.Fatal("expected error for no matching plugins, got nil")
	}
}
