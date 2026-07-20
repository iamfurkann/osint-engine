package engine

import (
	"context"

	"github.com/iamfurkann/osint-engine/internal/domain"
)

// Module, sisteme entegre edilecek tüm OSINT eklentilerinin (plugin)
// uyması gereken zorunlu bir sözleşmedir (Interface).
type Module interface {
	// Name, eklentinin sistemdeki benzersiz adını döndürür. (örn: "github-scraper")
	Name() string

	// Run, eklentinin çekirdek iş mantığıdır. Belirtilen hedefi (target) analiz eder
	// ve bulguları (Finding) döndürür.
	Run(ctx context.Context, target string) ([]*domain.Finding, error)
}
