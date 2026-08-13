import { useState } from 'react';

const TYPES = [
  ['username', 'Kullanıcı adı'],
  ['domain', 'Alan adı'],
  ['email', 'E-posta'],
  ['ip', 'IP adresi'],
  ['person', 'Kişi adı'],
];

export default function SearchPanel({ onStart, busy }) {
  const [target, setTarget] = useState('');
  const [type, setType] = useState('username');
  const [recursive, setRecursive] = useState(false);
  const [showHints, setShowHints] = useState(false);
  const [hints, setHints] = useState({
    email: '', phone: '', location: '', birth_year: '', known_usernames: '',
  });

  const submit = (e) => {
    e.preventDefault();
    if (!target.trim() || busy) return;

    onStart({
      target: target.trim(),
      type,
      recursive,
      hints: {
        email: hints.email.trim(),
        phone: hints.phone.trim(),
        location: hints.location.trim(),
        birth_year: hints.birth_year ? Number(hints.birth_year) : 0,
        known_usernames: hints.known_usernames
          .split(',').map((s) => s.trim()).filter(Boolean),
      },
    });
  };

  const setHint = (k) => (e) => setHints({ ...hints, [k]: e.target.value });

  return (
    <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <h2 className="section-title">Yeni Araştırma</h2>

      <div className="field">
        <label htmlFor="target">Hedef</label>
        <input
          id="target"
          type="text"
          value={target}
          onChange={(e) => setTarget(e.target.value)}
          placeholder="kullanıcıadı, example.com, 8.8.8.8"
          autoComplete="off"
        />
      </div>

      <div className="field">
        <label htmlFor="type">Tip</label>
        <select id="type" value={type} onChange={(e) => setType(e.target.value)}>
          {TYPES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
        </select>
      </div>

      <label className="checkbox">
        <input
          type="checkbox"
          checked={recursive}
          onChange={(e) => setRecursive(e.target.checked)}
        />
        <span>
          Özyinelemeli tarama
          <br />
          <span style={{ color: 'var(--text-faint)', fontSize: 11 }}>
            Biyografilerde bulunan e-posta, site ve @kullanıcı adları yeni tarama başlatır.
          </span>
        </span>
      </label>

      <button
        type="button"
        className="btn btn-ghost btn-sm"
        onClick={() => setShowHints(!showHints)}
      >
        {showHints ? '− İpuçlarını gizle' : '+ İpucu ekle (isteğe bağlı)'}
      </button>

      {showHints && (
        <>
          <div className="field">
            <label>Bilinen kullanıcı adları</label>
            <input type="text" value={hints.known_usernames}
              onChange={setHint('known_usernames')} placeholder="virgülle ayırın" />
          </div>
          <div className="field">
            <label>E-posta</label>
            <input type="text" value={hints.email} onChange={setHint('email')} />
          </div>
          <div className="field">
            <label>Konum</label>
            <input type="text" value={hints.location} onChange={setHint('location')} />
          </div>
          <div className="field">
            <label>Doğum yılı</label>
            <input type="number" value={hints.birth_year} onChange={setHint('birth_year')} />
          </div>
        </>
      )}

      <button className="btn" type="submit" disabled={busy || !target.trim()}>
        {busy ? 'Taranıyor…' : 'Araştırmayı başlat'}
      </button>
    </form>
  );
}
