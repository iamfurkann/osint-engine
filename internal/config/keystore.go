package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/iamfurkann/osint-engine/internal/crypto"
)

// Keystore API anahtarlarını şifreli olarak tutan eşzamanlılığa duyarlı yapıdır.
type Keystore struct {
	path  string
	key   []byte
	mutex sync.RWMutex
	keys  map[string]string
}

// NewKeystore şifreli anahtar deposunu başlatır.
func NewKeystore(masterKeyPath, storePath string) (*Keystore, error) {
	masterKey, err := crypto.EnsureMasterKey(masterKeyPath)
	if err != nil {
		return nil, err
	}

	ks := &Keystore{
		path: storePath,
		key:  masterKey,
		keys: make(map[string]string),
	}

	if err := ks.load(); err != nil && !os.IsNotExist(err) {
		// Dosya yoksa sorun değil, ama şifre çözülemiyorsa (farklı master key vs) hata döner
		return nil, err
	}

	return ks, nil
}

func (k *Keystore) load() error {
	data, err := os.ReadFile(k.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	plaintext, err := crypto.Decrypt(data, k.key)
	if err != nil {
		// Geriye dönük uyumluluk: dosya, OSINT_MASTER_KEY'in KDF'siz eski
		// türetmesiyle şifrelenmiş olabilir. Denenip başarılı olursa dosya
		// yeni anahtarla sessizce yeniden yazılır — kullanıcı anahtarlarını
		// kaybetmez ve bir daha bu yola girilmez.
		if legacy := crypto.LegacyEnvKey(); legacy != nil {
			if legacyPlaintext, legacyErr := crypto.Decrypt(data, legacy); legacyErr == nil {
				if err := json.Unmarshal(legacyPlaintext, &k.keys); err != nil {
					return err
				}
				return k.save()
			}
		}
		return err
	}

	return json.Unmarshal(plaintext, &k.keys)
}

// save, keystore'u ATOMİK olarak diske yazar.
//
// Önceki hâli doğrudan os.WriteFile idi: dosyayı yerinde kesip yeniden yazıyordu.
// Yazma sırasında bir çökme/güç kesintisi olursa TÜM API anahtarları birden
// kaybolurdu. Artık geçici dosyaya yazılıp fsync sonrası rename ediliyor;
// rename aynı dosya sisteminde atomiktir.
func (k *Keystore) save() error {
	data, err := json.Marshal(k.keys)
	if err != nil {
		return err
	}

	ciphertext, err := crypto.Encrypt(data, k.key)
	if err != nil {
		return err
	}

	dir := filepath.Dir(k.path)
	tmp, err := os.CreateTemp(dir, ".api_keys.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Hata hâlinde geçici dosyayı arkada bırakma.
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if err := tmp.Chmod(0600); err != nil {
		return err
	}
	if _, err := tmp.Write(ciphertext); err != nil {
		return err
	}
	// Rename'den önce veriyi diske indir; aksi hâlde rename atomik olsa da
	// içerik henüz diskte olmayabilir.
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, k.path)
}

// Set bir API anahtarını güvenli şekilde AES-256 ile şifreleyip kaydeder.
func (k *Keystore) Set(service, key string) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	k.keys[service] = key
	return k.save()
}

// Get ilgili servisin API anahtarını döner, yoksa boş string döner.
func (k *Keystore) Get(service string) string {
	k.mutex.RLock()
	defer k.mutex.RUnlock()
	return k.keys[service]
}
