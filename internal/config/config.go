package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

// Config ana yapılandırma şemasını temsil eder.
type Config struct {
	Global   GlobalConfig   `toml:"global"`
	Database DatabaseConfig `toml:"database"`
	Engine   EngineConfig   `toml:"engine"`
}

type GlobalConfig struct {
	Version  string `toml:"version"`
	LogLevel string `toml:"log_level"`
}

type DatabaseConfig struct {
	AppDBPath   string `toml:"app_db_path"`
	GraphDBPath string `toml:"graph_db_path"`
	CacheDBPath string `toml:"cache_db_path"`
}

type EngineConfig struct {
	MaxWorkers int  `toml:"max_workers"`
	UseCache   bool `toml:"use_cache"`
}

// DefaultConfig belirtilen çalışma dizinine göre varsayılan ayarları üretir.
func DefaultConfig(baseDir string) *Config {
	return &Config{
		Global: GlobalConfig{
			Version:  "v0.0.1",
			LogLevel: "info",
		},
		Database: DatabaseConfig{
			AppDBPath:   filepath.Join(baseDir, "osint.db"),
			GraphDBPath: filepath.Join(baseDir, "graph.db"),
			CacheDBPath: filepath.Join(baseDir, "cache.db"),
		},
		Engine: EngineConfig{
			MaxWorkers: 10,
			UseCache:   true,
		},
	}
}

// GetDefaultDir kullanıcı ev dizinindeki ~/.osint yolunu döner.
func GetDefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".osint"), nil
}

// Load yapılandırmayı yükler, dosya yoksa varsayılan alanı ve iskeleti oluşturur.
func Load() (*Config, error) {
	dir, err := GetDefaultDir()
	if err != nil {
		return nil, err
	}

	// Klasör yoksa oluştur (Kişisel güvenli izinler: 0700)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	configPath := filepath.Join(dir, "config.toml")
	apiKeyPath := filepath.Join(dir, "api_keys.enc")

	// Şifreli API anahtar deposu iskeletini (boş dosya) oluştur
	if _, err := os.Stat(apiKeyPath); os.IsNotExist(err) {
		if err := os.WriteFile(apiKeyPath, []byte{}, 0600); err != nil {
			return nil, err
		}
	}

	var cfg *Config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// config.toml yoksa varsayılan değerlerle oluştur
		cfg = DefaultConfig(dir)
		data, err := toml.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(configPath, data, 0600); err != nil {
			return nil, err
		}
	} else {
		// Varsa dosyadan oku
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		cfg = &Config{}
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Ortam değişkenleri (Environment) kontrolü ve override işlemi
	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyEnvOverrides ortam değişkeni varsa konfigürasyon değerlerini ezer.
func (c *Config) applyEnvOverrides() {
	if env := os.Getenv("OSINT_LOG_LEVEL"); env != "" {
		c.Global.LogLevel = env
	}
	if env := os.Getenv("OSINT_MAX_WORKERS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			c.Engine.MaxWorkers = val
		}
	}
}