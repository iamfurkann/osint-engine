package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TaskState, bir görevin checkpoint'taki durumunu temsil eder.
type TaskState struct {
	PluginName string `json:"plugin_name"`
	Target     string `json:"target"`
	Status     string `json:"status"` // "completed", "pending", "failed"
}

// Checkpoint, bir araştırmanın anlık durumudur.
type Checkpoint struct {
	InvestigationID string      `json:"investigation_id"`
	CompletedTasks  []string    `json:"completed_tasks"` // Tamamlanan görev ID'leri
	PendingTasks    []TaskState `json:"pending_tasks"`   // Bekleyen görevler
	FailedTasks     []TaskState `json:"failed_tasks"`    // Başarısız görevler
	Progress        float64     `json:"progress"`        // İlerleme yüzdesi
	Timestamp       time.Time   `json:"timestamp"`
}

// Manager, araştırma checkpoint'larını yöneten yapıdır.
type Manager struct {
	dir      string        // Checkpoint dizini
	interval time.Duration // Otomatik kayıt aralığı
	data     map[string]*Checkpoint
	mu       sync.RWMutex
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewManager, yeni bir checkpoint manager oluşturur.
func NewManager(dir string, interval time.Duration) (*Manager, error) {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("checkpoint: dizin oluşturulamadı: %w", err)
	}

	m := &Manager{
		dir:      dir,
		interval: interval,
		data:     make(map[string]*Checkpoint),
		stopCh:   make(chan struct{}),
	}

	return m, nil
}

// StartAutoSave, periyodik checkpoint kayıt döngüsünü başlatır.
func (m *Manager) StartAutoSave() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.SaveAll()
			case <-m.stopCh:
				m.SaveAll() // Son bir kez kaydet
				return
			}
		}
	}()
	log.Debug().Dur("interval", m.interval).Msg("Checkpoint auto-save started")
}

// Stop, auto-save döngüsünü durdurur.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// Update, bir araştırmanın checkpoint'ını günceller.
func (m *Manager) Update(cp *Checkpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp.Timestamp = time.Now().UTC()
	m.data[cp.InvestigationID] = cp
}

// MarkTaskCompleted, bir görevi tamamlandı olarak işaretler.
func (m *Manager) MarkTaskCompleted(investigationID, taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp, ok := m.data[investigationID]
	if !ok {
		cp = &Checkpoint{InvestigationID: investigationID}
		m.data[investigationID] = cp
	}

	// Tekrar eklemeyi önle
	for _, id := range cp.CompletedTasks {
		if id == taskID {
			return
		}
	}
	cp.CompletedTasks = append(cp.CompletedTasks, taskID)
	cp.Timestamp = time.Now().UTC()
}

// Save, tek bir araştırmanın checkpoint'ını diske yazar.
func (m *Manager) Save(investigationID string) error {
	m.mu.RLock()
	cp, ok := m.data[investigationID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("checkpoint: investigation %q not found", investigationID)
	}

	return m.writeToDisk(cp)
}

// SaveAll, tüm checkpoint'ları diske yazar.
func (m *Manager) SaveAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cp := range m.data {
		if err := m.writeToDisk(cp); err != nil {
			log.Error().Err(err).Str("investigation_id", cp.InvestigationID).Msg("Checkpoint save failed")
		}
	}
}

// Load, diskten bir araştırmanın checkpoint'ını yükler.
func (m *Manager) Load(investigationID string) (*Checkpoint, error) {
	filePath := m.filePath(investigationID)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("checkpoint: %q bulunamadı", investigationID)
		}
		return nil, fmt.Errorf("checkpoint: okuma hatası: %w", err)
	}

	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("checkpoint: parse hatası: %w", err)
	}

	// Belleğe de yükle
	m.mu.Lock()
	m.data[investigationID] = &cp
	m.mu.Unlock()

	return &cp, nil
}

// RecoverAll, diskteki tüm checkpoint'ları yükler (crash recovery).
func (m *Manager) RecoverAll() ([]*Checkpoint, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: dizin okunamadı: %w", err)
	}

	var checkpoints []*Checkpoint
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(m.dir, entry.Name()))
		if err != nil {
			continue
		}

		var cp Checkpoint
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}

		m.mu.Lock()
		m.data[cp.InvestigationID] = &cp
		m.mu.Unlock()

		checkpoints = append(checkpoints, &cp)
	}

	log.Info().Int("count", len(checkpoints)).Msg("Checkpoints recovered")
	return checkpoints, nil
}

// Remove, bir araştırmanın checkpoint'ını siler (tamamlandığında).
func (m *Manager) Remove(investigationID string) {
	m.mu.Lock()
	delete(m.data, investigationID)
	m.mu.Unlock()

	os.Remove(m.filePath(investigationID))
}

// --- Internal ---

func (m *Manager) writeToDisk(cp *Checkpoint) error {
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath(cp.InvestigationID), data, 0600)
}

func (m *Manager) filePath(investigationID string) string {
	return filepath.Join(m.dir, investigationID+".json")
}
