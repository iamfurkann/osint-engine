# Mimari

## Süreç topolojisi

```
┌──────────────┐   unix socket    ┌────────────────────────────────┐
│ osint (CLI)  │ ───────────────► │ osintd (daemon)                │
└──────────────┘  ~/.osint/       │                                │
                  osintd.sock     │  ┌──────────────────────────┐  │
┌──────────────┐   HTTP           │  │ Orchestrator             │  │
│ ui (React)   │ ───────────────► │  │  öncelikli kuyruk        │  │
└──────────────┘  127.0.0.1:8080  │  │  worker havuzu           │  │
                                  │  │  rate limit · retry      │  │
                                  │  │  lifecycle izolasyonu    │  │
                                  │  └───────────┬──────────────┘  │
                                  │              ▼                 │
                                  │        Connector'lar           │
                                  │              ▼                 │
                                  │   Resolution → Evidence Engine │
                                  │              ▼                 │
                                  │        SQLite · rapor          │
                                  └────────────────────────────────┘
```

Her şey tek bir Go binary'sinde, CGO olmadan (`modernc.org/sqlite`).

## Katmanlar

| Paket | Sorumluluk |
|---|---|
| `cmd/osint`, `cmd/osintd` | Giriş noktaları |
| `internal/cli` | Cobra komutları |
| `internal/ipc` | Unix socket, satır sonlu JSON protokolü |
| `internal/api` | HTTP API (yalnızca loopback) |
| `internal/daemon` | Süreç ömrü, PID, IPC handler kaydı |
| `internal/engine` | Registry, lifecycle, loader, scaffold |
| `internal/engine/orchestrator` | Görev akışının tamamı |
| `internal/engine/{queue,worker,ratelimit,retry,cache,checkpoint}` | Yürütme altyapısı |
| `pkg/plugin` | Plugin sözleşmesi (dışa açık) |
| `plugins/connectors` | Veri kaynakları |
| `internal/intel/resolution` | Bulgu → varlık çözümlemesi |
| `internal/intel/evidence` | Kanıt bağımsızlığı ve güven puanı |
| `internal/intel/calibration` | Puanların ölçümü |
| `internal/intel/correlation` | Varlıklar arası ilişkiler |
| `internal/report` | Nitelik gösterimi, HTML rapor |
| `internal/domain` | İş modelleri + repository arayüzleri |
| `internal/repository/sqlite` | Kalıcılık |

Bağımlılık yönü tek taraflıdır: `domain` arayüzleri tanımlar, `repository`
implemente eder. Orchestrator bağımlılıklarını `Deps` struct'ıyla dışarıdan alır.

## Veri akışı

```
hedef (string)
   │
   ├─ input.Detect()  →  username | domain | email | ip | url | hash
   │
   ├─ Registry: bu girdi tipini kabul eden aktif plugin'ler
   │
   ├─ her plugin için Task → PriorityQueue
   │
   ├─ worker havuzu (N goroutine)
   │     ├─ lifecycle.IsActive()?
   │     ├─ rate limit bekle
   │     ├─ cache kontrolü
   │     ├─ retry.Do → plugin.Run(ctx, target)
   │     └─ hata → lifecycle.MarkError()  (ardışık eşikle)
   │
   ├─ Result → domain.Finding → SQLite
   │
   ├─ pivot (opsiyonel): bulgu → yeni girdi tipi → yeni görev
   │
   └─ BuildGraph:
        findings → resolution.Entity
                 → evidence.Score()   ← güven puanı burada hesaplanır
                 → correlation
                 → graph
```

## Plugin sözleşmesi

```go
type Result struct {
    Type    string // "email", "username_presence", "open_port", ...
    Value   string
    Context string // JSON metadata — zenginleştirme verisi burada taşınır
}

type Plugin interface {
    Manifest() Manifest
    Timeout() time.Duration
    Run(ctx context.Context, target string) ([]Result, error)
}
```

Plugin'ler **derleme zamanında** bağlanır — `cmd/osintd/main.go` içinde
kaydedilirler. `plugin.Open`, gRPC veya alt süreç yoktur.

`Context` alanı kritiktir: gerçek istihbarat verisi (ad, biyografi, avatar,
takipçi sayısı) burada taşınır ve `resolution.Entity.Attributes`'a aktarılır.
`json.Marshal` ile üretin — `mustJSONContext()` yardımcısı bunun içindir.
Elle `fmt.Sprintf` ile JSON kurmak, kazınmış içerikte tırnak veya kontrol
karakteri geçtiğinde bozuk çıktı üretir.

## Yürütme altyapısı

**Öncelikli kuyruk** — `container/heap`, öncelik bandı içinde FIFO garantisi.

**Worker havuzu** — N goroutine, kuyruk sinyaliyle uyanır.

**Rate limit** — plugin başına token bucket. Manifest'teki `RateLimit`
**saniye başına** yorumlanır.

**Retry** — üstel geri çekilme + jitter. `retry.NewPermanentError` ile
sarmalanan hatalar tekrar denenmez.

**Lifecycle izolasyonu** — bir plugin hata verdiğinde çekirdek etkilenmez.
Tek bir geçici hata plugin'i devre dışı bırakmaz; ardışık hata eşiği vardır
ve başarılı çalıştırma sayacı sıfırlar. Bu olmadan tek bir geçici 502 bir
connector'ı daemon ömrü boyunca öldürüyordu.

**Pivot** — `--recursive` ile bulgular yeni taramalar tetikler. Bulgu tipleri
ile plugin girdi tipleri farklı sözlükler kullandığı için `orchestrator/pivot.go`
bir eşleme yapar ve kasıtlı olarak seçicidir: `username_presence`,
`social_profile`, `open_port` gibi tipler pivot **etmez** — zaten elimizdeki
kayıtlardır, taranmaları yalnızca pivot bütçesini tüketir.

## Veritabanı

SQLite, WAL modu, CGO'suz sürücü. Migration'lar `internal/db/migrate.go` içinde
sıralı bir dilimdir; dilim indeksi sürüm numarasıdır.

| Tablo | İçerik |
|---|---|
| `investigations` | Araştırma kaydı |
| `findings` | Connector çıktıları (`context` sütununda JSON metadata) |
| `plugins` | Plugin durum kaydı |
| `watchlist` | İzleme listesi |

## Güvenlik sınırları

- **HTTP API yalnızca `127.0.0.1`** dinler. CORS kapalı bir izin listesidir
  (Vite dev/preview portları). DNS rebinding'e karşı `Host` başlığı denetlenir.
- **IPC soketi** `0600`, dizin `0700`.
- **API anahtarları** AES-256-GCM ile şifrelenir. Ortam değişkeninden gelen
  parola argon2id ile türetilir. Keystore yazımı atomiktir (temp + fsync + rename).
- **Hata yollarında URL redaksiyonu** — bazı API'ler (Shodan) yalnızca query
  parametresiyle kimlik doğruluyor, ve `net/http` ağ hatalarında tam URL'i
  `*url.Error` içine koyuyor. Redaksiyon olmadan anahtar log dosyasına düşer.

## Bilinen mimari sınırlar

**Plugin sözleşmesi string girer, string çıkar.** İkili veri (görüntü, ham
dosya) taşıyamaz ve yapısal sonuç (geometri, koordinat, olasılık alanı)
ifade edemez. Görsel analiz gibi yönler için `Artifact` + `Observation`
soyutlamalarına ihtiyaç olur.

**Varlık çözümlemesi birebir string eşleşmesidir.** Normalizasyon tip önekiyle
yapıldığı için aynı değer farklı bulgu tipleriyle geldiğinde ayrı varlık olur:
`hostname:example.com` ≠ `dns_record:example.com`.

**Bağlanmamış alt sistemler var.** Aşağıdakiler yazılmış ve testli ama üretim
yolunda iş yapmıyor:

| Paket | Durum |
|---|---|
| `engine/cache` | `Deps.Cache` alanı `cmd/osintd` içinde hiç doldurulmuyor → nil → önbellek kapalı |
| `engine/checkpoint` | Kendi paketi dışında referans yok |
| `engine/loader` | Manifest tarama; repoda hiç `manifest.toml` yok |
| `internal/web` | Gömülü Cytoscape görüntüleyici; `NewServer` çağrılmıyor |
| `internal/tui` | Bubbletea monitör; `StartTUI` çağrılmıyor |
| `internal/watch` | `engine/watch` canlı olan; bu ikiz sahte veri üretiyor |
