package engine

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/errors"
	"github.com/rs/zerolog/log"
)

type Task struct {
	InvestigationID string
	Target          string
	PluginName      string
}

// TaskResult, modülün çalıştıktan sonra orkestratöre döndürdüğü karnesidir.
type TaskResult struct {
	InvestigationID string
	PluginName      string
	Success         bool
	Error           error
	FindingsCount   int
}

// Engine, OSINT modüllerini çalıştıran, görevleri dağıtan ve sonuçları toplayan çekirdek motordur.
// Tüm bağımlılıklar dışarıdan enjekte edilir (DIP): FindingRepository + Registry + LifecycleManager.
type Engine struct {
	findingRepo domain.FindingRepository
	registry    *Registry
	lifecycle   *LifecycleManager
	maxWorkers  int
	taskQueue   chan Task
	resultQueue chan TaskResult
	wg          sync.WaitGroup
	resultWg    sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewEngine, motoru başlatır. Tüm bağımlılıklar dışarıdan enjekte edilir:
//   - findingRepo: bulguların veritabanına yazılması için repository interface'i
//   - registry: kayıtlı plugin'lerin merkezi defteri
//   - lifecycle: plugin yaşam döngüsü yöneticisi
//   - maxWorkers: eşzamanlı çalışacak worker sayısı
func NewEngine(findingRepo domain.FindingRepository, registry *Registry, lifecycle *LifecycleManager, maxWorkers int) *Engine {
	ctx, cancel := context.WithCancel(context.Background())

	if maxWorkers <= 0 {
		maxWorkers = 5
	}

	return &Engine{
		findingRepo: findingRepo,
		registry:    registry,
		lifecycle:   lifecycle,
		maxWorkers:  maxWorkers,
		taskQueue:   make(chan Task, 100),
		resultQueue: make(chan TaskResult, 100),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Registry, Engine'in kullandığı plugin registry'sine erişim sağlar.
func (e *Engine) Registry() *Registry {
	return e.registry
}

// Lifecycle, Engine'in kullandığı lifecycle manager'a erişim sağlar.
func (e *Engine) Lifecycle() *LifecycleManager {
	return e.lifecycle
}

func (e *Engine) Start() {
	log.Info().Int("workers", e.maxWorkers).Msg("Starting OSINT Engine worker pool...")

	// 1. Tüm kayıtlı plugin'leri aktif et
	e.lifecycle.ActivateAll()

	// 2. Sonuç İşleyiciyi (Orkestratör) Başlat
	e.resultWg.Add(1)
	go e.resultProcessor()

	// 3. İşçileri (Workers) Başlat
	for i := 1; i <= e.maxWorkers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
}

func (e *Engine) Stop() {
	log.Info().Msg("Stopping OSINT Engine... Waiting for active tasks to finish.")

	close(e.taskQueue) // 1. Dışarıdan yeni görev alımını durdur
	e.cancel()         // 2. Motorun iptal sinyalini yay
	e.wg.Wait()        // 3. İşçilerin (workers) ellerindeki son işleri bitirmesini bekle

	close(e.resultQueue) // 4. İşçiler bittiğine göre sonuç kanalını güvenle kapatabiliriz
	e.resultWg.Wait()    // 5. Orkestratörün son kayıtları veritabanına/loga yazmasını bekle

	// 6. Tüm plugin'leri deaktif et
	e.lifecycle.DeactivateAll()

	log.Info().Msg("OSINT Engine stopped safely.")
}

func (e *Engine) SubmitTask(t Task) error {
	select {
	case <-e.ctx.Done():
		return errors.Wrap(errors.TypeInternal, "engine is stopping, cannot accept new tasks", nil)
	default:
		e.taskQueue <- t
		return nil
	}
}

func (e *Engine) worker(id int) {
	defer e.wg.Done()
	log.Debug().Int("worker_id", id).Msg("Worker started")

	for task := range e.taskQueue {
		// İşçi görevini yapar ve karnesini (result) alır
		result := e.processTask(id, task)
		// Karneyi orkestratöre iletir (Artık hatalar yutulmuyor!)
		e.resultQueue <- result
	}

	log.Debug().Int("worker_id", id).Msg("Worker stopped")
}

// resultProcessor, kanaldan gelen görev sonuçlarını toplayan merkezi birimdir
func (e *Engine) resultProcessor() {
	defer e.resultWg.Done()
	log.Debug().Msg("Result processor started")

	for res := range e.resultQueue {
		if !res.Success {
			log.Warn().
				Str("investigation_id", res.InvestigationID).
				Str("plugin", res.PluginName).
				Err(res.Error).
				Msg("Task completed with failure")
		} else {
			log.Info().
				Str("investigation_id", res.InvestigationID).
				Str("plugin", res.PluginName).
				Int("findings", res.FindingsCount).
				Msg("Task completed successfully")
		}

		// TODO: P2.4'te burada InvestigationRepository üzerinden
		// araştırmanın 'status' alanını duruma göre 'Completed' veya 'Failed' yapacağız.
	}

	log.Debug().Msg("Result processor stopped")
}

// processTask artık sadece log basıp geçmiyor, geriye somut bir karne (TaskResult) döndürüyor
func (e *Engine) processTask(workerID int, task Task) TaskResult {
	log.Info().
		Int("worker_id", workerID).
		Str("investigation_id", task.InvestigationID).
		Str("target", task.Target).
		Str("plugin", task.PluginName).
		Msg("Processing task...")

	// Varsayılan olarak başarısız kabul edilen bir karne oluştur
	result := TaskResult{
		InvestigationID: task.InvestigationID,
		PluginName:      task.PluginName,
		Success:         false,
	}

	// Lifecycle kontrolü: plugin aktif değilse görevi reddet (izolasyon)
	if !e.lifecycle.IsActive(task.PluginName) {
		result.Error = errors.Wrap(errors.TypeInternal, "plugin is not active (possibly in error state)", nil)
		log.Warn().Str("plugin", task.PluginName).Msg("Task rejected — plugin is not active")
		return result
	}

	module, err := e.registry.Get(task.PluginName)
	if err != nil {
		result.Error = errors.Wrap(errors.TypeNotFound, "plugin not found", err)
		log.Error().Str("plugin", task.PluginName).Msg("Plugin not found, dropping task")
		return result
	}

	timeout := module.Timeout()
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	results, err := module.Run(ctx, task.Target)
	if err != nil {
		// Hatalı plugin'i izole et — çekirdek etkilenmez
		e.lifecycle.MarkError(task.PluginName, err)
		result.Error = errors.Wrap(errors.TypeInternal, "module execution failed", err)
		log.Error().Err(err).Str("plugin", task.PluginName).Msg("Module execution failed, plugin marked as error")
		return result
	}

	savedCount := 0
	for _, res := range results {
		// Ham sonucu (Result), veritabanına yazılacak Zengin Bulguya (Finding) çevir!
		f := &domain.Finding{
			ID:              uuid.New().String(),
			InvestigationID: task.InvestigationID,
			Type:            domain.FindingType(res.Type), // Türü eşle
			Value:           res.Value,
			Context:         res.Context,
			FoundBy:         module.Manifest().Name, // Manifest'ten ismi al
			CreatedAt:       time.Now().UTC(),
		}

		if err := e.findingRepo.Create(ctx, f); err != nil {
			log.Error().Err(err).Msg("Failed to save finding to database")
			continue
		}

		log.Debug().Str("finding_id", f.ID).Str("value", f.Value).Msg("Finding saved")
		savedCount++
	}

	// Her şey sorunsuzsa karneyi "Başarılı" olarak işaretle
	result.Success = true
	result.FindingsCount = savedCount
	return result
}
