const API_BASE = 'http://localhost:9645';

// Get today's date as YYYY-MM-DD in local time
function todayString() {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

// Relative time label
function relTime(isoStr) {
  if (!isoStr) return '';
  const pub = new Date(isoStr);
  const diff = Date.now() - pub.getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

// Strip protocol+www for display
function displayHost(url) {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url;
  }
}

// Pick a random element from an array
function pickRandom(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

async function loadArticle() {
  const headlineCard = document.getElementById('headline-card');
  const headlineText = document.getElementById('headline-text');
  const headlineLink = document.getElementById('headline-link');
  const headlineMeta = document.getElementById('headline-meta');
  const setupGuide   = document.getElementById('setup-guide');
  const headlineWrap = document.getElementById('headline-wrap');

  try {
    // Try today's archive first — gives truly random article from today
    const today = todayString();
    let articles = [];

    try {
      const archiveResp = await fetch(`${API_BASE}/api/archive/${today}`, { signal: AbortSignal.timeout(2000) });
      if (archiveResp.ok) {
        const data = await archiveResp.json();
        if (Array.isArray(data) && data.length > 0) {
          articles = data;
        }
      }
    } catch (_) {
      // archive endpoint failed or empty, fall through to /api/latest
    }

    // Fallback: latest 20 articles
    if (articles.length === 0) {
      const latestResp = await fetch(`${API_BASE}/api/latest`, { signal: AbortSignal.timeout(2000) });
      if (!latestResp.ok) throw new Error('no data');
      const data = await latestResp.json();
      if (!Array.isArray(data) || data.length === 0) throw new Error('empty');
      articles = data;
    }

    const article = pickRandom(articles);
    if (!article || !article.Title) throw new Error('bad article');

    // Populate card
    headlineText.textContent = article.Title;

    const host = displayHost(article.Link);
    headlineLink.href        = article.Link;
    headlineLink.textContent = host + ' ↗';

    const when   = relTime(article.Published);
    const source = (article.SourceName || host).toUpperCase();
    const score  = article.Score || 0;

    let metaHTML = `<span class="meta-source">${source}</span>`;
    if (when) metaHTML += `<span class="meta-sep">·</span>${when}`;
    if (score >= 80) {
      metaHTML += `<span class="meta-sep">·</span><span class="meta-score-high">${score}</span>`;
    }
    headlineMeta.innerHTML = metaHTML;

    // Reveal
    headlineCard.classList.add('visible');

  } catch (err) {
    // Daemon offline — show setup guide
    headlineWrap.hidden = true;
    setupGuide.hidden   = false;
  }
}

loadArticle();
