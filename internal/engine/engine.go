package engine

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iamfurkann/osint-engine/internal/db"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/errors"
	"github.com/iamfurkann/osint-engine/internal/repository/sqlite"
	"github.com/rs/zerolog/log"
)

type Task struct {
	InvestigationID string
	Target          string
	PluginName      string
}

type Engine struct {
	db          *db.DB
	findingRepo domain.FindingRepository
	maxWorkers  int
	taskQueue   chan Task
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	modules     map[string]Module // Kayıtlı modüllerin tutulduğu sözlük
	moduleMutex sync.RWMutex      // Modül eklerken yarış durumlarını (race condition) önlemek için kilit
}

func NewEngine(database *db.DB, maxWorkers int) *Engine {
	ctx, cancel := context.WithCancel(context.Background())

	if maxWorkers <= 0 {
		maxWorkers = 5
	}

	return &Engine{
		db:          database,
		findingRepo: sqlite.NewFindingRepository(database), // P1.3'te yazdığımız repo
		maxWorkers:  maxWorkers,
		taskQueue:   make(chan Task, 100),
		ctx:         ctx,
		cancel:      cancel,
		modules:     make(map[string]Module),
	}
}

// RegisterModule, motora yeni bir OSINT eklentisi (plugin) tanıtır.
func (e *Engine) RegisterModule(m Module) {
	e.moduleMutex.Lock()
	defer e.moduleMutex.Unlock()
	e.modules[m.Name()] = m
	log.Info().Str("plugin", m.Name()).Msg("Module registered successfully")
}

func (e *Engine) Start() {
	log.Info().Int("workers", e.maxWorkers).Msg("Starting OSINT Engine worker pool...")

	for i := 1; i <= e.maxWorkers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
}

func (e *Engine) Stop() {
	log.Info().Msg("Stopping OSINT Engine... Waiting for active tasks to finish.")
	close(e.taskQueue)
	e.cancel()
	e.wg.Wait()
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
		e.processTask(id, task)
	}

	log.Debug().Int("worker_id", id).Msg("Worker stopped")
}

func (e *Engine) processTask(workerID int, task Task) {
	log.Info().
		Int("worker_id", workerID).
		Str("investigation_id", task.InvestigationID).
		Str("target", task.Target).
		Str("plugin", task.PluginName).
		Msg("Processing task...")

	// 1. Modülü bul (Okuma Kilidi)
	e.moduleMutex.RLock()
	module, exists := e.modules[task.PluginName]
	e.moduleMutex.RUnlock()

	if !exists {
		log.Error().Str("plugin", task.PluginName).Msg("Plugin not found, dropping task")
		return
	}

	// 2. Modülü çalıştır (3 dakikalık bir timeout veriyoruz)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	findings, err := module.Run(ctx, task.Target)
	if err != nil {
		log.Error().Err(err).Str("plugin", task.PluginName).Msg("Module execution failed")
		return
	}

	// 3. Çıkan sonuçları (Findings) veritabanına kaydet
	for _, f := range findings {
		// Güvenlik: Modül ID belirlememişse biz atıyoruz
		if f.ID == "" {
			f.ID = uuid.New().String()
		}
		f.InvestigationID = task.InvestigationID

		if err := e.findingRepo.Create(ctx, f); err != nil {
			log.Error().Err(err).Msg("Failed to save finding to database")
		} else {
			log.Debug().Str("finding_id", f.ID).Str("value", f.Value).Msg("Finding saved")
		}
	}
}
