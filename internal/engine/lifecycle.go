package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// PluginState, bir plugin'in mevcut çalışma durumunu temsil eder.
type PluginState string

const (
	StateInactive PluginState = "inactive" // Kayıtlı ama henüz başlatılmamış
	StateActive   PluginState = "active"   // Çalışıyor, görev alabilir
	StateError    PluginState = "error"    // Hata durumunda, görev alamaz
	StateStarting PluginState = "starting" // Başlatılıyor (geçici)
	StateStopping PluginState = "stopping" // Durduruluyor (geçici)
)

// PluginStatus, bir plugin'in çalışma zamanı durum bilgisini tutar.
type PluginStatus struct {
	State           PluginState // Mevcut durum
	LastError       error       // Son karşılaşılan hata
	ErrorCount      int         // Ardışık hata sayısı (başarılı çalıştırmada sıfırlanır)
	StartedAt       time.Time   // Son başlatılma zamanı
	StoppedAt       time.Time   // Son durdurulma zamanı
	LastHealthCheck time.Time   // Son sağlık kontrolü zamanı
}

// maxConsecutiveErrors, bir plugin devre dışı bırakılmadan önce tolere edilen
// ardışık hata sayısıdır. Geçici ağ hatalarının kalıcı arızadan ayrılmasını
// sağlar; başarılı her çalıştırma sayacı sıfırlar.
const maxConsecutiveErrors = 3

// LifecycleManager, tüm plugin'lerin yaşam döngüsünü yöneten merkezi birimdir.
// Thread-safe: eşzamanlı goroutine'lerden güvenle okunup yazılabilir.
type LifecycleManager struct {
	registry *Registry
	statuses map[string]*PluginStatus
	mu       sync.RWMutex
}

// NewLifecycleManager, belirtilen registry ile bir lifecycle manager oluşturur.
func NewLifecycleManager(registry *Registry) *LifecycleManager {
	return &LifecycleManager{
		registry: registry,
		statuses: make(map[string]*PluginStatus),
	}
}

// Track, bir plugin'i lifecycle takibine ekler (initial state: inactive).
// Registry.Register() sonrası otomatik çağrılır.
func (lm *LifecycleManager) Track(name string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.statuses[name] = &PluginStatus{
		State: StateInactive,
	}
}

// Untrack, bir plugin'i lifecycle takibinden çıkarır.
// Registry.Unregister() sonrası otomatik çağrılır.
func (lm *LifecycleManager) Untrack(name string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	delete(lm.statuses, name)
}

// Activate, bir plugin'i active durumuna geçirir.
// Sadece inactive veya error durumundaki plugin'ler aktif edilebilir.
func (lm *LifecycleManager) Activate(name string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	status, exists := lm.statuses[name]
	if !exists {
		return fmt.Errorf("lifecycle: plugin %q is not being tracked", name)
	}

	if status.State == StateActive {
		return nil // Zaten aktif, idempotent davranış
	}

	if status.State != StateInactive && status.State != StateError {
		return fmt.Errorf("lifecycle: cannot activate plugin %q from state %q (must be inactive or error)", name, status.State)
	}

	status.State = StateActive
	status.StartedAt = time.Now().UTC()
	status.LastHealthCheck = time.Now().UTC()

	log.Info().
		Str("plugin", name).
		Str("state", string(StateActive)).
		Msg("Plugin activated")

	return nil
}

// Deactivate, bir plugin'i inactive durumuna geçirir.
func (lm *LifecycleManager) Deactivate(name string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	status, exists := lm.statuses[name]
	if !exists {
		return fmt.Errorf("lifecycle: plugin %q is not being tracked", name)
	}

	if status.State == StateInactive {
		return nil // Zaten inaktif
	}

	status.State = StateInactive
	status.StoppedAt = time.Now().UTC()

	log.Info().
		Str("plugin", name).
		Str("state", string(StateInactive)).
		Msg("Plugin deactivated")

	return nil
}

// MarkError, bir plugin'i error durumuna geçirir.
// Hatalı plugin'ler görev alamaz — çekirdek etkilenmez (izolasyon).
func (lm *LifecycleManager) MarkError(name string, err error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	status, exists := lm.statuses[name]
	if !exists {
		return // Takip edilmiyor, sessizce geç
	}

	status.LastError = err
	status.ErrorCount++

	// TEK bir hata artık plugin'i devre dışı bırakmıyor.
	//
	// Önceki davranış: ilk hatada State = StateError. processTask ve
	// StartInvestigation IsActive() üzerinden kapı tuttuğu ve üretimde
	// Restart() hiç çağrılmadığı için, plugin daemon'ın ÖMRÜ BOYUNCA ölü
	// kalıyordu. Pratikte şu gözlendi: crt.sh'den gelen tek bir geçici 502,
	// connector'ı kalıcı olarak kapattı.
	//
	// Artık ardışık hata eşiği var. Başarılı bir çalıştırma sayacı sıfırlar
	// (bkz. MarkSuccess), böylece yalnızca ısrarla bozuk olan plugin izole
	// edilir — geçici ağ dalgalanmaları değil.
	if status.ErrorCount >= maxConsecutiveErrors {
		status.State = StateError
		status.StoppedAt = time.Now().UTC()

		log.Warn().
			Str("plugin", name).
			Err(err).
			Int("error_count", status.ErrorCount).
			Msg("Plugin marked as error — isolated from task assignment")
		return
	}

	log.Warn().
		Str("plugin", name).
		Err(err).
		Int("error_count", status.ErrorCount).
		Int("threshold", maxConsecutiveErrors).
		Msg("Plugin task failed — still active, will retry on next task")
}

// MarkSuccess, başarılı bir çalıştırmadan sonra ardışık hata sayacını sıfırlar.
// Bu olmadan eşik, birbirinden bağımsız ve aralıklı hataların birikmesiyle
// er ya da geç dolardı.
func (lm *LifecycleManager) MarkSuccess(name string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	status, exists := lm.statuses[name]
	if !exists {
		return
	}
	status.ErrorCount = 0
}

// Restart, error durumundaki bir plugin'i tekrar active durumuna geçirir.
// ErrorCount sıfırlanmaz (tarihsel takip için).
func (lm *LifecycleManager) Restart(name string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	status, exists := lm.statuses[name]
	if !exists {
		return fmt.Errorf("lifecycle: plugin %q is not being tracked", name)
	}

	if status.State != StateError {
		return fmt.Errorf("lifecycle: cannot restart plugin %q from state %q (must be in error state)", name, status.State)
	}

	status.State = StateActive
	status.StartedAt = time.Now().UTC()
	status.LastHealthCheck = time.Now().UTC()

	log.Info().
		Str("plugin", name).
		Int("error_count", status.ErrorCount).
		Msg("Plugin restarted from error state")

	return nil
}

// GetStatus, bir plugin'in durum bilgisini döndürür.
func (lm *LifecycleManager) GetStatus(name string) (*PluginStatus, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	status, exists := lm.statuses[name]
	if !exists {
		return nil, fmt.Errorf("lifecycle: plugin %q is not being tracked", name)
	}

	// Kopya döndür (dışarıda state değiştirilmesin)
	copy := *status
	return &copy, nil
}

// ListStatuses, tüm plugin'lerin durum bilgilerini döndürür.
func (lm *LifecycleManager) ListStatuses() map[string]PluginStatus {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make(map[string]PluginStatus, len(lm.statuses))
	for name, status := range lm.statuses {
		result[name] = *status
	}
	return result
}

// IsActive, bir plugin'in görev alabilir durumda olup olmadığını döndürür.
// Engine.processTask() çağrılmadan önce hızlı kontrol için kullanılır.
func (lm *LifecycleManager) IsActive(name string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	status, exists := lm.statuses[name]
	if !exists {
		return false
	}
	return status.State == StateActive
}

// ActivateAll, registry'deki tüm takip edilen plugin'leri aktif duruma geçirir.
// Engine başlangıcında çağrılır.
func (lm *LifecycleManager) ActivateAll() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	for name, status := range lm.statuses {
		if status.State == StateInactive {
			status.State = StateActive
			status.StartedAt = time.Now().UTC()
			status.LastHealthCheck = time.Now().UTC()
			log.Debug().Str("plugin", name).Msg("Plugin activated (batch)")
		}
	}
}

// DeactivateAll, tüm aktif plugin'leri inactive durumuna geçirir.
// Engine durdurulurken çağrılır.
func (lm *LifecycleManager) DeactivateAll() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	for name, status := range lm.statuses {
		if status.State == StateActive {
			status.State = StateInactive
			status.StoppedAt = time.Now().UTC()
			log.Debug().Str("plugin", name).Msg("Plugin deactivated (batch)")
		}
	}
}
