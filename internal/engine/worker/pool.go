package worker

import (
	"context"
	"sync"

	"github.com/iamfurkann/osint-engine/internal/engine/queue"
	"github.com/rs/zerolog/log"
)

// Result, bir görevin çalıştırılması sonucunda üretilen rapordur.
type Result struct {
	TaskID          string
	InvestigationID string
	PluginName      string
	Success         bool
	Error           error
	FindingsCount   int
}

// ProcessFunc, worker'ın her görev için çağıracağı fonksiyondur.
// Görevin nasıl işleneceğini (plugin çağrısı, rate limit, cache vb.) belirler.
// Bu sayede Pool, iş mantığından bağımsızdır (Strategy Pattern).
type ProcessFunc func(ctx context.Context, item *queue.Item) Result

// Pool, yapılandırılabilir boyutta goroutine havuzudur.
// PriorityQueue'dan görev alır, ProcessFunc ile işler, sonuçları ResultCh'e gönderir.
type Pool struct {
	workers   int
	queue     *queue.PriorityQueue
	processFn ProcessFunc
	ResultCh  chan Result // Dışarıdan okunabilir sonuç kanalı
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPool, yeni bir worker pool oluşturur.
//   - workers: eşzamanlı çalışacak goroutine sayısı
//   - q: görevlerin alınacağı öncelikli kuyruk
//   - fn: her görev için çağrılacak işleme fonksiyonu
func NewPool(workers int, q *queue.PriorityQueue, fn ProcessFunc) *Pool {
	if workers <= 0 {
		workers = 5
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pool{
		workers:   workers,
		queue:     q,
		processFn: fn,
		ResultCh:  make(chan Result, 100),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start, worker goroutine'lerini başlatır.
func (p *Pool) Start() {
	log.Info().Int("workers", p.workers).Msg("Worker pool starting...")

	for i := 1; i <= p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop, tüm worker'ları güvenle durdurur (graceful shutdown).
// Kuyruktaki mevcut görevlerin bitmesini bekler.
func (p *Pool) Stop() {
	log.Info().Msg("Worker pool stopping...")
	p.cancel()  // Worker'lara dur sinyali
	p.wg.Wait() // Tüm worker'ların bitmesini bekle
	close(p.ResultCh)
	log.Info().Msg("Worker pool stopped")
}

// worker, tek bir worker goroutine'inin yaşam döngüsüdür.
// Kuyruktan görev alır → ProcessFunc çağırır → sonucu ResultCh'e gönderir.
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	log.Debug().Int("worker_id", id).Msg("Worker started")

	for {
		select {
		case <-p.ctx.Done():
			log.Debug().Int("worker_id", id).Msg("Worker stopped (context cancelled)")
			return

		case _, ok := <-p.queue.Notify:
			if !ok {
				// Kuyruk kapatılmış
				log.Debug().Int("worker_id", id).Msg("Worker stopped (queue closed)")
				return
			}

			// Kuyruktan görev al (birden fazla sinyal olabilir, boşsa atla)
			item, exists := p.queue.Dequeue()
			if !exists {
				continue
			}

			log.Debug().
				Int("worker_id", id).
				Str("task_id", item.ID).
				Str("plugin", item.PluginName).
				Str("priority", item.Priority.String()).
				Msg("Worker processing task")

			// Görevi işle
			result := p.processFn(p.ctx, item)
			result.TaskID = item.ID

			// Sonucu gönder (context iptal edildiyse durma garantisi)
			select {
			case p.ResultCh <- result:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// WorkerCount, pool'daki worker sayısını döndürür.
func (p *Pool) WorkerCount() int {
	return p.workers
}
