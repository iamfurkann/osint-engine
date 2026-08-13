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
	// WAL modu: Çoklu okuma + Tek yazmayı donanım seviyesinde destekler.
	// busy_timeout(5000): Yazma kuyruğunda kilitlenme olursa Go goroutine'inin hata fırlatmadan önce 5 saniye beklemesini sağlar.
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, errors.Wrap(errors.TypeInternal, "failed to open database", err)
	}

	// Connection Pool (Bağlantı Havuzu) Ayarları
	// WAL modunun sunduğu eşzamanlı okuma avantajından yararlanmak için bağlantı üst sınırını artırıyoruz.
	// 4 ila 8 arası bağlantı, OSINT Engine gibi yoğun okuma/yazma yapan dağıtık yapılarda optimal performansı sağlar.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4) // Boştaki bağlantıları tamamen yok etmeyip 4 adet hazırda tutarak el sıkışma (handshake) maliyetini azaltıyoruz.
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
