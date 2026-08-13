package engine

import (
	"fmt"
	"sync"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
	"github.com/rs/zerolog/log"
)

// Registry, sisteme kayıtlı tüm plugin'lerin merkezi defteri (kayıt sistemi)dir.
// Thread-safe: eşzamanlı goroutine'lerden güvenle okunup yazılabilir.
type Registry struct {
	plugins   map[string]plugin.Plugin
	lifecycle *LifecycleManager // Opsiyonel: ayarlanmışsa Register/Unregister'da Track/Untrack çağrılır
	mu        sync.RWMutex
}

// NewRegistry, boş bir plugin registry'si oluşturur.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]plugin.Plugin),
	}
}

// SetLifecycle, registry'ye lifecycle manager bağlar.
// Register/Unregister işlemlerinde otomatik Track/Untrack çağrılır.
func (r *Registry) SetLifecycle(lm *LifecycleManager) {
	r.lifecycle = lm
}

// Register, bir plugin'i registry'ye kaydeder.
// Kaydedilmeden önce manifest doğrulaması yapılır.
// Aynı isimde bir plugin zaten kayıtlıysa hata döner.
func (r *Registry) Register(p plugin.Plugin) error {
	m := p.Manifest()

	// 1. Manifest doğrulaması
	if err := m.Validate(); err != nil {
		return fmt.Errorf("registry: cannot register plugin %q: %w", m.Name, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 2. Duplicate kontrolü
	if _, exists := r.plugins[m.Name]; exists {
		return fmt.Errorf("registry: plugin %q is already registered", m.Name)
	}

	r.plugins[m.Name] = p

	// Lifecycle hook: plugin'i yaşam döngüsü takibine ekle
	if r.lifecycle != nil {
		r.lifecycle.Track(m.Name)
	}

	log.Info().
		Str("plugin", m.Name).
		Str("version", m.Version).
		Str("type", string(m.Type)).
		Msg("Plugin registered in registry")

	return nil
}

// Get, ismiyle bir plugin döndürür. Bulunamazsa hata döner.
func (r *Registry) Get(name string) (plugin.Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("registry: plugin %q not found", name)
	}
	return p, nil
}

// List, kayıtlı tüm plugin'lerin manifest bilgilerini döndürür.
func (r *Registry) List() []plugin.Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	manifests := make([]plugin.Manifest, 0, len(r.plugins))
	for _, p := range r.plugins {
		manifests = append(manifests, p.Manifest())
	}
	return manifests
}

// ListByType, belirli bir tipteki plugin'lerin manifest'lerini döndürür.
func (r *Registry) ListByType(t plugin.PluginType) []plugin.Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var manifests []plugin.Manifest
	for _, p := range r.plugins {
		if p.Manifest().Type == t {
			manifests = append(manifests, p.Manifest())
		}
	}
	return manifests
}

// Unregister, bir plugin'i registry'den çıkarır. Bulunamazsa hata döner.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return fmt.Errorf("registry: plugin %q not found, cannot unregister", name)
	}

	delete(r.plugins, name)

	// Lifecycle hook: plugin'i yaşam döngüsü takibinden çıkar
	if r.lifecycle != nil {
		r.lifecycle.Untrack(name)
	}

	log.Info().Str("plugin", name).Msg("Plugin unregistered from registry")
	return nil
}

// Count, registry'deki kayıtlı plugin sayısını döndürür.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}
