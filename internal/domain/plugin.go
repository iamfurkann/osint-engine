package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// PluginStatus, eklentinin çalışma durumunu temsil eder.
type PluginStatus string

const (
	PluginStatusActive   PluginStatus = "active"
	PluginStatusInactive PluginStatus = "inactive"
	PluginStatusError    PluginStatus = "error"
)

// SemVer (Semantic Versioning) doğrulama kuralı
var semverRegex = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// Plugin, eklentilerin VERİTABANINDAKİ (runtime state) durumunu temsil eden nesnedir.
// Harici eklenti özelliklerini barındıran pkg/plugin.Manifest yapısı ile karıştırılmamalıdır.
type Plugin struct {
	Name        string       `json:"name"`        // Benzersiz isim (örn: "github-scraper")
	Description string       `json:"description"` // Ne işe yaradığı
	Version     string       `json:"version"`     // Sürüm (örn: "v1.0.0")
	Status      PluginStatus `json:"status"`
	Language    string       `json:"language"` // Hangi dille yazıldığı ("go", "python", "binary")
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Validate, eklenti nesnesinin kurallara uygun olup olmadığını denetler.
func (p *Plugin) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	if len(p.Name) > 100 {
		return fmt.Errorf("plugin name cannot exceed 100 characters")
	}

	if !semverRegex.MatchString(p.Version) {
		return fmt.Errorf("invalid plugin version format (expected semver, e.g. v1.0.0 or 2.1.3-beta): '%s'", p.Version)
	}

	if strings.TrimSpace(p.Language) == "" {
		return fmt.Errorf("plugin language cannot be empty")
	}

	switch p.Status {
	case PluginStatusActive, PluginStatusInactive, PluginStatusError:
		// Geçerli statü
	default:
		return fmt.Errorf("invalid plugin status: %s", p.Status)
	}

	return nil
}

// PluginRepository, eklenti verilerinin veritabanı sözleşmesidir.
type PluginRepository interface {
	Upsert(ctx context.Context, p *Plugin) error // Yoksa ekle, varsa güncelle
	GetByName(ctx context.Context, name string) (*Plugin, error)
	List(ctx context.Context) ([]*Plugin, error)
	UpdateStatus(ctx context.Context, name string, status PluginStatus) error
}
