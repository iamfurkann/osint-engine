// Nitelik gösterim kuralları.
//
// Go tarafındaki internal/report/attributes.go ile aynı mantığı uygular:
// sıra = gösterim önceliği, gürültü anahtarları gizlenir, boş değerler elenir.

const LABELS = [
  // Kimlik — araştırmacının önce görmek istedikleri
  ['name', 'Ad'],
  ['full_name', 'Tam ad'],
  ['display_name', 'Görünen ad'],
  ['profile_username', 'Profil adı'],
  ['bio', 'Biyografi'],
  ['location', 'Konum'],
  ['company', 'Şirket'],
  ['email', 'E-posta'],
  ['blog', 'Web sitesi'],
  ['twitter_username', 'X/Twitter'],

  // Hesap sinyalleri
  ['created', 'Hesap açılışı'],
  ['followers', 'Takipçi'],
  ['following', 'Takip edilen'],
  ['repos', 'Repo sayısı'],
  ['platform', 'Platform'],

  // Teknik
  ['repo', 'Repo'],
  ['language', 'Dil'],
  ['org', 'Organizasyon'],
  ['isp', 'ISP'],
  ['city', 'Şehir'],
  ['country', 'Ülke'],
  ['ports', 'Açık portlar'],
  ['tags', 'Etiketler'],
  ['vuln_count', 'Zafiyet sayısı'],
  ['ip', 'IP'],
];

// Kart yüzeyinde gösterilmeyenler: ya ayrı bir bileşende gösteriliyorlar
// (avatar, güven gerekçesi, rozet) ya da tekrar eden gürültüler.
const HIDDEN = new Set([
  'found', 'username', 'source', 'avatar', 'site',
  'confidence_basis', 'confidence_groups', 'confidence_logodds',
  'independent_sources', 'verification', 'match', 'extracted',
]);

const IDENTITY_KEYS = new Set([
  'name', 'full_name', 'display_name', 'profile_username',
  'bio', 'location', 'company', 'email', 'blog',
  'twitter_username', 'created', 'followers',
]);

export function formatValue(v) {
  if (v === null || v === undefined) return '';
  if (typeof v === 'boolean') return v ? 'evet' : 'hayır';
  if (Array.isArray(v)) return v.map(formatValue).filter(Boolean).join(', ');
  if (typeof v === 'number') return Number.isInteger(v) ? String(v) : String(v);
  return String(v).trim();
}

/** Bir varlığın niteliklerini gösterim sırasına dizer. */
export function orderedAttributes(attrs = {}) {
  const out = [];
  const seen = new Set();

  for (const [key, label] of LABELS) {
    if (HIDDEN.has(key) || !(key in attrs)) continue;
    seen.add(key);
    const value = formatValue(attrs[key]);
    if (value) out.push({ key, label, value });
  }

  // Tanımsız anahtarlar sessizce kaybolmasın.
  for (const key of Object.keys(attrs).sort()) {
    if (seen.has(key) || HIDDEN.has(key)) continue;
    const value = formatValue(attrs[key]);
    if (value) out.push({ key, label: key, value });
  }

  return out;
}

/**
 * Kimlik iddialarını KAYNAĞIYLA listeler.
 *
 * Birleştirme YAPILMAZ. Farklı platformlardaki aynı kullanıcı adı aynı kişi
 * değildir: canlı testte t.me/testuser gerçekten vardı ama başka birine
 * aitti, ve "Deniz Kaya" adı araştırılan kişinin kimliğiymiş gibi
 * gösterilmişti. Her iddia kendi kaynağıyla ayrı satır olarak durur.
 */
export function identityClaims(entities = []) {
  const claims = new Map(); // key -> Map(value -> Set(source))

  for (const e of entities) {
    const attrs = e.attributes || {};
    const source = attrs.platform || (e.sources && e.sources[0]) || 'bilinmiyor';

    for (const [key, value] of Object.entries(attrs)) {
      if (!IDENTITY_KEYS.has(key)) continue;
      const text = formatValue(value);
      if (!text) continue;

      if (!claims.has(key)) claims.set(key, new Map());
      const byValue = claims.get(key);
      if (!byValue.has(text)) byValue.set(text, new Set());
      byValue.get(text).add(source);
    }
  }

  const out = [];
  for (const [key, label] of LABELS) {
    const byValue = claims.get(key);
    if (!byValue) continue;
    for (const [value, sources] of byValue) {
      out.push({ key, label, value, sources: [...sources] });
    }
  }
  return out;
}

/** Güven puanı → renk/etiket bandı. */
export function confidenceBand(score) {
  if (score >= 80) return { level: 'high', label: 'Yüksek' };
  if (score >= 60) return { level: 'medium', label: 'Orta' };
  if (score >= 35) return { level: 'low', label: 'Düşük' };
  return { level: 'minimal', label: 'Çok düşük' };
}

/** Varlığın taşıdığı uyarı rozetleri. */
export function badges(entity) {
  const attrs = entity.attributes || {};
  const out = [];

  if (attrs.match) out.push({ kind: 'variant', text: 'varyant eşleşme' });
  if (attrs.verification) out.push({ kind: 'suspect', text: attrs.verification });
  if (attrs.extracted) out.push({ kind: 'derived', text: 'biyografiden çıkarıldı' });

  return out;
}

/** Bilgi yoğunluğuna göre sıralama — dolu kayıtlar üstte. */
export function sortByInformation(entities = []) {
  return [...entities].sort((a, b) => {
    const na = orderedAttributes(a.attributes).length;
    const nb = orderedAttributes(b.attributes).length;
    if (na !== nb) return nb - na;
    if (b.confidence !== a.confidence) return b.confidence - a.confidence;
    return (a.primary_value || '').localeCompare(b.primary_value || '');
  });
}
