package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeystore_SetAndGet(t *testing.T) {
	dir := t.TempDir()
	masterKeyPath := filepath.Join(dir, "master.key")
	storePath := filepath.Join(dir, "api_keys.enc")

	ks, err := NewKeystore(masterKeyPath, storePath)
	if err != nil {
		t.Fatalf("NewKeystore failed: %v", err)
	}

	// 1. Set ile anahtar kaydet
	if err := ks.Set("shodan", "abc123-shodan-key"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// 2. Get ile anahtarı geri al
	val := ks.Get("shodan")
	if val != "abc123-shodan-key" {
		t.Errorf("expected 'abc123-shodan-key', got %q", val)
	}

	// 3. Olmayan bir anahtar boş dönmeli
	empty := ks.Get("nonexistent")
	if empty != "" {
		t.Errorf("expected empty string for nonexistent key, got %q", empty)
	}
}

func TestKeystore_PersistenceAcrossReloads(t *testing.T) {
	dir := t.TempDir()
	masterKeyPath := filepath.Join(dir, "master.key")
	storePath := filepath.Join(dir, "api_keys.enc")

	// İlk oturum: kaydet
	ks1, err := NewKeystore(masterKeyPath, storePath)
	if err != nil {
		t.Fatalf("NewKeystore (1st) failed: %v", err)
	}
	if err := ks1.Set("github", "ghp_testtoken"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// İkinci oturum: yeniden oluştur ve oku
	ks2, err := NewKeystore(masterKeyPath, storePath)
	if err != nil {
		t.Fatalf("NewKeystore (2nd) failed: %v", err)
	}

	val := ks2.Get("github")
	if val != "ghp_testtoken" {
		t.Errorf("expected 'ghp_testtoken' after reload, got %q", val)
	}
}

func TestKeystore_WrongMasterKey(t *testing.T) {
	dir := t.TempDir()
	masterKeyPath := filepath.Join(dir, "master.key")
	storePath := filepath.Join(dir, "api_keys.enc")

	// İlk master key ile kaydet
	ks, err := NewKeystore(masterKeyPath, storePath)
	if err != nil {
		t.Fatalf("NewKeystore failed: %v", err)
	}
	if err := ks.Set("test", "secret"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Master key dosyasını boz (farklı bir anahtar yaz)
	fakeKey := make([]byte, 32)
	fakeKey[0] = 0xFF
	if err := os.WriteFile(masterKeyPath, fakeKey, 0600); err != nil {
		t.Fatalf("failed to write fake key: %v", err)
	}

	// Yanlış master key ile açmayı dene — hata dönmeli
	_, err = NewKeystore(masterKeyPath, storePath)
	if err == nil {
		t.Fatal("expected error when loading with wrong master key, got nil")
	}
}

func TestKeystore_MultipleKeys(t *testing.T) {
	dir := t.TempDir()
	masterKeyPath := filepath.Join(dir, "master.key")
	storePath := filepath.Join(dir, "api_keys.enc")

	ks, err := NewKeystore(masterKeyPath, storePath)
	if err != nil {
		t.Fatalf("NewKeystore failed: %v", err)
	}

	keys := map[string]string{
		"shodan":     "shodan-key-123",
		"github":     "ghp_token_abc",
		"virustotal": "vt-api-key-xyz",
		"hunter.io":  "hunter-key-456",
	}

	for service, key := range keys {
		if err := ks.Set(service, key); err != nil {
			t.Fatalf("Set(%s) failed: %v", service, err)
		}
	}

	for service, expected := range keys {
		got := ks.Get(service)
		if got != expected {
			t.Errorf("Get(%s): expected %q, got %q", service, expected, got)
		}
	}
}
