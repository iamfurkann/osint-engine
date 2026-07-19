package sqlite

import (
	"context"

	"github.com/iamfurkann/osint-engine/internal/db"
	"github.com/iamfurkann/osint-engine/internal/domain"
	"github.com/iamfurkann/osint-engine/internal/errors"
)

type findingRepo struct {
	db *db.DB
}

// NewFindingRepository, SQLite tabanlı yeni bir bulgu depo nesnesi üretir.
func NewFindingRepository(database *db.DB) domain.FindingRepository {
	return &findingRepo{db: database}
}

func (r *findingRepo) Create(ctx context.Context, f *domain.Finding) error {
	query := `INSERT INTO findings (id, investigation_id, type, value, context, found_by, created_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query, f.ID, f.InvestigationID, f.Type, f.Value, f.Context, f.FoundBy, f.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.TypeInternal, "failed to create finding", err)
	}
	return nil
}

func (r *findingRepo) GetByInvestigationID(ctx context.Context, invID string) ([]*domain.Finding, error) {
	query := `SELECT id, investigation_id, type, value, context, found_by, created_at 
	          FROM findings WHERE investigation_id = ? ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, invID)
	if err != nil {
		return nil, errors.Wrap(errors.TypeInternal, "failed to list findings", err)
	}
	defer func() { _ = rows.Close() }()

	var findings []*domain.Finding
	for rows.Next() {
		var f domain.Finding
		if err := rows.Scan(&f.ID, &f.InvestigationID, &f.Type, &f.Value, &f.Context, &f.FoundBy, &f.CreatedAt); err != nil {
			return nil, errors.Wrap(errors.TypeInternal, "failed to scan finding row", err)
		}
		findings = append(findings, &f)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(errors.TypeInternal, "error occurred during finding rows iteration", err)
	}
	return findings, nil
}

func (r *findingRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM findings WHERE id = ?`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(errors.TypeInternal, "failed to delete finding", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return errors.Wrap(errors.TypeNotFound, "finding not found for deletion", nil)
	}
	return nil
}
