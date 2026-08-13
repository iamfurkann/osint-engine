package ratelimit

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiter_AllowBurst(t *testing.T) {
	l := NewLimiter(5, 5)

	// Burst kadar izin vermeli
	for i := 0; i < 5; i++ {
		if !l.Allow() {
			t.Errorf("expected Allow() true for burst token %d", i)
		}
	}

	// Burst sonrası reddetmeli
	if l.Allow() {
		t.Error("expected Allow() false after burst exhausted")
	}
}

func TestLimiter_TokenRefill(t *testing.T) {
	l := NewLimiter(100, 5) // Saniyede 100 token → 10ms'de ~1 token

	// Tüm token'ları harca
	for l.Allow() {
	}

	// 50ms bekle — yeterli token oluşmuş olmalı
	time.Sleep(50 * time.Millisecond)

	if !l.Allow() {
		t.Error("expected Allow() true after refill wait")
	}
}

func TestLimiter_Wait(t *testing.T) {
	l := NewLimiter(100, 1) // Saniyede 100, burst 1

	// İlk token hemen alınmalı
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("first Wait failed: %v", err)
	}

	// İkinci token beklemeli ama çok uzun sürmemeli (~10ms)
	start := time.Now()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("second Wait failed: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Errorf("Wait took too long: %v", elapsed)
	}
}

func TestLimiter_WaitContextCancel(t *testing.T) {
	l := NewLimiter(1, 1) // Saniyede 1 token

	// İlk token'ı harca
	l.Allow()

	// 10ms timeout ile bekle — zaman aşımına uğramalı
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := l.Wait(ctx)
	if err == nil {
		t.Fatal("expected error on context cancellation, got nil")
	}
}

func TestLimiter_Tokens(t *testing.T) {
	l := NewLimiter(10, 10)

	initial := l.Tokens()
	if initial != 10 {
		t.Errorf("expected 10 initial tokens, got %f", initial)
	}

	l.Allow()
	after := l.Tokens()
	if after >= initial {
		t.Errorf("expected fewer tokens after Allow(), got %f", after)
	}
}

func TestPluginLimiters_GetOrCreate(t *testing.T) {
	pl := NewPluginLimiters(10)

	l1 := pl.GetOrCreate("shodan", 5)
	l2 := pl.GetOrCreate("shodan", 5)

	if l1 != l2 {
		t.Error("expected same limiter instance for same plugin")
	}

	// Farklı plugin → farklı limiter
	l3 := pl.GetOrCreate("hibp", 0) // 0 → defaultRate kullanılmalı
	if l1 == l3 {
		t.Error("expected different limiter for different plugin")
	}
}

func TestPluginLimiters_Remove(t *testing.T) {
	pl := NewPluginLimiters(10)
	pl.GetOrCreate("shodan", 5)
	pl.Remove("shodan")

	// Silindikten sonra yeni instance oluşturulmalı
	l := pl.GetOrCreate("shodan", 5)
	if l == nil {
		t.Fatal("expected new limiter after Remove + GetOrCreate")
	}
}

func TestPluginLimiters_ConcurrentAccess(t *testing.T) {
	pl := NewPluginLimiters(100)
	var ops atomic.Int64

	// 10 goroutine aynı anda GetOrCreate çağırsın
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				l := pl.GetOrCreate("concurrent-plugin", 50)
				if l.Allow() {
					ops.Add(1)
				}
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if ops.Load() == 0 {
		t.Error("expected at least some allowed operations")
	}
}
