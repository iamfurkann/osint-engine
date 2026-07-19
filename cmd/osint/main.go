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
	// 1. Yapılandırmayı yükle
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Konfigürasyon yüklenemedi: %v\n", err)
		os.Exit(1)
	}

	// 2. Log dizinini belirle ve Loglayıcıyı başlat
	baseDir, _ := config.GetDefaultDir()
	logDir := filepath.Join(baseDir, "logs")

	if err := logger.Init(cfg.Global.LogLevel, logDir, "osint-cli.log"); err != nil {
		fmt.Fprintf(os.Stderr, "Logger başlatılamadı: %v\n", err)
		os.Exit(1)
	}

	// Log sistemini kullanarak ilk kaydımızı atıyoruz
	log.Info().Str("version", cfg.Global.Version).Msg("OSINT Engine CLI başlatıldı")

	// Kullanıcıya yönelik standart çıktı (stdout)
	fmt.Printf("OSINT Engine CLI %s (Log Level: %s)\n", cfg.Global.Version, cfg.Global.LogLevel)
}
