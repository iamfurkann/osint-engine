package db

import (
	"context"
	"fmt"

	"github.com/iamfurkann/osint-engine/internal/errors"
	"github.com/rs/zerolog/log"
)

// migrations, sırasıyla çalıştırılacak SQL şema güncellemelerini tutar.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS _schema_migrations (version INTEGER PRIMARY KEY);`, // V0: Migrasyon takip tablosu
	`CREATE TABLE IF NOT EXISTS investigations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`, // V1: Araştırmalar tablosu
	`CREATE TABLE IF NOT EXISTS findings (
		id TEXT PRIMARY KEY,
		investigation_id TEXT NOT NULL,
		type TEXT NOT NULL,
		value TEXT NOT NULL,
		context TEXT,
		found_by TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(investigation_id) REFERENCES investigations(id) ON DELETE CASCADE
	);`, // V2: Bulgular tablosu
	`CREATE TABLE IF NOT EXISTS plugins (
		name TEXT PRIMARY KEY,
		description TEXT,
		version TEXT NOT NULL,
		status TEXT NOT NULL,
		language TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`, // V3: Eklentiler (Plugins) tablosu
	`CREATE TABLE IF NOT EXISTS watchlist (
		id TEXT PRIMARY KEY,
		target TEXT NOT NULL,
		type TEXT NOT NULL,
		interval INTEGER NOT NULL,
		last_run DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`, // V4: İzleme Listesi (Watchlist) tablosu
	`ALTER TABLE findings ADD COLUMN confidence INTEGER DEFAULT 0;`, // V5: Güven puanı sütunu
}

// Migrate, veritabanını en güncel şema versiyonuna yükseltir.
func (db *DB) Migrate(ctx context.Context) error {
	// Önce migrasyon takip tablosunu oluştur (eğer yoksa)
	if _, err := db.ExecContext(ctx, migrations[0]); err != nil {
		return errors.Wrap(errors.TypeInternal, "failed to create schema tracking table", err)
	}

	var currentVersion int
	err := db.QueryRowContext(ctx, "SELECT MAX(version) FROM _schema_migrations").Scan(&currentVersion)
	if err != nil {
		// Tablo boşsa (ilk kurulum), currentVersion 0 kalır.
		currentVersion = 0
	}

	// Eksik migrasyonları sırasıyla çalıştır
	for i := currentVersion + 1; i < len(migrations); i++ {
		log.Info().Int("version", i).Msg("Running database migration...")

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return errors.Wrap(errors.TypeInternal, "failed to begin migration transaction", err)
		}

		// SQL'i çalıştır
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			_ = tx.Rollback()
			return errors.Wrap(errors.TypeInternal, fmt.Sprintf("failed to execute migration v%d", i), err)
		}

		// Versiyonu kaydet
		if _, err := tx.ExecContext(ctx, "INSERT INTO _schema_migrations (version) VALUES (?)", i); err != nil {
			_ = tx.Rollback()
			return errors.Wrap(errors.TypeInternal, "failed to record migration version", err)
		}

		if err := tx.Commit(); err != nil {
			return errors.Wrap(errors.TypeInternal, "failed to commit migration", err)
		}
		log.Info().Int("version", i).Msg("Migration applied successfully")
	}

	return nil
}
