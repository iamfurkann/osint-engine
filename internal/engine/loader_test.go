package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

func TestLoader_ScanManifests_ValidDir(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(dir, registry)

	// Geçerli connector manifest'i oluştur
	connDir := filepath.Join(dir, "connectors", "shodan")
	if err := os.MkdirAll(connDir, 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	manifest := `
id      = "shodan-scanner"
name    = "shodan-scanner"
version = "v1.0.0"
type    = "connector"
inputs  = ["ip", "domain"]
`
	if err := os.WriteFile(filepath.Join(connDir, "manifest.toml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	manifests, errs := loader.ScanManifests()

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if manifests[0].ID != "shodan-scanner" {
		t.Errorf("expected id 'shodan-scanner', got %q", manifests[0].ID)
	}
	if manifests[0].Type != plugin.TypeConnector {
		t.Errorf("expected type 'connector', got %q", manifests[0].Type)
	}
}

func TestLoader_ScanManifests_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(dir, registry)

	// Geçersiz manifest (zorunlu alanlar eksik)
	connDir := filepath.Join(dir, "connectors", "broken")
	if err := os.MkdirAll(connDir, 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	badManifest := `
id = "broken"
# version, type ve inputs eksik
`
	if err := os.WriteFile(filepath.Join(connDir, "manifest.toml"), []byte(badManifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	manifests, errs := loader.ScanManifests()

	if len(manifests) != 0 {
		t.Errorf("expected 0 valid manifests, got %d", len(manifests))
	}
	if len(errs) == 0 {
		t.Fatal("expected at least 1 error for invalid manifest, got 0")
	}
}

func TestLoader_ScanManifests_TypeMismatch(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(dir, registry)

	// analyzer tipinde manifest'i connectors/ dizinine koy → tür uyumsuzluğu
	connDir := filepath.Join(dir, "connectors", "wrong-type")
	if err := os.MkdirAll(connDir, 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	manifest := `
id      = "wrong-type"
name    = "wrong-type"
version = "v1.0.0"
type    = "analyzer"
inputs  = ["text"]
`
	if err := os.WriteFile(filepath.Join(connDir, "manifest.toml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	manifests, errs := loader.ScanManifests()

	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests due to type mismatch, got %d", len(manifests))
	}
	if len(errs) == 0 {
		t.Fatal("expected error for type mismatch, got 0")
	}
}

func TestLoader_ScanManifests_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(dir, registry)

	manifests, errs := loader.ScanManifests()

	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests for empty dir, got %d", len(manifests))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for empty dir, got %d", len(errs))
	}
}

func TestLoader_ScanManifests_NonexistentDir(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader("/nonexistent/plugins", registry)

	manifests, errs := loader.ScanManifests()

	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests, got %d", len(manifests))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors for nonexistent dir, got %d", len(errs))
	}
}

func TestLoader_RegisterBuiltins(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(dir, registry)

	builtins := map[string]plugin.Plugin{
		"dummy-module": NewDummyModule(),
	}

	errs := loader.RegisterBuiltins(builtins)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}

	if registry.Count() != 1 {
		t.Errorf("expected 1 registered plugin, got %d", registry.Count())
	}

	p, err := registry.Get("dummy-module")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if p.Manifest().Name != "dummy-module" {
		t.Errorf("expected 'dummy-module', got %q", p.Manifest().Name)
	}
}

func TestLoader_EnsurePluginDirs(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(dir, registry)

	if err := loader.EnsurePluginDirs(); err != nil {
		t.Fatalf("EnsurePluginDirs failed: %v", err)
	}

	// Beklenen dizinlerin oluşturulduğunu doğrula
	expectedDirs := []string{"connectors", "analyzers", "reporters", "ai-providers"}
	for _, d := range expectedDirs {
		path := filepath.Join(dir, d)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected directory %q to exist, got error: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", d)
		}
	}
}

func TestLoader_ScanManifests_MultiplePlugins(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()
	loader := NewLoader(dir, registry)

	// 2 connector + 1 analyzer oluştur
	plugins := []struct {
		typeDir  string
		name     string
		manifest string
	}{
		{"connectors", "shodan", `id = "shodan" ` + "\n" + `name = "shodan" ` + "\n" + `version = "v1.0.0" ` + "\n" + `type = "connector" ` + "\n" + `inputs = ["ip"]`},
		{"connectors", "hibp", `id = "hibp" ` + "\n" + `name = "hibp" ` + "\n" + `version = "v2.1.0" ` + "\n" + `type = "connector" ` + "\n" + `inputs = ["email"]`},
		{"analyzers", "nlp", `id = "nlp" ` + "\n" + `name = "nlp" ` + "\n" + `version = "v0.5.0" ` + "\n" + `type = "analyzer" ` + "\n" + `inputs = ["text"]`},
	}

	for _, p := range plugins {
		plugDir := filepath.Join(dir, p.typeDir, p.name)
		if err := os.MkdirAll(plugDir, 0700); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(plugDir, "manifest.toml"), []byte(p.manifest), 0644); err != nil {
			t.Fatalf("failed to write manifest: %v", err)
		}
	}

	manifests, errs := loader.ScanManifests()

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if len(manifests) != 3 {
		t.Errorf("expected 3 manifests, got %d", len(manifests))
	}
}
