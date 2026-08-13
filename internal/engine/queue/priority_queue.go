package queue

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Priority, görev öncelik seviyesini temsil eder.
// Düşük sayı = yüksek öncelik.
type Priority int

const (
	PriorityCritical Priority = 0 // En yüksek — acil görevler
	PriorityNormal   Priority = 1 // Standart görevler
	PriorityLow      Priority = 2 // Düşük öncelikli arka plan görevleri
)

// String, Priority'nin okunabilir string halini döndürür.
func (p Priority) String() string {
	switch p {
	case PriorityCritical:
		return "critical"
	case PriorityNormal:
		return "normal"
	case PriorityLow:
		return "low"
	default:
		return fmt.Sprintf("unknown(%d)", p)
	}
}

// Item, kuyruktaki bir görev öğesini temsil eder.
type Item struct {
	ID              string
	InvestigationID string
	Target          string
	PluginName      string
	Priority        Priority
	EnqueuedAt      time.Time
	sequence        uint64 // FIFO garantisi için monoton artan sayaç
	index           int    // heap içindeki pozisyon
}

// itemHeap, container/heap interface'ini implemente eden dahili yapıdır.
type itemHeap []*Item

func (h itemHeap) Len() int { return len(h) }

// Less: Önce düşük Priority numarası (yüksek öncelik), eşitse FIFO (sequence sırası).
func (h itemHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	return h[i].sequence < h[j].sequence
}

func (h itemHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *itemHeap) Push(x interface{}) {
	item := x.(*Item)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *itemHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[:n-1]
	return item
}

// PriorityQueue, thread-safe öncelikli görev kuyruğudur.
type PriorityQueue struct {
	items      itemHeap
	lookup     map[string]*Item
	seqCounter uint64 // Monoton artan FIFO sayaç
	mu         sync.Mutex
	Notify     chan struct{}
	closed     bool
}

// NewPriorityQueue, yeni bir öncelikli kuyruk oluşturur.
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		items:  make(itemHeap, 0),
		lookup: make(map[string]*Item),
		Notify: make(chan struct{}, 256), // Buffered — sinyal kaybetmemek için
	}
	heap.Init(&pq.items)
	return pq
}

// Enqueue, yeni bir görev kuyruğa ekler ve görev ID'sini döndürür.
func (pq *PriorityQueue) Enqueue(investigationID, target, pluginName string, priority Priority) string {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	item := &Item{
		ID:              uuid.New().String(),
		InvestigationID: investigationID,
		Target:          target,
		PluginName:      pluginName,
		Priority:        priority,
		EnqueuedAt:      time.Now().UTC(),
		sequence:        pq.seqCounter,
	}
	pq.seqCounter++

	heap.Push(&pq.items, item)
	pq.lookup[item.ID] = item

	// Worker'lara yeni görev sinyali
	select {
	case pq.Notify <- struct{}{}:
	default:
	}

	return item.ID
}

// Dequeue, en yüksek öncelikli görevi kuyruktan çıkarır.
// Kuyruk boşsa false döner.
func (pq *PriorityQueue) Dequeue() (*Item, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.items.Len() == 0 {
		return nil, false
	}

	item := heap.Pop(&pq.items).(*Item)
	delete(pq.lookup, item.ID)
	return item, true
}

// Cancel, belirtilen ID'li görevi kuyruktan çıkarır.
// Görev bulunamazsa false döner.
func (pq *PriorityQueue) Cancel(id string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	item, exists := pq.lookup[id]
	if !exists {
		return false
	}

	heap.Remove(&pq.items, item.index)
	delete(pq.lookup, id)
	return true
}

// Len, kuyruktaki görev sayısını döndürür.
func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.items.Len()
}

// Close, kuyruğu kapatır. Notify kanalı kapatılır.
func (pq *PriorityQueue) Close() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if !pq.closed {
		close(pq.Notify)
		pq.closed = true
	}
}
