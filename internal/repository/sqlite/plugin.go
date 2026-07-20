package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/iamfurkann/osint-engine/internal/db"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/errors"
)

type pluginRepo struct {
	db *db.DB
}

// NewPluginRepository, SQLite tabanlı eklenti depo nesnesini üretir.
func NewPluginRepository(database *db.DB) domain.PluginRepository {
	return &pluginRepo{db: database}
}

func (r *pluginRepo) Upsert(ctx context.Context, p *domain.Plugin) error {
	p.UpdatedAt = time.Now().UTC()

	// SQLite 'UPSERT' yeteneği: Kayıt yoksa INSERT yap, varsa UPDATE yap.
	query := `
		INSERT INTO plugins (name, description, version, status, language, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			description = excluded.description,
			version = excluded.version,
			status = excluded.status,
			language = excluded.language,
			updated_at = excluded.updated_at
	`

	_, err := r.db.ExecContext(ctx, query, p.Name, p.Description, p.Version, p.Status, p.Language, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return errors.Wrap(errors.TypeInternal, "failed to upsert plugin", err)
	}
	return nil
}

func (r *pluginRepo) GetByName(ctx context.Context, name string) (*domain.Plugin, error) {
	query := `SELECT name, description, version, status, language, created_at, updated_at FROM plugins WHERE name = ?`
	row := r.db.QueryRowContext(ctx, query, name)

	var p domain.Plugin
	err := row.Scan(&p.Name, &p.Description, &p.Version, &p.Status, &p.Language, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Wrap(errors.TypeNotFound, "plugin not found", err)
		}
		return nil, errors.Wrap(errors.TypeInternal, "failed to fetch plugin", err)
	}
	return &p, nil
}

func (r *pluginRepo) List(ctx context.Context) ([]*domain.Plugin, error) {
	query := `SELECT name, description, version, status, language, created_at, updated_at FROM plugins ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.TypeInternal, "failed to list plugins", err)
	}
	defer func() { _ = rows.Close() }()

	var plugins []*domain.Plugin
	for rows.Next() {
		var p domain.Plugin
		if err := rows.Scan(&p.Name, &p.Description, &p.Version, &p.Status, &p.Language, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, errors.Wrap(errors.TypeInternal, "failed to scan plugin row", err)
		}
		plugins = append(plugins, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.TypeInternal, "error occurred during plugin rows iteration", err)
	}

	return plugins, nil
}

func (r *pluginRepo) UpdateStatus(ctx context.Context, name string, status domain.PluginStatus) error {
	query := `UPDATE plugins SET status = ?, updated_at = ? WHERE name = ?`
	res, err := r.db.ExecContext(ctx, query, status, time.Now().UTC(), name)
	if err != nil {
		return errors.Wrap(errors.TypeInternal, "failed to update plugin status", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return errors.Wrap(errors.TypeNotFound, "plugin not found for status update", nil)
	}
	return nil
}
