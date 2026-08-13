# Katkı Rehberi

## Kurulum

```bash
make build
```

```bash
make test         # testler
make test-race    # yarış koşulu detektörü (CI bunu kullanır)
make lint         # golangci-lint
```

CI'ın çalıştırdığı her şey yerelde de çalışmalı. Push öncesi:

```bash
go build ./... && make test-race && make lint && go mod tidy -diff
```

Arayüz için:

```bash
cd ui && npm run lint && npm run build
```

---

## Yeni connector eklemek

### 1. Sözleşmeyi uygulayın

```go
package connectors

type MyConnector struct {
    client  *http.Client
    baseURL string // testlerde httptest'e yönlendirmek için
}

func NewMyConnector() *MyConnector {
    return &MyConnector{
        client:  &http.Client{Timeout: 30 * time.Second},
        baseURL: "https://example.org",
    }
}

func (c *MyConnector) Manifest() plugin.Manifest {
    return plugin.Manifest{
        ID:          "my-connector",
        Name:        "my-connector",
        Version:     "v1.0.0",
        Type:        plugin.TypeConnector,
        Inputs:      []string{"domain"},
        Description: "Ne yaptığını bir cümlede anlatın",
        RateLimit:   2, // SANİYE başına istek
    }
}

func (c *MyConnector) Timeout() time.Duration { return 30 * time.Second }

func (c *MyConnector) Run(ctx context.Context, target string) ([]plugin.Result, error) {
    // ...
}
```

### 2. Kaydedin

`cmd/osintd/main.go` içindeki `toRegister` dilimine ekleyin. Plugin'ler
derleme zamanında bağlanır.

Connector API anahtarı gerektiriyorsa `keyed` dilimine ekleyin — anahtar
yapılandırılmamışsa hiç kaydedilmez.

### 3. Bağımsızlık sınıfını tanımlayın

**Bu adım zorunludur.** `internal/intel/evidence/cuegroup.go` içindeki
`sourceGroups` haritasına connector adınızı ekleyin.

Atlanırsa `GroupUnknown`'a düşer ve düşük ağırlıkla değerlendirilir — güvenli
bir varsayılan ama muhtemelen istediğiniz şey değil.

Doğru sınıfı seçmek için sorun: **bu kaynak, diğerlerinden bağımsız bir
gözlem mi?** Aynı temel sinyali paylaşan kaynaklar aynı sınıfa girmelidir.
Ayrıntı: [docs/EVIDENCE.md](docs/EVIDENCE.md)

---

## Kurallar

Bunlar stil tercihi değil; her biri gerçek bir hatadan öğrenildi.

### Ücretsiz kaynak kullanın

Ücretli abonelik gerektiren hiçbir kaynak çekirdeğe giremez. Ücretsiz katmanı
olan ama hesap isteyen servisler opsiyonel katmana konur ve anahtar yoksa
kaydedilmez.

Bir kaynak eklemeden önce **ücretsiz hesapla gerçekten çalıştığını** doğrulayın.
Örnek: Shodan'ın ana host API'si ücretsiz hesapla anahtar veriyor ama kredi
vermiyor — connector ücretsiz kurulumda hiç çalışmıyordu.

### "Veri yok" ile "hata" farklıdır

```go
if resp.StatusCode == http.StatusNotFound {
    return nil, nil          // kayıt yok — HATA DEĞİL
}
if resp.StatusCode == http.StatusForbidden {
    return nil, fmt.Errorf("...: rate limit aşıldı")  // gerçek hata
}
```

Bu ayrım kritiktir: hata döndürmek plugin'i devre dışı bırakabilir. Kaydı
olmayan tek bir hedef yüzünden connector'ı öldürmeyin.

Aynı şekilde **hataları yutmayın**. `return nil, nil` ile ağ hatasını
gizlemek, sonucun neden boş olduğunu anlaşılmaz kılar.

### Context'i `json.Marshal` ile üretin

```go
Context: mustJSONContext(map[string]any{
    "platform": "Example",
    "name":     name,
}),
```

Elle `fmt.Sprintf` ile JSON kurmayın. Bu alan kazınmış web içeriğinden
beslenir; tırnak, ters bölü veya kontrol karakteri geçtiğinde bozuk JSON
üretir ve rapor katmanında sessizce kaybolur.

### Anahtarları URL'e koymayın

Header kullanın. Servis yalnızca query parametresi destekliyorsa hata
yollarında `redactURLError()` uygulayın — `net/http` ağ hatalarında tam URL'i
`*url.Error` içine koyar ve o hata log dosyasına yazılır.

### Tahminleri bulgu olarak raporlamayın

Ürettiğiniz bir varsayımı (permütasyon, varyant, çıkarım) doğrulamadan sonuç
listesine koymayın. Doğrulananlar da tahmin oldukları belli edilerek
işaretlenmelidir.

### Testlerde gerçek kişi verisi kullanmayın

Sabit veriler nötr olmalı. Gerçek bir taramadan çıkan ad, kullanıcı adı veya
biyografi — **kendinizinki dahil, özellikle üçüncü şahıslarınki** — test
dosyalarına veya commit mesajlarına girmemelidir.

Bu bir kez ihlal edildi ve düzeltmek geçmişin yeniden yazılmasını gerektirdi.

---

## Test beklentileri

Yeni kod testsiz gelmemeli. Özellikle:

- **Sınır durumları**: boş girdi, bozuk JSON, 404, zaman aşımı
- **Davranış sözleşmeleri**: "404 hata değildir" gibi kurallar teste bağlanmalı,
  çünkü sessizce bozulmaları kolaydır
- **Regresyon testleri**: bir hata düzelttiyseniz, testin adı ve yorumu
  **neyin yanlış gittiğini** anlatsın

İyi bir örnek `plugins/connectors/redact_test.go`: önce redaksiyon olmadan
sırrın gerçekten sızdığını doğrular, sonra kapatıldığını.

---

## Commit mesajları

Ne değiştiğini değil, **neden** değiştiğini yazın. Diff zaten "ne"yi gösterir.

İyi bir mesaj şunlara cevap verir:
- Hangi somut sorun vardı?
- Nasıl fark edildi?
- Neden bu çözüm?
- Alternatif neden seçilmedi?

Ölçüm varsa ekleyin. "78 → 34 varlık" bir paragraf açıklamadan daha çok şey
anlatır.

---

## Bilerek yapılmayanlar

Bunlar eksiklik değil, karar:

**Ağırlıklar otomatik güncellenmiyor.** Kalibrasyon harness'ı ölçer ve önerir;
ayar kararı insana ait. Küçük setlerde aşırı öğrenme riski var.

**Güven puanları "kalibre edilmemiş" diyor.** Ölçüm yapılana kadar bu ibare
kalmalı. Ölçülmemiş bir yüzdeyi doğrulanmış gibi sunmak sistemin verebileceği
en büyük zarardır.

**Boş graf bir hata değil.** Kenar yalnızca farklı bağımsızlık sınıflarından
gelen kaynaklar aynı iki varlığı gördüğünde kurulur. Grafı doldurmak için bu
kuralı gevşetmeyin.
