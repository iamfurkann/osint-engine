# Kanıt, Güven ve Kalibrasyon

Bu belge projenin ayırt edici kısmını anlatır: bir bulgunun ne kadar
güvenilir olduğuna nasıl karar verildiği.

## Problem

Bir OSINT aracı 40 platformda aynı kullanıcı adını bulduğunda, elinde 40
bağımsız kanıt yoktur. **Bir tarama** vardır. Ama naif bir skorlama bunu 40
doğrulama sayar ve güveni tavana çıkarır.

Aynı hata daha büyük ölçekte de olur: beş farklı yapay zekâ modeli aynı
cevabı verdiğinde, hepsi aynı temel özellikleri kullanıyorsa bu beş kanıt
değil, yaklaşık bir buçuk kanıttır. Hepsi birlikte yanılabilir.

Bu, sistemin verebileceği en büyük zarardır: **kanıt varmış gibi görünen ama
aslında olmayan kesinlik.**

## Çözüm: bağımsızlık sınıfları (CueGroup)

Her kaynak bir bağımsızlık sınıfına aittir.

| Sınıf | Ne iddia eder | Ağırlık |
|---|---|---|
| `platform.profile` | Sayfada gerçek bir kimlik yayınlanıyor | 1.30 |
| `dns` | Yetkili DNS kaydı | 1.20 |
| `certificate` | Sertifika şeffaflığı kaydı | 1.20 |
| `infra.scan` | Port/servis taraması | 1.00 |
| `email` | E-posta keşif servisi | 0.90 |
| `reputation` | İtibar servisi | 0.80 |
| `web.archive` | Arşiv kaydı | 0.70 |
| `web.search` | Arama motoru sonucu | 0.50 |
| `platform.presence` | Sayfa 200 döndü | 0.45 |
| `unknown` | Tanımsız kaynak | 0.30 |
| `derived` | **Sistemin kendi tahmini** | 0.05 |

İki ayrım özellikle önemli:

**`platform.presence` ile `platform.profile` ayrıdır.** Varlık kontrolü
"sayfa 200 döndü" der. Profil verisi "sayfada gerçek bir kimlik var" der.
Bunlar farklı iddialardır ve farklı ağırlık taşırlar.

**`derived` neredeyse sıfırdır.** Kullanıcı adı permütasyonları ve
biyografiden çıkarılan desenler dış dünyadan gelen gözlem değil, sistemin
kendi hipotezidir. Kanıt olarak sayılmamalıdırlar.

## Skorlama

Log-odds birikimi:

```
logit = prior + Σ_gruplar  damped(grup)
```

Grup içinde sönümleme:

```
damped(grup) = min( en_güçlü + α · Σ(kalanlar),  en_güçlü × tavan )  + negatifler
               α = 0.20, tavan = 2.0
```

### Tavan neden gerekli

Sönümleme tek başına yetmiyor — testi bunu yakaladı.

`α · Σ(kalan)` ifadesi gözlem sayısıyla **doğrusal büyür**. α = 0.20 ile bile
40 gözlem toplam +3.96 katkı üretiyordu, yani %94 güven. Tek bir taramadan.

Tavanla birlikte bir bağımsızlık sınıfı, kaç gözlem içerirse içersin skoru
domine edemez. **Yüksek güven ancak farklı gruplardan gelen kanıtla mümkündür**
— motorun bütün amacı budur.

### Kanıt gücü ayarlayıcıları

| Durum | Etki |
|---|---|
| Profil verisi var (ad/bio/avatar) | ×1.6 |
| Varyant eşleşmesi (aranan adın kendisi değil) | ×0.5 |
| Şüpheli işaretli (platform kalibrasyonundan) | **−0.25** |
| Sistem tahmini | ağırlık 0.05 |

Şüpheli sonuç kanıt değil, **karşı kanıttır**: connector bize açıkça "bu
platform ayırt edemiyor" diyor.

Negatif katkılar sönümlenmeden ve tavansız uygulanır. Karşı kanıtı yumuşatmak,
sahte kesinlik üretmenin bir başka yoludur.

## Yanlış pozitiflere karşı: platform kalibrasyonu

Bazı platformlar var olmayan **her** kullanıcı adı için HTTP 200 döner.
Bazıları ayrıca jenerik Open Graph etiketleri yayınlar, yani "metadata var mı"
testi de onları eleyemez.

Sabit bir kural listesi yazmak yerine her platform **kendi kontrol örneğiyle**
kalibre edilir:

1. Rastgele, var olmayan bir kullanıcı adı istenir.
2. "Kullanıcı yok" sayfasının parmak izi çıkarılır (görünen ad + biyografi).
3. Gerçek bir sonuç bu imzayla eşleşiyorsa yanlış pozitiftir.

Karşılaştırma kullanıcı adına duyarsızdır — bazı siteler adı sayfa metnine
gömer (`"Contact @kullanıcı"`), ham karşılaştırma hiçbir zaman eşleşmez.

Bu yaklaşım kendi kendini kalibre eder: bir platform davranışını değiştirse
bile kod değişikliği gerekmez. Maliyeti platform başına tek bir ek istektir.

İki kademeli davranış:

- **varyant + eşleşme** → hiç raporlanmaz (zaten tahmindi, kanıt yok)
- **tam eşleşme + eşleşme** → raporlanır ama "şüpheli" işaretiyle
  (kullanıcı aradığı adı sonuçlarda hiç görmemektense şüpheli görmeli)

## Korelasyon: ortak köken ilişki değildir

"Aynı connector buldu" bir ilişki değildir. İki port numarasının ilişkili
olduğu, ikisini de aynı taramanın bulmuş olmasından çıkarılamaz. Bu bilgi
zaten `Entity.Sources` alanında durur.

Eskiden bu kural yoktu ve sonucu ölçüldü: tek bir HTTP isteğinden 13 varlık
çıkaran bir araştırma **78 "ilişki"** raporluyordu — tam bir K₁₃ grafı, sıfır
bilgi. Kenar sayısı ayrıca API katmanında çift sayılıyordu.

Kenar artık yalnızca paylaşılan kaynaklar **farklı bağımsızlık sınıflarına**
yayıldığında kurulur. O zaman gerçek bir çapraz doğrulama vardır.

## Kimlik: aynı kullanıcı adı aynı kişi değildir

Kimlik bilgileri tek bir profile **birleştirilmez**.

Bu kural bir hatadan öğrenildi. İlk tasarım tüm platformlardan gelen değerleri
birleştiriyordu. Ama bir platformdaki hesap gerçekten var olabilir ve **başka
birine** ait olabilir — o kişinin adı araştırılan kişinin kimliğiymiş gibi
görünür.

Artık her iddia kendi kaynağıyla ayrı satırda durur:

```
Görünen ad   Deniz Kaya      ← Telegram
             Ada Yilmaz      ← Hugging Face
             Mira            ← Twitter/X
Profil adı   ornekkullanici  ← GitHub, Twitter/X
```

Çelişkiler görünür kalır ve karar araştırmacıya aittir. Aynı değer birden çok
kaynakta geçiyorsa tek satırda toplanır — **bu gerçek bir çapraz doğrulama
sinyalidir**.

## Kalibrasyon: puanlar gerçekten doğru mu

Yukarıdaki ağırlıkların hepsi **mühendislik tahminidir**. `Score.Calibrated`
alanı bunu taşır ve her açıklama "kalibre edilmemiş" ibaresiyle biter. Bunu
zorunlu kılan bir test vardır.

Kalibre edilmemiş bir yüzdeyi ölçülmüş gibi sunmak araştırmacıyı yanıltır.

### Ölçüm

```bash
osint calibrate init <inv-id> --out cases.toml
```

`unlabeled` altındaki değerleri `confirmed` / `rejected` listelerine taşıyın.
Yer gerçeğini elle etiketlemekten kaçış yoktur — sistemin haklı olup olmadığını
ancak doğruyu bilen biri söyleyebilir.

```bash
osint calibrate run cases.toml
```

Çıktı:

| Metrik | Anlamı |
|---|---|
| **ECE** | Kova ağırlıklı ortalama sapma. <0.05 iyi, >0.20 yanıltıcı |
| **MCE** | En kötü kovadaki sapma |
| **Brier** | Ortalama kare hata — kalibrasyon + ayırt edicilik |
| **Güvenilirlik diyagramı** | Her güven aralığında iddia vs gerçek |
| **Kaynak grubu güvenilirliği** | Yukarıdaki ağırlıkların **ölçülmüş** karşılığı |

30 örneğin altında "yetersiz örnek" verdikti verilir — kalibrasyonun kendi
kendini kandırmasına karşı koruma.

### Düzeltme: neden yalnızca sıcaklık yetmez

Klasik yöntem sıcaklık ölçeklemedir: `p' = sigmoid(logOdds / T)`.

Bu **yetmez** ve testi bunu yakaladı. T yalnızca keskinliği değiştirir,
işareti değiştiremez: `logOdds = +3.0` iken `sigmoid(3/T)` T ne olursa olsun
0.5'in altına inemez. Oysa elle ayarlanmış ağırlıklardan beklenen asıl hata
**sistematik yanlılıktır**.

Bu yüzden Platt ölçekleme kullanılır:

```
p' = sigmoid(logOdds / T + b)
```

Kayma terimi `b` yanlılığı düzeltir. İki parametre olduğu için küçük
doğrulama setlerinde bile aşırı öğrenme riski düşüktür.

### Ağırlıklar neden otomatik güncellenmiyor

Küçük setlerde aşırı öğrenme riski vardır. Harness **ölçer ve önerir**; ayar
kararı insana aittir. Güvenilir bir ölçüm için en az 100 etiketli örnek gerekir.

## Özet: sahte kesinliğe karşı kontrol listesi

| Risk | Önlem |
|---|---|
| Korelasyonlu kanıt patlaması | CueGroup sönümlemesi + grup tavanı |
| Tek kaynağın skoru domine etmesi | Grup katkısı tavanı |
| Tahminlerin kanıt sayılması | `derived` sınıfı, ağırlık 0.05 |
| Platform yanlış pozitifleri | Kontrol örneğiyle kalibrasyon |
| Farklı kişilerin karışması | Kimlik iddiaları birleştirilmez |
| Ortak kökenin ilişki sanılması | Kenar için farklı sınıf şartı |
| Kalibre edilmemiş yüzde | `Calibrated` alanı + açıklamada ibare |
| Sessiz çelişki | Karşı kanıt sönümlenmez, görünür kalır |
