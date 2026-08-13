package report

import (
	"html/template"
	"time"
)

// ReportData, HTML şablona verilecek olan tüm veriyi içerir.
type ReportData struct {
	InvestigationID string
	Target          string
	Status          string
	Progress        float64
	StartTime       time.Time
	GeneratedAt     time.Time
	Entities        []EntityData
	Correlations    []CorrelationData
	GraphStats      struct {
		NodeCount int
		EdgeCount int
	}

	// Identity, tüm bulgulardan derlenen kimlik özetidir. GenerateHTML
	// tarafından doldurulur; şablon doğrudan bunu render eder.
	Identity []Attribute
}

// EntityData, raporda gösterilecek entity formatı.
type EntityData struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Value      string   `json:"primary_value"`
	Confidence int      `json:"confidence"`
	Sources    []string `json:"sources"`

	// Attributes, connector'ın topladığı gerçek istihbarat verisidir
	// (ad, konum, bio, takipçi, hesap yaşı). Önceden bu veri Finding.Context
	// içinde kalıp hiçbir zaman raporlanmıyordu.
	Attributes map[string]any `json:"attributes,omitempty"`
}

// Details, entity'nin niteliklerini gösterim sırasına dizilmiş hâlde döndürür.
// HTML şablonu bunu doğrudan çağırır.
func (e EntityData) Details() []Attribute {
	return OrderedAttributes(e.Attributes)
}

// CorrelationData, raporda gösterilecek korelasyon formatı.
type CorrelationData struct {
	SourceValue string `json:"source_value"`
	SourceType  string `json:"source_type"`
	TargetValue string `json:"target_value"`
	TargetType  string `json:"target_type"`
	Type        string `json:"type"`
	Confidence  int    `json:"confidence"`
	Evidence    string `json:"evidence"`
}

// defaultHTMLTemplate, tek dosya (CSS dahil) karanlık temalı rapor şablonudur.
var defaultHTMLTemplate = template.Must(template.New("report").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>OSINT Raporu: {{.Target}}</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-base: #050505;
            --surface-1: rgba(255, 255, 255, 0.03);
            --surface-2: rgba(255, 255, 255, 0.05);
            --border-glow: rgba(88, 166, 255, 0.2);
            --text-main: #e2e8f0;
            --text-muted: #94a3b8;
            --accent: #38bdf8;
            --accent-glow: rgba(56, 189, 248, 0.5);
            --success: #10b981;
            --warning: #f59e0b;
            --danger: #ef4444;
        }

        body {
            font-family: 'Inter', sans-serif;
            background-color: var(--bg-base);
            background-image: 
                radial-gradient(circle at 15% 50%, rgba(56, 189, 248, 0.05) 0%, transparent 50%),
                radial-gradient(circle at 85% 30%, rgba(139, 92, 246, 0.05) 0%, transparent 50%);
            color: var(--text-main);
            margin: 0;
            padding: 40px 20px;
            line-height: 1.6;
            min-height: 100vh;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
            animation: fadeIn 0.8s ease-out;
        }

        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(20px); }
            to { opacity: 1; transform: translateY(0); }
        }

        header {
            background: var(--surface-1);
            backdrop-filter: blur(12px);
            -webkit-backdrop-filter: blur(12px);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 16px;
            padding: 30px;
            margin-bottom: 40px;
            box-shadow: 0 4px 30px rgba(0, 0, 0, 0.1);
            position: relative;
            overflow: hidden;
        }
        
        header::before {
            content: '';
            position: absolute;
            top: 0; left: 0; right: 0; height: 2px;
            background: linear-gradient(90deg, transparent, var(--accent), transparent);
        }

        h1 {
            color: #fff;
            margin: 0 0 15px 0;
            font-size: 2.2em;
            letter-spacing: -0.5px;
            text-shadow: 0 0 20px var(--accent-glow);
        }

        .header-meta {
            display: flex;
            gap: 20px;
            color: var(--text-muted);
        }

        .header-meta span {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .header-meta strong {
            color: var(--text-main);
        }

        h2 {
            font-size: 1.5em;
            color: #fff;
            margin: 40px 0 20px 0;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        h2::before {
            content: '';
            display: block;
            width: 8px;
            height: 24px;
            background: var(--accent);
            border-radius: 4px;
            box-shadow: 0 0 10px var(--accent-glow);
        }

        .summary-cards {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
            gap: 20px;
            margin-bottom: 50px;
        }

        .card {
            background: var(--surface-1);
            backdrop-filter: blur(12px);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 16px;
            padding: 24px;
            transition: all 0.3s ease;
            position: relative;
            overflow: hidden;
        }
        
        .card:hover {
            transform: translateY(-5px);
            background: var(--surface-2);
            border-color: var(--border-glow);
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
        }

        .card-title {
            font-size: 0.85em;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-bottom: 10px;
            font-weight: 600;
        }

        .card-value {
            font-size: 2.5em;
            font-weight: 700;
            color: #fff;
            line-height: 1;
        }

        table {
            width: 100%;
            border-collapse: separate;
            border-spacing: 0 8px;
            margin-bottom: 40px;
        }

        th {
            text-align: left;
            padding: 0 20px 10px 20px;
            color: var(--text-muted);
            font-weight: 600;
            font-size: 0.9em;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        td {
            padding: 16px 20px;
            background: var(--surface-1);
            border: 1px solid rgba(255, 255, 255, 0.05);
            border-style: solid none solid none;
        }
        
        td:first-child {
            border-left: 1px solid rgba(255, 255, 255, 0.05);
            border-radius: 12px 0 0 12px;
        }
        
        td:last-child {
            border-right: 1px solid rgba(255, 255, 255, 0.05);
            border-radius: 0 12px 12px 0;
        }

        tr {
            transition: all 0.2s ease;
        }

        tbody tr:hover td {
            background: var(--surface-2);
            border-color: rgba(255, 255, 255, 0.1);
        }

        .badge {
            display: inline-flex;
            align-items: center;
            padding: 4px 10px;
            border-radius: 20px;
            font-size: 0.75em;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
            color: #fff;
        }

        a.url-link {
            color: var(--accent);
            text-decoration: none;
            position: relative;
            font-family: monospace;
            font-size: 1.05em;
            transition: color 0.2s;
        }
        
        a.url-link:hover {
            color: #fff;
            text-shadow: 0 0 8px var(--accent-glow);
        }

        .value-cell {
            font-family: monospace;
            font-size: 1.05em;
            color: #fff;
        }

        .confidence-high { color: var(--success); text-shadow: 0 0 10px rgba(16, 185, 129, 0.4); }
        .confidence-medium { color: var(--warning); text-shadow: 0 0 10px rgba(245, 158, 11, 0.4); }
        .confidence-low { color: var(--danger); text-shadow: 0 0 10px rgba(239, 68, 68, 0.4); }

        .footer {
            margin-top: 60px;
            padding-top: 30px;
            text-align: center;
            font-size: 0.9em;
            color: var(--text-muted);
            border-top: 1px solid rgba(255, 255, 255, 0.1);
        }

        .empty-state {
            background: var(--surface-1);
            border: 1px dashed rgba(255, 255, 255, 0.1);
            border-radius: 16px;
            padding: 40px;
            text-align: center;
            color: var(--text-muted);
            font-style: italic;
        }
        /* --- Kimlik özeti ve nitelik listeleri --- */
        .identity {
            background: rgba(255,255,255,0.03);
            border: 1px solid rgba(255,255,255,0.08);
            border-radius: 10px;
            padding: 18px 22px;
            margin-bottom: 28px;
        }
        .identity-row {
            display: flex;
            gap: 16px;
            padding: 7px 0;
            border-bottom: 1px solid rgba(255,255,255,0.05);
        }
        .identity-row:last-child { border-bottom: none; }
        .identity-label {
            flex: 0 0 150px;
            color: #8b93a7;
            font-size: 0.86rem;
            text-transform: uppercase;
            letter-spacing: 0.04em;
        }
        .identity-value {
            flex: 1;
            color: #7ee787;
            font-weight: 600;
            word-break: break-word;
        }
        .identity-source {
            flex: 0 0 auto;
            color: #6e7681;
            font-size: 0.8rem;
            font-style: italic;
            white-space: nowrap;
        }
        p.warn {
            background: rgba(210, 153, 34, 0.10);
            border-left: 3px solid #d29922;
            color: #d8b463;
            padding: 10px 16px;
            border-radius: 6px;
            margin: 0 0 16px;
            font-size: 0.9rem;
            line-height: 1.5;
        }
        dl.attrs {
            margin: 8px 0 0;
            display: grid;
            grid-template-columns: auto 1fr;
            gap: 2px 12px;
            font-size: 0.84rem;
        }
        dl.attrs dt {
            color: #8b93a7;
            white-space: nowrap;
        }
        dl.attrs dd {
            margin: 0;
            color: #c9d1d9;
            word-break: break-word;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>OSINT Araştırma Raporu</h1>
            <div class="header-meta">
                <span><strong>Hedef:</strong> {{.Target}}</span>
                <span><strong>ID:</strong> {{.InvestigationID}}</span>
            </div>
        </header>

        <div class="summary-cards">
            <div class="card">
                <div class="card-title">Durum</div>
                <div class="card-value">{{.Status}}</div>
            </div>
            <div class="card">
                <div class="card-title">İlerleme</div>
                <div class="card-value">%{{printf "%.1f" .Progress}}</div>
            </div>
            <div class="card">
                <div class="card-title">Bulunan Varlık</div>
                <div class="card-value">{{.GraphStats.NodeCount}}</div>
            </div>
            <div class="card">
                <div class="card-title">Kurulan İlişki</div>
                <div class="card-value">{{.GraphStats.EdgeCount}}</div>
            </div>
        </div>

        {{with .Identity}}
        <h2>👤 Kimlik İddiaları</h2>
        <p class="warn">⚠ Farklı platformlardaki aynı kullanıcı adı <strong>aynı kişi olmayabilir</strong>.
        Aşağıdaki her satır ilgili platformun iddiasıdır — doğrulanmış gerçek değildir.</p>
        <div class="identity">
            {{range .}}
            <div class="identity-row">
                <span class="identity-label">{{.Label}}</span>
                <span class="identity-value auto-link">{{.Value}}</span>
                <span class="identity-source">{{.Source}}</span>
            </div>
            {{end}}
        </div>
        {{end}}

        <h2>Bulunan Varlıklar (Entities)</h2>
        {{if .Entities}}
        <table>
            <thead>
                <tr>
                    <th>Tip</th>
                    <th>Değer / Bağlantı</th>
                    <th>Güven Puanı</th>
                    <th>Kaynaklar</th>
                </tr>
            </thead>
            <tbody>
                {{range .Entities}}
                <tr>
                    <td><span class="badge">{{.Type}}</span></td>
                    <td class="value-cell auto-link">
                        {{.Value}}
                        {{with .Details}}
                        <dl class="attrs">
                            {{range .}}
                            <dt>{{.Label}}</dt><dd>{{.Value}}</dd>
                            {{end}}
                        </dl>
                        {{end}}
                    </td>
                    <td>
                        <strong class="{{if ge .Confidence 80}}confidence-high{{else if ge .Confidence 50}}confidence-medium{{else}}confidence-low{{end}}">
                            {{.Confidence}}%
                        </strong>
                    </td>
                    <td>
                        {{range .Sources}}
                        <span class="badge">{{.}}</span>
                        {{end}}
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{else}}
        <div class="empty-state">Henüz herhangi bir varlık (entity) bulunamadı.</div>
        {{end}}

        <h2>İlişkiler ve Korelasyonlar</h2>
        {{if .Correlations}}
        <table>
            <thead>
                <tr>
                    <th>Kaynak</th>
                    <th>İlişki Tipi</th>
                    <th>Hedef</th>
                    <th>Güven</th>
                </tr>
            </thead>
            <tbody>
                {{range .Correlations}}
                <tr>
                    <td><span class="badge">{{.SourceType}}</span> <span class="auto-link">{{.SourceValue}}</span></td>
                    <td><span style="color:var(--text-muted); font-size:0.9em; text-transform:uppercase;">{{.Type}}</span></td>
                    <td><span class="badge">{{.TargetType}}</span> <span class="auto-link">{{.TargetValue}}</span></td>
                    <td>
                        <strong class="{{if ge .Confidence 80}}confidence-high{{else if ge .Confidence 50}}confidence-medium{{else}}confidence-low{{end}}">
                            {{.Confidence}}%
                        </strong>
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{else}}
        <div class="empty-state">Henüz varlıklar arası bir ilişki kurulamadı.</div>
        {{end}}

        <div class="footer">
            Oluşturulma Tarihi: {{.GeneratedAt.Format "02-01-2006 15:04:05"}} | OSINT-Engine İstihbarat Platformu
        </div>
    </div>

    <script>
        // Metin içindeki HTTP/HTTPS URL'lerini tıklanabilir bağlantılara dönüştür
        document.addEventListener("DOMContentLoaded", function() {
            const cells = document.querySelectorAll('.auto-link');
            const urlRegex = /(https?:\/\/[^\s]+)/g;
            cells.forEach(cell => {
                const text = cell.textContent;
                if (urlRegex.test(text)) {
                    cell.innerHTML = text.replace(urlRegex, function(url) {
                        return '<a href="' + url + '" target="_blank" class="url-link">' + url + '</a>';
                    });
                }
            });
        });
    </script>
</body>
</html>`))
