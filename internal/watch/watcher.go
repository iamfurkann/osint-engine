package watch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// WatchItem, sürekli izlenen bir hedefin yapılandırmasını tutar.
type WatchItem struct {
	ID        string        `json:"id"`
	Target    string        `json:"target"`
	Type      string        `json:"type"`
	Interval  time.Duration `json:"interval"`
	LastRun   time.Time     `json:"last_run"`
	LastHash  string        `json:"last_hash"`
	CreatedAt time.Time     `json:"created_at"`
}

// Watcher, zamanlanmış izleme görevlerini yönetir.
type Watcher struct {
	items  map[string]*WatchItem
	mu     sync.RWMutex
	stopCh chan struct{}
	wg     sync.WaitGroup
	// Bağımlılıklar (orkestratör, bildirim sistemi vb.) burada olacak.
	// Şimdilik sadece callback kullanıyoruz.
	onChanged func(item *WatchItem, newHash string)
}

// NewWatcher, yeni bir izleme yöneticisi oluşturur.
func NewWatcher() *Watcher {
	return &Watcher{
		items:  make(map[string]*WatchItem),
		stopCh: make(chan struct{}),
	}
}

// SetOnChangeCallback, değişiklik olduğunda çağrılacak fonksiyonu belirler.
func (w *Watcher) SetOnChangeCallback(cb func(item *WatchItem, newHash string)) {
	w.onChanged = cb
}

// Add, yeni bir izleme görevi ekler.
func (w *Watcher) Add(target, inputType string, interval time.Duration) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	id := fmt.Sprintf("%s:%s", inputType, target)
	if _, ok := w.items[id]; ok {
		return fmt.Errorf("hedef zaten izleniyor: %s", target)
	}

	w.items[id] = &WatchItem{
		ID:        id,
		Target:    target,
		Type:      inputType,
		Interval:  interval,
		CreatedAt: time.Now().UTC(),
	}

	log.Info().Str("target", target).Dur("interval", interval).Msg("Watch item eklendi")
	return nil
}

// Remove, bir izleme görevini siler.
func (w *Watcher) Remove(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.items[id]; !ok {
		return fmt.Errorf("izleme görevi bulunamadı: %s", id)
	}

	delete(w.items, id)
	return nil
}

// List, tüm izleme görevlerini döndürür.
func (w *Watcher) List() []WatchItem {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var result []WatchItem
	for _, item := range w.items {
		result = append(result, *item)
	}
	return result
}

// Start, zamanlayıcı döngüsünü başlatır. (Her dakika kontrol eder).
func (w *Watcher) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.checkItems()
			case <-w.stopCh:
				return
			}
		}
	}()
}

// Stop, zamanlayıcıyı durdurur.
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// checkItems, zamanı gelen görevleri kontrol eder.
func (w *Watcher) checkItems() {
	w.mu.RLock()
	var toRun []*WatchItem
	now := time.Now().UTC()

	for _, item := range w.items {
		if item.LastRun.IsZero() || now.Sub(item.LastRun) >= item.Interval {
			toRun = append(toRun, item)
		}
	}
	w.mu.RUnlock()

	for _, item := range toRun {
		w.runItem(item)
	}
}

// runItem, tek bir izleme görevini çalıştırır.
func (w *Watcher) runItem(item *WatchItem) {
	// Not: Gerçek implementasyonda burada orchestrator çalışacak.
	// Şimdilik sadece simüle ediyoruz.
	log.Debug().Str("target", item.Target).Msg("Watch check çalışıyor...")

	// Sahte veri hash'i (simülasyon)
	dummyData := map[string]string{"target": item.Target, "timestamp": time.Now().Format("2006010215")}
	data, _ := json.Marshal(dummyData)
	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	w.mu.Lock()
	changed := !item.LastRun.IsZero() && item.LastHash != "" && item.LastHash != hash
	item.LastHash = hash
	item.LastRun = time.Now().UTC()
	w.mu.Unlock()

	if changed && w.onChanged != nil {
		log.Info().Str("target", item.Target).Msg("Hedefte değişiklik tespit edildi!")
		w.onChanged(item, hash)
	}
}
