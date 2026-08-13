package watch

import (
	"testing"
	"time"
)

func TestWatcher_AddRemove(t *testing.T) {
	w := NewWatcher()

	err := w.Add("example.com", "domain", 1*time.Hour)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Duplicate
	err = w.Add("example.com", "domain", 2*time.Hour)
	if err == nil {
		t.Error("expected error for duplicate add")
	}

	items := w.List()
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}

	id := "domain:example.com"
	err = w.Remove(id)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	items = w.List()
	if len(items) != 0 {
		t.Errorf("expected 0 items after remove, got %d", len(items))
	}
}

func TestWatcher_RemoveNonexistent(t *testing.T) {
	w := NewWatcher()
	err := w.Remove("fake-id")
	if err == nil {
		t.Error("expected error for removing nonexistent item")
	}
}

func TestWatcher_CheckItems(t *testing.T) {
	w := NewWatcher()

	// Kısa aralıklı görev
	if err := w.Add("test.com", "domain", 10*time.Millisecond); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	changeDetected := false
	w.SetOnChangeCallback(func(item *WatchItem, newHash string) {
		changeDetected = true
	})

	// İlk çalıştırma (değişiklik yok sayılır, sadece hash alınır)
	w.checkItems()

	items := w.List()
	if items[0].LastHash == "" {
		t.Error("expected LastHash to be populated after first run")
	}
	if items[0].LastRun.IsZero() {
		t.Error("expected LastRun to be populated after first run")
	}

	// Süre geçmesini bekle (10ms)
	time.Sleep(15 * time.Millisecond)

	// Simüle edilmiş hash değişikliğini yakalamak için item hash'ini manuel boz
	w.mu.Lock()
	w.items["domain:test.com"].LastHash = "old_hash_data"
	w.mu.Unlock()

	// İkinci çalıştırma (değişiklik tetiklenmeli)
	w.checkItems()

	if !changeDetected {
		t.Error("expected change detection callback to be called")
	}
}

func TestWatcher_StartStop(t *testing.T) {
	w := NewWatcher()
	w.Start()

	// Biraz çalışmasına izin ver
	time.Sleep(50 * time.Millisecond)

	w.Stop() // Block yapmamalı ve temiz kapanmalı
}
