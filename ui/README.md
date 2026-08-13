# OSINT Engine — Web Arayüzü

React + Vite. `osintd` daemon'ının HTTP API'sini tüketir.

## Çalıştırma

Önce daemon:

```bash
cd .. && make build && ./bin/osintd start
```

Sonra arayüz:

```bash
npm install
npm run dev
```

http://localhost:5173 adresinde açılır.

> Daemon `127.0.0.1:8080`'e bağlıdır ve CORS **kapalı bir izin listesi** kullanır.
> Vite'ın geliştirme (5173) ve önizleme (4173) portları listede. Farklı bir port
> kullanacaksanız `internal/api/server.go` içindeki `allowedOrigins` listesine
> eklemeniz gerekir.

## Yapı

```
src/
├── lib/
│   ├── api.js          HTTP istemcisi
│   └── attributes.js   nitelik sıralama, kimlik iddiaları, güven bantları
│                       (Go tarafındaki internal/report/attributes.go ile aynı mantık)
├── components/
│   ├── SearchPanel.jsx   hedef girişi, tip seçimi, özyinelemeli mod, ipuçları
│   ├── WatchPanel.jsx    izleme listesi
│   ├── IdentityPanel.jsx kimlik iddiaları — her biri kaynağıyla
│   ├── EntityCard.jsx    varlık kartı: avatar, güven çubuğu, rozetler, nitelikler
│   └── GraphView.jsx     Cytoscape ilişki grafı
├── App.jsx             kabuk, durum, sekmeler, filtreler
└── styles.css          tasarım sistemi (token tabanlı, satır içi stil yok)
```

## Tasarım kararları

**Kimlik "özeti" değil, "iddiaları".** Farklı platformlardaki aynı kullanıcı adı
aynı kişi değildir. Panel değerleri birleştirmez; her iddiayı kaynağıyla ayrı
satırda gösterir ve çelişkileri görünür bırakır. Aynı değer birden çok kaynakta
geçiyorsa tek satırda toplanır — bu gerçek bir çapraz doğrulama sinyalidir.

**Güven puanı her zaman gerekçesiyle.** Kartlarda yüzdenin altında hangi
bağımsızlık sınıflarının ne kadar katkı verdiği yazar ve puanın
"kalibre edilmemiş" olduğu belirtilir.

**Boş graf bir hata değil.** Kenar yalnızca farklı bağımsızlık sınıflarından
gelen kaynaklar aynı iki varlığı birlikte gördüğünde kurulur. "Aynı connector
buldu" bir ilişki değil, ortak kökendir.

## Komutlar

| Komut | Açıklama |
|---|---|
| `npm run dev` | Geliştirme sunucusu |
| `npm run build` | Üretim derlemesi (`dist/`) |
| `npm run preview` | Derlemeyi önizle |
| `npm run lint` | oxlint |
