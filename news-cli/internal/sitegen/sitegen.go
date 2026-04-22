package sitegen

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"news-cli/internal/database"
	"news-cli/internal/models"
	"os"
	"path/filepath"
	"time"
)

type articleView struct {
	Title       string
	Link        string
	Description string
	SourceName  string
	Score       int
	When        string
}

type archiveDayView struct {
	Date  string
	Count int
	Href  string
}

type navView struct {
	Label string
	Href  string
}

type pageData struct {
	Title          string
	StyleHref      string
	HomeHref       string
	ArchiveHref    string
	GraphHref      string
	FeedHref       string
	Updated        string
	Trending       []models.Entity
	Days           []archiveDayView
	Date           string
	Articles       []articleView
	Prev           *navView
	Next           *navView
	GraphInline    template.JS
	GraphArticles  template.JS
	GraphPageTitle string
}

type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	PubDate     string    `xml:"pubDate"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

type graphPayload struct {
	Nodes []models.EntityNode `json:"nodes"`
	Edges []models.EntityEdge `json:"edges"`
}

type graphArticle struct {
	Title string `json:"title"`
	Link  string `json:"link"`
	When  string `json:"when"`
}

const styleCSS = `:root {
  --bg-color: #09111a;
  --bg-panel: rgba(15, 23, 42, 0.92);
  --bg-card: rgba(17, 24, 39, 0.92);
  --bg-hover: #172233;
  --text-primary: #eef2ff;
  --text-secondary: #94a3b8;
  --accent: #f59e0b;
  --accent-cool: #22d3ee;
  --border: #223247;
  --font-sans: 'Inter', system-ui, -apple-system, sans-serif;
  --font-mono: 'IBM Plex Mono', 'Menlo', monospace;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: var(--font-sans);
  font-feature-settings: 'cv02', 'cv03', 'cv04', 'cv11';
  background:
    radial-gradient(circle at top left, rgba(34, 211, 238, 0.12), transparent 28%),
    radial-gradient(circle at top right, rgba(245, 158, 11, 0.10), transparent 24%),
    var(--bg-color);
  color: var(--text-primary);
  line-height: 1.6;
  min-height: 100vh;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
a { color: inherit; text-decoration: none; }
.wrap { max-width: 1120px; margin: 0 auto; padding: 2.2rem 1.4rem 2.8rem; }
.topbar {
  border: 1px solid var(--border);
  border-radius: 20px;
  background: linear-gradient(180deg, rgba(9, 17, 26, 0.94), rgba(15, 23, 42, 0.9));
  backdrop-filter: blur(14px);
  padding: 1.35rem 1.35rem 1.15rem;
}
.eyebrow {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--accent-cool);
  letter-spacing: 0.14em;
  text-transform: uppercase;
}
.title {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 1rem;
  margin-top: 0.35rem;
  flex-wrap: wrap;
}
.title h1 { font-size: 1.75rem; font-weight: 700; line-height: 1.1; }
.title h1 {
  font-family: var(--font-mono);
  color: var(--accent-cool);
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
.subtitle { color: var(--text-secondary); font-size: 0.9rem; font-weight: 400; margin-top: 0.45rem; letter-spacing: 0.01em; }
.updated {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0.25rem 0.7rem;
  background: rgba(17, 24, 39, 0.75);
}
.links { display: flex; gap: 0.8rem; flex-wrap: wrap; margin-top: 0.9rem; }
.links a {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--accent);
  border: 1px solid rgba(245, 158, 11, 0.35);
  border-radius: 999px;
  padding: 0.22rem 0.7rem;
  background: rgba(245, 158, 11, 0.10);
}
.links a:hover { background: rgba(245, 158, 11, 0.16); }
.section { margin-top: 1.4rem; }
.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.85rem;
}
.section-head h2 { font-size: 0.7rem; font-weight: 600; color: var(--text-secondary); letter-spacing: 0.1em; text-transform: uppercase; font-family: var(--font-mono); }
.pills { display: flex; gap: 0.55rem; overflow-x: auto; padding-bottom: 0.2rem; }
.pill {
  flex: 0 0 auto;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0.32rem 0.75rem;
  background: rgba(17, 24, 39, 0.85);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  color: var(--text-secondary);
  white-space: nowrap;
}
.pill b { color: var(--text-primary); font-weight: 500; }
.pill i { color: var(--accent-cool); font-style: normal; margin-left: 0.35rem; }
.grid { display: grid; grid-template-columns: repeat(12, 1fr); gap: 0.9rem; }
.card {
  grid-column: span 12;
  display: block;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--bg-card);
  padding: 1.05rem 1.05rem 0.95rem;
  transition: transform 0.15s, background 0.15s, border-color 0.15s;
}
.card:hover {
  background: var(--bg-hover);
  border-color: rgba(245, 158, 11, 0.55);
  transform: translateY(-1px);
}
.meta {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  color: var(--text-secondary);
  letter-spacing: 0.02em;
}
.source { color: var(--accent-cool); font-weight: 500; text-transform: uppercase; letter-spacing: 0.06em; }
.score {
  display: inline-block;
  margin-left: 0.5rem;
  padding: 0.1rem 0.42rem;
  border-radius: 999px;
  background: rgba(245, 158, 11, 0.14);
  color: #fcd34d;
}
.card h3 {
  font-size: 1rem;
  font-weight: 500;
  line-height: 1.4;
  margin-top: 0.55rem;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}
.card:hover h3 { color: var(--accent); }
.desc {
  margin-top: 0.5rem;
  color: var(--text-secondary);
  font-size: 0.875rem;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.footer {
  margin-top: 2rem;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 0.6rem;
  border-top: 1px solid var(--border);
  padding-top: 1rem;
}
.footer a { color: var(--accent-cool); }
.footer a:hover { text-decoration: underline; }
.days { display: grid; grid-template-columns: repeat(12, 1fr); gap: 0.8rem; }
.day {
  grid-column: span 12;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.78);
  padding: 0.85rem 1rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
}
.day:hover { background: rgba(23, 34, 51, 0.9); border-color: rgba(34, 211, 238, 0.4); }
.day span { font-family: var(--font-mono); color: var(--text-secondary); font-size: 0.75rem; }
.navrow { display: flex; justify-content: space-between; gap: 0.8rem; margin: 1rem 0; flex-wrap: wrap; }
.navrow a {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--accent);
  border: 1px solid rgba(245, 158, 11, 0.35);
  border-radius: 999px;
  padding: 0.22rem 0.7rem;
  background: rgba(245, 158, 11, 0.10);
}
.navrow a:hover { background: rgba(245, 158, 11, 0.16); }
@media (min-width: 900px) {
  .card { grid-column: span 6; }
  .day { grid-column: span 6; }
}
`

const baseHead = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}}</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=IBM+Plex+Mono:wght@400;500&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="{{.StyleHref}}">
</head>
<body>
  <div class="wrap">
    <div class="topbar">
      <div class="eyebrow">Recon</div>
      <div class="title">
        <h1>RECON</h1>
        {{if .Updated}}<div class="updated">{{.Updated}}</div>{{end}}
      </div>
      <div class="subtitle">High-Signal Security Intelligence</div>
      <div class="links">
        <a href="{{.HomeHref}}">Home</a>
        <a href="{{.ArchiveHref}}">Archive</a>
        <a href="{{.GraphHref}}">Graph</a>
        <a href="{{.FeedHref}}">RSS</a>
      </div>
    </div>
`

const baseFoot = `
    <div class="footer">
      <div>Recon static site</div>
      <div>
        <a href="{{.ArchiveHref}}">/archive</a>
        <a href="{{.GraphHref}}">/graph</a>
        <a href="{{.FeedHref}}">/feed.xml</a>
      </div>
    </div>
  </div>
</body>
</html>
`

const indexBody = `
    <div class="section">
      <div class="section-head">
        <h2>Trending Entities</h2>
      </div>
      <div class="pills">
        {{if .Trending}}
          {{range .Trending}}
            <div class="pill"><b>{{.Name}}</b><i>{{.Mentions}}</i></div>
          {{end}}
        {{else}}
          <div class="pill"><b>No signals</b><i>0</i></div>
        {{end}}
      </div>
    </div>

    <div class="section">
      <div class="section-head">
        <h2>Today’s Intel</h2>
      </div>
      <div class="grid">
        {{if .Articles}}
          {{range .Articles}}
            <a class="card" href="{{.Link}}" target="_blank" rel="noreferrer">
              <div class="meta"><span class="source">{{.SourceName}}</span><span class="score">{{.Score}}</span> · {{.When}}</div>
              <h3>{{.Title}}</h3>
              <div class="desc">{{.Description}}</div>
            </a>
          {{end}}
        {{else}}
          <div class="card">
            <div class="meta"><span class="source">Recon</span><span class="score">0</span> · idle</div>
            <h3>No recent articles</h3>
            <div class="desc">Run the daemon or sync again to populate the nexus.</div>
          </div>
        {{end}}
      </div>
    </div>
`

const archiveIndexBody = `
    <div class="section">
      <div class="section-head">
        <h2>Archive</h2>
      </div>
      <div class="days">
        {{if .Days}}
          {{range .Days}}
            <a class="day" href="{{.Href}}">
              <div style="font-weight:700">{{.Date}}</div>
              <span>{{.Count}} articles</span>
            </a>
          {{end}}
        {{else}}
          <div class="day">
            <div style="font-weight:700">No archive</div>
            <span>0 articles</span>
          </div>
        {{end}}
      </div>
    </div>
`

const archiveDayBody = `
    <div class="section">
      <div class="section-head">
        <h2>{{.Date}}</h2>
      </div>
      <div class="navrow">
        {{if .Prev}}<a href="{{.Prev.Href}}">← {{.Prev.Label}}</a>{{else}}<span></span>{{end}}
        {{if .Next}}<a href="{{.Next.Href}}">{{.Next.Label}} →</a>{{else}}<span></span>{{end}}
      </div>
      <div class="grid">
        {{if .Articles}}
          {{range .Articles}}
            <a class="card" href="{{.Link}}" target="_blank" rel="noreferrer">
              <div class="meta"><span class="source">{{.SourceName}}</span><span class="score">{{.Score}}</span> · {{.When}}</div>
              <h3>{{.Title}}</h3>
              <div class="desc">{{.Description}}</div>
            </a>
          {{end}}
        {{else}}
          <div class="card">
            <div class="meta"><span class="source">Recon</span><span class="score">0</span> · idle</div>
            <h3>No articles for this day</h3>
            <div class="desc">This date exists but has no stored items.</div>
          </div>
        {{end}}
      </div>
      <div class="navrow">
        {{if .Prev}}<a href="{{.Prev.Href}}">← {{.Prev.Label}}</a>{{else}}<span></span>{{end}}
        {{if .Next}}<a href="{{.Next.Href}}">{{.Next.Label}} →</a>{{else}}<span></span>{{end}}
      </div>
    </div>
`

const graphBody = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}}</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=IBM+Plex+Mono:wght@400;500&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="{{.StyleHref}}">
  <style>
    body { overflow: hidden; }
    .wrap { max-width: none; padding: 1.35rem; height: 100vh; display: flex; flex-direction: column; gap: 1rem; }
    .topbar { flex: 0 0 auto; }
    .graphShell { flex: 1 1 auto; display: grid; grid-template-columns: 1fr 360px; gap: 0.9rem; min-height: 0; }
    .graphPanel {
      border: 1px solid var(--border);
      border-radius: 18px;
      background: rgba(15, 23, 42, 0.82);
      overflow: hidden;
      position: relative;
      min-height: 0;
    }
    .sidePanel {
      border: 1px solid var(--border);
      border-radius: 18px;
      background: rgba(17, 24, 39, 0.86);
      overflow: hidden;
      display: flex;
      flex-direction: column;
      min-height: 0;
    }
    .sideHead {
      padding: 0.9rem 1rem 0.75rem;
      border-bottom: 1px solid var(--border);
      background: linear-gradient(180deg, rgba(9, 17, 26, 0.92), rgba(15, 23, 42, 0.9));
    }
    .searchRow { display: flex; gap: 0.6rem; margin-top: 0.65rem; }
    .searchRow input {
      flex: 1;
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 0.55rem 0.65rem;
      background: rgba(9, 17, 26, 0.65);
      color: var(--text-primary);
      font-family: 'IBM Plex Mono', monospace;
      font-size: 0.85rem;
      outline: none;
    }
    .sideBody { padding: 0.9rem 1rem 1rem; overflow: auto; }
    .sideBody a { display: block; padding: 0.65rem 0; border-bottom: 1px solid rgba(34,50,71,0.6); }
    .sideBody a:last-child { border-bottom: none; }
    .sideBody .t { font-weight: 700; line-height: 1.25; }
    .sideBody .m { color: var(--text-secondary); font-family: 'IBM Plex Mono', monospace; font-size: 0.78rem; margin-top: 0.25rem; }
    .tooltip {
      position: absolute;
      pointer-events: none;
      background: rgba(9, 17, 26, 0.92);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 0.5rem 0.6rem;
      color: var(--text-primary);
      font-family: 'IBM Plex Mono', monospace;
      font-size: 0.78rem;
      opacity: 0;
      transition: opacity 0.08s;
    }
    @media (max-width: 980px) {
      body { overflow: auto; }
      .wrap { height: auto; }
      .graphShell { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="topbar">
      <div class="eyebrow">Recon</div>
      <div class="title">
        <h1>RECON</h1>
        {{if .Updated}}<div class="updated">{{.Updated}}</div>{{end}}
      </div>
      <div class="subtitle">Entity Relationship Graph</div>
      <div class="links">
        <a href="{{.HomeHref}}">Home</a>
        <a href="{{.ArchiveHref}}">Archive</a>
        <a href="{{.FeedHref}}">RSS</a>
      </div>
    </div>

    <div class="graphShell">
      <div class="graphPanel" id="graphPanel">
        <div class="tooltip" id="tooltip"></div>
      </div>
      <div class="sidePanel">
        <div class="sideHead">
          <div class="eyebrow" id="sideEyebrow">Select a node</div>
          <div class="subtitle" id="sideSub">Click an entity to view recent related articles</div>
          <div class="searchRow">
            <input id="search" placeholder="Search entity (e.g. CVE-2026-1234)" />
          </div>
        </div>
        <div class="sideBody" id="sideBody"></div>
      </div>
    </div>
  </div>

  <script src="https://cdn.jsdelivr.net/npm/d3@7"></script>
  <script>
    const graphData = {{.GraphInline}};
    const entityArticles = {{.GraphArticles}};

    const panel = document.getElementById('graphPanel');
    const tooltip = document.getElementById('tooltip');
    const sideEyebrow = document.getElementById('sideEyebrow');
    const sideSub = document.getElementById('sideSub');
    const sideBody = document.getElementById('sideBody');
    const search = document.getElementById('search');

    const w = panel.clientWidth;
    const h = panel.clientHeight;

    const svg = d3.select(panel).append('svg').attr('width', w).attr('height', h);
    const g = svg.append('g');

    const zoom = d3.zoom().scaleExtent([0.2, 6]).on('zoom', (event) => g.attr('transform', event.transform));
    svg.call(zoom);

    const byId = new Map(graphData.nodes.map(n => [n.id, n]));
    const neighbors = new Map();
    graphData.nodes.forEach(n => neighbors.set(n.id, new Set()));
    graphData.edges.forEach(e => {
      if (!neighbors.has(e.source) || !neighbors.has(e.target)) return;
      neighbors.get(e.source).add(e.target);
      neighbors.get(e.target).add(e.source);
    });

    const colorFor = (d) => {
      const t = (d.type || '').toLowerCase();
      if (t === 'cve') return '#f59e0b';
      if (t === 'apt') return '#ef4444';
      if (t === 'malware') return '#a855f7';
      return '#22d3ee';
    };

    const radiusFor = (d) => Math.max(4, Math.min(26, 4 + Math.sqrt(Math.max(1, d.mentions || 1)) * 1.7));

    const link = g.append('g').attr('stroke', 'rgba(148,163,184,0.25)').selectAll('line')
      .data(graphData.edges)
      .join('line')
      .attr('stroke-width', d => Math.max(1, Math.min(6, (d.weight || 1) / 2)));

    const node = g.append('g').selectAll('circle')
      .data(graphData.nodes)
      .join('circle')
      .attr('r', radiusFor)
      .attr('fill', colorFor)
      .attr('stroke', 'rgba(9,17,26,0.9)')
      .attr('stroke-width', 1.5)
      .call(d3.drag()
        .on('start', (event, d) => {
          if (!event.active) sim.alphaTarget(0.3).restart();
          d.fx = d.x; d.fy = d.y;
        })
        .on('drag', (event, d) => { d.fx = event.x; d.fy = event.y; })
        .on('end', (event, d) => {
          if (!event.active) sim.alphaTarget(0);
          d.fx = null; d.fy = null;
        })
      );

    node.on('mousemove', (event, d) => {
      tooltip.style.opacity = '1';
      tooltip.style.left = (event.offsetX + 14) + 'px';
      tooltip.style.top = (event.offsetY + 12) + 'px';
      tooltip.innerHTML = '<div style="color:' + colorFor(d) + '; font-weight:700; text-transform:uppercase">' + (d.type || 'entity') + '</div>' +
        '<div style="font-weight:700">' + d.id + '</div>' +
        '<div style="color:rgba(148,163,184,0.95)">mentions ' + (d.mentions || 0) + '</div>';
    });
    node.on('mouseleave', () => { tooltip.style.opacity = '0'; });

    const setSidebar = (id) => {
      sideEyebrow.textContent = 'Entity';
      sideSub.textContent = id;
      const list = entityArticles[id] || [];
      if (!list.length) {
        sideBody.innerHTML = '<div class="meta" style="padding:0.4rem 0">No recent linked articles.</div>';
        return;
      }
      sideBody.innerHTML = list.map(a => {
        const t = (a.title || '').replaceAll('<','&lt;').replaceAll('>','&gt;');
        return '<a href="' + a.link + '" target="_blank" rel="noreferrer"><div class="t">' + t + '</div><div class="m">' + a.when + '</div></a>';
      }).join('');
    };

    const highlight = (id) => {
      const n = neighbors.get(id) || new Set();
      node.attr('opacity', d => (d.id === id || n.has(d.id)) ? 1 : 0.12);
      link.attr('opacity', d => (d.source === id || d.target === id) ? 1 : 0.08);
    };

    node.on('click', (event, d) => {
      highlight(d.id);
      setSidebar(d.id);
    });

    const sim = d3.forceSimulation(graphData.nodes)
      .force('link', d3.forceLink(graphData.edges).id(d => d.id).distance(d => Math.max(40, 140 - Math.min(100, (d.weight || 1) * 8))))
      .force('charge', d3.forceManyBody().strength(-180))
      .force('center', d3.forceCenter(w / 2, h / 2))
      .force('collide', d3.forceCollide().radius(d => radiusFor(d) + 3));

    sim.on('tick', () => {
      link
        .attr('x1', d => byId.get(d.source)?.x ?? d.source.x)
        .attr('y1', d => byId.get(d.source)?.y ?? d.source.y)
        .attr('x2', d => byId.get(d.target)?.x ?? d.target.x)
        .attr('y2', d => byId.get(d.target)?.y ?? d.target.y);

      node
        .attr('cx', d => d.x)
        .attr('cy', d => d.y);
    });

    const focusOn = (id) => {
      const n = byId.get(id);
      if (!n) return false;
      const scale = 1.8;
      const tx = w / 2 - n.x * scale;
      const ty = h / 2 - n.y * scale;
      svg.transition().duration(450).call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
      highlight(id);
      setSidebar(id);
      return true;
    };

    search.addEventListener('keydown', (e) => {
      if (e.key !== 'Enter') return;
      const q = (search.value || '').trim().toLowerCase();
      if (!q) return;
      const match = graphData.nodes.find(n => (n.id || '').toLowerCase().includes(q));
      if (match) focusOn(match.id);
    });
  </script>
 </body>
 </html>
`

func Generate(db *database.IntelligenceDB, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	archiveDir := filepath.Join(outDir, "archive")
	graphDir := filepath.Join(outDir, "graph")
	apiDir := filepath.Join(outDir, "api")
	assetsDir := filepath.Join(outDir, "assets")

	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(graphDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(assetsDir, "style.css"), []byte(styleCSS), 0644); err != nil {
		return err
	}

	now := time.Now().UTC()
	lastSync := db.GetLastSyncTime().UTC()
	updated := "Updated just now"
	if !lastSync.IsZero() {
		updated = "Updated " + relTime(lastSync, now)
	}

	recent, err := db.GetRecentArticles(20)
	if err != nil {
		return err
	}
	trending, err := db.GetTrendingEntities(24, 20)
	if err != nil {
		return err
	}

	indexData := pageData{
		Title:       "Recon — High-Signal Security Intelligence",
		StyleHref:   "./assets/style.css",
		HomeHref:    "./index.html",
		ArchiveHref: "./archive/index.html",
		GraphHref:   "./graph/index.html",
		FeedHref:    "./feed.xml",
		Updated:     updated,
		Trending:    trending,
		Articles:    toArticleViews(recent, now),
	}
	if err := writeTemplate(filepath.Join(outDir, "index.html"), baseHead+indexBody+baseFoot, indexData); err != nil {
		return err
	}

	if err := writeAPI(db, apiDir); err != nil {
		return err
	}

	if err := writeRSS(db, filepath.Join(outDir, "feed.xml")); err != nil {
		return err
	}

	days, err := db.GetArchiveDays()
	if err != nil {
		return err
	}

	var dayViews []archiveDayView
	for _, d := range days {
		dayViews = append(dayViews, archiveDayView{Date: d.Date, Count: d.Count, Href: "./" + d.Date + ".html"})
	}

	archiveIndexData := pageData{
		Title:       "Recon — Archive",
		StyleHref:   "../assets/style.css",
		HomeHref:    "../index.html",
		ArchiveHref: "./index.html",
		GraphHref:   "../graph/index.html",
		FeedHref:    "../feed.xml",
		Updated:     updated,
		Days:        dayViews,
	}
	if err := writeTemplate(filepath.Join(archiveDir, "index.html"), baseHead+archiveIndexBody+baseFoot, archiveIndexData); err != nil {
		return err
	}

	for i := range days {
		date := days[i].Date
		arts, err := db.GetArticlesByDate(date, 2000)
		if err != nil {
			return err
		}

		var prev *navView
		var next *navView
		if i+1 < len(days) {
			prev = &navView{Label: days[i+1].Date, Href: "./" + days[i+1].Date + ".html"}
		}
		if i-1 >= 0 {
			next = &navView{Label: days[i-1].Date, Href: "./" + days[i-1].Date + ".html"}
		}

		dayData := pageData{
			Title:       "Recon — " + date,
			StyleHref:   "../assets/style.css",
			HomeHref:    "../index.html",
			ArchiveHref: "./index.html",
			GraphHref:   "../graph/index.html",
			FeedHref:    "../feed.xml",
			Updated:     updated,
			Date:        date,
			Articles:    toArticleViews(arts, now),
			Prev:        prev,
			Next:        next,
		}
		if err := writeTemplate(filepath.Join(archiveDir, date+".html"), baseHead+archiveDayBody+baseFoot, dayData); err != nil {
			return err
		}
	}

	if err := writeGraph(db, filepath.Join(graphDir, "index.html"), updated); err != nil {
		return err
	}

	return nil
}

func writeAPI(db *database.IntelligenceDB, apiDir string) error {
	latest, err := db.GetRecentArticles(20)
	if err != nil {
		return err
	}
	latestJSON, err := json.MarshalIndent(latest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(apiDir, "latest.json"), latestJSON, 0644); err != nil {
		return err
	}

	head, err := db.GetRecentArticles(1)
	if err != nil {
		return err
	}
	var headline any = map[string]any{}
	if len(head) > 0 {
		headline = head[0]
	}
	headJSON, err := json.MarshalIndent(headline, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(apiDir, "headline.json"), headJSON, 0644); err != nil {
		return err
	}

	nodes, edges, err := db.GetEntityGraph()
	if err != nil {
		return err
	}
	graphJSON, err := json.MarshalIndent(map[string]any{"nodes": nodes, "edges": edges}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(apiDir, "graph.json"), graphJSON, 0644); err != nil {
		return err
	}

	return nil
}

func writeRSS(db *database.IntelligenceDB, path string) error {
	arts, err := db.GetRecentArticles(50)
	if err != nil {
		return err
	}

	ch := rssChannel{
		Title:       "Recon Security Intelligence",
		Link:        "https://recon.local/",
		Description: "Curated high-signal security intelligence from Recon",
		PubDate:     time.Now().UTC().Format(time.RFC1123Z),
	}

	for _, a := range arts {
		desc := a.Description
		if desc == "" {
			desc = a.Title
		}
		ch.Items = append(ch.Items, rssItem{
			Title:       a.Title,
			Link:        a.Link,
			Description: desc,
			PubDate:     a.Published.UTC().Format(time.RFC1123Z),
			GUID:        a.Link,
		})
	}

	doc := rss{Version: "2.0", Channel: ch}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append([]byte(xml.Header), out...)
	return os.WriteFile(path, out, 0644)
}

func writeGraph(db *database.IntelligenceDB, path string, updated string) error {
	nodes, edges, err := db.GetEntityGraph()
	if err != nil {
		return err
	}

	maxNodes := 160
	if len(nodes) > maxNodes {
		nodes = nodes[:maxNodes]
	}

	keep := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		keep[n.ID] = true
	}

	maxEdges := 420
	var keptEdges []models.EntityEdge
	for _, e := range edges {
		if len(keptEdges) >= maxEdges {
			break
		}
		if keep[e.Source] && keep[e.Target] {
			keptEdges = append(keptEdges, e)
		}
	}

	payload := graphPayload{Nodes: nodes, Edges: keptEdges}
	graphBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	articleMap := make(map[string][]graphArticle, len(nodes))
	for _, n := range nodes {
		arts, err := db.GetArticlesByEntity(n.ID)
		if err != nil {
			return err
		}
		var list []graphArticle
		for _, a := range arts {
			list = append(list, graphArticle{
				Title: a.Title,
				Link:  a.Link,
				When:  a.Published.UTC().Format("2006-01-02 15:04"),
			})
		}
		articleMap[n.ID] = list
	}
	articleBytes, err := json.Marshal(articleMap)
	if err != nil {
		return err
	}

	data := pageData{
		Title:          "Recon — Graph",
		StyleHref:      "../assets/style.css",
		HomeHref:       "../index.html",
		ArchiveHref:    "../archive/index.html",
		GraphHref:      "./index.html",
		FeedHref:       "../feed.xml",
		Updated:        updated,
		GraphInline:    template.JS(graphBytes),
		GraphArticles:  template.JS(articleBytes),
		GraphPageTitle: "Entity Relationship Graph",
	}

	return writeTemplate(path, graphBody, data)
}

func writeTemplate(path string, tpl string, data pageData) error {
	t, err := template.New("page").Parse(tpl)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0644)
}

func toArticleViews(arts []models.Article, now time.Time) []articleView {
	var out []articleView
	for _, a := range arts {
		desc := a.Description
		if desc == "" {
			desc = a.Title
		}
		out = append(out, articleView{
			Title:       a.Title,
			Link:        a.Link,
			Description: desc,
			SourceName:  a.SourceName,
			Score:       a.Score,
			When:        relTime(a.Published.UTC(), now),
		})
	}
	return out
}

func relTime(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	if now.Before(t) {
		return "just now"
	}
	d := now.Sub(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	if d < 14*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("2006-01-02 15:04")
}
