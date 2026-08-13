package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
	"github.com/rs/zerolog/log"
)

// pluginTypeDirs, PluginType'a karşılık gelen dizin adlarını eşleştirir.
// Mimari rapordaki dizin yapısını yansıtır:
//
//	plugins/connectors/{name}/manifest.toml
//	plugins/analyzers/{name}/manifest.toml
//	plugins/reporters/{name}/manifest.toml
//	plugins/ai-providers/{name}/manifest.toml
var pluginTypeDirs = map[string]plugin.PluginType{
	"connectors":   plugin.TypeConnector,
	"analyzers":    plugin.TypeAnalyzer,
	"reporters":    plugin.TypeReporter,
	"ai-providers": plugin.TypeAI,
}

// Loader, plugin dizinini tarayan ve manifest dosyalarını okuyan bileşendir.
type Loader struct {
	pluginDir string
	registry  *Registry
}

// NewLoader, belirtilen plugin dizini ve registry ile bir Loader oluşturur.
func NewLoader(pluginDir string, registry *Registry) *Loader {
	return &Loader{
		pluginDir: pluginDir,
		registry:  registry,
	}
}

// ScanManifests, plugin dizinini tarar ve bulduğu tüm manifest.toml dosyalarını
// okuyup doğrular. Geçerli manifest'ler ve hatalar ayrı ayrı döndürülür;
// tek bir hatalı manifest diğerlerini engellemez.
func (l *Loader) ScanManifests() ([]plugin.Manifest, []error) {
	var manifests []plugin.Manifest
	var errs []error

	// Plugin dizini yoksa hata değil, boş sonuç dön (henüz plugin kurulmamış olabilir)
	if _, err := os.Stat(l.pluginDir); os.IsNotExist(err) {
		log.Debug().Str("dir", l.pluginDir).Msg("Plugin directory does not exist, skipping scan")
		return manifests, errs
	}

	// Her tip dizinini (connectors/, analyzers/, ...) tara
	for dirName, expectedType := range pluginTypeDirs {
		typeDir := filepath.Join(l.pluginDir, dirName)

		entries, err := os.ReadDir(typeDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Bu tip dizini yoksa geç (normal durum)
			}
			errs = append(errs, fmt.Errorf("loader: cannot read directory %q: %w", typeDir, err))
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue // Sadece alt dizinleri tara (her plugin kendi dizininde)
			}

			manifestPath := filepath.Join(typeDir, entry.Name(), "manifest.toml")
			m, err := plugin.ParseManifestFile(manifestPath)
			if err != nil {
				errs = append(errs, fmt.Errorf("loader: plugin %q: %w", entry.Name(), err))
				continue
			}

			// Manifest'teki type ile dizin türünü karşılaştır
			if m.Type != expectedType {
				errs = append(errs, fmt.Errorf(
					"loader: plugin %q declares type %q but is located in %q directory",
					m.Name, m.Type, dirName,
				))
				continue
			}

			manifests = append(manifests, *m)
			log.Debug().
				Str("plugin", m.Name).
				Str("version", m.Version).
				Str("type", string(m.Type)).
				Msg("Manifest scanned successfully")
		}
	}

	return manifests, errs
}

// RegisterBuiltins, derleme zamanında (compile-time) gömülü olan Go plugin'leri
// registry'ye kaydeder. builtins parametresi manifest name → Plugin implementasyonu eşlemesidir.
//
// Kullanım (main.go veya bootstrap):
//
//	loader.RegisterBuiltins(map[string]plugin.Plugin{
//	    "dummy-module": engine.NewDummyModule(),
//	})
func (l *Loader) RegisterBuiltins(builtins map[string]plugin.Plugin) []error {
	var errs []error
	for name, p := range builtins {
		if err := l.registry.Register(p); err != nil {
			errs = append(errs, fmt.Errorf("loader: failed to register builtin %q: %w", name, err))
		}
	}
	return errs
}

// EnsurePluginDirs, plugin dizin yapısını oluşturur (yoksa).
// Bu fonksiyon ilk çalıştırmada çağrılarak boş iskeletin hazır olmasını sağlar.
func (l *Loader) EnsurePluginDirs() error {
	for dirName := range pluginTypeDirs {
		dirPath := filepath.Join(l.pluginDir, dirName)
		if err := os.MkdirAll(dirPath, 0700); err != nil {
			return fmt.Errorf("loader: failed to create plugin directory %q: %w", dirPath, err)
		}
	}
	log.Debug().Str("dir", l.pluginDir).Msg("Plugin directory structure ensured")
	return nil
}
