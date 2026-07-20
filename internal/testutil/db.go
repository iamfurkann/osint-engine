package testutil

import (
	"context"
	"testing"

	"github.com/iamfurkann/osint-engine/internal/db"
)

// SetupTestDB, testler için izole bir SQLite veritabanı kurar ve test bitiminde temizler.
func SetupTestDB(t *testing.T) *db.DB {
	t.Helper()
	dbPath := TempSQLiteDBPath(t)
	ctx := context.Background()

	database, err := db.Connect(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// Tabloları oluşturmak için migrasyonu çalıştır
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}
