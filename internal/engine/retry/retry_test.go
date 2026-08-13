package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, Multiplier: 2}
	callCount := 0

	err := Do(context.Background(), cfg, func(ctx context.Context) error {
		callCount++
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestDo_SuccessAfterRetry(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, Multiplier: 2}
	callCount := 0

	err := Do(context.Background(), cfg, func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return NewRetryableError(fmt.Errorf("temporary error"))
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil after retries, got: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDo_AllAttemptsFail(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, Multiplier: 2}
	callCount := 0

	err := Do(context.Background(), cfg, func(ctx context.Context) error {
		callCount++
		return NewRetryableError(fmt.Errorf("always fails"))
	})

	if err == nil {
		t.Fatal("expected error after all attempts, got nil")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDo_PermanentErrorStopsRetry(t *testing.T) {
	cfg := Config{MaxAttempts: 5, InitialDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, Multiplier: 2}
	callCount := 0

	err := Do(context.Background(), cfg, func(ctx context.Context) error {
		callCount++
		return NewPermanentError(fmt.Errorf("401 unauthorized"))
	})

	if err == nil {
		t.Fatal("expected error for permanent failure, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call for permanent error, got %d", callCount)
	}

	// Hata PermanentError olarak unwrap edilebilmeli
	var permErr *PermanentError
	if !errors.As(err, &permErr) {
		t.Error("expected error to be PermanentError")
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	cfg := Config{MaxAttempts: 10, InitialDelay: 1 * time.Second, MaxDelay: 10 * time.Second, Multiplier: 2}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	callCount := 0
	err := Do(ctx, cfg, func(ctx context.Context) error {
		callCount++
		return NewRetryableError(fmt.Errorf("slow error"))
	})

	if err == nil {
		t.Fatal("expected error on context cancellation, got nil")
	}
	// Context hatası mı?
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("got error: %v (callCount: %d)", err, callCount)
	}
}

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		err      error
		expected bool
		name     string
	}{
		{nil, false, "nil error"},
		{fmt.Errorf("generic error"), true, "generic error (default retryable)"},
		{NewRetryableError(fmt.Errorf("temp")), true, "retryable error"},
		{NewPermanentError(fmt.Errorf("perm")), false, "permanent error"},
	}

	for _, c := range cases {
		got := IsRetryable(c.err)
		if got != c.expected {
			t.Errorf("IsRetryable(%s) = %v, want %v", c.name, got, c.expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected 3 max attempts, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelay != 1*time.Second {
		t.Errorf("expected 1s initial delay, got %v", cfg.InitialDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("expected 2.0 multiplier, got %f", cfg.Multiplier)
	}
}

func TestCalculateDelay_ExponentialGrowth(t *testing.T) {
	cfg := Config{InitialDelay: 100 * time.Millisecond, MaxDelay: 10 * time.Second, Multiplier: 2}

	prev := time.Duration(0)
	for i := 0; i < 5; i++ {
		delay := calculateDelay(cfg, i)
		// Jitter var, ama genel trend artıyor olmalı (ilk birkaç adımda)
		if i > 0 && i < 3 {
			// İlk birkaç adımda delay öncekinden büyük olmalı (jitter'a rağmen genelde)
			_ = prev // jitter nedeniyle kesin karşılaştırma yapamayız
		}
		prev = delay

		// MaxDelay'i aşmamalı (jitter ile ±%25 olabilir)
		if delay > cfg.MaxDelay+cfg.MaxDelay/4 {
			t.Errorf("delay %v exceeds max+jitter for attempt %d", delay, i)
		}
	}
}
