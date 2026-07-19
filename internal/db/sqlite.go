package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/iamfurkann/osint-engine/internal/errors"
	_ "modernc.org/sqlite" // Pure Go SQLite sürücüsü
)

// DB, standart sql.DB nesnesini sarmalar (ileride özel metodlar ekleyebilmek için).
type DB struct {
	*sql.DB
}

// Connect, belirtilen dosya yolundaki SQLite veritabanına bağlanır ve ayarlarını yapar.
func Connect(ctx context.Context, dbPath string) (*DB, error) {
	// Veritabanı dosya yolu parametreleri (Timeout ve WAL modu için)
	// modernc.org/sqlite için DSN formatı
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, errors.Wrap(errors.TypeInternal, "failed to open database", err)
	}

	// Connection Pool (Bağlantı Havuzu) Ayarları
	// SQLite'da yazma işlemlerinin kilitlenmemesi için MaxOpenConns genellikle 1 tutulur.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	// Bağlantının gerçekten sağlanıp sağlanmadığını test et
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, errors.Wrap(errors.TypeInternal, "failed to ping database", err)
	}

	return &DB{DB: db}, nil
}

// Close, veritabanı bağlantısını güvenli bir şekilde kapatır.
func (db *DB) Close() error {
	return db.DB.Close()
}
