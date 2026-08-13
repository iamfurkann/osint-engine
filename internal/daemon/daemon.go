package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/iamfurkann/osint-engine/internal/api"
	"github.com/iamfurkann/osint-engine/internal/config"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/engine/orchestrator"
	"github.com/iamfurkann/osint-engine/internal/engine/watch"
	"github.com/iamfurkann/osint-engine/internal/input"

	"github.com/iamfurkann/osint-engine/internal/ipc"
	"github.com/rs/zerolog/log"
)

// Config, daemon yapılandırmasını tutar.
type Config struct {
	BaseDir    string // ~/.osint
	SocketPath string // BaseDir/osintd.sock
	PIDPath    string // BaseDir/osintd.pid
}

// DefaultConfig, varsayılan daemon yapılandırmasını döndürür.
func DefaultConfig() Config {
	baseDir := filepath.Join(os.ExpandEnv("$HOME"), ".osint")
	return Config{
		BaseDir:    baseDir,
		SocketPath: filepath.Join(baseDir, "osintd.sock"),
		PIDPath:    filepath.Join(baseDir, "osintd.pid"),
	}
}

// Daemon, arka plan sürecinin yönetimini sağlar.
type Daemon struct {
	config       Config
	ipcServer    *ipc.Server
	orchestrator *orchestrator.Orchestrator
	watcher      *watch.Watcher
	apiServer    *api.Server
}

// New, yeni bir daemon instance oluşturur.
func New(cfg Config, appCfg *config.Config, orch *orchestrator.Orchestrator, watcher *watch.Watcher) *Daemon {
	d := &Daemon{
		config:       cfg,
		ipcServer:    ipc.NewServer(cfg.SocketPath),
		orchestrator: orch,
		watcher:      watcher,
	}
	if orch != nil && watcher != nil {
		// Loopback'e bağlanır. Daha önce ":8080" idi, yani TÜM ağ arayüzleri —
		// kimlik doğrulaması olmayan bir API'yi yerel ağa (kafe/ofis Wi-Fi'ı dahil)
		// açıyordu. Bu API toplanmış tüm OSINT verisini okuyabiliyor.
		d.apiServer = api.NewServer("127.0.0.1:8080", appCfg, orch, watcher)
	}
	return d
}

// Start, daemon'ı başlatır: dizinleri oluşturur, PID yazar, IPC sunucusunu başlatır.
func (d *Daemon) Start() error {
	// Zaten çalışıyor mu?
	if d.IsRunning() {
		return fmt.Errorf("daemon zaten çalışıyor (PID: %d)", d.readPID())
	}

	// Dizinleri oluştur
	if err := os.MkdirAll(d.config.BaseDir, 0700); err != nil {
		return fmt.Errorf("daemon: dizin oluşturulamadı: %w", err)
	}

	// PID dosyasını yaz
	if err := d.writePID(); err != nil {
		return fmt.Errorf("daemon: PID yazılamadı: %w", err)
	}

	// Varsayılan handler'ları kaydet
	d.registerDefaultHandlers()

	// IPC sunucusunu başlat
	if err := d.ipcServer.Listen(); err != nil {
		d.removePID()
		return fmt.Errorf("daemon: IPC başlatılamadı: %w", err)
	}

	if d.watcher != nil {
		d.watcher.Start()
	}

	if d.apiServer != nil {
		if err := d.apiServer.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start API server")
		}
	}

	log.Info().
		Int("pid", os.Getpid()).
		Str("socket", d.config.SocketPath).
		Msg("Daemon started")

	return nil
}

// Stop, daemon'ı güvenle durdurur.
func (d *Daemon) Stop() {
	log.Info().Msg("Daemon stopping...")
	if d.apiServer != nil {
		d.apiServer.Stop()
	}
	if d.watcher != nil {
		d.watcher.Stop()
	}
	d.ipcServer.Shutdown()
	d.removePID()
	log.Info().Msg("Daemon stopped")
}

// IPCServer, IPC sunucusuna erişim sağlar (handler kaydetmek için).
func (d *Daemon) IPCServer() *ipc.Server {
	return d.ipcServer
}

// IsRunning, daemon'ın çalışıp çalışmadığını PID dosyasından kontrol eder.
func (d *Daemon) IsRunning() bool {
	pid := d.readPID()
	if pid <= 0 {
		return false
	}

	// PID'in gerçekten çalışıp çalışmadığını kontrol et
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 → süreç var mı kontrol (öldürmez)
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// SendStop, çalışan daemon'a SIGTERM gönderir.
func (d *Daemon) SendStop() error {
	pid := d.readPID()
	if pid <= 0 {
		return fmt.Errorf("daemon çalışmıyor (PID dosyası bulunamadı)")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("süreç bulunamadı (PID: %d): %w", pid, err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("SIGTERM gönderilemedi (PID: %d): %w", pid, err)
	}

	log.Info().Int("pid", pid).Msg("SIGTERM sent to daemon")
	return nil
}

// registerDefaultHandlers, temel IPC handler'larını kaydeder.
func (d *Daemon) registerDefaultHandlers() {
	d.ipcServer.RegisterHandler("ping", func(params json.RawMessage) (interface{}, error) {
		return map[string]interface{}{
			"status": "running",
			"pid":    os.Getpid(),
		}, nil
	})

	d.ipcServer.RegisterHandler("status", func(params json.RawMessage) (interface{}, error) {
		return map[string]interface{}{
			"status": "running",
			"pid":    os.Getpid(),
			"socket": d.config.SocketPath,
		}, nil
	})

	d.ipcServer.RegisterHandler("investigation.start", func(params json.RawMessage) (interface{}, error) {
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}

		target := req["target"]
		inputType := string(input.Detect(target))

		if inputType == string(input.TypeUnknown) {
			return nil, fmt.Errorf("bilinmeyen hedef formatı: %s", target)
		}

		// Basit bir UUID yerine target formatlı okunaklı bir ID
		invID := fmt.Sprintf("inv-%s-%d", strings.ReplaceAll(target, ".", "-"), time.Now().Unix())

		// Gerçek motoru tetikle
		recursive := req["recursive"] == "true"

		ctx := context.Background() // TODO: Lifecycle'a bağlanabilir
		if err := d.orchestrator.StartInvestigation(ctx, invID, target, inputType, recursive); err != nil {
			return nil, fmt.Errorf("araştırma başlatılamadı: %w", err)
		}

		return map[string]string{"investigation_id": invID}, nil
	})

	d.ipcServer.RegisterHandler("investigation.list", func(params json.RawMessage) (interface{}, error) {
		return d.orchestrator.ActiveInvestigations(), nil
	})

	d.ipcServer.RegisterHandler("investigation.status", func(params json.RawMessage) (interface{}, error) {
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		return d.orchestrator.GetProgress(req["id"])
	})

	d.ipcServer.RegisterHandler("investigation.pause", func(params json.RawMessage) (interface{}, error) {
		// Orchestrator'da henüz pause/resume yok. Gelecekte eklenecek.
		return nil, fmt.Errorf("pause not implemented yet")
	})

	d.ipcServer.RegisterHandler("investigation.resume", func(params json.RawMessage) (interface{}, error) {
		return nil, fmt.Errorf("resume not implemented yet")
	})

	d.ipcServer.RegisterHandler("investigation.cancel", func(params json.RawMessage) (interface{}, error) {
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("geçersiz parametreler: %w", err)
		}
		if err := d.orchestrator.Cancel(req["id"]); err != nil {
			return nil, err
		}
		return true, nil
	})

	d.ipcServer.RegisterHandler("investigation.graph", func(params json.RawMessage) (interface{}, error) {
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("geçersiz parametreler: %w", err)
		}

		g, entities, _, err := d.orchestrator.BuildGraph(context.Background(), req["id"])
		if err != nil {
			return nil, err
		}

		// Graf yapısını (adjacency list) Cytoscape veya GraphML'e uyumlu formata dök
		var nodes []map[string]interface{}
		var edges []map[string]interface{}

		for _, node := range g.Nodes() {
			nodes = append(nodes, map[string]interface{}{
				"data": map[string]interface{}{
					"id":         node.ID,
					"label":      node.Value,
					"type":       node.Type,
					"confidence": node.Confidence,
				},
			})
		}

		for _, edge := range g.Edges() {
			edges = append(edges, map[string]interface{}{
				"data": map[string]interface{}{
					"source":     edge.Source,
					"target":     edge.Target,
					"label":      edge.Type,
					"confidence": edge.Confidence,
				},
			})
		}

		return map[string]interface{}{
			"nodes":    nodes,
			"edges":    edges,
			"entities": entities, // Export veya raporlama için ek veri
		}, nil
	})

	d.ipcServer.RegisterHandler("investigation.report", func(params json.RawMessage) (interface{}, error) {
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("geçersiz parametreler: %w", err)
		}

		invID := req["id"]
		progress, err := d.orchestrator.GetProgress(invID)
		if err != nil {
			return nil, err
		}

		g, entities, correlations, err := d.orchestrator.BuildGraph(context.Background(), invID)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"InvestigationID": invID,
			"Target":          invID,     // TODO: hedefi progress veya DB'den okuyabiliriz
			"Status":          "running", // Progress.Percent == 100 ise completed hesaplanabilir
			"Progress":        progress.Percent,
			"GraphStats":      map[string]interface{}{"NodeCount": len(g.Nodes()), "EdgeCount": g.EdgeCount()},
			"Entities":        entities,
			"Correlations":    correlations,
		}, nil
	})

	d.ipcServer.RegisterHandler("watch.add", func(params json.RawMessage) (interface{}, error) {
		// Önceden burada map[string]interface{} üzerinde denetimsiz tip iddiaları
		// vardı (req["interval"].(float64) gibi). Handler'da recover() olmadığı
		// için TEK bir bozuk istek daemon'ın tamamını panikletiyordu.
		// Tiplenmiş struct ile hem panik hem sessiz sıfır-değer riski kalkıyor.
		var req struct {
			Target   string `json:"target"`
			Type     string `json:"type"`
			Interval int64  `json:"interval"` // nanosaniye
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("geçersiz parametreler: %w", err)
		}

		target := strings.TrimSpace(req.Target)
		typ := strings.TrimSpace(req.Type)
		intervalInt := req.Interval

		if target == "" {
			return nil, fmt.Errorf("'target' alanı zorunlu")
		}
		if typ == "" {
			return nil, fmt.Errorf("'type' alanı zorunlu")
		}
		if intervalInt <= 0 {
			return nil, fmt.Errorf("'interval' pozitif olmalı (nanosaniye), alınan: %d", intervalInt)
		}

		id := fmt.Sprintf("%s:%s", typ, target)

		item := &domain.WatchItem{
			ID:        id,
			Target:    target,
			Type:      typ,
			Interval:  time.Duration(intervalInt),
			LastRun:   time.Time{}, // Hiç çalışmadı
			CreatedAt: time.Now().UTC(),
		}

		err := d.watcher.Add(context.Background(), item)
		if err != nil {
			return nil, err
		}
		return true, nil
	})

	d.ipcServer.RegisterHandler("watch.list", func(params json.RawMessage) (interface{}, error) {
		items, err := d.watcher.List(context.Background())
		if err != nil {
			return nil, err
		}
		return items, nil
	})

	d.ipcServer.RegisterHandler("watch.remove", func(params json.RawMessage) (interface{}, error) {
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("geçersiz parametreler: %w", err)
		}
		err := d.watcher.Remove(context.Background(), req["id"])
		if err != nil {
			return nil, err
		}
		return true, nil
	})

}

// writePID, PID dosyasını yazar.
func (d *Daemon) writePID() error {
	return os.WriteFile(d.config.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0600)
}

// removePID, PID dosyasını siler.
func (d *Daemon) removePID() {
	os.Remove(d.config.PIDPath)
}

// readPID, PID dosyasından PID değerini okur.
func (d *Daemon) readPID() int {
	data, err := os.ReadFile(d.config.PIDPath)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}
