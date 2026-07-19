package domain

import (
	"context"
	"time"
)

// FindingType, bulunan verinin türünü standartlaştırır.
type FindingType string

const (
	TypeEmail  FindingType = "email"
	TypeDomain FindingType = "domain"
	TypeIP     FindingType = "ip"
	TypePerson FindingType = "person"
	TypeURL    FindingType = "url"
)

// Finding, OSINT modülleri tarafından tespit edilen tekil bir veriyi temsil eder.
type Finding struct {
	ID              string      `json:"id"`
	InvestigationID string      `json:"investigation_id"`
	Type            FindingType `json:"type"`
	Value           string      `json:"value"`    // Örn: "admin@example.com"
	Context         string      `json:"context"`  // Zengin veri, genellikle JSON string (örn: {"source": "github"})
	FoundBy         string      `json:"found_by"` // Modülün adı (örn: "github-scraper")
	CreatedAt       time.Time   `json:"created_at"`
}

// FindingRepository, bulguların veritabanı işlemlerini soyutlayan sözleşmedir.
type FindingRepository interface {
	Create(ctx context.Context, f *Finding) error
	GetByInvestigationID(ctx context.Context, invID string) ([]*Finding, error)
	Delete(ctx context.Context, id string) error
}
