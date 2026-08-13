package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// pluginTypeToDirName, PluginType'ı dizin adına eşler.
var pluginTypeToDirName = map[plugin.PluginType]string{
	plugin.TypeConnector: "connectors",
	plugin.TypeAnalyzer:  "analyzers",
	plugin.TypeReporter:  "reporters",
	plugin.TypeAI:        "ai-providers",
}

// ScaffoldOptions, plugin iskeleti oluşturmak için gereken parametreleri tanımlar.
type ScaffoldOptions struct {
	Name       string            // Plugin adı (örn: "shodan-scanner")
	Type       plugin.PluginType // Plugin tipi (connector, analyzer, reporter, ai-provider)
	Language   string            // "go" veya "python"
	PluginsDir string            // Plugin kök dizini (örn: ~/.osint/plugins)
}

// Scaffold, yeni bir plugin iskeleti oluşturur.
// Dizin yapısı, manifest.toml, şablon kaynak dosyası ve README.md üretir.
// Dizin zaten varsa hata döner (üzerine yazma koruması).
func Scaffold(opts ScaffoldOptions) (string, error) {
	// Validasyon
	if strings.TrimSpace(opts.Name) == "" {
		return "", fmt.Errorf("scaffold: plugin name is required")
	}
	if opts.Language == "" {
		opts.Language = "go" // Varsayılan dil
	}
	if opts.Language != "go" && opts.Language != "python" {
		return "", fmt.Errorf("scaffold: unsupported language %q (supported: go, python)", opts.Language)
	}

	dirName, ok := pluginTypeToDirName[opts.Type]
	if !ok {
		return "", fmt.Errorf("scaffold: invalid plugin type %q", opts.Type)
	}

	pluginDir := filepath.Join(opts.PluginsDir, dirName, opts.Name)

	// Üzerine yazma koruması
	if _, err := os.Stat(pluginDir); err == nil {
		return "", fmt.Errorf("scaffold: directory %q already exists — will not overwrite", pluginDir)
	}

	// Dizin oluştur
	if err := os.MkdirAll(pluginDir, 0700); err != nil {
		return "", fmt.Errorf("scaffold: failed to create directory %q: %w", pluginDir, err)
	}

	// Dosyaları yaz
	files := map[string]string{
		"manifest.toml": generateManifest(opts),
		"README.md":     generateREADME(opts),
	}

	if opts.Language == "go" {
		files["plugin.go"] = generateGoPlugin(opts)
	} else {
		files["main.py"] = generatePythonPlugin(opts)
	}

	for name, content := range files {
		path := filepath.Join(pluginDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("scaffold: failed to write %q: %w", path, err)
		}
	}

	return pluginDir, nil
}

// generateManifest, manifest.toml şablonu üretir.
func generateManifest(opts ScaffoldOptions) string {
	return fmt.Sprintf(`# Plugin Manifest — %s
# Bu dosya plugin'in kimlik kartıdır. Engine bu dosyayı okuyarak
# plugin'i tanır, doğrular ve registry'ye kaydeder.

# --- Zorunlu Alanlar ---
id      = "%s"
name    = "%s"
version = "v0.1.0"
type    = "%s"
inputs  = ["domain"]       # Desteklenen girdi tipleri: domain, email, ip, username, phone, text

# --- İsteğe Bağlı Alanlar ---
description = "%s plugin"
language    = "%s"
# rate_limit  = 5           # Saniyede max istek sayısı
# auth        = ["api_key"] # Gerekli API anahtarları
# confidence  = 75          # Varsayılan güvenilirlik skoru (0-100)
# dependencies = []         # Bağımlı olduğu diğer plugin'ler
# engine_min_version = "v0.1.0"  # Gereken minimum engine sürümü
`, opts.Name, opts.Name, opts.Name, string(opts.Type), opts.Name, opts.Language)
}

// generateGoPlugin, Go plugin şablonu üretir.
func generateGoPlugin(opts ScaffoldOptions) string {
	// PascalCase struct adı üret: "shodan-scanner" → "ShodanScanner"
	structName := toPascalCase(opts.Name)

	return fmt.Sprintf(`package main

import (
	"context"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// %s, %s tipinde bir OSINT plugin'idir.
type %s struct{}

func New%s() *%s {
	return &%s{}
}

// Manifest, plugin'in kimlik bilgilerini döndürür.
func (p *%s) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:      "%s",
		Name:    "%s",
		Version: "v0.1.0",
		Type:    "%s",
		Inputs:  []string{"domain"},
	}
}

// Timeout, plugin'in maksimum çalışma süresini döndürür.
func (p *%s) Timeout() time.Duration {
	return 30 * time.Second
}

// Run, hedef üzerinde OSINT sorgusunu çalıştırır ve sonuçları döndürür.
func (p *%s) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	// TODO: Burada gerçek OSINT sorgulama mantığını implement edin.
	//
	// Örnek:
	//   1. API'ye istek at
	//   2. Yanıtı parse et
	//   3. plugin.Result slice'ı döndür
	//
	// Her Result şunları içermeli:
	//   - Type:    bulgu tipi ("email", "ip", "subdomain", vb.)
	//   - Value:   bulunan değer
	//   - Context: ek bağlam (JSON formatında)

	return nil, nil
}
`, structName, string(opts.Type),
		structName,
		structName, structName, structName,
		structName,
		opts.Name, opts.Name, string(opts.Type),
		structName,
		structName)
}

// generatePythonPlugin, Python plugin şablonu üretir.
func generatePythonPlugin(opts ScaffoldOptions) string {
	return fmt.Sprintf(`#!/usr/bin/env python3
"""
%s — %s plugin

Bu dosya OSINT Engine'in Python plugin arayüzünü implemente eder.
Engine bu plugin'i subprocess olarak çalıştırır ve stdout üzerinden
JSON formatında sonuç alır.
"""

import json
import sys


def run(target: str) -> list[dict]:
    """
    Hedef üzerinde OSINT sorgusunu çalıştırır.

    Args:
        target: Sorgulanacak hedef (domain, email, ip vb.)

    Returns:
        Her biri {"type": str, "value": str, "context": str} içeren
        bulgu listesi.
    """
    # TODO: Burada gerçek OSINT sorgulama mantığını implement edin.
    #
    # Örnek:
    #   results = []
    #   response = requests.get(f"https://api.example.com/lookup/{target}")
    #   for item in response.json():
    #       results.append({
    #           "type": "email",
    #           "value": item["email"],
    #           "context": json.dumps({"source": "%s"})
    #       })
    #   return results

    return []


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python main.py <target>", file=sys.stderr)
        sys.exit(1)

    target = sys.argv[1]
    results = run(target)
    print(json.dumps(results))
`, opts.Name, string(opts.Type), opts.Name)
}

// generateREADME, plugin README şablonu üretir.
func generateREADME(opts ScaffoldOptions) string {
	return fmt.Sprintf(`# %s

> **Tip:** %s · **Dil:** %s · **Sürüm:** v0.1.0

## Açıklama

Bu plugin henüz geliştirme aşamasındadır.

## Desteklenen Girdiler

- `+"`domain`"+`

## Kurulum

1. Gerekli API anahtarlarını ekleyin:
   `+"`"+`osint keys set <service_name> <api_key>`+"`"+`

2. Plugin'i test edin:
   `+"`"+`osint search domain example.com`+"`"+`

## Geliştirme

### Yerel Test
`+"```bash"+`
# Go plugin
go test ./...

# Python plugin  
python main.py example.com
`+"```"+`

## Lisans

Bu plugin OSINT Engine projesinin bir parçasıdır.
`, opts.Name, string(opts.Type), opts.Language)
}

// toPascalCase, kebab-case string'i PascalCase'e çevirir.
// "shodan-scanner" → "ShodanScanner"
func toPascalCase(s string) string {
	parts := strings.Split(s, "-")
	var result strings.Builder
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		result.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			result.WriteString(part[1:])
		}
	}
	return result.String()
}
