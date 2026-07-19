package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/iamfurkann/osint-engine/internal/db"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/errors"
)

type investigationRepo struct {
	db *db.DB
}

// NewInvestigationRepository, SQLite tabanlı yeni bir repository nesnesi üretir.
func NewInvestigationRepository(database *db.DB) domain.InvestigationRepository {
	return &investigationRepo{db: database}
}

func (r *investigationRepo) Create(ctx context.Context, inv *domain.Investigation) error {
	query := `INSERT INTO investigations (id, name, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`

	// Parametrik sorgu (?) kullanılarak SQL Injection önlenir.
	_, err := r.db.ExecContext(ctx, query, inv.ID, inv.Name, inv.Status, inv.CreatedAt, inv.UpdatedAt)
	if err != nil {
		return errors.Wrap(errors.TypeInternal, "failed to create investigation", err)
	}
	return nil
}

func (r *investigationRepo) GetByID(ctx context.Context, id string) (*domain.Investigation, error) {
	query := `SELECT id, name, status, created_at, updated_at FROM investigations WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)

	var inv domain.Investigation
	err := row.Scan(&inv.ID, &inv.Name, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows { // Go'nun standart sql paketinden geliyor
			return nil, errors.Wrap(errors.TypeNotFound, "investigation not found", err)
		}
		return nil, errors.Wrap(errors.TypeInternal, "failed to fetch investigation", err)
	}
	return &inv, nil
}

func (r *investigationRepo) Update(ctx context.Context, inv *domain.Investigation) error {
	inv.UpdatedAt = time.Now().UTC()
	query := `UPDATE investigations SET name = ?, status = ?, updated_at = ? WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query, inv.Name, inv.Status, inv.UpdatedAt, inv.ID)
	if err != nil {
		return errors.Wrap(errors.TypeInternal, "failed to update investigation", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return errors.Wrap(errors.TypeNotFound, "investigation not found for update", nil)
	}

	return nil
}

func (r *investigationRepo) List(ctx context.Context) ([]*domain.Investigation, error) {
	query := `SELECT id, name, status, created_at, updated_at FROM investigations ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(errors.TypeInternal, "failed to list investigations", err)
	}
	defer func() { _ = rows.Close() }()

	var invs []*domain.Investigation
	for rows.Next() {
		var inv domain.Investigation
		if err := rows.Scan(&inv.ID, &inv.Name, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, errors.Wrap(errors.TypeInternal, "failed to scan investigation row", err)
		}
		invs = append(invs, &inv)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.TypeInternal, "error occurred during investigation rows iteration", err)
	}
	return invs, nil
}

func (r *investigationRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM investigations WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(errors.TypeInternal, "failed to delete investigation", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return errors.Wrap(errors.TypeNotFound, "investigation not found for deletion", nil)
	}
	return nil
}
