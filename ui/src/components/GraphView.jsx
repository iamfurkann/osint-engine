import { useEffect, useRef } from 'react';
import CytoscapeComponent from 'react-cytoscapejs';

// Varlık tipine göre renk. Önceki sürüm stil sayfasında 'data(color)'
// kullanıyordu ama düğüm verisine hiçbir zaman 'color' yazmıyordu — bu yüzden
// bütün düğümler tanımsız renkle çiziliyordu.
const TYPE_COLORS = {
  username_presence: '#5b8cff',
  social_profile: '#3fb950',
  web_result: '#8b6dd6',
  email: '#d29922',
  domain: '#39a0b8',
  hostname: '#39a0b8',
  ip: '#db6d28',
  open_port: '#6e7681',
  username: '#5b8cff',
  vulnerability: '#f85149',
};

const DEFAULT_COLOR = '#6e7681';

export default function GraphView({ entities = [], correlations = [] }) {
  const cyRef = useRef(null);

  const nodes = entities.map((e) => {
    const attrs = e.attributes || {};
    const label =
      attrs.name || attrs.display_name || attrs.platform || e.primary_value || e.id;
    return {
      data: {
        id: e.id,
        label: label.length > 26 ? `${label.slice(0, 24)}…` : label,
        color: TYPE_COLORS[e.type] || DEFAULT_COLOR,
        // Güven puanı düğüm boyutuna yansır — zayıf kanıt görsel olarak da zayıf.
        size: 18 + Math.round((e.confidence || 0) / 5),
      },
    };
  });

  const nodeIds = new Set(nodes.map((n) => n.data.id));
  const edges = correlations
    .filter((c) => nodeIds.has(c.source_entity_id) && nodeIds.has(c.target_entity_id))
    .map((c, i) => ({
      data: {
        id: `e${i}`,
        source: c.source_entity_id,
        target: c.target_entity_id,
        label: c.rule || '',
        weight: Math.max(1, Math.round((c.confidence || 50) / 25)),
      },
    }));

  const elements = [...nodes, ...edges];

  useEffect(() => {
    const cy = cyRef.current;
    if (cy && elements.length > 0) {
      cy.layout({ name: 'cose', animate: false, padding: 40 }).run();
      cy.fit(undefined, 40);
    }
  }, [entities, correlations]); // eslint-disable-line react-hooks/exhaustive-deps

  if (elements.length === 0) {
    return (
      <div className="empty">
        <h3>Graf boş</h3>
        <p>Henüz çizilecek varlık yok.</p>
      </div>
    );
  }

  return (
    <div className="graph-wrap">
      <CytoscapeComponent
        elements={elements}
        style={{ width: '100%', height: '100%' }}
        cy={(cy) => { cyRef.current = cy; }}
        stylesheet={[
          {
            selector: 'node',
            style: {
              'background-color': 'data(color)',
              'border-color': 'data(color)',
              'border-width': 2,
              'border-opacity': 0.35,
              width: 'data(size)',
              height: 'data(size)',
              label: 'data(label)',
              color: '#e6e9ef',
              'font-size': 9,
              'font-family': 'Inter, sans-serif',
              'text-valign': 'bottom',
              'text-margin-y': 5,
              'text-outline-color': '#0b0d12',
              'text-outline-width': 2,
            },
          },
          {
            selector: 'edge',
            style: {
              width: 'data(weight)',
              'line-color': '#333b4d',
              'curve-style': 'bezier',
              'target-arrow-shape': 'none',
              opacity: 0.7,
            },
          },
        ]}
      />
    </div>
  );
}
