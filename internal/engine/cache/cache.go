package cache

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite sürücüsü
)

// Stats, cache istatistiklerini tutar.
type Stats struct {
	Hits   int64
	Misses int64
}

// HitRate, cache isabet oranını yüzde olarak döndürür.
func (s *Stats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// Cache, SQLite tabanlı key-value önbellek sistemidir.
// Sorgu sonuçlarını yapılandırılabilir TTL ile saklar.
type Cache struct {
	db         *sql.DB
	defaultTTL time.Duration
	hits       atomic.Int64
	misses     atomic.Int64
}

// NewCache, belirtilen dosya yolunda SQLite cache veritabanı oluşturur.
// defaultTTL, Set çağrılarında TTL belirtilmezse kullanılacak varsayılan süredir.
func NewCache(dbPath string, defaultTTL time.Duration) (*Cache, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("cache: failed to open database: %w", err)
	}

	// Cache tablosunu oluştur
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cache (
			key        TEXT PRIMARY KEY,
			value      BLOB NOT NULL,
			expires_at INTEGER NOT NULL,
			created_at INTEGER DEFAULT (strftime('%%s','now'))
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("cache: failed to create table: %w", err)
	}

	// Süresi dolmuş kayıtları temizleme indeksi
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache(expires_at)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("cache: failed to create index: %w", err)
	}

	if defaultTTL <= 0 {
		defaultTTL = 1 * time.Hour
	}

	return &Cache{
		db:         db,
		defaultTTL: defaultTTL,
	}, nil
}

// MakeKey, plugin adı ve hedeften deterministik cache key'i üretir.
// Format: SHA256(pluginName:target) — uzun hedeflerde bile sabit uzunluk.
func MakeKey(pluginName, target string) string {
	h := sha256.Sum256([]byte(pluginName + ":" + target))
	return fmt.Sprintf("%x", h)
}

// Get, cache'den bir değer okur. Süresi dolmuş kayıtlar döndürülmez.
// Cache hit → (value, true), miss veya expired → (nil, false).
func (c *Cache) Get(key string) ([]byte, bool) {
	var value []byte
	now := time.Now().Unix()
	err := c.db.QueryRow(
		`SELECT value FROM cache WHERE key = ? AND expires_at > ?`,
		key, now,
	).Scan(&value)

	if err != nil {
		c.misses.Add(1)
		return nil, false
	}

	c.hits.Add(1)
	return value, true
}

// Set, cache'e bir değer yazar. ttl <= 0 ise defaultTTL kullanılır.
// Aynı key varsa üzerine yazılır (UPSERT).
func (c *Cache) Set(key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	expiresAt := time.Now().Add(ttl).Unix()

	_, err := c.db.Exec(
		`INSERT INTO cache (key, value, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at`,
		key, value, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("cache: failed to set key %q: %w", key, err)
	}
	return nil
}

// Delete, cache'den bir kaydı siler.
func (c *Cache) Delete(key string) error {
	_, err := c.db.Exec(`DELETE FROM cache WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("cache: failed to delete key %q: %w", key, err)
	}
	return nil
}

// Purge, süresi dolmuş tüm kayıtları temizler.
// Periyodik olarak çağrılmalıdır (background goroutine ile).
func (c *Cache) Purge() (int64, error) {
	now := time.Now().Unix()
	result, err := c.db.Exec(`DELETE FROM cache WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, fmt.Errorf("cache: purge failed: %w", err)
	}
	return result.RowsAffected()
}

// Stats, mevcut cache istatistiklerini döndürür.
func (c *Cache) Stats() Stats {
	return Stats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
	}
}

// Close, cache veritabanını kapatır.
func (c *Cache) Close() error {
	return c.db.Close()
}
