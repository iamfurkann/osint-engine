package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iamfurkann/osint-engine/internal/config"
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

	// Daemon logları ayrı dosyaya yazılır
	if err := logger.Init(cfg.Global.LogLevel, logDir, "osintd.log"); err != nil {
		fmt.Fprintf(os.Stderr, "Logger başlatılamadı: %v\n", err)
		os.Exit(1)
	}

	log.Info().
		Str("version", cfg.Global.Version).
		Int("max_workers", cfg.Engine.MaxWorkers).
		Msg("OSINT Engine Daemon başlatıldı")

	fmt.Printf("OSINT Engine Daemon %s (Max Workers: %d)\n", cfg.Global.Version, cfg.Engine.MaxWorkers)
}
