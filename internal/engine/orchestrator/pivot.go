package orchestrator

import (
	"net/url"
	"strings"
)

// pivotTarget, bir bulgudan türetilen yeni tarama hedefidir.
type pivotTarget struct {
	Target    string
	InputType string
}

// pivotTargets, bir bulgunun hangi yeni taramaları tetikleyeceğine karar verir.
//
// Bu eşleme olmadan pivot zinciri HİÇ çalışmıyordu. Sebep: bulgu tipleri ile
// plugin girdi tipleri farklı kelime dağarcıkları kullanıyor.
//
//	Connector'ların ürettiği tipler : username_presence, social_profile,
//	                                  web_result, hostname, url, open_port, ...
//	Plugin'lerin kabul ettiği tipler: email, domain, ip, username, person, hash
//
// EnqueuePivot bulgu tipini doğrudan girdi tipi olarak kullanıyordu, dolayısıyla
// "username_presence" hiçbir plugin'le eşleşmiyor ve zincir daha ilk adımda
// kopuyordu.
//
// Kasıtlı olarak SEÇİCİ: her bulgu pivot etmez. Örneğin username_presence
// kayıtları zaten elimizdeki profillerdir; onları tekrar taramak yalnızca
// maxPivots bütçesini tüketir.
func pivotTargets(findingType, value string) []pivotTarget {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	switch findingType {
	case "email":
		// E-posta hem kendisi hem de domaini üzerinden pivot eder.
		out := []pivotTarget{{Target: value, InputType: "email"}}
		if i := strings.LastIndex(value, "@"); i > 0 && i < len(value)-1 {
			out = append(out, pivotTarget{Target: value[i+1:], InputType: "domain"})
		}
		return out

	case "username":
		return []pivotTarget{{Target: value, InputType: "username"}}

	case "domain":
		// Bazı connector'lar "domain" tipine tam URL yazıyor (GitHub blog
		// alanı gibi). Host'a indir.
		if h := hostFromURL(value); h != "" {
			return []pivotTarget{{Target: h, InputType: "domain"}}
		}
		return []pivotTarget{{Target: value, InputType: "domain"}}

	case "ip":
		return []pivotTarget{{Target: value, InputType: "ip"}}

	case "url":
		if h := hostFromURL(value); h != "" {
			return []pivotTarget{{Target: h, InputType: "domain"}}
		}
		return nil

	case "hostname":
		return []pivotTarget{{Target: value, InputType: "domain"}}

	default:
		// username_presence, social_profile, web_result, open_port, cpe,
		// vulnerability, certificate, breach... — bunlar ya zaten elimizdeki
		// profillerdir ya da pivot edilecek bir kimlik taşımazlar.
		return nil
	}
}

// hostFromURL, bir URL veya çıplak host dizesinden host adını çıkarır.
// Şema yoksa ekleyip dener; başarısız olursa boş döner.
func hostFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if !strings.Contains(raw, "://") {
		if !strings.Contains(raw, ".") || strings.ContainsAny(raw, " /") {
			return ""
		}
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}

	host := u.Host
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}
