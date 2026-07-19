package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// SetupTempDir testler için geçici bir dizin oluşturur.
// Test bitiminde Go bu dizini otomatik olarak temizler.
func SetupTempDir(t *testing.T) string {
	t.Helper() // Bu sayede hata fırlatıldığında bu fonksiyon değil, testi çağıran satır gösterilir.
	return t.TempDir()
}

// SetEnv, test süresince geçerli olacak bir ortam değişkeni atar.
// Test bittiğinde ortam değişkenini eski haline getirir veya siler.
func SetEnv(t *testing.T, key, value string) {
	t.Helper()

	originalValue, exists := os.LookupEnv(key)

	err := os.Setenv(key, value)
	if err != nil {
		t.Fatalf("Failed to set env %s: %v", key, err)
	}

	// Test bittiğinde çalışacak temizlik fonksiyonu
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(key, originalValue)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// LoadFixture testler için mock veri (fixture) barındıran dosyaları okur.
// testdata/ klasörü içindeki dosyaları hedefler.
func LoadFixture(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to load fixture %s: %v", filename, err)
	}
	return data
}

// TempSQLiteDBPath Phase 1'de kullanılmak üzere geçici bir SQLite veritabanı yolu üretir.
func TempSQLiteDBPath(t *testing.T) string {
	t.Helper()
	dir := SetupTempDir(t)
	return filepath.Join(dir, "osint_test.db")
}
