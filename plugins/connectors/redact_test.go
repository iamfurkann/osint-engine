package connectors

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

const secret = "s3cr3t-api-key-do-not-leak"

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "shodan key masked",
			in:   "https://api.shodan.io/shodan/host/1.2.3.4?key=" + secret,
			want: "https://api.shodan.io/shodan/host/1.2.3.4?key=REDACTED",
		},
		{
			name: "api_key masked",
			in:   "https://api.hunter.io/v2/domain-search?api_key=" + secret + "&domain=example.com",
			want: "https://api.hunter.io/v2/domain-search?api_key=REDACTED&domain=example.com",
		},
		{
			name: "url without secrets is untouched",
			in:   "https://crt.sh/?q=%25.example.com&output=json",
			want: "https://crt.sh/?q=%25.example.com&output=json",
		},
		{
			name: "unparseable url is fully hidden",
			in:   "://not a url?key=" + secret,
			want: "[redacted-url]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactURL(tt.in)
			if got != tt.want {
				t.Errorf("redactURL()\n got: %s\nwant: %s", got, tt.want)
			}
			if strings.Contains(got, secret) {
				t.Errorf("SECRET LEAKED in redactURL output: %s", got)
			}
		})
	}
}

// TestRedactURLError, asıl sızıntı senaryosunu doğrular: net/http bir ağ
// hatasında isteğin TAM URL'ini *url.Error içine koyar ve bu hata log'a yazılır.
func TestRedactURLError(t *testing.T) {
	raw := "https://api.shodan.io/shodan/host/1.2.3.4?key=" + secret

	urlErr := &url.Error{
		Op:  "Get",
		URL: raw,
		Err: errors.New("dial tcp: connection refused"),
	}

	// Ön koşul: redaksiyon olmadan anahtar gerçekten sızıyor.
	if !strings.Contains(urlErr.Error(), secret) {
		t.Fatal("test kurulumu geçersiz: url.Error zaten anahtarı içermiyor")
	}

	got := redactURLError(urlErr)
	if strings.Contains(got.Error(), secret) {
		t.Errorf("SECRET LEAKED: %s", got.Error())
	}
	if !strings.Contains(got.Error(), "key=REDACTED") {
		t.Errorf("beklenen maskeleme yok: %s", got.Error())
	}

	// Sarmalanmış (wrapped) hâlde de sızmamalı — üretimde böyle kullanılıyor.
	wrapped := fmt.Errorf("shodan: request failed: %w", redactURLError(urlErr))
	if strings.Contains(wrapped.Error(), secret) {
		t.Errorf("SECRET LEAKED when wrapped: %s", wrapped.Error())
	}

	// errors.Is zinciri korunmalı.
	if !errors.Is(got, urlErr.Err) {
		t.Error("redactURLError hata zincirini kopardı")
	}
}

func TestRedactURLErrorPassthrough(t *testing.T) {
	if redactURLError(nil) != nil {
		t.Error("nil hata nil dönmeli")
	}

	plain := errors.New("boring error")
	if got := redactURLError(plain); got != plain {
		t.Errorf("url.Error olmayan hata olduğu gibi dönmeli, alınan: %v", got)
	}
}
