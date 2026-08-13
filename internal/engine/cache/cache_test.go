package cache

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_cache.db")
	c, err := NewCache(dbPath, 1*time.Hour)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestCache_SetAndGet(t *testing.T) {
	c := newTestCache(t)

	key := MakeKey("shodan", "example.com")
	value := []byte(`{"results": []}`)

	if err := c.Set(key, value, 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if string(got) != string(value) {
		t.Errorf("expected %q, got %q", value, got)
	}
}

func TestCache_GetMiss(t *testing.T) {
	c := newTestCache(t)

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}

	stats := c.Stats()
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	c := newTestCache(t)

	key := "expiring-key"
	if err := c.Set(key, []byte("data"), 2*time.Second); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Hemen okunabilmeli
	_, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit before expiry")
	}

	// TTL sonrası okunammalı
	time.Sleep(3 * time.Second)
	_, ok = c.Get(key)
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestCache_Delete(t *testing.T) {
	c := newTestCache(t)

	key := "delete-me"
	_ = c.Set(key, []byte("data"), 0)

	if err := c.Delete(key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, ok := c.Get(key)
	if ok {
		t.Error("expected cache miss after delete")
	}
}

func TestCache_Purge(t *testing.T) {
	c := newTestCache(t)

	// Kısa TTL ile kayıt ekle
	_ = c.Set("short-1", []byte("a"), 2*time.Second)
	_ = c.Set("short-2", []byte("b"), 2*time.Second)
	_ = c.Set("long", []byte("c"), 1*time.Hour)

	time.Sleep(3 * time.Second)

	purged, err := c.Purge()
	if err != nil {
		t.Fatalf("Purge failed: %v", err)
	}
	if purged != 2 {
		t.Errorf("expected 2 purged, got %d", purged)
	}

	// Uzun TTL'li kayıt hâlâ okunabilmeli
	_, ok := c.Get("long")
	if !ok {
		t.Error("expected long-TTL entry to survive purge")
	}
}

func TestCache_Upsert(t *testing.T) {
	c := newTestCache(t)

	key := "upsert-key"
	_ = c.Set(key, []byte("v1"), 0)
	_ = c.Set(key, []byte("v2"), 0) // Üzerine yazmalı

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got) != "v2" {
		t.Errorf("expected v2 after upsert, got %q", got)
	}
}

func TestCache_Stats(t *testing.T) {
	c := newTestCache(t)

	_ = c.Set("key", []byte("val"), 0)
	c.Get("key")         // hit
	c.Get("key")         // hit
	c.Get("nonexistent") // miss

	stats := c.Stats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}

	rate := stats.HitRate()
	if rate < 66 || rate > 67 {
		t.Errorf("expected ~66.67%% hit rate, got %.2f%%", rate)
	}
}

func TestMakeKey(t *testing.T) {
	k1 := MakeKey("shodan", "example.com")
	k2 := MakeKey("shodan", "example.com")
	k3 := MakeKey("hibp", "example.com")

	if k1 != k2 {
		t.Error("same inputs should produce same key")
	}
	if k1 == k3 {
		t.Error("different plugin should produce different key")
	}
	if len(k1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("expected 64 char key, got %d", len(k1))
	}
}
