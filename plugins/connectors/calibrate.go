package connectors

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
)

// negativeSignature, bir platformun "kullanıcı yok" sayfasının parmak izidir.
type negativeSignature struct {
	// unreliable, platformun var olmayan kullanıcı için de "bulundu"
	// döndürdüğü anlamına gelir.
	unreliable bool

	displayName string
	bio         string

	// control, imzayı üretirken kullanılan kullanıcı adıdır. Karşılaştırmada
	// her iki metinden de çıkarılır: Telegram gibi siteler "kullanıcı yok"
	// sayfasında adı metne gömüyor ("Telegram: Contact @X"), bu yüzden ham
	// karşılaştırma hiçbir zaman eşleşmiyordu.
	control string
}

// stripUsername, karşılaştırmayı ada duyarsız hâle getirir.
func stripUsername(text, username string) string {
	if username == "" {
		return text
	}
	return strings.ReplaceAll(text, strings.ToLower(username), "\x00")
}

// matches, bir sonucun "kullanıcı yok" sayfasıyla aynı olup olmadığını söyler.
func (n negativeSignature) matches(attrs map[string]any) bool {
	if !n.unreliable {
		return false // platform güvenilir, filtreleme gereksiz
	}

	got := func(k string) string {
		if v, ok := attrs[k]; ok {
			if s, isStr := v.(string); isStr {
				return strings.TrimSpace(strings.ToLower(s))
			}
		}
		return ""
	}

	candidate := ""
	if v, ok := attrs["username"].(string); ok {
		candidate = v
	}

	dn := stripUsername(got("display_name"), candidate)
	bio := stripUsername(got("bio"), candidate)
	ctrlDN := stripUsername(n.displayName, n.control)
	ctrlBio := stripUsername(n.bio, n.control)

	// Hiç kanıt yok + platform güvenilmez → ayırt edilemez, ele.
	if dn == "" && bio == "" {
		return true
	}
	// Kontrol sayfasıyla birebir aynı metin → bu bir profil değil.
	if dn != "" && dn == ctrlDN {
		return true
	}
	if bio != "" && bio == ctrlBio {
		return true
	}
	return false
}

// randomControlUsername, hiçbir platformda var olmayacak bir kullanıcı adı
// üretir. Rastgele: sabit bir dize zamanla gerçekten kaydedilebilir.
func randomControlUsername() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "zqxjklmnbvcxz98271"
	}
	return "zqx" + hex.EncodeToString(b)
}

// calibratePlatforms, her platformun "kullanıcı yok" davranışını ÖLÇER.
//
// Neden gerekli: canlı testte Facebook, Bluesky, Twitch ve TikTok var olmayan
// HER kullanıcı adı için HTTP 200 döndürdü. Bazıları ayrıca jenerik Open Graph
// etiketleri yayınlıyor, yani "metadata var mı" testi de onları eleyemiyor.
//
// Sabit kural listesi yazmak yerine platformu kendi kontrol örneğiyle
// karşılaştırıyoruz. Bu yaklaşım kendi kendini kalibre eder: bir platform
// yarın davranışını değiştirse bile kod değişikliği gerekmez.
//
// Maliyet: platform başına tek bir ek istek.
func (u *UsernameCheck) calibratePlatforms(ctx context.Context, list []platformCheck) map[string]negativeSignature {
	control := randomControlUsername()

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out = make(map[string]negativeSignature, len(list))
	)

	sem := make(chan struct{}, 10)
	for _, p := range list {
		wg.Add(1)
		go func(p platformCheck) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			found, body := u.checkPlatform(ctx, p, control)

			sig := negativeSignature{unreliable: found, control: control}
			if found {
				meta := extractProfileMetaFromBytes(body)
				if s, ok := meta["display_name"].(string); ok {
					sig.displayName = strings.TrimSpace(strings.ToLower(
						cleanDisplayName(s, p.Name, control)))
				}
				if s, ok := meta["bio"].(string); ok {
					sig.bio = strings.TrimSpace(strings.ToLower(s))
				}
			}

			mu.Lock()
			out[p.Name] = sig
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	return out
}
