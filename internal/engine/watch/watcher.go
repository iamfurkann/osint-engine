package watch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/engine/orchestrator"
	"github.com/rs/zerolog/log"
)

// Watcher, zamanlanmış hedeflerin düzenli olarak taranmasından sorumludur.
type Watcher struct {
	repo         domain.WatchRepository
	orchestrator *orchestrator.Orchestrator
	ticker       *time.Ticker
	quit         chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
}

// NewWatcher, yeni bir Watcher servisi oluşturur.
func NewWatcher(repo domain.WatchRepository, orch *orchestrator.Orchestrator) *Watcher {
	return &Watcher{
		repo:         repo,
		orchestrator: orch,
		quit:         make(chan struct{}),
	}
}

// Add, yeni bir hedefi izleme listesine ekler.
func (w *Watcher) Add(ctx context.Context, item *domain.WatchItem) error {
	return w.repo.Add(ctx, item)
}

// List, izleme listesini getirir.
func (w *Watcher) List(ctx context.Context) ([]*domain.WatchItem, error) {
	return w.repo.List(ctx)
}

// Remove, hedefi izleme listesinden çıkarır.
func (w *Watcher) Remove(ctx context.Context, id string) error {
	return w.repo.Remove(ctx, id)
}

// Start, izleme döngüsünü başlatır.
func (w *Watcher) Start() {
	// Her 1 dakikada bir kontrol et
	w.ticker = time.NewTicker(1 * time.Minute)
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		log.Info().Msg("Watcher service started")
		for {
			select {
			case <-w.ticker.C:
				w.checkWatchlist()
			case <-w.quit:
				log.Info().Msg("Watcher service stopped")
				w.ticker.Stop()
				return
			}
		}
	}()
}

// Stop, izleme döngüsünü durdurur.
func (w *Watcher) Stop() {
	close(w.quit)
	w.wg.Wait()
}

// checkWatchlist, listeyi tarar ve süresi gelenleri çalıştırır.
func (w *Watcher) checkWatchlist() {
	w.mu.Lock()
	defer w.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items, err := w.repo.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Watcher failed to list items")
		return
	}

	now := time.Now().UTC()
	for _, item := range items {
		// Süresi dolduysa
		if now.Sub(item.LastRun) >= item.Interval {
			w.runInvestigation(item)
		}
	}
}

// runInvestigation, belirli bir izleme hedefi için araştırmayı tetikler.
func (w *Watcher) runInvestigation(item *domain.WatchItem) {
	log.Info().Str("service", "osintd").Str("target", item.Target).Str("type", item.Type).Msg("Watcher triggering investigation")

	// Araştırma ID'sini belirle (Zaman damgalı)
	cleanTarget := strings.ReplaceAll(item.Target, ".", "-")
	invID := fmt.Sprintf("watch-%s-%s-%d", item.Type, cleanTarget, time.Now().Unix())

	ctx := context.Background()
	err := w.orchestrator.StartInvestigation(ctx, invID, item.Target, item.Type, false)
	if err != nil {
		log.Error().Err(err).Str("target", item.Target).Msg("Watcher failed to start investigation")
		return
	}

	// LastRun güncelle
	if err := w.repo.UpdateLastRun(ctx, item.ID, time.Now().UTC()); err != nil {
		log.Error().Err(err).Str("target", item.Target).Msg("Watcher failed to update last_run")
	}
}
