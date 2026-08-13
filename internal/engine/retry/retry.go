package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Config, retry davranışını yapılandırır.
type Config struct {
	MaxAttempts  int           // Maksimum deneme sayısı (ilk deneme dahil)
	InitialDelay time.Duration // İlk bekleme süresi
	MaxDelay     time.Duration // Maksimum bekleme süresi
	Multiplier   float64       // Her denemede bekleme çarpanı (exponential backoff)
}

// DefaultConfig, makul varsayılan retry ayarlarını döndürür.
// 3 deneme, 1s başlangıç, 30s maksimum, 2x çarpan.
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// RetryableError, yeniden denenebilir olarak işaretlenmiş bir hatadır.
// Plugin'ler geçici hataları bu tiple sarmalayarak retry mekanizmasını tetikleyebilir.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable: %v", e.Err)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError, bir hatayı retryable olarak işaretler.
func NewRetryableError(err error) error {
	return &RetryableError{Err: err}
}

// PermanentError, kalıcı (retry edilmemesi gereken) bir hatadır.
// 401, 404 gibi durumlar için kullanılır.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("permanent: %v", e.Err)
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

// NewPermanentError, bir hatayı kalıcı olarak işaretler. Retry yapılmaz.
func NewPermanentError(err error) error {
	return &PermanentError{Err: err}
}

// IsRetryable, bir hatanın yeniden denenebilir olup olmadığını kontrol eder.
//   - RetryableError → true
//   - PermanentError → false
//   - Diğer hatalar  → true (varsayılan olarak retry edilir; güvenli taraf)
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	var permanent *PermanentError
	if errors.As(err, &permanent) {
		return false
	}

	return true
}

// Do, verilen fonksiyonu retry kurallarına göre çalıştırır.
// Fonksiyon başarılı olursa nil döner. Tüm denemeler başarısızsa son hata döner.
// Context iptal edilirse derhal döner.
//
// Exponential backoff + jitter:
//
//	delay = min(initialDelay * multiplier^attempt, maxDelay) + random_jitter
func Do(ctx context.Context, cfg Config, fn func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		// Context kontrolü
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Fonksiyonu çalıştır
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil // Başarılı
		}

		// Kalıcı hata → retry yapma
		if !IsRetryable(lastErr) {
			return lastErr
		}

		// Son deneme ise bekleme yapma
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		// Exponential backoff + jitter hesapla
		delay := calculateDelay(cfg, attempt)

		// Bekleme (context'e duyarlı)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("retry: all %d attempts failed, last error: %w", cfg.MaxAttempts, lastErr)
}

// calculateDelay, exponential backoff + jitter ile bekleme süresini hesaplar.
func calculateDelay(cfg Config, attempt int) time.Duration {
	// Exponential: initialDelay * multiplier^attempt
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt))

	// Maksimum sınır
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Jitter: ±%25 rastgele sapma (thundering herd önleme)
	jitter := delay * 0.25 * (rand.Float64()*2 - 1) // [-0.25, +0.25]
	delay += jitter

	if delay < 0 {
		delay = float64(cfg.InitialDelay)
	}

	return time.Duration(delay)
}
