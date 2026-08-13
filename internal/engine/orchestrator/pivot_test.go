package orchestrator

import "testing"

// TestPivotTargets, pivot zincirinin neden hiç çalışmadığını gösteren eşlemeyi
// doğrular: bulgu tipleri ile plugin girdi tipleri farklı sözlükler kullanıyor.
func TestPivotTargets(t *testing.T) {
	cases := []struct {
		name        string
		findingType string
		value       string
		want        []pivotTarget
	}{
		{
			name:        "e-posta hem kendisi hem domaini üzerinden pivot eder",
			findingType: "email", value: "ada@example.com",
			want: []pivotTarget{
				{Target: "ada@example.com", InputType: "email"},
				{Target: "example.com", InputType: "domain"},
			},
		},
		{
			name: "kullanıcı adı doğrudan", findingType: "username", value: "testuser",
			want: []pivotTarget{{Target: "testuser", InputType: "username"}},
		},
		{
			name: "url host'a indirgenir", findingType: "url", value: "https://example.org/hakkimda?x=1",
			want: []pivotTarget{{Target: "example.org", InputType: "domain"}},
		},
		{
			name:        "domain alanına yazılmış URL de host'a indirgenir",
			findingType: "domain", value: "https://www.example.org/",
			want: []pivotTarget{{Target: "example.org", InputType: "domain"}},
		},
		{
			name: "hostname domain sayılır", findingType: "hostname", value: "dns.google",
			want: []pivotTarget{{Target: "dns.google", InputType: "domain"}},
		},
		// Bunlar pivot ETMEMELİ: zaten elimizdeki profiller ya da kimlik
		// taşımayan teknik kayıtlar. Aksi hâlde maxPivots bütçesi boşa gider.
		{name: "username_presence pivot etmez", findingType: "username_presence", value: "https://x.com/a"},
		{name: "social_profile pivot etmez", findingType: "social_profile", value: "https://github.com/a"},
		{name: "open_port pivot etmez", findingType: "open_port", value: "443"},
		{name: "boş değer", findingType: "email", value: "   "},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pivotTargets(c.findingType, c.value)
			if len(got) != len(c.want) {
				t.Fatalf("%d hedef bekleniyordu, alınan %d: %+v", len(c.want), len(got), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("[%d] %+v, beklenen %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestHostFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.example.com/a/b": "example.com",
		"http://example.com:8080":     "example.com",
		"example.com":                 "example.com",
		"https://alt.example.co.uk/":  "alt.example.co.uk",
		"":                            "",
		"boşluklu metin":              "",
		"nokta-yok":                   "",
	}
	for in, want := range cases {
		if got := hostFromURL(in); got != want {
			t.Errorf("hostFromURL(%q) = %q, beklenen %q", in, got, want)
		}
	}
}
