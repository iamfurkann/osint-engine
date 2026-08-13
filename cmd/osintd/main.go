package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/iamfurkann/osint-engine/internal/config"
	"github.com/iamfurkann/osint-engine/internal/daemon"
	"github.com/iamfurkann/osint-engine/internal/db"
	"github.com/iamfurkann/osint-engine/internal/engine"
	"github.com/iamfurkann/osint-engine/internal/engine/orchestrator"
	"github.com/iamfurkann/osint-engine/internal/engine/retry"
	"github.com/iamfurkann/osint-engine/internal/engine/watch"
	"github.com/iamfurkann/osint-engine/internal/intel/plugins"
	"github.com/iamfurkann/osint-engine/internal/repository/sqlite"
	"github.com/iamfurkann/osint-engine/pkg/plugin"
	"github.com/iamfurkann/osint-engine/plugins/connectors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var version = "v0.1.0-dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "osintd",
		Short:   "OSINT Engine Daemon",
		Version: version,
	}

	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(stopCmd())
	rootCmd.AddCommand(statusCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Hata: %v\n", err)
		os.Exit(1)
	}
}

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Daemon'ı başlat",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
				With().Timestamp().Str("service", "osintd").Logger()

			// 1. Config Yükle
			appCfg, err := config.Load()
			if err != nil {
				log.Warn().Err(err).Msg("Yapılandırma dosyası okunamadı, varsayılanlar kullanılacak")
				appCfg = config.DefaultConfig(filepath.Join(os.ExpandEnv("$HOME"), ".osint"))
			}

			// 2. DB Bağlantısı
			dbPath := filepath.Join(os.ExpandEnv("$HOME"), ".osint", "findings.db")
			ctx := context.Background()
			database, err := db.Connect(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("veritabanı bağlantı hatası: %w", err)
			}
			defer database.Close()

			// Veritabanı tablolarını oluştur (Migrate)
			if err := database.Migrate(ctx); err != nil {
				return fmt.Errorf("veritabanı migrasyon hatası: %w", err)
			}

			repo := sqlite.NewFindingRepository(database)

			// 3. Engine & Registry
			registry := engine.NewRegistry()
			lifecycle := engine.NewLifecycleManager(registry)
			registry.SetLifecycle(lifecycle)

			// Eklentileri (Plugins) kaydet
			var vtKey, hunterKey string
			if appCfg != nil && appCfg.Keys != nil {
				vtKey = appCfg.Keys.Get("virustotal")
				hunterKey = appCfg.Keys.Get("hunter")
			}

			// Anahtar gerektirmeyen, tamamen ücretsiz kaynaklar.
			// Çekirdek YALNIZCA bunlarla tam çalışır — hiçbir API anahtarı
			// yapılandırılmasa bile sistem kullanılabilir durumdadır.
			toRegister := []plugin.Plugin{
				connectors.NewShodanInternetDB(),
				connectors.NewGravatar(),
				connectors.NewUsernameCheck(),
				connectors.NewDNSWhois(),
				connectors.NewCrtSh(),
				connectors.NewWebScraper(),
				connectors.NewSocialProfile(),
				&plugins.NameGeneratorPlugin{},
				connectors.NewWaybackPlugin(),
			}

			// Ücretsiz katmanı olan ama HESAP + API ANAHTARI isteyenler.
			//
			// Anahtar yapılandırılmamışsa connector HİÇ KAYDEDİLMEZ. Önceden
			// koşulsuz kaydediliyordu ve şu israfa yol açıyordu: görev kuyruğa
			// giriyor, worker alıyor, rate limiter bekletiyor, Run() "API
			// anahtarı gerekli" hatası veriyor, retry bunu 3 kez üstel
			// beklemeyle tekrarlıyor, sonuç başarısız görev olarak raporlanıyor.
			// Hiçbiri asla başarılı olamayacak bir durum için.
			keyed := []struct {
				name string
				key  string
				make func() plugin.Plugin
			}{
				{"virustotal", vtKey, func() plugin.Plugin { return connectors.NewVirusTotal(vtKey) }},
				{"hunter", hunterKey, func() plugin.Plugin { return connectors.NewHunter(hunterKey) }},
			}
			for _, k := range keyed {
				if k.key == "" {
					log.Info().
						Str("plugin", k.name).
						Msgf("API anahtarı yok — atlanıyor ('osint keys set %s <key>' ile ekleyebilirsiniz)", k.name)
					continue
				}
				toRegister = append(toRegister, k.make())
			}

			// Kayıt hataları artık YUTULMUYOR. Önceden dönüş değeri atılıyordu;
			// manifest doğrulaması başarısız olan bir connector sessizce
			// devre dışı kalıyor ve kullanıcı bunu asla öğrenemiyordu.
			for _, p := range toRegister {
				if err := registry.Register(p); err != nil {
					log.Error().Err(err).
						Str("plugin", p.Manifest().Name).
						Msg("Plugin kaydedilemedi — bu eklenti kullanılamayacak")
				}
			}

			// Tüm eklentileri aktif et (varsayılan olarak Inactive gelirler)
			for _, m := range registry.List() {
				if err := lifecycle.Activate(m.Name); err != nil {
					log.Error().Err(err).Str("plugin", m.Name).Msg("Plugin aktive edilemedi")
				}
			}

			// 4. Orchestrator
			deps := orchestrator.Deps{
				Registry:    registry,
				Lifecycle:   lifecycle,
				FindingRepo: repo,
				InvRepo:     sqlite.NewInvestigationRepository(database),
				RetryConfig: retry.DefaultConfig(),
				MaxWorkers:  10,
				DefaultRate: 1,
			}
			orch := orchestrator.NewOrchestrator(deps)
			orch.Start() // Worker'ları dinlemeye başlat

			// 5. Daemon
			watchRepo := sqlite.NewWatchRepository(database.DB)
			watcher := watch.NewWatcher(watchRepo, orch)

			cfg := daemon.DefaultConfig()
			d := daemon.New(cfg, appCfg, orch, watcher)

			if err := d.Start(); err != nil {
				return err
			}

			fmt.Printf("✅ Daemon başlatıldı (PID: %d)\n", os.Getpid())
			fmt.Printf("   Socket: %s\n", cfg.SocketPath)

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			sig := <-sigCh
			log.Info().Str("signal", sig.String()).Msg("Signal received, shutting down...")
			d.Stop()

			return nil
		},
	}
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Daemon'ı durdur",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := daemon.DefaultConfig()
			d := daemon.New(cfg, nil, nil, nil)

			if !d.IsRunning() {
				fmt.Println("Daemon çalışmıyor.")
				return nil
			}

			if err := d.SendStop(); err != nil {
				return err
			}

			fmt.Println("✅ Daemon durduruldu.")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Daemon durumunu gösterir",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := daemon.DefaultConfig()
			d := daemon.New(cfg, nil, nil, nil)

			if d.IsRunning() {
				fmt.Println("✅ Daemon çalışıyor.")
			} else {
				fmt.Println("❌ Daemon çalışmıyor.")
			}
			return nil
		},
	}
}
