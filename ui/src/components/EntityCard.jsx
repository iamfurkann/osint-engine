import { useState } from 'react';
import { orderedAttributes, confidenceBand, badges } from '../lib/attributes';

/**
 * Tek bir varlığın kartı.
 *
 * Önceki arayüz yalnızca URL listeliyordu. Connector'lar ad, biyografi,
 * avatar ve hesap yaşını zaten topluyordu ama hiçbiri gösterilmiyordu.
 */
export default function EntityCard({ entity }) {
  // Instagram, Twitch ve X avatarları hotlink'i 403 ile engelliyor.
  // Kırık görseli gizlemek boşluk bırakıyordu; baş harfe düşülüyor.
  const [avatarFailed, setAvatarFailed] = useState(false);

  const attrs = entity.attributes || {};
  const band = confidenceBand(entity.confidence);
  const marks = badges(entity);

  const displayName =
    attrs.name || attrs.display_name || attrs.full_name || null;
  const platform = attrs.platform || attrs.site || entity.type;
  const isURL = /^https?:\/\//.test(entity.primary_value || '');

  // Kart başlığında ayrıca gösterilenler listeden çıkarılır.
  const details = orderedAttributes(attrs).filter(
    (a) => !['bio', 'name', 'display_name', 'full_name', 'platform'].includes(a.key),
  );

  return (
    <article className="card">
      <div className="card-head">
        {attrs.avatar && !avatarFailed ? (
          <img
            className="avatar"
            src={attrs.avatar}
            alt=""
            loading="lazy"
            referrerPolicy="no-referrer"
            onError={() => setAvatarFailed(true)}
          />
        ) : (
          <div className="avatar-fallback">
            {(displayName || platform || '?').charAt(0).toUpperCase()}
          </div>
        )}

        <div className="card-title">
          {displayName && <div className="card-name">{displayName}</div>}
          <div className="card-platform">{platform}</div>
          {isURL ? (
            <a
              className="card-link"
              href={entity.primary_value}
              target="_blank"
              rel="noreferrer noopener"
            >
              {entity.primary_value}
            </a>
          ) : (
            <div className="card-link">{entity.primary_value}</div>
          )}
        </div>
      </div>

      {attrs.bio && <div className="card-bio">{attrs.bio}</div>}

      <div className="confidence">
        <div className="confidence-head">
          <span className={`pct c-${band.level}`}>{entity.confidence}%</span>
          <span className="lbl">{band.label} güven</span>
        </div>
        <div className="bar">
          <span
            className={`c-${band.level}`}
            style={{ width: `${Math.max(entity.confidence, 2)}%` }}
          />
        </div>
        {attrs.confidence_basis && (
          <div className="confidence-basis">{attrs.confidence_basis}</div>
        )}
      </div>

      {(marks.length > 0 || (entity.sources || []).length > 0) && (
        <div className="badges">
          {marks.map((m, i) => (
            <span key={i} className={`badge badge-${m.kind}`}>{m.text}</span>
          ))}
          {(entity.sources || []).map((s) => (
            <span key={s} className="badge badge-source">{s}</span>
          ))}
        </div>
      )}

      {details.length > 0 && (
        <dl className="attrs">
          {details.map((a) => (
            <div key={a.key} style={{ display: 'contents' }}>
              <dt>{a.label}</dt>
              <dd>{a.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </article>
  );
}
