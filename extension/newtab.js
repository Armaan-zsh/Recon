const API_BASE = 'http://127.0.0.1:9645';
const LANE_KEY = 'recon_lane_pref';

function getLane() {
  const lane = localStorage.getItem(LANE_KEY);
  return lane === 'expert' ? 'expert' : 'general';
}

function setLane(lane) {
  localStorage.setItem(LANE_KEY, lane === 'expert' ? 'expert' : 'general');
}

function applyLaneUI() {
  const lane = getLane();
  const generalBtn = document.getElementById('lane-general');
  const expertBtn = document.getElementById('lane-expert');
  if (!generalBtn || !expertBtn) return;
  generalBtn.classList.toggle('active', lane === 'general');
  expertBtn.classList.toggle('active', lane === 'expert');
}

function relTime(isoStr) {
  if (!isoStr) return '';
  const diff = Date.now() - new Date(isoStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function renderArticle(article) {
  document.getElementById('mystery-box').style.display = 'none';
  document.getElementById('setup-guide').hidden = true;
  
  const headlineCard = document.getElementById('headline-card');
  const headlineText = document.getElementById('headline-text');
  const headlineLink = document.getElementById('headline-link');
  const headlineMeta = document.getElementById('headline-meta');

  headlineText.textContent = article.Title;
  headlineLink.href = article.Link;
  
  const host = new URL(article.Link).hostname.replace(/^www\./, '');
  const when = relTime(article.Published);
  const source = (article.SourceName || host).toUpperCase();
  const laneLabel = article.__lane ? article.__lane.toUpperCase() : 'GENERAL';
  
  headlineMeta.innerHTML = `<span class="meta-source">${laneLabel}</span><span class="meta-sep">·</span>${source}<span class="meta-sep">·</span>${when}`;
  headlineCard.classList.add('visible');
}

async function loadArticle() {
  // 1. Instant render from cache (0ms latency!)
  const cached = localStorage.getItem('recon_cached_article');
  if (cached) {
    try {
      renderArticle(JSON.parse(cached));
    } catch(e) {}
  }

  // 2. Fetch fresh data in the background
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 1500); // 1.5s timeout so it never hangs
    
    const resp = await fetch(`${API_BASE}/api/latest/lanes`, { 
      signal: controller.signal,
      mode: 'cors',
      cache: 'no-cache'
    });
    clearTimeout(timeoutId);
    
    if (!resp.ok) throw new Error('API Error');
    const payload = await resp.json();
    const lane = getLane();
    const laneArticles = lane === 'expert' ? (payload?.expert || []) : (payload?.general || []);
    if (laneArticles.length === 0) throw new Error('Empty');

    const topWindow = laneArticles.slice(0, 5);
    const picked = topWindow[Math.floor(Math.random() * topWindow.length)];
    const freshArticle = { ...picked, __lane: lane };
    
    // Cache for the *next* new tab
    localStorage.setItem('recon_cached_article', JSON.stringify(freshArticle));

    // If we didn't have a cache, render this fresh one immediately
    if (!cached) {
      renderArticle(freshArticle);
    }
  } catch (err) {
    try {
      const fallbackResp = await fetch(`${API_BASE}/api/latest`, {
        mode: 'cors',
        cache: 'no-cache'
      });
      if (!fallbackResp.ok) throw new Error('Fallback API Error');
      const articles = await fallbackResp.json();
      if (!articles || articles.length === 0) throw new Error('Fallback Empty');
      const freshArticle = { ...articles[0], __lane: getLane() };
      localStorage.setItem('recon_cached_article', JSON.stringify(freshArticle));
      if (!cached) renderArticle(freshArticle);
      return;
    } catch (fallbackErr) {
      console.error("Recon Fetch Error:", err, fallbackErr);
      if (!cached) {
        document.getElementById('mystery-box').style.display = 'none';
        document.getElementById('setup-guide').hidden = false;
      }
    }
  }
}

function setupLaneToggle() {
  const generalBtn = document.getElementById('lane-general');
  const expertBtn = document.getElementById('lane-expert');
  if (!generalBtn || !expertBtn) return;

  generalBtn.addEventListener('click', () => {
    setLane('general');
    applyLaneUI();
    loadArticle();
  });
  expertBtn.addEventListener('click', () => {
    setLane('expert');
    applyLaneUI();
    loadArticle();
  });
}

document.addEventListener('DOMContentLoaded', () => {
  applyLaneUI();
  setupLaneToggle();
  loadArticle();
});
