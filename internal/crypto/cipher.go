package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

// argon2id parametreleri. Yerel, etkileşimli bir araç için makul seviye:
// ~64 MB bellek, 3 geçiş. Bu değerler DEĞİŞTİRİLİRSE mevcut parolalardan
// türetilen anahtarlar geçersiz olur.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	argonKeyLen  = 32
)

// argonSalt, uygulamaya özgü sabit tuzdur.
//
// Not: Kullanıcı başına rastgele tuz kriptografik olarak daha iyi olurdu, ancak
// OSINT_MASTER_KEY'in tüm amacı anahtarı diskte HİÇ tutmamaktır — tuzu diske
// yazmak bunu bozar. Sabit tuz, ham parolayı doğrudan AES anahtarı olarak
// kullanmaktan kat kat iyidir; asıl güvenlik parolanın entropisinden gelir.
var argonSalt = []byte("osint-engine/master-key/v1")

// EnsureMasterKey AES-256 için 32 byte'lık bir anahtar üretir veya var olanı okur.
func EnsureMasterKey(path string) ([]byte, error) {
	// Kullanıcı ortam değişkeni ile parola vermişse ondan anahtar TÜRET.
	//
	// Önceki davranış []byte(envKey[:32]) idi: yazılan parola doğrudan AES-256
	// anahtarı oluyordu. 32 karakterlik bir parolanın entropisi 256 bit'in çok
	// altındadır; KDF olmadan kaba kuvvet saldırısı ucuzdur.
	if envKey := os.Getenv("OSINT_MASTER_KEY"); envKey != "" {
		return DeriveKey(envKey), nil
	}

	key, err := os.ReadFile(path)
	if err == nil && len(key) == 32 {
		return key, nil
	}

	// Anahtar yoksa veya bozuksa yeni bir 32-byte kriptografik anahtar üret
	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}

	// Sadece dosya sahibinin okuyabileceği (0600) şekilde diske kaydet
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}

	return key, nil
}

// DeriveKey, bir paroladan argon2id ile 32 byte'lık AES-256 anahtarı türetir.
func DeriveKey(passphrase string) []byte {
	return argon2.IDKey([]byte(passphrase), argonSalt, argonTime, argonMemory, argonThreads, argonKeyLen)
}

// LegacyEnvKey, OSINT_MASTER_KEY için ESKİ (KDF'siz) türetmeyi döndürür.
//
// Yalnızca geriye dönük uyumluluk içindir: eski sürümle şifrelenmiş bir
// api_keys.enc dosyası yeni anahtarla açılamayacağı için keystore önce yeni
// anahtarı, başarısız olursa bunu dener ve dosyayı sessizce yeni anahtarla
// yeniden yazar. Böylece yükseltme sırasında anahtar kaybı olmaz.
// Env değişkeni yoksa veya 32 byte'tan kısaysa nil döner.
func LegacyEnvKey() []byte {
	envKey := os.Getenv("OSINT_MASTER_KEY")
	if len(envKey) < 32 {
		return nil
	}
	return []byte(envKey[:32])
}

// Encrypt veriyi AES-256-GCM kullanarak şifreler.
func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// gcm.Seal ciphertext'in sonuna auth tag ekler, biz de nonce'u başa ekliyoruz (çözerken gerekecek)
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt AES-256-GCM şifresini çözer.
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, actualCiphertext, nil)
}
