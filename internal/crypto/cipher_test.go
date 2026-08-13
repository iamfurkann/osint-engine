package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i) // Deterministik test anahtarı
	}

	plaintext := []byte("Bu bir OSINT API anahtarıdır: sk-abc123xyz")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Şifreli veri açık metinden farklı olmalı
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext should not equal plaintext")
	}

	// Şifreli verinin boyutu en az nonce + plaintext + tag kadar olmalı
	if len(ciphertext) <= len(plaintext) {
		t.Error("ciphertext should be longer than plaintext (nonce + auth tag)")
	}

	// Gidiş-dönüş (round trip) testi
	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted text does not match original: got %q", string(decrypted))
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	correctKey := make([]byte, 32)
	wrongKey := make([]byte, 32)
	wrongKey[0] = 0xFF // Tek bit bile farklı olsa çözümleme başarısız olmalı

	plaintext := []byte("sensitive data")

	ciphertext, err := Encrypt(plaintext, correctKey)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(ciphertext, wrongKey)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key, got nil")
	}
}

func TestDecrypt_CorruptedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("test data")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Son byte'ı boz (auth tag'ı yıkar)
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err = Decrypt(ciphertext, key)
	if err == nil {
		t.Fatal("expected error when decrypting corrupted ciphertext, got nil")
	}
}

func TestDecrypt_TooShortCiphertext(t *testing.T) {
	key := make([]byte, 32)

	_, err := Decrypt([]byte("short"), key)
	if err == nil {
		t.Fatal("expected error for too-short ciphertext, got nil")
	}
}

func TestEnsureMasterKey_CreatesNewKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "master.key")

	key, err := EnsureMasterKey(keyPath)
	if err != nil {
		t.Fatalf("EnsureMasterKey failed: %v", err)
	}

	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d bytes", len(key))
	}

	// Dosya oluşturulmuş olmalı
	stat, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("key file was not created: %v", err)
	}

	// Dosya izinleri 0600 olmalı (sadece dosya sahibi okuyabilir)
	perm := stat.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected file permissions 0600, got %o", perm)
	}
}

func TestEnsureMasterKey_ReadsExistingKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "master.key")

	// İlk çağrı — oluştur
	key1, err := EnsureMasterKey(keyPath)
	if err != nil {
		t.Fatalf("first EnsureMasterKey failed: %v", err)
	}

	// İkinci çağrı — var olanı oku
	key2, err := EnsureMasterKey(keyPath)
	if err != nil {
		t.Fatalf("second EnsureMasterKey failed: %v", err)
	}

	if !bytes.Equal(key1, key2) {
		t.Error("expected same key on second read, got different key")
	}
}

func TestEnsureMasterKey_EnvOverride(t *testing.T) {
	envKey := "01234567890123456789012345678901" // tam 32 karakter
	t.Setenv("OSINT_MASTER_KEY", envKey)

	key, err := EnsureMasterKey("/nonexistent/path") // Dosya yolu önemsiz — env kullanılacak
	if err != nil {
		t.Fatalf("EnsureMasterKey with env failed: %v", err)
	}

	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d bytes", len(key))
	}

	// Anahtar argon2id ile TÜRETİLMELİ, parolanın ham baytları OLMAMALI.
	// Eski davranış []byte(envKey[:32]) idi ve parolayı doğrudan AES anahtarı
	// yapıyordu; bu, KDF'siz kaba kuvvet saldırısına açıktı.
	if string(key) == envKey[:32] {
		t.Error("master key ham parola baytları — argon2id türetmesi uygulanmamış")
	}

	if !bytes.Equal(key, DeriveKey(envKey)) {
		t.Error("EnsureMasterKey, DeriveKey ile aynı sonucu vermeli")
	}
}

func TestDeriveKey_DeterministicAndDistinct(t *testing.T) {
	a1 := DeriveKey("correct horse battery staple")
	a2 := DeriveKey("correct horse battery staple")
	b := DeriveKey("correct horse battery stapler")

	if !bytes.Equal(a1, a2) {
		t.Error("DeriveKey deterministik olmalı — aynı parola aynı anahtarı vermeli")
	}
	if bytes.Equal(a1, b) {
		t.Error("farklı parolalar farklı anahtar üretmeli")
	}
	if len(a1) != 32 {
		t.Errorf("AES-256 için 32 byte bekleniyor, alınan %d", len(a1))
	}
}

// TestLegacyEnvKey, yükseltme sırasında eski keystore dosyalarının
// kurtarılabilmesi için gereken eski türetmeyi doğrular.
func TestLegacyEnvKey(t *testing.T) {
	t.Run("env yoksa nil", func(t *testing.T) {
		t.Setenv("OSINT_MASTER_KEY", "")
		if LegacyEnvKey() != nil {
			t.Error("env değişkeni yokken nil dönmeli")
		}
	})

	t.Run("kısa parola nil", func(t *testing.T) {
		t.Setenv("OSINT_MASTER_KEY", "kisa")
		if LegacyEnvKey() != nil {
			t.Error("32 byte'tan kısa parolada nil dönmeli")
		}
	})

	t.Run("eski türetme korunur", func(t *testing.T) {
		envKey := "01234567890123456789012345678901"
		t.Setenv("OSINT_MASTER_KEY", envKey)
		if string(LegacyEnvKey()) != envKey[:32] {
			t.Error("eski türetme değişmemeli — aksi hâlde eski dosyalar açılamaz")
		}
	})
}

// TestKeystoreLegacyMigration, eski anahtarla şifrelenmiş bir kasanın
// yükseltmeden sonra hâlâ okunabildiğini doğrular (veri kaybı olmamalı).
func TestKeystoreLegacyMigration(t *testing.T) {
	envKey := "01234567890123456789012345678901"
	t.Setenv("OSINT_MASTER_KEY", envKey)

	secret := []byte(`{"shodan":"eski-anahtar"}`)

	// Eski (KDF'siz) anahtarla şifrele
	legacyCipher, err := Encrypt(secret, LegacyEnvKey())
	if err != nil {
		t.Fatalf("legacy encrypt failed: %v", err)
	}

	// Yeni anahtarla açılamamalı...
	if _, err := Decrypt(legacyCipher, DeriveKey(envKey)); err == nil {
		t.Fatal("yeni anahtar eski şifreli veriyi açmamalı (test ön koşulu)")
	}

	// ...ama eski anahtarla açılabilmeli — keystore.load() bu yolu kullanıyor.
	got, err := Decrypt(legacyCipher, LegacyEnvKey())
	if err != nil {
		t.Fatalf("eski anahtarla çözülemedi: %v", err)
	}
	if !bytes.Equal(got, secret) {
		t.Errorf("veri bozuldu: %s", got)
	}
}
