package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter, token-bucket algoritması ile istek hızını sınırlar.
// Thread-safe: birden fazla goroutine'den güvenle kullanılabilir.
type Limiter struct {
	rate       float64   // Saniyede eklenen token sayısı
	burst      int       // Maksimum biriktirilebilecek token (bucket boyutu)
	tokens     float64   // Mevcut token sayısı
	lastRefill time.Time // Son token ekleme zamanı
	mu         sync.Mutex
}

// NewLimiter, belirtilen rate (saniyede token) ve burst (max birikme) ile limiter oluşturur.
// rate=5 → saniyede 5 istek izni, burst=10 → en fazla 10 token birikir.
func NewLimiter(rate int, burst int) *Limiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate
	}
	return &Limiter{
		rate:       float64(rate),
		burst:      burst,
		tokens:     float64(burst), // Başlangıçta tam dolu
		lastRefill: time.Now(),
	}
}

// refill, geçen süreye göre token'ları yeniler.
// mu.Lock() altında çağrılmalıdır.
func (l *Limiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens += elapsed * l.rate
	if l.tokens > float64(l.burst) {
		l.tokens = float64(l.burst)
	}
	l.lastRefill = now
}

// Allow, anlık olarak bir token tüketmeye çalışır.
// Token varsa true döner ve token tüketilir. Yoksa false döner, bloklamaz.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()
	if l.tokens >= 1.0 {
		l.tokens--
		return true
	}
	return false
}

// Wait, bir token hazır olana kadar bekler (context-aware).
// Token alındığında nil döner. Context iptal edilirse ctx.Err() döner.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		l.mu.Lock()
		l.refill()
		if l.tokens >= 1.0 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}

		// Bir sonraki token ne zaman hazır olacak?
		waitDuration := time.Duration(float64(time.Second) / l.rate)
		if waitDuration < time.Millisecond {
			waitDuration = time.Millisecond
		}
		l.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
		}
	}
}

// Tokens, mevcut token sayısını döndürür (debug/monitoring amaçlı).
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	return l.tokens
}

// PluginLimiters, her plugin için ayrı rate limiter tutan merkezi yöneticidir.
// Manifest'teki rate_limit değerine göre otomatik limiter oluşturur.
type PluginLimiters struct {
	limiters    map[string]*Limiter
	defaultRate int // Manifest'te rate_limit yoksa kullanılacak varsayılan
	mu          sync.RWMutex
}

// NewPluginLimiters, plugin bazlı rate limiter yöneticisi oluşturur.
// defaultRate: manifest'te rate_limit belirtilmemişse kullanılacak saniyede istek sayısı.
func NewPluginLimiters(defaultRate int) *PluginLimiters {
	if defaultRate <= 0 {
		defaultRate = 10
	}
	return &PluginLimiters{
		limiters:    make(map[string]*Limiter),
		defaultRate: defaultRate,
	}
}

// GetOrCreate, plugin adına göre limiter döndürür.
// Yoksa belirtilen rate ile yeni bir tane oluşturur.
// rateLimit <= 0 ise defaultRate kullanılır.
func (pl *PluginLimiters) GetOrCreate(pluginName string, rateLimit int) *Limiter {
	pl.mu.RLock()
	if limiter, exists := pl.limiters[pluginName]; exists {
		pl.mu.RUnlock()
		return limiter
	}
	pl.mu.RUnlock()

	pl.mu.Lock()
	defer pl.mu.Unlock()

	// Double-check pattern
	if limiter, exists := pl.limiters[pluginName]; exists {
		return limiter
	}

	if rateLimit <= 0 {
		rateLimit = pl.defaultRate
	}

	limiter := NewLimiter(rateLimit, rateLimit*2)
	pl.limiters[pluginName] = limiter
	return limiter
}

// Remove, bir plugin'in limiter'ını siler.
func (pl *PluginLimiters) Remove(pluginName string) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	delete(pl.limiters, pluginName)
}
