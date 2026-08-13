package plugin

import (
	"context"
	"time"
)

// Result, eklentinin hedeften elde ettiği ham OSINT verisidir.
// (Geliştirici DB detaylarını, UUID veya InvestigationID bilmek zorunda değildir).
type Result struct {
	Type    string // "email", "ip", "subdomain" vb.
	Value   string
	Context string // JSON formatında ek metadata
}

// Plugin, OSINT Engine'e eklenecek tüm araçların uyması gereken yegane sözleşmedir.
type Plugin interface {
	// Manifest, eklentinin kimlik kartını döndürür.
	Manifest() Manifest

	// Timeout, eklentinin çalışması için gereken maksimum süreyi (zaman aşımı) belirtir.
	Timeout() time.Duration

	// Run, eklentinin çekirdek iş mantığıdır. Ham sonuçları (Result) döndürür.
	Run(ctx context.Context, target string) ([]Result, error)
}
