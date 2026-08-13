// API istemcisi.
//
// Daemon artık 127.0.0.1'e bağlı (tüm arayüzler değil) ve CORS kapalı bir
// izin listesi kullanıyor; bu origin (Vite dev sunucusu) listede.
const API_BASE = 'http://127.0.0.1:8080/api';

async function request(path, options = {}) {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });

  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(text.trim() || `HTTP ${res.status}`);
  }

  if (res.status === 204) return null;
  const text = await res.text();
  return text ? JSON.parse(text) : null;
}

export const api = {
  startInvestigation: (body) =>
    request('/investigations', { method: 'POST', body: JSON.stringify(body) }),

  getGraph: (id) => request(`/investigations/${id}/graph`),

  getReport: (id) => request(`/investigations/${id}/report`),

  listWatch: () => request('/watch'),

  addWatch: (body) =>
    request('/watch', { method: 'POST', body: JSON.stringify(body) }),

  removeWatch: (id) =>
    request('/watch', { method: 'DELETE', body: JSON.stringify({ id }) }),
};
