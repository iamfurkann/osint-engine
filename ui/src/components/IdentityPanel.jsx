import { identityClaims } from '../lib/attributes';

/**
 * Kimlik iddiaları — HER BİRİ KAYNAĞIYLA.
 *
 * Bu panel bilerek "özet" değil "iddialar" diyor. Farklı platformlardaki aynı
 * kullanıcı adı aynı kişi değildir: canlı testte t.me/testuser gerçekten
 * vardı ama başka birine aitti, ve önceki birleştirmeli tasarım "Deniz Kaya"
 * adını araştırılan kişinin kimliğiymiş gibi göstermişti.
 */
export default function IdentityPanel({ entities }) {
  const claims = identityClaims(entities);
  if (claims.length === 0) return null;

  let lastLabel = null;

  return (
    <section>
      <h2 className="section-title">
        Kimlik İddiaları <span className="count">{claims.length}</span>
      </h2>

      <div className="notice">
        <strong>⚠ Farklı platformlardaki aynı kullanıcı adı aynı kişi olmayabilir.</strong>
        {' '}Aşağıdaki her satır ilgili platformun iddiasıdır — doğrulanmış gerçek değildir.
        Aynı değer birden çok kaynakta geçiyorsa tek satırda toplanır; bu gerçek
        bir çapraz doğrulama sinyalidir.
      </div>

      <div className="claims">
        {claims.map((c, i) => {
          const showLabel = c.label !== lastLabel;
          lastLabel = c.label;
          return (
            <div className="claim-row" key={`${c.key}-${i}`}>
              <span className="claim-label">{showLabel ? c.label : ''}</span>
              <span className="claim-value">{c.value}</span>
              <span className="claim-source">← {c.sources.join(', ')}</span>
            </div>
          );
        })}
      </div>
    </section>
  );
}
