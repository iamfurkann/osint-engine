package domain

import (
	"context"
	"time"
)

// InvestigationStatus, araştırmanın o anki durumunu belirtir.
type InvestigationStatus string

const (
	StatusActive    InvestigationStatus = "active"
	StatusPaused    InvestigationStatus = "paused"
	StatusCompleted InvestigationStatus = "completed"
	StatusArchived  InvestigationStatus = "archived"
)

// Investigation, sistemdeki ana OSINT araştırmasını temsil eden iş modelidir.
type Investigation struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Status    InvestigationStatus `json:"status"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

// InvestigationRepository, veritabanı işlemlerini soyutlayan sözleşmedir (Interface).
type InvestigationRepository interface {
	Create(ctx context.Context, inv *Investigation) error
	GetByID(ctx context.Context, id string) (*Investigation, error)
	Update(ctx context.Context, inv *Investigation) error
	List(ctx context.Context) ([]*Investigation, error)
	Delete(ctx context.Context, id string) error
}
