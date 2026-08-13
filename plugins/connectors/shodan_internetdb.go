package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// ShodanInternetDB, Shodan'ın ÜCRETSİZ InternetDB uç noktası üzerinden IP
// bilgisi sorgular.
//
// Neden ana Shodan REST API'si değil:
// api.shodan.io/shodan/host/{ip} en az bir Shodan Membership (tek seferlik
// ücretli yükseltme) gerektiriyor. Ücretsiz hesap bir API anahtarı veriyor ama
// sorgu kredisi vermiyor, dolayısıyla çağrılar hata dönüyor. Projenin
// "tamamen ücretsiz kaynak" politikası gereği bu uç nokta kullanılamaz.
//
// InternetDB ise anahtarsız, hesapsız ve kotasızdır. Karşılığında daha az
// ayrıntı verir: banner ve ürün/sürüm bilgisi YOKTUR. Buna karşılık açık
// portlar, hostname'ler, CVE'ler ve CPE'ler mevcuttur — OSINT'te kullanılan
// çekirdek bilginin büyük kısmı bu.
//
// Doküman: https://internetdb.shodan.io/
type ShodanInternetDB struct {
	client  *http.Client
	baseURL string // testlerde httptest sunucusuna yönlendirmek için
}

func NewShodanInternetDB() *ShodanInternetDB {
	return &ShodanInternetDB{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://internetdb.shodan.io",
	}
}

func (s *ShodanInternetDB) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "shodan-internetdb",
		Name:        "shodan-internetdb",
		Version:     "v1.0.0",
		Type:        plugin.TypeConnector,
		Inputs:      []string{"ip"},
		Description: "Shodan InternetDB (ücretsiz, anahtarsız) — açık portlar, hostname'ler, CVE'ler",
		RateLimit:   1,
		// Ana API'den düşük: banner/ürün doğrulaması yok, veri daha kaba.
		Confidence: 85,
	}
}

func (s *ShodanInternetDB) Timeout() time.Duration { return 30 * time.Second }

// internetDBResponse, https://internetdb.shodan.io/{ip} yanıt şemasıdır.
type internetDBResponse struct {
	IP        string   `json:"ip"`
	Ports     []int    `json:"ports"`
	Hostnames []string `json:"hostnames"`
	Tags      []string `json:"tags"`
	Vulns     []string `json:"vulns"`
	CPEs      []string `json:"cpes"`
}

func (s *ShodanInternetDB) Run(ctx context.Context, target string) ([]plugin.Result, error) {
	url := fmt.Sprintf("%s/%s", s.baseURL, target)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("shodan-internetdb: request creation failed: %w", redactURLError(err))
	}
	req.Header.Set("User-Agent", "OSINT-Engine/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("shodan-internetdb: request failed: %w", redactURLError(err))
	}
	defer resp.Body.Close()

	// 404 = "bu IP hakkında kayıt yok". Bu bir HATA DEĞİLDİR.
	//
	// Bu ayrım kritik: LifecycleManager.MarkError() bir plugin hata döndürdüğünde
	// onu devre dışı bırakıyor ve üretimde Restart() hiç çağrılmıyor. Yani
	// "veri yok" durumunu hata olarak döndürmek, kayıtsız tek bir IP yüzünden
	// connector'ı daemon'ın ömrü boyunca öldürürdü.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shodan-internetdb: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("shodan-internetdb: failed to read response: %w", err)
	}

	var r internetDBResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("shodan-internetdb: failed to parse response: %w", err)
	}

	ip := r.IP
	if ip == "" {
		ip = target
	}

	results := []plugin.Result{{
		Type:  "shodan_host",
		Value: ip,
		Context: mustJSONContext(map[string]any{
			"source":     "shodan-internetdb",
			"ports":      r.Ports,
			"tags":       r.Tags,
			"vuln_count": len(r.Vulns),
		}),
	}}

	for _, hostname := range r.Hostnames {
		results = append(results, plugin.Result{
			Type:    "hostname",
			Value:   hostname,
			Context: mustJSONContext(map[string]any{"ip": ip, "source": "shodan-internetdb"}),
		})
	}

	for _, port := range r.Ports {
		results = append(results, plugin.Result{
			Type:  "open_port",
			Value: strconv.Itoa(port),
			Context: mustJSONContext(map[string]any{
				"ip":     ip,
				"source": "shodan-internetdb",
				// InternetDB transport/ürün/sürüm bilgisi vermiyor; ana Shodan
				// API'sinin aksine burada yalnızca port numarası var.
			}),
		})
	}

	for _, vuln := range r.Vulns {
		results = append(results, plugin.Result{
			Type:    "vulnerability",
			Value:   vuln,
			Context: mustJSONContext(map[string]any{"ip": ip, "source": "shodan-internetdb"}),
		})
	}

	for _, cpe := range r.CPEs {
		results = append(results, plugin.Result{
			Type:    "cpe",
			Value:   cpe,
			Context: mustJSONContext(map[string]any{"ip": ip, "source": "shodan-internetdb"}),
		})
	}

	return results, nil
}
