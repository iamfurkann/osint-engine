package plugin

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// PluginType eklentinin hangi kategoride çalıştığını belirtir.
type PluginType string

const (
	TypeConnector PluginType = "connector"
	TypeAnalyzer  PluginType = "analyzer"
	TypeReporter  PluginType = "reporter"
	TypeAI        PluginType = "ai-provider"
)

// validPluginTypes, Validate() içinde geçerli tipi kontrol etmek için kullanılır.
var validPluginTypes = map[PluginType]bool{
	TypeConnector: true,
	TypeAnalyzer:  true,
	TypeReporter:  true,
	TypeAI:        true,
}

// semverRegex, SemVer (Semantic Versioning) formatını doğrular.
var semverRegex = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// --- Manifest-Spesifik Sentinel Hatalar ---
// Bunlar MDP'nin "doğrulama hataları açık mesajlarla bildirilir" gereksinimini karşılar.
var (
	ErrManifestMissingField = fmt.Errorf("manifest: required field is missing")
	ErrManifestInvalidType  = fmt.Errorf("manifest: invalid plugin type")
	ErrManifestInvalidValue = fmt.Errorf("manifest: field value is out of valid range")
)

// Manifest, bir eklentinin (plugin) metadata ve yapılandırma özelliklerini barındırır.
// Bu veri eklentinin yanındaki manifest.toml dosyasından okunur.
type Manifest struct {
	// --- Zorunlu Alanlar ---
	ID      string     `toml:"id"      json:"id"`
	Name    string     `toml:"name"    json:"name"`
	Version string     `toml:"version" json:"version"` // SemVer formatında olmalı (örn: v1.0.0)
	Type    PluginType `toml:"type"    json:"type"`
	Inputs  []string   `toml:"inputs"  json:"inputs"` // Desteklenen girdiler: ["email", "domain"]

	// --- İsteğe Bağlı Alanlar ---
	Description      string   `toml:"description,omitempty"       json:"description,omitempty"`
	Language         string   `toml:"language,omitempty"           json:"language,omitempty"` // "go", "python", "binary"
	RateLimit        int      `toml:"rate_limit,omitempty"         json:"rate_limit,omitempty"`
	Auth             []string `toml:"auth,omitempty"               json:"auth,omitempty"`               // Gerekli API anahtarları: ["shodan", "github"]
	Confidence       int      `toml:"confidence,omitempty"         json:"confidence,omitempty"`         // Ürettiği verinin varsayılan güvenilirliği (0-100)
	Dependencies     []string `toml:"dependencies,omitempty"       json:"dependencies,omitempty"`       // Çalışması için gereken diğer araçlar
	EngineMinVersion string   `toml:"engine_min_version,omitempty" json:"engine_min_version,omitempty"` // Gereken minimum engine sürümü (örn: v0.2.0)
}

// ParseManifestFile, belirtilen dosya yolundaki TOML manifest'i okur,
// ayrıştırır (parse) ve doğrular (validate). Hatalı veya eksik alanlar
// varsa açık ve yardımcı mesajlarla bildirilir.
func ParseManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: failed to read file %q: %w", path, err)
	}

	return ParseManifest(data)
}

// ParseManifest, ham TOML byte'larını Manifest struct'a dönüştürür ve doğrular.
// Dosya okuma yapmadan doğrudan TOML verisi ile çalışmak isteyenler için kullanılabilir.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: failed to parse TOML: %w", err)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

// Validate, manifest'in zorunlu alanlarının dolu olup olmadığını ve
// isteğe bağlı alanların geçerli aralıkta olup olmadığını denetler.
// Her hata, kullanıcıya yardımcı olacak açık bir mesaj içerir.
func (m *Manifest) Validate() error {
	// --- Zorunlu Alan Kontrolleri ---
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("%w: 'id' field is required in manifest (example: id = \"shodan-scanner\")", ErrManifestMissingField)
	}

	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("%w: 'version' field is required in manifest (example: version = \"v1.0.0\")", ErrManifestMissingField)
	}

	if !semverRegex.MatchString(m.Version) {
		return fmt.Errorf("%w: version %q is not valid SemVer (expected format: v1.0.0 or 2.1.3-beta)", ErrManifestInvalidValue, m.Version)
	}

	if m.Type == "" {
		return fmt.Errorf("%w: 'type' field is required in manifest (valid types: connector, analyzer, reporter, ai-provider)", ErrManifestMissingField)
	}

	if !validPluginTypes[m.Type] {
		return fmt.Errorf("%w: type %q is not recognized (valid types: connector, analyzer, reporter, ai-provider)", ErrManifestInvalidType, m.Type)
	}

	if len(m.Inputs) == 0 {
		return fmt.Errorf("%w: 'inputs' field is required and must contain at least one entry (example: inputs = [\"email\", \"domain\"])", ErrManifestMissingField)
	}

	// --- İsteğe Bağlı Alan Sınır Kontrolleri ---
	if m.Confidence < 0 || m.Confidence > 100 {
		return fmt.Errorf("%w: confidence must be between 0 and 100, got %d", ErrManifestInvalidValue, m.Confidence)
	}

	if m.RateLimit < 0 {
		return fmt.Errorf("%w: rate_limit cannot be negative, got %d", ErrManifestInvalidValue, m.RateLimit)
	}

	return nil
}
