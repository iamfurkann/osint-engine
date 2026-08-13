package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

func TestScaffold_GoConnector(t *testing.T) {
	dir := t.TempDir()
	opts := ScaffoldOptions{
		Name:       "shodan-scanner",
		Type:       plugin.TypeConnector,
		Language:   "go",
		PluginsDir: dir,
	}

	createdDir, err := Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}

	expectedDir := filepath.Join(dir, "connectors", "shodan-scanner")
	if createdDir != expectedDir {
		t.Errorf("expected dir %q, got %q", expectedDir, createdDir)
	}

	// Dosyaların varlığını kontrol et
	expectedFiles := []string{"manifest.toml", "plugin.go", "README.md"}
	for _, f := range expectedFiles {
		path := filepath.Join(createdDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q to exist", f)
		}
	}

	// manifest.toml içeriğini doğrula
	manifest, err := os.ReadFile(filepath.Join(createdDir, "manifest.toml"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	content := string(manifest)
	if !strings.Contains(content, `id      = "shodan-scanner"`) {
		t.Error("manifest should contain plugin id")
	}
	if !strings.Contains(content, `type    = "connector"`) {
		t.Error("manifest should contain plugin type")
	}

	// plugin.go içeriğini doğrula
	goFile, err := os.ReadFile(filepath.Join(createdDir, "plugin.go"))
	if err != nil {
		t.Fatalf("failed to read plugin.go: %v", err)
	}
	goContent := string(goFile)
	if !strings.Contains(goContent, "ShodanScanner") {
		t.Error("Go template should contain PascalCase struct name 'ShodanScanner'")
	}
	if !strings.Contains(goContent, "plugin.Manifest") || !strings.Contains(goContent, "plugin.Result") {
		t.Error("Go template should reference plugin.Manifest and plugin.Result types")
	}
}

func TestScaffold_PythonAnalyzer(t *testing.T) {
	dir := t.TempDir()
	opts := ScaffoldOptions{
		Name:       "nlp-analyzer",
		Type:       plugin.TypeAnalyzer,
		Language:   "python",
		PluginsDir: dir,
	}

	createdDir, err := Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}

	expectedDir := filepath.Join(dir, "analyzers", "nlp-analyzer")
	if createdDir != expectedDir {
		t.Errorf("expected dir %q, got %q", expectedDir, createdDir)
	}

	// Python dosyası olmalı, Go dosyası olmamalı
	if _, err := os.Stat(filepath.Join(createdDir, "main.py")); os.IsNotExist(err) {
		t.Error("expected main.py to exist for python plugin")
	}
	if _, err := os.Stat(filepath.Join(createdDir, "plugin.go")); !os.IsNotExist(err) {
		t.Error("plugin.go should NOT exist for python plugin")
	}

	// Python dosyası içeriğini kontrol et
	pyFile, err := os.ReadFile(filepath.Join(createdDir, "main.py"))
	if err != nil {
		t.Fatalf("failed to read main.py: %v", err)
	}
	if !strings.Contains(string(pyFile), "def run(target") {
		t.Error("Python template should contain run function")
	}
}

func TestScaffold_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	opts := ScaffoldOptions{
		Name:       "existing-plugin",
		Type:       plugin.TypeConnector,
		Language:   "go",
		PluginsDir: dir,
	}

	// İlk oluşturma başarılı olmalı
	_, err := Scaffold(opts)
	if err != nil {
		t.Fatalf("first Scaffold failed: %v", err)
	}

	// İkinci oluşturma hata vermeli (üzerine yazma koruması)
	_, err = Scaffold(opts)
	if err == nil {
		t.Fatal("expected error for existing directory, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
}

func TestScaffold_ManifestParseable(t *testing.T) {
	dir := t.TempDir()
	opts := ScaffoldOptions{
		Name:       "parseable-test",
		Type:       plugin.TypeReporter,
		Language:   "go",
		PluginsDir: dir,
	}

	createdDir, err := Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}

	// Oluşturulan manifest.toml ParseManifestFile ile okunabilmeli
	manifestPath := filepath.Join(createdDir, "manifest.toml")
	m, err := plugin.ParseManifestFile(manifestPath)
	if err != nil {
		t.Fatalf("generated manifest is not parseable: %v", err)
	}

	if m.ID != "parseable-test" {
		t.Errorf("expected id 'parseable-test', got %q", m.ID)
	}
	if m.Type != plugin.TypeReporter {
		t.Errorf("expected type 'reporter', got %q", m.Type)
	}
	if m.Version != "v0.1.0" {
		t.Errorf("expected version 'v0.1.0', got %q", m.Version)
	}
}

func TestScaffold_InvalidInputs(t *testing.T) {
	dir := t.TempDir()

	// Boş isim
	_, err := Scaffold(ScaffoldOptions{Name: "", Type: plugin.TypeConnector, PluginsDir: dir})
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	// Geçersiz dil
	_, err = Scaffold(ScaffoldOptions{Name: "test", Type: plugin.TypeConnector, Language: "rust", PluginsDir: dir})
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}

	// Geçersiz tip
	_, err = Scaffold(ScaffoldOptions{Name: "test", Type: "scraper", Language: "go", PluginsDir: dir})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestScaffold_DefaultLanguageIsGo(t *testing.T) {
	dir := t.TempDir()
	opts := ScaffoldOptions{
		Name:       "default-lang",
		Type:       plugin.TypeConnector,
		PluginsDir: dir,
		// Language boş bırakılıyor — varsayılan "go" olmalı
	}

	createdDir, err := Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(createdDir, "plugin.go")); os.IsNotExist(err) {
		t.Error("expected plugin.go to exist when language defaults to go")
	}
}

func TestToPascalCase(t *testing.T) {
	cases := map[string]string{
		"shodan-scanner": "ShodanScanner",
		"hibp":           "Hibp",
		"my-cool-plugin": "MyCoolPlugin",
		"single":         "Single",
	}
	for input, expected := range cases {
		got := toPascalCase(input)
		if got != expected {
			t.Errorf("toPascalCase(%q) = %q, want %q", input, got, expected)
		}
	}
}
