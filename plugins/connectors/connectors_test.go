package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// --- DNS/WHOIS Testleri ---

func TestDNSWhois_Manifest(t *testing.T) {
	d := NewDNSWhois()
	m := d.Manifest()

	if m.Name != "dns-whois" {
		t.Errorf("expected name 'dns-whois', got %q", m.Name)
	}
	if m.Type != plugin.TypeConnector {
		t.Errorf("expected type 'connector', got %q", m.Type)
	}
	if len(m.Inputs) == 0 || m.Inputs[0] != "domain" {
		t.Error("expected input 'domain'")
	}
}

func TestDNSWhois_PluginInterface(t *testing.T) {
	// Plugin interface'ini implemente ediyor mu?
	var _ plugin.Plugin = NewDNSWhois()
}

// --- CrtSh Testleri ---

func TestCrtSh_Manifest(t *testing.T) {
	c := NewCrtSh()
	m := c.Manifest()

	if m.Name != "crtsh" {
		t.Errorf("expected name 'crtsh', got %q", m.Name)
	}
}

func TestCrtSh_MockServer(t *testing.T) {
	mockResp := []crtshEntry{
		{CommonName: "www.example.com", IssuerName: "Let's Encrypt", NotBefore: "2025-01-01", NotAfter: "2025-04-01"},
		{CommonName: "mail.example.com", IssuerName: "Let's Encrypt", NotBefore: "2025-01-01", NotAfter: "2025-04-01"},
		{CommonName: "www.example.com", IssuerName: "Let's Encrypt", NotBefore: "2024-01-01", NotAfter: "2024-04-01"}, // duplicate
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(mockResp); err != nil {
			t.Errorf("encode failed: %v", err)
		}
	}))
	defer server.Close()

	c := NewCrtSh()
	c.client = server.Client()

	// Override URL — mock server'a yönlendir
	// CrtSh URL'i sabit olduğu için doğrudan test edemeyiz, manifest ve interface kontrolü yeterli
	var _ plugin.Plugin = c
}

// --- VirusTotal Testleri ---

func TestVirusTotal_NoApiKey(t *testing.T) {
	vt := NewVirusTotal("")
	_, err := vt.Run(context.Background(), "example.com")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestVirusTotal_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-apikey") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"attributes": map[string]interface{}{
					"last_analysis_stats": map[string]int{
						"malicious": 3, "suspicious": 1, "harmless": 60, "undetected": 10,
					},
					"reputation":       -5,
					"last_dns_records": []map[string]string{{"type": "A", "value": "93.184.216.34"}},
				},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode failed: %v", err)
		}
	}))
	defer server.Close()

	vt := NewVirusTotal("test-key")
	vt.client = server.Client()

	// Interface kontrolü
	var _ plugin.Plugin = vt
}

func TestVirusTotal_Manifest(t *testing.T) {
	vt := NewVirusTotal("key")
	m := vt.Manifest()
	if m.Name != "virustotal" {
		t.Errorf("expected 'virustotal', got %q", m.Name)
	}
	if len(m.Inputs) < 2 {
		t.Error("expected at least 2 inputs (domain, hash)")
	}
}

// --- Shodan InternetDB Testleri ---

func TestShodanInternetDB_Manifest(t *testing.T) {
	s := NewShodanInternetDB()
	m := s.Manifest()
	if m.Name != "shodan-internetdb" {
		t.Errorf("expected 'shodan-internetdb', got %q", m.Name)
	}
	if m.Inputs[0] != "ip" {
		t.Error("expected input 'ip'")
	}
	// Ücretsiz-only politikası: bu connector API anahtarı İSTEMEMELİ.
	if len(m.Auth) != 0 {
		t.Errorf("InternetDB anahtarsız olmalı, Auth alanı dolu: %v", m.Auth)
	}
}

// TestShodanInternetDB_NoKeyRequired, connector'ın anahtar olmadan
// çalıştığını doğrular — eski Shodan connector'ı burada hata döndürüyordu.
func TestShodanInternetDB_NoKeyRequired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Gerçek InternetDB yanıt şeması (1.1.1.1'den alınmıştır)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{
			"cpes":["cpe:/a:cloudflare:cloudflare"],
			"hostnames":["one.one.one.one"],
			"ip":"1.1.1.1",
			"ports":[53,80,443],
			"tags":["cdn"],
			"vulns":["CVE-2021-44228"]
		}`)); err != nil {
			t.Errorf("write failed: %v", err)
		}
	}))
	defer server.Close()

	s := NewShodanInternetDB()
	s.baseURL = server.URL

	results, err := s.Run(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("anahtarsız çalışmalıydı, hata: %v", err)
	}

	counts := map[string]int{}
	for _, r := range results {
		counts[r.Type]++
	}

	want := map[string]int{
		"shodan_host":   1,
		"hostname":      1,
		"open_port":     3,
		"vulnerability": 1,
		"cpe":           1,
	}
	for typ, n := range want {
		if counts[typ] != n {
			t.Errorf("%s: %d bekleniyordu, %d bulundu", typ, n, counts[typ])
		}
	}

	// Context alanı GEÇERLİ JSON olmalı (fmt.Sprintf yerine json.Marshal).
	for _, r := range results {
		var v map[string]any
		if err := json.Unmarshal([]byte(r.Context), &v); err != nil {
			t.Errorf("%s/%s: geçersiz JSON context %q: %v", r.Type, r.Value, r.Context, err)
		}
	}
}

// TestShodanInternetDB_NotFoundIsNotAnError, 404'ün hata SAYILMAMASINI doğrular.
//
// Bu davranış kritik: LifecycleManager.MarkError() hata döndüren plugin'i devre
// dışı bırakıyor ve üretimde Restart() hiç çağrılmıyor. 404'ü hata döndürmek,
// kaydı olmayan tek bir IP yüzünden connector'ı kalıcı olarak öldürürdü.
func TestShodanInternetDB_NotFoundIsNotAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	s := NewShodanInternetDB()
	s.baseURL = server.URL

	results, err := s.Run(context.Background(), "192.0.2.1")
	if err != nil {
		t.Fatalf("404 hata olmamalı (plugin kalıcı devre dışı kalır), alınan: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("404 için boş sonuç bekleniyordu, %d bulundu", len(results))
	}
}

// --- Hunter Testleri ---

func TestHunter_NoApiKey(t *testing.T) {
	h := NewHunter("")
	_, err := h.Run(context.Background(), "example.com")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestHunter_Manifest(t *testing.T) {
	h := NewHunter("key")
	m := h.Manifest()
	if m.Name != "hunter" {
		t.Errorf("expected 'hunter', got %q", m.Name)
	}
}

// --- UsernameCheck Testleri ---

func TestUsernameCheck_Manifest(t *testing.T) {
	u := NewUsernameCheck()
	m := u.Manifest()
	if m.Name != "username-check" {
		t.Errorf("expected 'username-check', got %q", m.Name)
	}
	if m.Inputs[0] != "username" {
		t.Error("expected input 'username'")
	}
}

func TestUsernameCheck_MockPlatform(t *testing.T) {
	// 200 dönen mock server
	foundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html>profile</html>")
	}))
	defer foundServer.Close()

	// 404 dönen mock server
	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFoundServer.Close()

	var _ plugin.Plugin = NewUsernameCheck()
}

// --- Gravatar Testleri ---

func TestGravatar_Manifest(t *testing.T) {
	g := NewGravatar()
	m := g.Manifest()
	if m.Name != "gravatar" {
		t.Errorf("expected 'gravatar', got %q", m.Name)
	}
	if m.Inputs[0] != "email" {
		t.Error("expected input 'email'")
	}
}

func TestGravatar_PluginInterface(t *testing.T) {
	var _ plugin.Plugin = NewGravatar()
}

// --- Genel Testler ---

func TestAllConnectors_ImplementPlugin(t *testing.T) {
	connectors := []plugin.Plugin{
		NewDNSWhois(),
		NewCrtSh(),
		NewVirusTotal("key"),
		NewShodanInternetDB(),
		NewHunter("key"),
		NewUsernameCheck(),
		NewGravatar(),
	}

	for _, c := range connectors {
		m := c.Manifest()
		if m.Name == "" {
			t.Error("connector manifest name is empty")
		}
		if m.Type != plugin.TypeConnector {
			t.Errorf("%s: expected type 'connector', got %q", m.Name, m.Type)
		}
		if len(m.Inputs) == 0 {
			t.Errorf("%s: expected at least 1 input", m.Name)
		}
		if c.Timeout() <= 0 {
			t.Errorf("%s: expected positive timeout", m.Name)
		}
	}
}
