package main

import (
	"context"
	"fmt"

	"github.com/iamfurkann/osint-engine/internal/app"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx := context.Background()

	// Çekirdek bileşenleri ortak bootstrap üzerinden ayağa kaldırıyoruz
	cfg, database := app.Bootstrap(ctx, "osint-cli.log")
	defer func() { _ = database.Close() }()

	log.Info().Str("version", cfg.Global.Version).Msg("OSINT Engine CLI başlatıldı")

	fmt.Printf("OSINT Engine CLI %s (Log Level: %s)\n", cfg.Global.Version, cfg.Global.LogLevel)
	fmt.Println("Veritabanı bağlantısı ve şema kontrolleri başarılı.")
}
