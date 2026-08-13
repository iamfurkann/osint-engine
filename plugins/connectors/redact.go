package connectors

import (
	"errors"
	"fmt"
	"net/url"
)

// sensitiveQueryParams, URL query string'inde geçtiğinde maskelenmesi gereken
// parametre adlarıdır.
var sensitiveQueryParams = []string{
	"key", "api_key", "apikey", "apiKey",
	"token", "access_token", "auth", "password",
}

// redactURL, bir URL'in query string'indeki hassas parametreleri maskeler.
// Ayrıştırılamayan URL'ler tümüyle gizlenir — kısmi sızıntı riskine girmeyiz.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[redacted-url]"
	}

	q := u.Query()
	changed := false
	for _, p := range sensitiveQueryParams {
		if q.Has(p) {
			q.Set(p, "REDACTED")
			changed = true
		}
	}
	if changed {
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// redactURLError, net/http'nin döndürdüğü *url.Error içindeki TAM URL'i maskeler.
//
// Bu, teorik bir önlem değil somut bir sızıntıyı kapatır: Shodan REST API'si
// yalnızca "?key=" query parametresiyle kimlik doğruluyor (header desteği yok),
// ve net/http bir ağ hatasında isteğin tam URL'ini *url.Error içine koyuyor.
// Bu hata çağrı zincirinde yukarı taşınıp zerolog ile log dosyasına yazılınca
// API anahtarı diske düşüyordu.
func redactURLError(err error) error {
	if err == nil {
		return nil
	}
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return fmt.Errorf("%s %s: %w", uerr.Op, redactURL(uerr.URL), uerr.Err)
	}
	return err
}
