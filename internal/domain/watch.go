package domain

import (
	"context"
	"time"
)

// WatchItem, sürekli izlenen bir OSINT hedefini temsil eder.
type WatchItem struct {
	ID        string        `json:"id"`
	Target    string        `json:"target"`
	Type      string        `json:"type"` // domain, email, username vb.
	Interval  time.Duration `json:"interval"`
	LastRun   time.Time     `json:"last_run"`
	CreatedAt time.Time     `json:"created_at"`
}

// WatchRepository, izleme listesi (watchlist) için veritabanı işlemlerini soyutlar.
type WatchRepository interface {
	Add(ctx context.Context, item *WatchItem) error
	List(ctx context.Context) ([]*WatchItem, error)
	Remove(ctx context.Context, id string) error
	UpdateLastRun(ctx context.Context, id string, lastRun time.Time) error
}
