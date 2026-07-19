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

	os.Setenv("OSINT_LOG_LEVEL", "debug")
	os.Setenv("OSINT_MAX_WORKERS", "25")
	defer func() {
		os.Unsetenv("OSINT_LOG_LEVEL")
		os.Unsetenv("OSINT_MAX_WORKERS")
	}()

	cfg.applyEnvOverrides()

	if cfg.Global.LogLevel != "debug" {
		t.Errorf("expected overridden log level debug, got %s", cfg.Global.LogLevel)
	}

	if cfg.Engine.MaxWorkers != 25 {
		t.Errorf("expected overridden max workers 25, got %d", cfg.Engine.MaxWorkers)
	}
}
