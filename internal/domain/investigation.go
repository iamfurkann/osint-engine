package domain

import (
	"context"
	"fmt"
	"strings"
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

// Validate, araştırma nesnesinin kurallara uygun olup olmadığını denetler.
func (i *Investigation) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return fmt.Errorf("investigation ID cannot be empty")
	}

	name := strings.TrimSpace(i.Name)
	if name == "" {
		return fmt.Errorf("investigation name cannot be empty")
	}
	if len(name) > 255 {
		return fmt.Errorf("investigation name cannot exceed 255 characters")
	}

	switch i.Status {
	case StatusActive, StatusPaused, StatusCompleted, StatusArchived:
		// Geçerli statü
	default:
		return fmt.Errorf("invalid investigation status: %s", i.Status)
	}

	return nil
}

// InvestigationRepository, veritabanı işlemlerini soyutlayan sözleşmedir (Interface).
type InvestigationRepository interface {
	Create(ctx context.Context, inv *Investigation) error
	GetByID(ctx context.Context, id string) (*Investigation, error)
	Update(ctx context.Context, inv *Investigation) error
	List(ctx context.Context) ([]*Investigation, error)
	Delete(ctx context.Context, id string) error
}
