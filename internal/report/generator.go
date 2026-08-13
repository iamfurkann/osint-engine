package report

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Generator, rapor üretimi için kullanılan ana bileşendir.
type Generator struct{}

// NewGenerator, yeni bir Generator döner.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateHTML, verilen ReportData'yı kullanarak HTML rapor oluşturur.
func (g *Generator) GenerateHTML(data ReportData, outputPath string) error {
	// Eksik alanları doldur
	if data.GeneratedAt.IsZero() {
		data.GeneratedAt = time.Now()
	}

	// Bulguların Context alanından derlenen kimlik özeti ve bilgi
	// yoğunluğuna göre sıralama. Bu olmadan rapor yalnızca bir URL
	// listesiydi — toplanan ad/konum/hesap yaşı verisi hiç görünmüyordu.
	if data.Identity == nil {
		data.Identity = IdentitySummary(data.Entities)
	}
	SortByInformation(data.Entities)

	// Çıktı dosyasını oluştur
	// Eğer klasör belirtilmişse ve yoksa oluştur
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("rapor dizini oluşturulamadı: %w", err)
		}
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("rapor dosyası oluşturulamadı: %w", err)
	}
	defer file.Close()

	// Şablonu dosyaya uygula
	if err := defaultHTMLTemplate.Execute(file, data); err != nil {
		return fmt.Errorf("şablon oluşturma hatası: %w", err)
	}

	return nil
}
