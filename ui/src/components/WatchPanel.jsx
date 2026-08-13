import { useState } from 'react';

// Daemon interval'ı NANOSANİYE bekliyor (CLI ile aynı sözleşme).
// Eski arayüz saniye gönderip nanosaniye varsayarak gösteriyordu; sonuç
// olarak eklenen her kayıt "0s" görünüyordu.
const NS_PER_MINUTE = 60 * 1e9;

function humanInterval(ns) {
  if (!ns || ns <= 0) return '—';
  const minutes = Math.round(ns / NS_PER_MINUTE);
  if (minutes < 60) return `${minutes} dk`;
  const hours = Math.round(minutes / 60);
  return hours < 24 ? `${hours} sa` : `${Math.round(hours / 24)} gün`;
}

export default function WatchPanel({ items, onAdd, onRemove }) {
  const [target, setTarget] = useState('');
  const [minutes, setMinutes] = useState(60);

  const submit = (e) => {
    e.preventDefault();
    if (!target.trim()) return;
    onAdd({
      target: target.trim(),
      type: 'username',
      interval: Math.max(1, Number(minutes)) * NS_PER_MINUTE,
    });
    setTarget('');
  };

  return (
    <section>
      <h2 className="section-title">
        İzleme Listesi <span className="count">{items.length}</span>
      </h2>

      <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <input
          type="text"
          value={target}
          onChange={(e) => setTarget(e.target.value)}
          placeholder="izlenecek hedef"
        />
        <div style={{ display: 'flex', gap: 8 }}>
          <input
            type="number"
            min="1"
            value={minutes}
            onChange={(e) => setMinutes(e.target.value)}
            style={{ width: 90 }}
          />
          <span style={{ alignSelf: 'center', color: 'var(--text-faint)', fontSize: 12 }}>
            dakikada bir
          </span>
          <button className="btn btn-ghost btn-sm" type="submit" style={{ marginLeft: 'auto' }}>
            Ekle
          </button>
        </div>
      </form>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 12 }}>
        {items.length === 0 && (
          <span style={{ color: 'var(--text-faint)', fontSize: 12 }}>
            Henüz izlenen hedef yok.
          </span>
        )}
        {items.map((w) => (
          <div className="watch-item" key={w.id}>
            <div>
              <div>{w.target}</div>
              <div className="meta">{w.type} · {humanInterval(w.interval)}</div>
            </div>
            <button
              className="btn btn-ghost btn-sm"
              onClick={() => onRemove(w.id)}
              aria-label={`${w.target} izlemesini kaldır`}
            >
              ✕
            </button>
          </div>
        ))}
      </div>
    </section>
  );
}
