package domain

import (
	"context"
	"time"
)

// PluginStatus, eklentinin çalışma durumunu temsil eder.
type PluginStatus string

const (
	PluginStatusActive   PluginStatus = "active"
	PluginStatusInactive PluginStatus = "inactive"
	PluginStatusError    PluginStatus = "error"
)

// Plugin, sisteme kayıtlı olan OSINT modüllerini (Python, Go vb.) temsil eder.
type Plugin struct {
	Name        string       `json:"name"`        // Benzersiz isim (örn: "github-scraper")
	Description string       `json:"description"` // Ne işe yaradığı
	Version     string       `json:"version"`     // Sürüm (örn: "v1.0.0")
	Status      PluginStatus `json:"status"`
	Language    string       `json:"language"` // Hangi dille yazıldığı ("go", "python", "binary")
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// PluginRepository, eklenti verilerinin veritabanı sözleşmesidir.
type PluginRepository interface {
	Upsert(ctx context.Context, p *Plugin) error // Yoksa ekle, varsa güncelle
	GetByName(ctx context.Context, name string) (*Plugin, error)
	List(ctx context.Context) ([]*Plugin, error)
	UpdateStatus(ctx context.Context, name string, status PluginStatus) error
}
