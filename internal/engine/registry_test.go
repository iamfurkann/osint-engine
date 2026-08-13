package engine

import (
	"context"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// --- Test Helper: ikinci bir DummyModule ---

type analyzerModule struct{}

func (m *analyzerModule) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:      "test-analyzer-1",
		Name:    "test-analyzer",
		Version: "v1.0.0",
		Type:    plugin.TypeAnalyzer,
		Inputs:  []string{"text"},
	}
}

func (m *analyzerModule) Timeout() time.Duration { return 1 * time.Second }

func (m *analyzerModule) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	return nil, nil
}

// --- Registry Testleri ---

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	dummy := NewDummyModule()

	if err := r.Register(dummy); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	p, err := r.Get("dummy-module")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if p.Manifest().Name != "dummy-module" {
		t.Errorf("expected name 'dummy-module', got %q", p.Manifest().Name)
	}

	if r.Count() != 1 {
		t.Errorf("expected count 1, got %d", r.Count())
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()
	dummy := NewDummyModule()

	if err := r.Register(dummy); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	// Aynı isimde ikinci kayıt hata vermeli
	err := r.Register(dummy)
	if err == nil {
		t.Fatal("expected error for duplicate registration, got nil")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent plugin, got nil")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	dummy := NewDummyModule()

	_ = r.Register(dummy)

	if err := r.Unregister("dummy-module"); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("expected count 0 after unregister, got %d", r.Count())
	}

	// Tekrar unregister hata vermeli
	err := r.Unregister("dummy-module")
	if err == nil {
		t.Fatal("expected error for unregistering nonexistent plugin, got nil")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(NewDummyModule())
	_ = r.Register(&analyzerModule{})

	manifests := r.List()
	if len(manifests) != 2 {
		t.Errorf("expected 2 manifests, got %d", len(manifests))
	}
}

func TestRegistry_ListByType(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(NewDummyModule())  // TypeConnector
	_ = r.Register(&analyzerModule{}) // TypeAnalyzer

	connectors := r.ListByType(plugin.TypeConnector)
	if len(connectors) != 1 {
		t.Errorf("expected 1 connector, got %d", len(connectors))
	}
	if connectors[0].Name != "dummy-module" {
		t.Errorf("expected 'dummy-module', got %q", connectors[0].Name)
	}

	analyzers := r.ListByType(plugin.TypeAnalyzer)
	if len(analyzers) != 1 {
		t.Errorf("expected 1 analyzer, got %d", len(analyzers))
	}

	reporters := r.ListByType(plugin.TypeReporter)
	if len(reporters) != 0 {
		t.Errorf("expected 0 reporters, got %d", len(reporters))
	}
}

func TestRegistry_InvalidManifest(t *testing.T) {
	r := NewRegistry()

	// Geçersiz manifest: boş ID, boş version, boş type, boş inputs
	bad := &badManifestModule{}
	err := r.Register(bad)
	if err == nil {
		t.Fatal("expected error for invalid manifest, got nil")
	}

	// Registry'ye eklenmemiş olmalı
	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}
}

// badManifestModule, geçersiz manifest döndüren test modülü
type badManifestModule struct{}

func (m *badManifestModule) Manifest() plugin.Manifest {
	return plugin.Manifest{
		// Tüm zorunlu alanlar boş → Validate() hata verecek
	}
}

func (m *badManifestModule) Timeout() time.Duration { return 1 * time.Second }

func (m *badManifestModule) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	return nil, nil
}
