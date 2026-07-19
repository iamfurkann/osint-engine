package logger

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Init sistem genelinde kullanılacak yapılandırılmış loglayıcıyı başlatır.
func Init(level string, logDir string, filename string) error {
	// 1. Log seviyesini ayarla (varsayılan: info)
	zLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		zLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(zLevel)

	// Zaman formatını ayarla (RFC3339 standart)
	zerolog.TimeFieldFormat = time.RFC3339

	// 2. Klasör yolunu güvenceye al (izinler 0700)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return err
	}

	// 3. Dosya rotasyonu için lumberjack ayarları
	fileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, filename),
		MaxSize:    10,   // Her dosya maksimum 10 MB
		MaxBackups: 5,    // En fazla 5 eski log tut
		MaxAge:     30,   // En fazla 30 günlük logları sakla
		Compress:   true, // Eski logları gzip ile sıkıştır
	}

	// 4. Konsol çıktısı için okunabilir format (Stderr)
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
	}

	// 5. MultiWriter: Aynı anda hem diske (JSON) hem konsola (Renkli Metin) yaz
	multi := zerolog.MultiLevelWriter(consoleWriter, fileWriter)

	// Global loglayıcıyı güncelle
	log.Logger = zerolog.New(multi).With().Timestamp().Caller().Logger()

	return nil
}

// NewInvestigationLogger spesifik bir araştırma için izole log dosyası oluşturur.
// (Phase 3'te Orkestratör tarafından kullanılacak)
func NewInvestigationLogger(logDir string, invID string) zerolog.Logger {
	invLogDir := filepath.Join(logDir, "investigations")
	_ = os.MkdirAll(invLogDir, 0700) // Hata kontrolü göz ardı ediliyor, üst katmanda yapıldığı varsayılır

	fileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(invLogDir, invID+".log"),
		MaxSize:    5,
		MaxBackups: 2,
		MaxAge:     14,
		Compress:   true,
	}

	// Araştırma logları konsolu kirletmez, sadece izole JSON dosyasına yazılır.
	return zerolog.New(fileWriter).With().Timestamp().Str("investigation_id", invID).Logger()
}