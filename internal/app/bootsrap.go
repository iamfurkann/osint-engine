package app

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

// Bootstrap, uygulamanın çekirdek bileşenlerini (Config, Logger, DB) ayağa kaldırır.
func Bootstrap(ctx context.Context, logFilename string) (*config.Config, *db.DB) {
	// 1. Konfigürasyonu Yükle
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Konfigürasyon yüklenemedi: %v\n", err)
		os.Exit(1)
	}

	// 2. Logger'ı Başlat
	baseDir, _ := config.GetDefaultDir()
	logDir := filepath.Join(baseDir, "logs")

	if err := logger.Init(cfg.Global.LogLevel, logDir, logFilename); err != nil {
		fmt.Fprintf(os.Stderr, "Logger başlatılamadı: %v\n", err)
		os.Exit(1)
	}

	// 3. Veritabanına Bağlan
	database, err := db.Connect(ctx, cfg.Database.AppDBPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Veritabanına bağlanılamadı")
	}

	// 4. Migrasyonları Çalıştır
	if err := database.Migrate(ctx); err != nil {
		log.Fatal().Err(err).Msg("Veritabanı migrasyonu başarısız")
	}

	return cfg, database
}
