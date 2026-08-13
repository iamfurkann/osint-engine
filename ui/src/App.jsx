import { useState, useEffect, useCallback, useMemo } from 'react';
import ReactMarkdown from 'react-markdown';

import { api } from './lib/api';
import { sortByInformation } from './lib/attributes';
import SearchPanel from './components/SearchPanel';
import WatchPanel from './components/WatchPanel';
import IdentityPanel from './components/IdentityPanel';
import EntityCard from './components/EntityCard';
import GraphView from './components/GraphView';

const POLL_MS = 2000;

export default function App() {
  const [activeInv, setActiveInv] = useState(null);
  const [data, setData] = useState(null);
  const [watchlist, setWatchlist] = useState([]);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const [tab, setTab] = useState('findings');
  const [minConfidence, setMinConfidence] = useState(0);
  const [query, setQuery] = useState('');
  const [hideSuspect, setHideSuspect] = useState(false);

  const [aiReport, setAiReport] = useState(null);
  const [aiLoading, setAiLoading] = useState(false);

  const refreshWatchlist = useCallback(async () => {
    try {
      setWatchlist((await api.listWatch()) || []);
    } catch {
      /* izleme listesi kritik değil, sessiz geç */
    }
  }, []);

  useEffect(() => { refreshWatchlist(); }, [refreshWatchlist]);

  // Araştırma tamamlanana kadar yokla.
  useEffect(() => {
    if (!activeInv) return undefined;

    let cancelled = false;
    const tick = async () => {
      try {
        const res = await api.getGraph(activeInv);
        if (cancelled) return;
        setData(res);
        if ((res?.Progress ?? 0) >= 100) {
          clearInterval(timer);
          setBusy(false);
        }
      } catch (e) {
        if (!cancelled) setError(e.message);
      }
    };

    tick();
    const timer = setInterval(tick, POLL_MS);
    return () => { cancelled = true; clearInterval(timer); };
  }, [activeInv]);

  const startInvestigation = async (body) => {
    setError(null);
    setAiReport(null);
    setData(null);
    setBusy(true);
    try {
      const res = await api.startInvestigation(body);
      setActiveInv(res.id);
      setTab('findings');
    } catch (e) {
      setError(e.message);
      setBusy(false);
    }
  };

  const runAiReport = async () => {
    if (!activeInv) return;
    setAiLoading(true);
    setError(null);
    try {
      const res = await api.getReport(activeInv);
      setAiReport(res.report);
      setTab('ai');
    } catch (e) {
      setError(e.message);
    } finally {
      setAiLoading(false);
    }
  };

  const entities = useMemo(
    () => sortByInformation(data?.Entities || []),
    [data],
  );

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return entities.filter((e) => {
      if (e.confidence < minConfidence) return false;
      if (hideSuspect && (e.attributes || {}).verification) return false;
      if (!q) return true;
      const hay = [
        e.primary_value,
        e.type,
        ...Object.values(e.attributes || {}).map(String),
      ].join(' ').toLowerCase();
      return hay.includes(q);
    });
  }, [entities, minConfidence, query, hideSuspect]);

  const progress = data?.Progress ?? 0;
  const suspectCount = entities.filter((e) => (e.attributes || {}).verification).length;
  const strongCount = entities.filter((e) => e.confidence >= 60).length;

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <h1>OSINT Engine<span className="dot">.</span></h1>
          <small>v0.2</small>
        </div>

        <SearchPanel onStart={startInvestigation} busy={busy} />

        <WatchPanel
          items={watchlist}
          onAdd={async (b) => { await api.addWatch(b); refreshWatchlist(); }}
          onRemove={async (id) => { await api.removeWatch(id); refreshWatchlist(); }}
        />
      </aside>

      <main className="main">
        {error && (
          <div className="notice notice-error">
            <strong>Hata:</strong> {error}
          </div>
        )}

        {!activeInv && !error && (
          <div className="empty">
            <h3>Araştırma başlatın</h3>
            <p>Soldaki panelden bir hedef girin. Çekirdek connector'ların hiçbiri API anahtarı gerektirmez.</p>
          </div>
        )}

        {activeInv && (
          <>
            <div className="stats">
              <div className="stat">
                <div className="k">Bulunan varlık</div>
                <div className="v">{entities.length}</div>
              </div>
              <div className="stat">
                <div className="k">Güçlü kanıt (≥60%)</div>
                <div className="v c-high">{strongCount}</div>
              </div>
              <div className="stat">
                <div className="k">Şüpheli</div>
                <div className="v c-low">{suspectCount}</div>
              </div>
              <div className="stat">
                <div className="k">İlişki</div>
                <div className="v">{data?.GraphStats?.EdgeCount ?? 0}</div>
              </div>
              <div className="stat">
                <div className="k">İlerleme</div>
                <div className="v">{Math.round(progress)}%</div>
                <div className="progress"><span style={{ width: `${progress}%` }} /></div>
              </div>
            </div>

            <div className="tabs">
              <button
                className={`tab ${tab === 'findings' ? 'active' : ''}`}
                onClick={() => setTab('findings')}
              >
                Bulgular
              </button>
              <button
                className={`tab ${tab === 'graph' ? 'active' : ''}`}
                onClick={() => setTab('graph')}
              >
                Graf
              </button>
              <button
                className={`tab ${tab === 'ai' ? 'active' : ''}`}
                onClick={() => setTab('ai')}
              >
                AI Analizi
              </button>
            </div>

            {tab === 'findings' && (
              <>
                <IdentityPanel entities={entities} />

                <h2 className="section-title">
                  Bulgular <span className="count">{visible.length}/{entities.length}</span>
                </h2>

                <div className="toolbar">
                  <input
                    type="text"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder="Filtrele…"
                    style={{ width: 200 }}
                  />
                  <select
                    value={minConfidence}
                    onChange={(e) => setMinConfidence(Number(e.target.value))}
                  >
                    <option value={0}>Tüm güven seviyeleri</option>
                    <option value={35}>≥ 35% (düşük ve üstü)</option>
                    <option value={60}>≥ 60% (orta ve üstü)</option>
                    <option value={80}>≥ 80% (yüksek)</option>
                  </select>
                  <label className="checkbox">
                    <input
                      type="checkbox"
                      checked={hideSuspect}
                      onChange={(e) => setHideSuspect(e.target.checked)}
                    />
                    <span>Şüphelileri gizle</span>
                  </label>
                </div>

                {visible.length === 0 ? (
                  <div className="empty">
                    <h3>Eşleşen bulgu yok</h3>
                    <p>Filtreleri gevşetmeyi deneyin.</p>
                  </div>
                ) : (
                  <div className="entity-grid">
                    {visible.map((e) => <EntityCard key={e.id} entity={e} />)}
                  </div>
                )}
              </>
            )}

            {tab === 'graph' && (
              <>
                <h2 className="section-title">İlişki Grafı</h2>
                {(data?.GraphStats?.EdgeCount ?? 0) === 0 && (
                  <div className="notice">
                    Hiç ilişki bulunamadı — ve bu <strong>doğru bir sonuç</strong> olabilir.
                    Kenar yalnızca <strong>farklı bağımsızlık sınıflarından</strong> gelen
                    kaynaklar aynı iki varlığı birlikte gördüğünde kurulur.
                    "Aynı connector buldu" bir ilişki değil, ortak kökendir.
                  </div>
                )}
                <GraphView
                  entities={entities}
                  correlations={data?.Correlations || []}
                />
              </>
            )}

            {tab === 'ai' && (
              <>
                <h2 className="section-title">AI Analizi</h2>
                <div className="notice">
                  <strong>⚠ Bu özellik bulgularınızı Google'a gönderir.</strong>{' '}
                  Yerel bir modele geçilene kadar hassas araştırmalarda kullanmayın.
                  API anahtarı gerekir: <code>osint keys set gemini &lt;key&gt;</code>
                </div>

                {!aiReport && (
                  <button className="btn" onClick={runAiReport} disabled={aiLoading}>
                    {aiLoading ? 'Analiz ediliyor…' : 'Analiz oluştur'}
                  </button>
                )}

                {aiReport && (
                  <div className="markdown">
                    <ReactMarkdown>{aiReport}</ReactMarkdown>
                  </div>
                )}
              </>
            )}
          </>
        )}
      </main>
    </div>
  );
}
