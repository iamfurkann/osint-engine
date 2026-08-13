package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/iamfurkann/osint-engine/internal/domain"
)

// WatchRepository, SQLite tabanlı WatchItem depolama katmanı.
type WatchRepository struct {
	db *sql.DB
}

// NewWatchRepository, yeni bir WatchRepository oluşturur.
func NewWatchRepository(db *sql.DB) *WatchRepository {
	return &WatchRepository{db: db}
}

// Add, listeye yeni bir hedef ekler (zaten varsa günceller).
func (r *WatchRepository) Add(ctx context.Context, item *domain.WatchItem) error {
	query := `
	INSERT INTO watchlist (id, target, type, interval, last_run, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		interval = excluded.interval,
		last_run = excluded.last_run;
	`
	_, err := r.db.ExecContext(ctx, query,
		item.ID,
		item.Target,
		item.Type,
		int64(item.Interval),
		item.LastRun,
		item.CreatedAt,
	)
	return err
}

// List, tüm izleme listesini getirir.
func (r *WatchRepository) List(ctx context.Context) ([]*domain.WatchItem, error) {
	query := `SELECT id, target, type, interval, last_run, created_at FROM watchlist`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.WatchItem
	for rows.Next() {
		var item domain.WatchItem
		var intervalInt int64
		if err := rows.Scan(
			&item.ID,
			&item.Target,
			&item.Type,
			&intervalInt,
			&item.LastRun,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Interval = time.Duration(intervalInt)
		items = append(items, &item)
	}

	return items, rows.Err()
}

// Remove, hedefi listeden çıkarır.
func (r *WatchRepository) Remove(ctx context.Context, id string) error {
	query := `DELETE FROM watchlist WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("item not found")
	}
	return nil
}

// UpdateLastRun, hedefin son taranma zamanını günceller.
func (r *WatchRepository) UpdateLastRun(ctx context.Context, id string, lastRun time.Time) error {
	query := `UPDATE watchlist SET last_run = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, lastRun, id)
	return err
}
