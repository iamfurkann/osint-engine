package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	baseDir := "/tmp/.osint"
	cfg := DefaultConfig(baseDir)

	if cfg.Global.LogLevel != "info" {
		t.Errorf("expected log level info, got %s", cfg.Global.LogLevel)
	}

	expectedDB := filepath.Join(baseDir, "osint.db")
	if cfg.Database.AppDBPath != expectedDB {
		t.Errorf("expected app db path %s, got %s", expectedDB, cfg.Database.AppDBPath)
	}
}

func TestEnvOverrides(t *testing.T) {
	baseDir := "/tmp/.osint"
	cfg := DefaultConfig(baseDir)

	_ = os.Setenv("OSINT_LOG_LEVEL", "debug")
	_ = os.Setenv("OSINT_MAX_WORKERS", "25")
	defer func() {
		_ = os.Unsetenv("OSINT_LOG_LEVEL")
		_ = os.Unsetenv("OSINT_MAX_WORKERS")
	}()

	cfg.applyEnvOverrides()

	if cfg.Global.LogLevel != "debug" {
		t.Errorf("expected overridden log level debug, got %s", cfg.Global.LogLevel)
	}

	if cfg.Engine.MaxWorkers != 25 {
		t.Errorf("expected overridden max workers 25, got %d", cfg.Engine.MaxWorkers)
	}
}

func TestLoad(t *testing.T) {
	// 1. Gerçek ~/.osint klasörünü kirletmemek için geçici bir izole klasör oluştur
	tempHome := t.TempDir()

	// 2. İşletim sisteminin HOME (Linux/Mac) ve USERPROFILE (Windows) değişkenlerini geçici klasöre yönlendir
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tempHome)
	os.Setenv("USERPROFILE", tempHome)

	// Test bitince ortam değişkenlerini eski (gerçek) haline geri getir
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalUserProfile)
	}()

	// 3. İLK YÜKLEME (Dosyalar Yokken): Sıfırdan oluşturulmasını test et
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed on first call: %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected config, got nil")
	}

	// Gerekli dosyaların oluşturulduğunu doğrula
	osintDir := filepath.Join(tempHome, ".osint")
	expectedFiles := []string{
		"config.toml",
		"master.key",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(osintDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to be created, but it was not", file)
		}
	}

	// 4. İKİNCİ YÜKLEME (Dosyalar Varken): Var olanın okunmasını test et
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load() failed on second call: %v", err)
	}
	if cfg2.Global.Version != cfg.Global.Version {
		t.Errorf("expected version %s, got %s", cfg.Global.Version, cfg2.Global.Version)
	}

	// Keystore'un başarıyla bellek nesnesine bağlandığını doğrula
	if cfg2.Keys == nil {
		t.Errorf("expected keystore to be initialized, got nil")
	}
}
