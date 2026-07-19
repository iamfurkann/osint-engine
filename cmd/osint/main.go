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

	if err := logger.Init(cfg.Global.LogLevel, logDir, "osint-cli.log"); err != nil {
		fmt.Fprintf(os.Stderr, "Logger başlatılamadı: %v\n", err)
		os.Exit(1)
	}

	log.Info().Str("version", cfg.Global.Version).Msg("OSINT Engine CLI başlatıldı")

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

	fmt.Printf("OSINT Engine CLI %s (Log Level: %s)\n", cfg.Global.Version, cfg.Global.LogLevel)
	fmt.Println("Veritabanı bağlantısı ve şema kontrolleri başarılı.")
}
