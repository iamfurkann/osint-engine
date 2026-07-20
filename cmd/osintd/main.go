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
	cfg, database := app.Bootstrap(ctx, "osintd.log")
	defer func() { _ = database.Close() }()

	log.Info().Int("max_workers", cfg.Engine.MaxWorkers).Msg("OSINT Engine Daemon başlatıldı")

	fmt.Printf("OSINT Engine Daemon %s (Max Workers: %d)\n", cfg.Global.Version, cfg.Engine.MaxWorkers)
	fmt.Println("Veritabanı hazır ve dinlemede...")
}
