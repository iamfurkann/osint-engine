let cy;

document.addEventListener('DOMContentLoaded', () => {
    // URL parametresinden ID almayı dene
    const urlParams = new URLSearchParams(window.location.search);
    const idParam = urlParams.get('id');
    if (idParam) {
        document.getElementById('invId').value = idParam;
        loadGraph(idParam);
    }

    document.getElementById('loadBtn').addEventListener('click', () => {
        const id = document.getElementById('invId').value.trim();
        if (id) {
            loadGraph(id);
        }
    });
});

function loadGraph(invId) {
    document.getElementById('loading').style.display = 'block';

    fetch(`/api/graph?id=${invId}`)
        .then(response => {
            if (!response.ok) {
                return response.text().then(text => { throw new Error(text) });
            }
            return response.json();
        })
        .then(data => {
            document.getElementById('loading').style.display = 'none';
            renderGraph(data);
        })
        .catch(error => {
            document.getElementById('loading').style.display = 'none';
            alert(`Hata: ${error.message}`);
        });
}

function renderGraph(elements) {
    if (cy) {
        cy.destroy();
    }

    cy = cytoscape({
        container: document.getElementById('cy'),
        elements: elements,
        style: [
            {
                selector: 'node',
                style: {
                    'background-color': '#58a6ff',
                    'label': 'data(label)',
                    'color': '#c9d1d9',
                    'font-size': '12px',
                    'text-valign': 'bottom',
                    'text-margin-y': '5px',
                    'width': '30px',
                    'height': '30px'
                }
            },
            {
                // Düğüm tiplerine göre renkler
                selector: 'node[type="domain"]',
                style: { 'background-color': '#238636' }
            },
            {
                selector: 'node[type="email"]',
                style: { 'background-color': '#da3633' }
            },
            {
                selector: 'node[type="ip"]',
                style: { 'background-color': '#d29922' }
            },
            {
                selector: 'edge',
                style: {
                    'width': 2,
                    'line-color': '#30363d',
                    'target-arrow-color': '#30363d',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier',
                    'label': 'data(label)',
                    'font-size': '10px',
                    'color': '#8b949e',
                    'text-rotation': 'autorotate',
                    'text-background-color': '#0d1117',
                    'text-background-opacity': 1,
                    'text-background-padding': '2px'
                }
            }
        ],
        layout: {
            name: 'cose',
            padding: 50,
            animate: true
        }
    });

    // Tıklama olayları
    cy.on('tap', 'node', function(evt){
        const node = evt.target;
        showNodeInfo(node.data());
    });

    cy.on('tap', 'edge', function(evt){
        const edge = evt.target;
        showEdgeInfo(edge.data());
    });
}

function showNodeInfo(data) {
    const panel = document.getElementById('infoPanel');
    const content = document.getElementById('infoContent');
    
    let html = `
        <div class="info-row"><span class="info-label">ID:</span> ${data.id}</div>
        <div class="info-row"><span class="info-label">Tip:</span> <span class="badge">${data.type}</span></div>
        <div class="info-row"><span class="info-label">Değer:</span> ${data.value}</div>
        <div class="info-row"><span class="info-label">Güven Puanı:</span> ${data.confidence || 0}%</div>
    `;

    if (data.sources) {
        html += `<div class="info-row"><span class="info-label">Kaynaklar:</span><br>`;
        data.sources.split(',').forEach(src => {
            if(src.trim()) html += `<span class="badge" style="margin:2px;">${src}</span>`;
        });
        html += `</div>`;
    }

    content.innerHTML = html;
    panel.style.display = 'block';
}

function showEdgeInfo(data) {
    const panel = document.getElementById('infoPanel');
    const content = document.getElementById('infoContent');
    
    let html = `
        <div class="info-row"><span class="info-label">Kaynak:</span> ${data.source}</div>
        <div class="info-row"><span class="info-label">Hedef:</span> ${data.target}</div>
        <div class="info-row"><span class="info-label">Tip:</span> <span class="badge">${data.type}</span></div>
        <div class="info-row"><span class="info-label">Güven Puanı:</span> ${data.confidence || 0}%</div>
        <div class="info-row"><span class="info-label">Kanıt:</span> ${data.evidence || '-'}</div>
    `;

    content.innerHTML = html;
    panel.style.display = 'block';
}
