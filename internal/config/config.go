package config

import (
	"fmt"
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
	Keys     *Keystore      `toml:"-"` // Şifreli depo (TOML dosyasına düz metin olarak yazılmasını engeller)
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
	MaxWorkers int    `toml:"max_workers"`
	UseCache   bool   `toml:"use_cache"`
	PluginsDir string `toml:"plugins_dir"`
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
			PluginsDir: filepath.Join(baseDir, "plugins"),
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

// Load yapılandırmayı yükler, dosya yoksa varsayılan alanı ve şifreli depoyu oluşturur.
func Load() (*Config, error) {
	dir, err := GetDefaultDir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	// 1. Şifreli API Anahtar Deposunu (Keystore) Başlat
	masterKeyPath := filepath.Join(dir, "master.key")
	apiKeyPath := filepath.Join(dir, "api_keys.enc")

	keystore, err := NewKeystore(masterKeyPath, apiKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize keystore: %w", err)
	}

	configPath := filepath.Join(dir, "config.toml")
	var cfg *Config

	// 2. TOML Yapılandırmasını Yükle
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg = DefaultConfig(dir)
		data, err := toml.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(configPath, data, 0600); err != nil {
			return nil, err
		}
	} else {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		cfg = &Config{}
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	cfg.Keys = keystore // Bellekteki konfigürasyon nesnesine depoyu bağla
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
