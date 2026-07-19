package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iamfurkann/osint-engine/internal/config"
	"github.com/iamfurkann/osint-engine/internal/db"
	"github.com/iamfurkann/osint-engine/internal/logger"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Konfigürasyon yüklenemedi: %v\n", err)
		os.Exit(1)
	}

	baseDir, _ := config.GetDefaultDir()
	logDir := filepath.Join(baseDir, "logs")

	if err := logger.Init(cfg.Global.LogLevel, logDir, "osintd.log"); err != nil {
		fmt.Fprintf(os.Stderr, "Logger başlatılamadı: %v\n", err)
		os.Exit(1)
	}

	log.Info().Int("max_workers", cfg.Engine.MaxWorkers).Msg("OSINT Engine Daemon başlatıldı")

	// --- VERİTABANI BAĞLANTISI ---
	ctx := context.Background()
	database, err := db.Connect(ctx, cfg.Database.AppDBPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Veritabanına bağlanılamadı")
	}
	defer func() { _ = database.Close() }()

	if err := database.Migrate(ctx); err != nil {
		log.Fatal().Err(err).Msg("Veritabanı migrasyonu başarısız")
	}
	// -----------------------------

	fmt.Printf("OSINT Engine Daemon %s (Max Workers: %d)\n", cfg.Global.Version, cfg.Engine.MaxWorkers)
	fmt.Println("Veritabanı hazır ve dinlemede...")
}
