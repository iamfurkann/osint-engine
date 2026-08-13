package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/engine"
	"github.com/iamfurkann/osint-engine/internal/engine/cache"
	"github.com/iamfurkann/osint-engine/internal/engine/queue"
	"github.com/iamfurkann/osint-engine/internal/engine/ratelimit"
	"github.com/iamfurkann/osint-engine/internal/engine/retry"
	"github.com/iamfurkann/osint-engine/internal/engine/worker"
	"github.com/iamfurkann/osint-engine/pkg/plugin"
	"github.com/rs/zerolog/log"
)

// Progress, bir araştırmanın ilerleme durumunu temsil eder.
//
// JSON etiketleri zorunlu: bu struct IPC üzerinden CLI'a gidiyor ve etiketsiz
// hâlde Go alan adlarıyla ("InvestigationID", "Percent") serileştiriliyordu.
// CLI ise snake_case bekliyordu, sonuç olarak `osint inv status` her alanı
// nil okuyup "%!s(<nil>)" basıyordu.
type Progress struct {
	InvestigationID string  `json:"investigation_id"`
	Total           int     `json:"total"`     // Toplam görev sayısı
	Completed       int     `json:"completed"` // Tamamlanan görev sayısı
	Failed          int     `json:"failed"`    // Başarısız görev sayısı
	Percent         float64 `json:"percent"`   // İlerleme yüzdesi
}

// investigationTracker, bir araştırmanın görev ilerlemesini takip eder.
type investigationTracker struct {
	total      int
	completed  int
	failed     int
	recursive  bool
	pivotCount int // Recursive modda oluşan pivot sayısı
	visited    map[string]bool
	cancel     context.CancelFunc
}

const maxPivots = 20 // Recursive modda maksimum pivot sayısı

// Deps, Orchestrator'ın dışarıdan aldığı bağımlılıklardır (DIP).
type Deps struct {
	Registry    *engine.Registry
	Lifecycle   *engine.LifecycleManager
	FindingRepo domain.FindingRepository
	InvRepo     domain.InvestigationRepository
	Cache       *cache.Cache // nil olabilir — cache devre dışı
	RetryConfig retry.Config
	MaxWorkers  int
	DefaultRate int // Rate limiter varsayılan hız
}

// Orchestrator, tüm araştırma bileşenlerini birleştiren üst katmandır.
// Girdi → tip tespiti → uygun plugin'leri belirleme → görev oluşturma →
// kuyruğa yazma → worker pool → rate limit → cache → retry → sonuç toplama.
type Orchestrator struct {
	registry     *engine.Registry
	lifecycle    *engine.LifecycleManager
	findingRepo  domain.FindingRepository
	invRepo      domain.InvestigationRepository
	cache        *cache.Cache
	retryConfig  retry.Config
	queue        *queue.PriorityQueue
	pool         *worker.Pool
	rateLimiters *ratelimit.PluginLimiters
	trackers     map[string]*investigationTracker
	trackerMu    sync.RWMutex
	resultWg     sync.WaitGroup
}

// NewOrchestrator, tüm bağımlılıkları alarak orchestrator oluşturur.
func NewOrchestrator(deps Deps) *Orchestrator {
	if deps.DefaultRate <= 0 {
		deps.DefaultRate = 10
	}
	if deps.MaxWorkers <= 0 {
		deps.MaxWorkers = 10
	}

	q := queue.NewPriorityQueue()
	rl := ratelimit.NewPluginLimiters(deps.DefaultRate)

	o := &Orchestrator{
		registry:     deps.Registry,
		lifecycle:    deps.Lifecycle,
		findingRepo:  deps.FindingRepo,
		invRepo:      deps.InvRepo,
		cache:        deps.Cache,
		retryConfig:  deps.RetryConfig,
		queue:        q,
		rateLimiters: rl,
		trackers:     make(map[string]*investigationTracker),
	}

	// Worker pool oluştur — processFunc olarak orchestrator'ın kendi metodunu ver
	o.pool = worker.NewPool(deps.MaxWorkers, q, o.processTask)

	return o
}

// Start, orchestrator'ı başlatır (worker pool + result collector).
func (o *Orchestrator) Start() {
	log.Info().Msg("Orchestrator starting...")
	o.pool.Start()

	// Sonuç toplayıcı
	o.resultWg.Add(1)
	go o.collectResults()

	log.Info().Msg("Orchestrator started")
}

// Stop, orchestrator'ı güvenle durdurur.
func (o *Orchestrator) Stop() {
	log.Info().Msg("Orchestrator stopping...")
	o.queue.Close()
	o.pool.Stop()
	o.resultWg.Wait()
	log.Info().Msg("Orchestrator stopped")
}

// StartInvestigation, yeni bir araştırma başlatır.
// recursive = true ise, çıkan sonuçlar da otomatik olarak yeni aramaları tetikler.
func (o *Orchestrator) StartInvestigation(ctx context.Context, investigationID, target, inputType string, recursive bool) error {
	// Girdi tipine uygun aktif plugin'leri bul
	manifests := o.registry.List()
	var matchingPlugins []plugin.Manifest

	for _, m := range manifests {
		// Plugin aktif mi?
		if !o.lifecycle.IsActive(m.Name) {
			continue
		}

		// Girdi tipini destekliyor mu?
		for _, input := range m.Inputs {
			if input == inputType {
				matchingPlugins = append(matchingPlugins, m)
				break
			}
		}
	}

	if len(matchingPlugins) == 0 {
		return fmt.Errorf("orchestrator: no active plugins found for input type %q", inputType)
	}

	// Veritabanına kaydet
	if o.invRepo != nil {
		inv := &domain.Investigation{
			ID:        investigationID,
			Name:      target,
			Status:    domain.StatusActive,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := o.invRepo.Create(ctx, inv); err != nil {
			return fmt.Errorf("orchestrator: failed to create investigation in db: %w", err)
		}
	}

	// Tracker oluştur
	invCtx, cancel := context.WithCancel(ctx)
	_ = invCtx // Gelecekte kullanılacak

	o.trackerMu.Lock()
	o.trackers[investigationID] = &investigationTracker{
		total:     len(matchingPlugins),
		recursive: recursive,
		visited:   make(map[string]bool),
		cancel:    cancel,
	}
	// Hedefin işlendiği pluginleri işaretleyelim ki sonsuz döngü olmasın
	for _, m := range matchingPlugins {
		o.trackers[investigationID].visited[target+":"+m.Name] = true
	}
	o.trackerMu.Unlock()

	// Her plugin için görev kuyruğa ekle
	for _, m := range matchingPlugins {
		o.queue.Enqueue(investigationID, target, m.Name, queue.PriorityNormal)
		log.Info().
			Str("investigation_id", investigationID).
			Str("plugin", m.Name).
			Str("target", target).
			Msg("Task enqueued")
	}

	return nil
}

// EnqueuePivot, recursive mod açıkken yeni bulgulardan yeni taramalar türetir.
func (o *Orchestrator) EnqueuePivot(investigationID, target, inputType string) {
	o.trackerMu.Lock()
	tracker, exists := o.trackers[investigationID]
	if !exists {
		o.trackerMu.Unlock()
		return
	}

	if !tracker.recursive || tracker.pivotCount >= maxPivots {
		o.trackerMu.Unlock()
		return
	}

	// Bu tip için pluginleri bul ve kuyruğa ekle
	manifests := o.registry.List()
	for _, m := range manifests {
		if !o.lifecycle.IsActive(m.Name) {
			continue
		}

		match := false
		for _, inType := range m.Inputs {
			if inType == inputType {
				match = true
				break
			}
		}

		if match {
			// Hedef ve Plugin ikilisini kontrol et
			visitKey := target + ":" + m.Name
			if tracker.visited[visitKey] {
				continue
			}

			// İşaretle
			tracker.visited[visitKey] = true
			tracker.total++
			tracker.pivotCount++

			o.queue.Enqueue(investigationID, target, m.Name, queue.PriorityNormal)
			log.Info().
				Str("investigation_id", investigationID).
				Str("plugin", m.Name).
				Str("target", target).
				Msg("Pivot task enqueued")
		}
	}

	o.trackerMu.Unlock()
}

// ListInvestigations, tüm araştırmaları listeler.
func (o *Orchestrator) ListInvestigations(ctx context.Context) ([]*domain.Investigation, error) {
	if o.invRepo == nil {
		return nil, fmt.Errorf("investigation repository is not configured")
	}
	return o.invRepo.List(ctx)
}

// GetProgress, bir araştırmanın ilerleme durumunu döndürür.
func (o *Orchestrator) GetProgress(investigationID string) (Progress, error) {
	o.trackerMu.RLock()
	defer o.trackerMu.RUnlock()

	tracker, exists := o.trackers[investigationID]
	if !exists {
		// Bellekte yoksa veritabanından kontrol et (eğer tamamlanmış ve geçmişte kalmışsa)
		if o.invRepo != nil {
			inv, err := o.invRepo.GetByID(context.Background(), investigationID)
			if err == nil && inv != nil {
				return Progress{
					InvestigationID: investigationID,
					Total:           1,
					Completed:       1,
					Failed:          0,
					Percent:         100.0,
				}, nil
			}
		}
		return Progress{}, fmt.Errorf("orchestrator: investigation %q not found", investigationID)
	}

	completed := tracker.completed + tracker.failed
	percent := 0.0
	if tracker.total > 0 {
		percent = float64(completed) / float64(tracker.total) * 100
	}

	return Progress{
		InvestigationID: investigationID,
		Total:           tracker.total,
		Completed:       tracker.completed,
		Failed:          tracker.failed,
		Percent:         percent,
	}, nil
}

// Cancel, bir araştırmayı iptal eder.
func (o *Orchestrator) Cancel(investigationID string) error {
	o.trackerMu.RLock()
	tracker, exists := o.trackers[investigationID]
	o.trackerMu.RUnlock()

	if !exists {
		return fmt.Errorf("orchestrator: investigation %q not found", investigationID)
	}

	tracker.cancel()
	log.Info().Str("investigation_id", investigationID).Msg("Investigation cancelled")
	return nil
}

// processTask, worker pool'un her görev için çağırdığı fonksiyondur.
// Rate limit → Cache kontrol → Plugin çalıştır (retry ile) → Sonuçları DB'ye yaz.
func (o *Orchestrator) processTask(ctx context.Context, item *queue.Item) worker.Result {
	result := worker.Result{
		InvestigationID: item.InvestigationID,
		PluginName:      item.PluginName,
	}

	// 1. Plugin'i registry'den al
	p, err := o.registry.Get(item.PluginName)
	if err != nil {
		result.Error = fmt.Errorf("plugin not found: %w", err)
		return result
	}

	// 2. Rate limit bekle
	limiter := o.rateLimiters.GetOrCreate(item.PluginName, p.Manifest().RateLimit)
	if err := limiter.Wait(ctx); err != nil {
		result.Error = fmt.Errorf("rate limit wait cancelled: %w", err)
		return result
	}

	// 3. Cache kontrol
	cacheKey := ""
	if o.cache != nil {
		cacheKey = cache.MakeKey(item.PluginName, item.Target)
		if cached, ok := o.cache.Get(cacheKey); ok {
			// Cache hit — sonuçları decode et
			var cachedResults []plugin.Result
			if err := json.Unmarshal(cached, &cachedResults); err == nil {
				savedCount := o.saveFindings(ctx, item, p, cachedResults)
				result.Success = true
				result.FindingsCount = savedCount
				log.Debug().Str("plugin", item.PluginName).Msg("Result served from cache")
				return result
			}
			// Decode hatası → cache'i yoksay, tekrar çalıştır
		}
	}

	// 4. Plugin'i çalıştır (retry ile)
	var pluginResults []plugin.Result
	err = retry.Do(ctx, o.retryConfig, func(ctx context.Context) error {
		timeout := p.Timeout()
		if timeout <= 0 {
			timeout = 3 * time.Minute
		}

		taskCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		var runErr error
		pluginResults, runErr = p.Run(taskCtx, item.Target)
		return runErr
	})

	if err != nil {
		// Hatalı plugin'i izole et
		o.lifecycle.MarkError(item.PluginName, err)
		result.Error = err
		return result
	}

	// 5. Sonuçları cache'e yaz
	if o.cache != nil && cacheKey != "" && len(pluginResults) > 0 {
		if data, err := json.Marshal(pluginResults); err == nil {
			_ = o.cache.Set(cacheKey, data, 0) // defaultTTL kullanılır
		}
	}

	// 6. Sonuçları DB'ye kaydet
	savedCount := o.saveFindings(ctx, item, p, pluginResults)

	// Başarılı çalıştırma ardışık hata sayacını sıfırlar; böylece uzun süre
	// çalışan bir daemon'da aralıklı ağ hataları birikip eşiği doldurmaz.
	o.lifecycle.MarkSuccess(item.PluginName)

	result.Success = true
	result.FindingsCount = savedCount

	return result
}

// saveFindings, plugin sonuçlarını Finding'e dönüştürüp DB'ye yazar.
func (o *Orchestrator) saveFindings(ctx context.Context, item *queue.Item, p plugin.Plugin, results []plugin.Result) int {
	savedCount := 0
	for _, res := range results {
		f := &domain.Finding{
			ID:              uuid.New().String(),
			InvestigationID: item.InvestigationID,
			Type:            domain.FindingType(res.Type),
			Value:           res.Value,
			Context:         res.Context,
			FoundBy:         p.Manifest().Name,
			Confidence:      p.Manifest().Confidence,
			CreatedAt:       time.Now().UTC(),
		}

		if err := o.findingRepo.Create(ctx, f); err != nil {
			log.Error().Err(err).Str("finding_id", f.ID).Msg("Failed to save finding")
			continue
		}

		// Recursive mod açıksa yeni taramalar türet. Bulgu tipi ile plugin
		// girdi tipi farklı sözlükler kullandığı için eşleme şart (pivot.go).
		for _, pt := range pivotTargets(string(res.Type), res.Value) {
			o.EnqueuePivot(item.InvestigationID, pt.Target, pt.InputType)
		}

		savedCount++
	}
	return savedCount
}

// collectResults, worker pool'dan gelen sonuçları toplar ve tracker'ları günceller.
func (o *Orchestrator) collectResults() {
	defer o.resultWg.Done()

	for result := range o.pool.ResultCh {
		o.trackerMu.Lock()
		if tracker, exists := o.trackers[result.InvestigationID]; exists {
			if result.Success {
				tracker.completed++
			} else {
				tracker.failed++
			}
		}
		o.trackerMu.Unlock()

		if result.Success {
			log.Info().
				Str("investigation_id", result.InvestigationID).
				Str("plugin", result.PluginName).
				Int("findings", result.FindingsCount).
				Msg("Task completed successfully")
		} else {
			log.Warn().
				Str("investigation_id", result.InvestigationID).
				Str("plugin", result.PluginName).
				Err(result.Error).
				Msg("Task completed with failure")
		}
	}
}
