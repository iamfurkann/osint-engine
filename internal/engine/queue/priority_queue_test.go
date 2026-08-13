package queue

import (
	"testing"
)

func TestPriorityQueue_PriorityOrder(t *testing.T) {
	pq := NewPriorityQueue()

	pq.Enqueue("inv-1", "target", "low-plugin", PriorityLow)
	pq.Enqueue("inv-1", "target", "crit-plugin", PriorityCritical)
	pq.Enqueue("inv-1", "target", "norm-plugin", PriorityNormal)

	// Critical ilk çıkmalı
	item, ok := pq.Dequeue()
	if !ok || item.PluginName != "crit-plugin" {
		t.Errorf("expected crit-plugin first, got %v", item)
	}

	// Normal ikinci
	item, ok = pq.Dequeue()
	if !ok || item.PluginName != "norm-plugin" {
		t.Errorf("expected norm-plugin second, got %v", item)
	}

	// Low son
	item, ok = pq.Dequeue()
	if !ok || item.PluginName != "low-plugin" {
		t.Errorf("expected low-plugin last, got %v", item)
	}
}

func TestPriorityQueue_FIFO_SamePriority(t *testing.T) {
	pq := NewPriorityQueue()

	pq.Enqueue("inv-1", "target", "first", PriorityNormal)
	pq.Enqueue("inv-1", "target", "second", PriorityNormal)
	pq.Enqueue("inv-1", "target", "third", PriorityNormal)

	// Aynı öncelikte FIFO sırası
	item, _ := pq.Dequeue()
	if item.PluginName != "first" {
		t.Errorf("expected 'first', got %q", item.PluginName)
	}
	item, _ = pq.Dequeue()
	if item.PluginName != "second" {
		t.Errorf("expected 'second', got %q", item.PluginName)
	}
	item, _ = pq.Dequeue()
	if item.PluginName != "third" {
		t.Errorf("expected 'third', got %q", item.PluginName)
	}
}

func TestPriorityQueue_DequeueEmpty(t *testing.T) {
	pq := NewPriorityQueue()

	_, ok := pq.Dequeue()
	if ok {
		t.Error("expected false for empty queue dequeue")
	}
}

func TestPriorityQueue_Cancel(t *testing.T) {
	pq := NewPriorityQueue()

	id1 := pq.Enqueue("inv-1", "target", "plugin-1", PriorityNormal)
	_ = pq.Enqueue("inv-1", "target", "plugin-2", PriorityNormal)

	if pq.Len() != 2 {
		t.Errorf("expected len 2, got %d", pq.Len())
	}

	// İlk görevi iptal et
	if !pq.Cancel(id1) {
		t.Error("expected Cancel to return true for existing item")
	}

	if pq.Len() != 1 {
		t.Errorf("expected len 1 after cancel, got %d", pq.Len())
	}

	// Kalan görev plugin-2 olmalı
	item, ok := pq.Dequeue()
	if !ok || item.PluginName != "plugin-2" {
		t.Errorf("expected plugin-2 after cancel, got %v", item)
	}

	// Olmayan ID'yi iptal etme
	if pq.Cancel("nonexistent") {
		t.Error("expected Cancel to return false for nonexistent item")
	}
}

func TestPriorityQueue_Len(t *testing.T) {
	pq := NewPriorityQueue()

	if pq.Len() != 0 {
		t.Errorf("expected len 0, got %d", pq.Len())
	}

	pq.Enqueue("inv-1", "target", "plugin-1", PriorityNormal)
	pq.Enqueue("inv-1", "target", "plugin-2", PriorityCritical)

	if pq.Len() != 2 {
		t.Errorf("expected len 2, got %d", pq.Len())
	}

	pq.Dequeue()
	if pq.Len() != 1 {
		t.Errorf("expected len 1 after dequeue, got %d", pq.Len())
	}
}

func TestPriorityQueue_Notify(t *testing.T) {
	pq := NewPriorityQueue()

	pq.Enqueue("inv-1", "target", "plugin-1", PriorityNormal)

	// Notify kanalında sinyal olmalı
	select {
	case <-pq.Notify:
		// Beklenen
	default:
		t.Error("expected notification after enqueue")
	}
}

func TestPriorityQueue_Close(t *testing.T) {
	pq := NewPriorityQueue()
	pq.Close()

	// Kapalı Notify kanalından okuma yapılabilmeli (zero value döner)
	_, ok := <-pq.Notify
	if ok {
		t.Error("expected closed channel")
	}
}

func TestPriority_String(t *testing.T) {
	cases := map[Priority]string{
		PriorityCritical: "critical",
		PriorityNormal:   "normal",
		PriorityLow:      "low",
		Priority(99):     "unknown(99)",
	}
	for p, expected := range cases {
		if p.String() != expected {
			t.Errorf("Priority(%d).String() = %q, want %q", p, p.String(), expected)
		}
	}
}
