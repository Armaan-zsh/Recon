const API_BASE = 'http://localhost:9645';

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

function displayHost(url) {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url;
  }
}

function pickRandom(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

async function loadArticle() {
  const mysteryBox = document.getElementById('mystery-box');
  const headlineCard = document.getElementById('headline-card');
  const headlineText = document.getElementById('headline-text');
  const headlineLink = document.getElementById('headline-link');
  const headlineMeta = document.getElementById('headline-meta');
  const setupGuide   = document.getElementById('setup-guide');

  try {
    const latestResp = await fetch(`${API_BASE}/api/latest`, { signal: AbortSignal.timeout(2000) });
    if (!latestResp.ok) throw new Error('no data');
    const articles = await latestResp.json();
    if (!Array.isArray(articles) || articles.length === 0) throw new Error('empty');

    const article = pickRandom(articles);
    
    // Set content
    headlineText.textContent = article.Title;
    headlineLink.href = article.Link;

    const host = displayHost(article.Link);
    const when = relTime(article.Published);
    const source = (article.SourceName || host).toUpperCase();
    
    headlineMeta.innerHTML = `<span class="meta-source">${source}</span><span class="meta-sep">·</span>${when}`;

    // Reveal after a short delay for effect
    setTimeout(() => {
      mysteryBox.classList.add('hidden');
      headlineCard.classList.add('visible');
    }, 800);

  } catch (err) {
    mysteryBox.classList.add('hidden');
    setupGuide.hidden = false;
  }
}

document.addEventListener('DOMContentLoaded', loadArticle);
