package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// --- ParseManifestFile Testleri ---

func TestParseManifestFile_ValidComplete(t *testing.T) {
	// Tam ve geçerli bir manifest dosyası
	tomlContent := `
id       = "shodan-scanner"
name     = "Shodan IP Scanner"
version  = "v1.2.0"
type     = "connector"
language = "python"
inputs   = ["ip", "domain"]

rate_limit   = 5
auth         = ["shodan"]
confidence   = 85
dependencies = ["nmap"]
description  = "Scans Shodan for open ports and banners"
`
	path := writeTempManifest(t, tomlContent)

	m, err := ParseManifestFile(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if m.ID != "shodan-scanner" {
		t.Errorf("expected id 'shodan-scanner', got %q", m.ID)
	}
	if m.Version != "v1.2.0" {
		t.Errorf("expected version 'v1.2.0', got %q", m.Version)
	}
	if m.Type != TypeConnector {
		t.Errorf("expected type 'connector', got %q", m.Type)
	}
	if len(m.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(m.Inputs))
	}
	if m.Confidence != 85 {
		t.Errorf("expected confidence 85, got %d", m.Confidence)
	}
	if m.RateLimit != 5 {
		t.Errorf("expected rate_limit 5, got %d", m.RateLimit)
	}
	if len(m.Auth) != 1 || m.Auth[0] != "shodan" {
		t.Errorf("expected auth [shodan], got %v", m.Auth)
	}
	if m.Language != "python" {
		t.Errorf("expected language 'python', got %q", m.Language)
	}
}

func TestParseManifestFile_MinimalValid(t *testing.T) {
	// Sadece zorunlu alanlar — isteğe bağlı alanlar hiç verilmemiş
	tomlContent := `
id      = "basic-tool"
version = "v0.1.0"
type    = "analyzer"
inputs  = ["email"]
`
	path := writeTempManifest(t, tomlContent)

	m, err := ParseManifestFile(path)
	if err != nil {
		t.Fatalf("expected no error for minimal manifest, got: %v", err)
	}
	if m.Confidence != 0 {
		t.Errorf("expected default confidence 0, got %d", m.Confidence)
	}
	if m.RateLimit != 0 {
		t.Errorf("expected default rate_limit 0, got %d", m.RateLimit)
	}
}

func TestParseManifestFile_FileNotFound(t *testing.T) {
	_, err := ParseManifestFile("/nonexistent/manifest.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseManifestFile_InvalidTOML(t *testing.T) {
	path := writeTempManifest(t, `this is not valid toml [[[`)

	_, err := ParseManifestFile(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

// --- Validate: Zorunlu Alan Testleri ---

func TestValidate_MissingID(t *testing.T) {
	m := validManifest()
	m.ID = ""

	err := m.Validate()
	assertSentinelError(t, err, ErrManifestMissingField, "id")
}

func TestValidate_MissingVersion(t *testing.T) {
	m := validManifest()
	m.Version = ""

	err := m.Validate()
	assertSentinelError(t, err, ErrManifestMissingField, "version")
}

func TestValidate_MissingType(t *testing.T) {
	m := validManifest()
	m.Type = ""

	err := m.Validate()
	assertSentinelError(t, err, ErrManifestMissingField, "type")
}

func TestValidate_MissingInputs(t *testing.T) {
	m := validManifest()
	m.Inputs = nil

	err := m.Validate()
	assertSentinelError(t, err, ErrManifestMissingField, "inputs")
}

func TestValidate_EmptyInputs(t *testing.T) {
	m := validManifest()
	m.Inputs = []string{}

	err := m.Validate()
	assertSentinelError(t, err, ErrManifestMissingField, "inputs")
}

// --- Validate: Geçersiz Değer Testleri ---

func TestValidate_InvalidVersion(t *testing.T) {
	cases := []string{"abc", "1.2", "v1", "latest", "1.2.3.4"}
	for _, ver := range cases {
		m := validManifest()
		m.Version = ver
		err := m.Validate()
		if err == nil {
			t.Errorf("expected error for invalid version %q, got nil", ver)
		}
		if !errors.Is(err, ErrManifestInvalidValue) {
			t.Errorf("expected ErrManifestInvalidValue for version %q, got: %v", ver, err)
		}
	}
}

func TestValidate_ValidVersionFormats(t *testing.T) {
	cases := []string{"v1.0.0", "0.1.0", "v2.3.4-beta", "1.0.0-rc.1", "3.0.0+build.123"}
	for _, ver := range cases {
		m := validManifest()
		m.Version = ver
		err := m.Validate()
		if err != nil {
			t.Errorf("expected no error for valid version %q, got: %v", ver, err)
		}
	}
}

func TestValidate_InvalidType(t *testing.T) {
	m := validManifest()
	m.Type = "scraper"

	err := m.Validate()
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
	if !errors.Is(err, ErrManifestInvalidType) {
		t.Errorf("expected ErrManifestInvalidType, got: %v", err)
	}
}

func TestValidate_AllValidTypes(t *testing.T) {
	types := []PluginType{TypeConnector, TypeAnalyzer, TypeReporter, TypeAI}
	for _, pt := range types {
		m := validManifest()
		m.Type = pt
		if err := m.Validate(); err != nil {
			t.Errorf("expected no error for valid type %q, got: %v", pt, err)
		}
	}
}

// --- Validate: İsteğe Bağlı Alan Sınır Testleri ---

func TestValidate_ConfidenceOutOfRange(t *testing.T) {
	m := validManifest()
	m.Confidence = 101

	err := m.Validate()
	assertSentinelError(t, err, ErrManifestInvalidValue, "confidence")

	m.Confidence = -1
	err = m.Validate()
	assertSentinelError(t, err, ErrManifestInvalidValue, "confidence")
}

func TestValidate_NegativeRateLimit(t *testing.T) {
	m := validManifest()
	m.RateLimit = -5

	err := m.Validate()
	assertSentinelError(t, err, ErrManifestInvalidValue, "rate_limit")
}

func TestValidate_ZeroConfidenceAndRateLimitAreValid(t *testing.T) {
	m := validManifest()
	m.Confidence = 0
	m.RateLimit = 0

	if err := m.Validate(); err != nil {
		t.Errorf("expected no error for zero optional values, got: %v", err)
	}
}

// --- Yardımcı Fonksiyonlar ---

// validManifest, testlerde baz olarak kullanılacak geçerli bir manifest döndürür.
func validManifest() Manifest {
	return Manifest{
		ID:      "test-plugin",
		Name:    "Test Plugin",
		Version: "v1.0.0",
		Type:    TypeConnector,
		Inputs:  []string{"email"},
	}
}

// writeTempManifest, geçici bir manifest.toml dosyası oluşturur ve yolunu döndürür.
func writeTempManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp manifest: %v", err)
	}
	return path
}

// assertSentinelError, hatanın beklenen sentinel hatayı sarmaladığını ve bağlam mesajı içerdiğini doğrular.
func assertSentinelError(t *testing.T, err error, sentinel error, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", context)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected error to wrap %v, got: %v", sentinel, err)
	}
}
