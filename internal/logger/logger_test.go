package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestInitAndLogLevels(t *testing.T) {
	tempDir := t.TempDir()
	logFile := "test.log"

	err := Init("debug", tempDir, logFile)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Errorf("Expected log level Debug, got %v", zerolog.GlobalLevel())
	}

	// Log dosyasına yazma testi
	log.Debug().Msg("This is a debug message")
	log.Info().Msg("This is an info message")

	// Dosyanın oluştuğunu doğrula
	filePath := filepath.Join(tempDir, logFile)
	if stat, err := os.Stat(filePath); os.IsNotExist(err) || stat.Size() == 0 {
		t.Errorf("Log file was not created or is empty")
	}
}

func TestNewInvestigationLogger(t *testing.T) {
	tempDir := t.TempDir()
	invID := "inv-001"

	invLog := NewInvestigationLogger(tempDir, invID)
	invLog.Info().Msg("Investigation started")

	filePath := filepath.Join(tempDir, "investigations", invID+".log")
	if stat, err := os.Stat(filePath); os.IsNotExist(err) || stat.Size() == 0 {
		t.Errorf("Investigation log file was not created")
	}
}
