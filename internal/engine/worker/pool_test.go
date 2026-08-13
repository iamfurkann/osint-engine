package worker

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/internal/engine/queue"
)

func TestPool_BasicProcessing(t *testing.T) {
	q := queue.NewPriorityQueue()
	var processed atomic.Int64

	fn := func(ctx context.Context, item *queue.Item) Result {
		processed.Add(1)
		return Result{
			InvestigationID: item.InvestigationID,
			PluginName:      item.PluginName,
			Success:         true,
			FindingsCount:   1,
		}
	}

	pool := NewPool(2, q, fn)
	pool.Start()

	// 5 görev ekle
	for i := 0; i < 5; i++ {
		q.Enqueue("inv-1", "example.com", fmt.Sprintf("plugin-%d", i), queue.PriorityNormal)
	}

	// Sonuçları topla
	results := make([]Result, 0, 5)
	timeout := time.After(3 * time.Second)
	for len(results) < 5 {
		select {
		case r := <-pool.ResultCh:
			results = append(results, r)
		case <-timeout:
			t.Fatalf("timeout waiting for results, got %d/5", len(results))
		}
	}

	pool.Stop()

	if processed.Load() != 5 {
		t.Errorf("expected 5 processed, got %d", processed.Load())
	}

	for _, r := range results {
		if !r.Success {
			t.Errorf("expected success for task %s", r.TaskID)
		}
	}
}

func TestPool_GracefulShutdown(t *testing.T) {
	q := queue.NewPriorityQueue()

	fn := func(ctx context.Context, item *queue.Item) Result {
		// Yavaş görev simülasyonu
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return Result{PluginName: item.PluginName, Error: ctx.Err()}
		}
		return Result{PluginName: item.PluginName, Success: true}
	}

	pool := NewPool(2, q, fn)
	pool.Start()

	q.Enqueue("inv-1", "target", "slow-plugin", queue.PriorityNormal)

	// Hemen stop — graceful shutdown test
	time.Sleep(10 * time.Millisecond) // Görevin alınmasını bekle
	pool.Stop()

	// ResultCh kapalı olmalı
	_, ok := <-pool.ResultCh
	if ok {
		// Son sonuç gelebilir, ama kanal sonunda kapanmalı
		_, ok = <-pool.ResultCh
		_ = ok // Kanal kapanana kadar oku
	}
}

func TestPool_PriorityOrder(t *testing.T) {
	q := queue.NewPriorityQueue()

	// Önce hepsini ekle (tek worker ile sıra test et)
	q.Enqueue("inv-1", "target", "low-plugin", queue.PriorityLow)
	q.Enqueue("inv-1", "target", "crit-plugin", queue.PriorityCritical)
	q.Enqueue("inv-1", "target", "norm-plugin", queue.PriorityNormal)

	order := make([]string, 0, 3)
	fn := func(ctx context.Context, item *queue.Item) Result {
		order = append(order, item.PluginName)
		return Result{PluginName: item.PluginName, Success: true}
	}

	pool := NewPool(1, q, fn) // Tek worker — sıra garantisi
	pool.Start()

	timeout := time.After(3 * time.Second)
	collected := 0
	for collected < 3 {
		select {
		case <-pool.ResultCh:
			collected++
		case <-timeout:
			t.Fatalf("timeout, collected %d/3", collected)
		}
	}

	pool.Stop()

	if len(order) != 3 {
		t.Fatalf("expected 3 items, got %d", len(order))
	}
	if order[0] != "crit-plugin" {
		t.Errorf("expected crit-plugin first, got %q", order[0])
	}
	if order[1] != "norm-plugin" {
		t.Errorf("expected norm-plugin second, got %q", order[1])
	}
	if order[2] != "low-plugin" {
		t.Errorf("expected low-plugin last, got %q", order[2])
	}
}

func TestPool_ErrorHandling(t *testing.T) {
	q := queue.NewPriorityQueue()

	fn := func(ctx context.Context, item *queue.Item) Result {
		return Result{
			PluginName: item.PluginName,
			Success:    false,
			Error:      fmt.Errorf("plugin crashed"),
		}
	}

	pool := NewPool(1, q, fn)
	pool.Start()

	q.Enqueue("inv-1", "target", "bad-plugin", queue.PriorityNormal)

	select {
	case r := <-pool.ResultCh:
		if r.Success {
			t.Error("expected failure result")
		}
		if r.Error == nil {
			t.Error("expected error in result")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for error result")
	}

	pool.Stop()
}

func TestPool_WorkerCount(t *testing.T) {
	q := queue.NewPriorityQueue()
	fn := func(ctx context.Context, item *queue.Item) Result { return Result{} }

	pool := NewPool(8, q, fn)
	if pool.WorkerCount() != 8 {
		t.Errorf("expected 8 workers, got %d", pool.WorkerCount())
	}

	// Negatif → varsayılan 5
	pool2 := NewPool(-1, q, fn)
	if pool2.WorkerCount() != 5 {
		t.Errorf("expected default 5 workers, got %d", pool2.WorkerCount())
	}
}
