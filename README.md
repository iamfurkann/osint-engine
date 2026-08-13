# OSINT Engine

Plugin tabanlı, tamamen ücretsiz kaynaklarla çalışan açık kaynak istihbarat motoru.
Go ile yazılmış tek binary çekirdek + CLI + daemon + web arayüzü.

**Tasarım ilkesi:** araç "şu kişi burada" demez. Ne bulduğunu, **nereden** bulduğunu
ve **ne kadar emin olmadığını** söyler.

---

## Neden bir tane daha OSINT aracı

Çoğu kullanıcı adı tarayıcısı size bir URL listesi verir. Sorun şu ki:

- Listenin bir kısmı **uydurmadır** — birçok platform var olmayan kullanıcı adları için de HTTP 200 döner.
- Bulunan profillerden **hiçbir şey çıkarılmaz** — "şu adreste bir hesap var" der, geçer.
- Farklı platformlardaki aynı kullanıcı adı **aynı kişi sanılır**.
- "Güven skoru" varsa bile genellikle **ölçülmemiş** bir sayıdır.

Bu araç dördünü de doğrudan hedefler.

---

## Hızlı başlangıç

```bash
make build
```

Daemon'ı başlatın:

```bash
./bin/osintd start
```

Bir araştırma çalıştırın:

```bash
./bin/osint inv create ornekkullanici
```

Sonuçlara bakın:

```bash
./bin/osint view <inv-id>
```

HTML rapor üretin:

```bash
./bin/osint report generate <inv-id> --out rapor.html
```

Web arayüzü (ayrı terminal):

```bash
cd ui && npm install && npm run dev
```

Bitince:

```bash
./bin/osintd stop
```

> **Hiçbir API anahtarı gerekmez.** Çekirdek connector'ların tamamı ücretsiz ve
> anahtarsız kaynaklara bağlanır. Ayrıntı: [Ücretsiz-only politikası](#ücretsiz-only-politikası)

---

## Ne yapar

Bir hedef (kullanıcı adı, alan adı, IP, e-posta) verirsiniz; motor uygun
connector'ları paralel çalıştırır, bulguları varlıklara çözümler, **kanıt
bağımsızlığını** hesaba katarak güven puanı üretir ve gerekçesiyle raporlar.

### Kullanıcı adı araştırmasında

1. **Varlık kontrolü** — 76 platformda kullanıcı adı aranır.
2. **Platform kalibrasyonu** — her platform rastgele, var olmayan bir kullanıcı
   adıyla test edilir; "kullanıcı yok" sayfasının parmak izi çıkarılır. Sonuçla
   eşleşen her şey yanlış pozitif olarak elenir.
3. **Varyant denemesi** — `_ad`, `ad_`, `_ad_`, `adofficial`, önek soyma. İstenen
   kullanıcı adı alınmışsa gerçek hesap genellikle burada bulunur.
4. **Zenginleştirme** — bulunan her profilin Open Graph etiketlerinden görünen ad,
   biyografi ve avatar çıkarılır. Sayfa zaten indirilmiş durumda olduğu için
   **ek istek yapılmaz**.
5. **Varlık çıkarma** — biyografilerdeki e-posta, kişisel site ve çapraz platform
   `@kullanıcı adları` ayrıştırılır.
6. **Pivot** (`--recursive`) — çıkarılan varlıklar yeni taramalar tetikler.
7. **Kanıt füzyonu** — bağımsızlık sınıflarına göre güven puanı ve gerekçe.

---

## Komutlar

| Komut | Ne yapar |
|---|---|
| `osint inv create <hedef> [--recursive]` | Araştırma başlatır (tip otomatik algılanır) |
| `osint inv list` | Bellekteki aktif araştırmalar |
| `osint inv status <id>` | İlerleme |
| `osint inv cancel <id>` | İptal |
| `osint inv export <id> --format json\|csv\|graphml` | Dışa aktarım |
| `osint view <id>` | Bulguları terminalde gösterir |
| `osint report generate <id> --out x.html` | HTML rapor |
| `osint watch add\|list\|remove` | Sürekli izleme |
| `osint calibrate init\|run` | Güven puanlarını ölçer ([detay](#kalibrasyon)) |
| `osint keys set <servis> <key>` | Opsiyonel servisler için anahtar |
| `osintd start\|stop\|status` | Daemon yönetimi |

### Henüz çalışmayanlar

Dürüstlük adına: bu komutlar mevcut ama **iş yapmıyor**.

| Komut | Durum |
|---|---|
| `osint search <tip> <hedef>` | Orchestrator'ı hiç çağırmıyor. `inv create` kullanın |
| `osint graph stats\|neighbors\|path` | Hep boş bir grafa bakıyor |
| `osint plugin list\|info` | Sabit metin basıyor |
| `osint keys list` | Desteklenmiyor |
| `osint inv pause\|resume` | Daemon "not implemented" döndürür |

---

## Veri kaynakları

Hiçbiri anahtar gerektirmez:

| Connector | Kaynak | Girdi |
|---|---|---|
| `username-check` | 76 platformda kullanıcı adı varlığı + profil metadata'sı | username |
| `social-profile` | GitHub kullanıcı API'si (kimliksiz) | username |
| `shodan-internetdb` | `internetdb.shodan.io` — açık port, hostname, CVE, CPE | ip |
| `dns-whois` | DNS kayıtları (A/AAAA/MX/NS/TXT/CNAME) | domain |
| `crtsh` | Sertifika şeffaflığı → alt alan adları | domain |
| `wayback` | Wayback Machine arşivi | domain, username |
| `web-scraper` | DuckDuckGo HTML sonuçları | person, username, email |
| `gravatar` | Gravatar profili | email |
| `name-generator` | Ad → kullanıcı adı permütasyonları (yerel, ağ yok) | person |

Opsiyonel — ücretsiz katmanı var ama **hesap ve anahtar** ister. Anahtar
yapılandırılmamışsa connector hiç kaydedilmez:

| Connector | Ücretsiz katman |
|---|---|
| `virustotal` | 4 istek/dk, 500/gün |
| `hunter` | 25 arama/ay |

---

## Ücretsiz-only politikası

Ücretli abonelik gerektiren hiçbir kaynak çekirdeğe girmez.

Bu yalnızca bütçe meselesi değil, **daha iyi OSINT tradecraft'ı**: bir bulguyu
ancak ücretli bir kapının arkasından doğrulayabiliyorsanız, o bulgu bağımsız
olarak tekrarlanamaz. Ayrıca yerel ve açık kaynaklar OPSEC açısından üstündür —
üçüncü bir tarafa neyi araştırdığınızı söylemezsiniz.

Denetim sonucu iki connector kaldırıldı veya değiştirildi:

- **Shodan** → `internetdb.shodan.io`. Ana REST API'si (`/shodan/host/`) bir
  Membership gerektiriyor; ücretsiz hesap anahtar veriyor ama sorgu kredisi
  vermiyor, yani connector ücretsiz kurulumda **zaten hiç çalışmıyordu**.
  InternetDB anahtarsız ve kotasız; karşılığında banner ve ürün/sürüm bilgisi yok.
- **HIBP** → kaldırıldı. `/breachedaccount/` aylık abonelik gerektiriyor.

### Açık kalan gizlilik sorunu

`internal/ai/gemini.go` rapor üretimi için bulguları Google'a gönderir. Ücretsiz
katmanı var ama **bulut**. Yerel bir modele geçilene kadar hassas araştırmalarda
kullanmayın — arayüz bu uyarıyı gösterir.

---

## Güven puanları ve kanıt

Bu, projenin ayırt edici kısmı. Ayrıntılı anlatım: **[docs/EVIDENCE.md](docs/EVIDENCE.md)**

Özet olarak üç kural:

**1. Aynı kaynaktan gelen kanıt çarpılmaz.** Her connector bir *bağımsızlık
sınıfına* (CueGroup) aittir. Bir taramanın 40 sonucu 40 bağımsız kanıt değil,
1 taramadır. Grup içi katkı sönümlenir ve tavanlanır; yüksek güven ancak
**farklı** gruplardan gelen kanıtla mümkündür.

**2. Aynı kullanıcı adı aynı kişi değildir.** Kimlik bilgileri tek bir profile
birleştirilmez. Her iddia kendi kaynağıyla ayrı gösterilir ve çelişkiler
görünür kalır:

```
Görünen ad   Deniz Kaya      ← Telegram
             Ada Yilmaz      ← Hugging Face
             Mira            ← Twitter/X
```

**3. Puanlar kalibre edilmemiştir.** Ağırlıklar mühendislik tahminidir ve her
açıklama bunu açıkça söyler. Ölçmek için bir harness var (aşağıda) ama ölçüm
yapılmadan hiçbir yüzde "doğrulanmış" gibi sunulmaz.

---

## Kalibrasyon

"%70 güven" ifadesi, ölçülmüş bir doğrulama setine dayanmıyorsa yanıltıcıdır.
Harness sorulması gereken soruyu sorar: sistem %70 dediğinde gerçekten 10
vakanın 7'sinde haklı mı?

```bash
./bin/osint calibrate init <inv-id> --out cases.toml
```

`cases.toml` içinde `unlabeled` altındaki değerleri `confirmed` / `rejected`
listelerine taşıyın — yer gerçeğini ancak doğruyu bilen bir insan verebilir.

```bash
./bin/osint calibrate run cases.toml
```

Çıktı: ECE, MCE, Brier skoru, güvenilirlik diyagramı ve **kaynak grubu başına
ölçülmüş isabet oranı**. Ayrıca Platt ölçekleme parametreleri önerir.

Ağırlıklar bilerek otomatik güncellenmez — küçük setlerde aşırı öğrenme riski
vardır. Harness ölçer ve önerir; ayar kararı insana aittir.

---

## Mimari

Ayrıntı: **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**

```
osint (CLI) ──unix socket──► osintd (daemon) ──► HTTP API :8080 ──► ui (React)
                                   │
                             Orchestrator
                    öncelikli kuyruk · worker havuzu
                    rate limit · retry · lifecycle izolasyonu
                                   │
                              Connector'lar
                                   │
                        Resolution → Evidence Engine
                                   │
                            SQLite · rapor
```

- **Tek binary**, CGO'suz (`modernc.org/sqlite`)
- **Plugin sözleşmesi:** `Run(ctx, target string) ([]Result, error)`
- Daemon **yalnızca `127.0.0.1`** dinler, CORS kapalı izin listesi kullanır

---

## Geliştirme

```bash
make build        # her iki binary
make test         # testler
make test-race    # yarış koşulu detektörü (CI bunu kullanır)
make lint         # golangci-lint
```

Yeni connector eklemek ve uyulması gereken kurallar: **[CONTRIBUTING.md](CONTRIBUTING.md)**

---

## Durum ve bilinen sınırlar

Çalışıyor: connector'lar, orchestrator, zenginleştirme, varyantlar, platform
kalibrasyonu, Evidence Engine, pivot zinciri, HTML rapor, web arayüzü,
kalibrasyon harness'ı.

Bilinen sınırlar:

- **Güven puanları kalibre edilmemiş.** Ölçüm aracı var, ölçüm yapılmadı.
- **Varlık çözümlemesi birebir string eşleşmesi.** Aynı değer farklı bulgu
  tipleriyle geldiğinde ayrı varlık olur (`hostname:x` ≠ `dns_record:x`).
- **`web-scraper` kırılgan.** DuckDuckGo HTML'i kazıyor; sayfa değişirse susar.
- Yukarıdaki [stub komutlar](#henüz-çalışmayanlar).
- **Lisans dosyası yok.** Depoya bir lisans eklenmesi gerekiyor.

---

## Etik

Bu araç kişilerin çevrimiçi varlığını haritalayabilir. Meşru kullanım
(gazetecilik, doğrulama, kendi maruziyetinizi ölçme) ile taciz arasındaki fark
araçta değil kullanımdadır — ancak tasarım bu farkı destekler:

- Belirsizlik gizlenmez; aşırı güvenli çıktı yanlış kişinin hedef alınmasına yol açar
- Her iddia kaynağıyla gösterilir, çelişkiler bastırılmaz
- Varsayılan olarak hiçbir veri üçüncü tarafa gönderilmez

Kişisel veri işlerken KVKK/GDPR yükümlülükleri sizindir.
