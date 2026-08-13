package plugins

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type NameGeneratorPlugin struct{}

func (p *NameGeneratorPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "name-generator",
		Name:        "name-generator",
		Description: "Kişi adından muhtemel kullanıcı adları türetir",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"person"},
		RateLimit:   10, // Saniyede maksimum 10 işlem (zaten lokal işlem)
		Confidence:  30,
	}
}

func (p *NameGeneratorPlugin) Initialize(ctx context.Context, config map[string]string) error {
	return nil
}

// removeAccents Türkçe karakterleri (ç,ş,ğ,ü,ö,ı) ingilizce karşılıklarına çevirir.
func removeAccents(s string) string {
	s = strings.ReplaceAll(s, "ı", "i")
	s = strings.ReplaceAll(s, "İ", "i")
	s = strings.ReplaceAll(s, "ğ", "g")
	s = strings.ReplaceAll(s, "Ğ", "g")
	s = strings.ReplaceAll(s, "ü", "u")
	s = strings.ReplaceAll(s, "Ü", "u")
	s = strings.ReplaceAll(s, "ş", "s")
	s = strings.ReplaceAll(s, "Ş", "s")
	s = strings.ReplaceAll(s, "ö", "o")
	s = strings.ReplaceAll(s, "Ö", "o")
	s = strings.ReplaceAll(s, "ç", "c")
	s = strings.ReplaceAll(s, "Ç", "c")

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return strings.ToLower(result)
}

func (p *NameGeneratorPlugin) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	// İsim soyisim ayrımı
	parts := strings.Fields(target)
	if len(parts) == 0 {
		return nil, nil
	}

	for i := range parts {
		parts[i] = removeAccents(parts[i])
	}

	var baseCombinations []string

	if len(parts) == 1 {
		baseCombinations = append(baseCombinations, parts[0])
	} else if len(parts) >= 2 {
		first := parts[0]
		last := parts[len(parts)-1]

		// Temel varyasyonlar
		baseCombinations = append(baseCombinations, strings.Join(parts, "")) // adanovayilmaz
		if len(parts) > 2 {
			baseCombinations = append(baseCombinations, strings.Join(parts[:len(parts)-1], "")) // adanova
		}
		baseCombinations = append(baseCombinations, first+last)            // adayilmaz
		baseCombinations = append(baseCombinations, string(first[0])+last) // ayilmaz
		baseCombinations = append(baseCombinations, last+first)            // yilmazada
		baseCombinations = append(baseCombinations, first+"_"+last)        // ada_yilmaz
		baseCombinations = append(baseCombinations, first+"."+last)        // ada.yilmaz
	}

	// Sosyal medya varyantları sadece ilk 3 base'e uygula
	var allUsernames []string
	for i, base := range baseCombinations {
		allUsernames = append(allUsernames, base)
		if i < 3 {
			allUsernames = append(allUsernames, "iam"+base)
			allUsernames = append(allUsernames, "_"+base+"_")
			allUsernames = append(allUsernames, "real"+base)
			allUsernames = append(allUsernames, "the"+base)
		}
	}

	var results []plugin.Result
	seen := make(map[string]bool)

	for _, username := range allUsernames {
		if seen[username] || len(username) < 3 {
			continue
		}
		seen[username] = true

		results = append(results, plugin.Result{
			Type:    "username",
			Value:   username,
			Context: `{"source": "name-generator"}`,
		})
	}

	// Email generation
	if len(parts) >= 2 {
		first := parts[0]
		last := parts[len(parts)-1]
		initial := string(last[0])
		providers := []string{"gmail.com", "hotmail.com", "outlook.com", "yahoo.com"}

		for _, provider := range providers {
			results = append(results, plugin.Result{
				Type:    "email",
				Value:   fmt.Sprintf("%s%s@%s", first, last, provider),
				Context: `{"source": "name-generator"}`,
			})
			results = append(results, plugin.Result{
				Type:    "email",
				Value:   fmt.Sprintf("%s.%s@%s", first, last, provider),
				Context: `{"source": "name-generator"}`,
			})
			results = append(results, plugin.Result{
				Type:    "email",
				Value:   fmt.Sprintf("%s@%s", strings.Join(parts, ""), provider),
				Context: `{"source": "name-generator"}`,
			})
			results = append(results, plugin.Result{
				Type:    "email",
				Value:   fmt.Sprintf("%s%s@%s", first, initial, provider),
				Context: `{"source": "name-generator"}`,
			})
		}
	}

	return results, nil
}

func (p *NameGeneratorPlugin) Timeout() time.Duration {
	return 10 * time.Second
}
