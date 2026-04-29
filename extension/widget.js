(() => {
  if (window.top !== window.self) return;

  const API_BASE = "http://127.0.0.1:9645";
  const STORAGE_KEY = "recon_widget_state_v1";

  const state = {
    lane: "general",
    locked: true,
    x: null,
    y: null,
  };

  const root = document.createElement("div");
  root.id = "recon-floating-widget";
  root.className = "recon-locked";
  root.innerHTML = `
    <div class="recon-fw-header">
      <div class="recon-fw-tag">Recon Widget</div>
      <div class="recon-fw-actions">
        <button class="recon-fw-btn" id="recon-lane-general" type="button">General</button>
        <button class="recon-fw-btn" id="recon-lane-expert" type="button">Expert</button>
        <button class="recon-fw-btn" id="recon-lock-btn" type="button" title="Toggle lock">Locked</button>
      </div>
    </div>
    <a class="recon-fw-content" id="recon-article-link" target="_blank" rel="noopener noreferrer">
      <div class="recon-fw-meta" id="recon-article-meta">Loading...</div>
      <h3 class="recon-fw-title" id="recon-article-title">Fetching signal...</h3>
    </a>
    <div class="recon-fw-state" id="recon-widget-state" hidden></div>
  `;
  document.documentElement.appendChild(root);

  const laneGeneralBtn = root.querySelector("#recon-lane-general");
  const laneExpertBtn = root.querySelector("#recon-lane-expert");
  const lockBtn = root.querySelector("#recon-lock-btn");
  const linkEl = root.querySelector("#recon-article-link");
  const metaEl = root.querySelector("#recon-article-meta");
  const titleEl = root.querySelector("#recon-article-title");
  const statusEl = root.querySelector("#recon-widget-state");
  const headerEl = root.querySelector(".recon-fw-header");

  function relTime(isoStr) {
    if (!isoStr) return "just now";
    const diff = Date.now() - new Date(isoStr).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return "just now";
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    return `${Math.floor(hrs / 24)}d ago`;
  }

  function updateButtons() {
    laneGeneralBtn.classList.toggle("recon-active", state.lane === "general");
    laneExpertBtn.classList.toggle("recon-active", state.lane === "expert");
    lockBtn.textContent = state.locked ? "Locked" : "Unlocked";
    root.classList.toggle("recon-locked", state.locked);
  }

  function getStorage() {
    return new Promise((resolve) => {
      if (!chrome?.storage?.local) return resolve(null);
      chrome.storage.local.get([STORAGE_KEY], (res) => resolve(res[STORAGE_KEY] || null));
    });
  }

  function setStorage(next) {
    if (!chrome?.storage?.local) return;
    chrome.storage.local.set({ [STORAGE_KEY]: next });
  }

  function applyPosition() {
    if (Number.isFinite(state.x) && Number.isFinite(state.y)) {
      root.style.left = `${state.x}px`;
      root.style.top = `${state.y}px`;
      root.style.right = "auto";
    }
  }

  async function fetchLaneHeadline() {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 1800);
    try {
      const resp = await fetch(`${API_BASE}/api/latest/lanes`, {
        signal: controller.signal,
        mode: "cors",
        cache: "no-cache",
      });
      clearTimeout(timeoutId);
      if (!resp.ok) throw new Error("lane endpoint unavailable");
      const payload = await resp.json();
      const lane = state.lane === "expert" ? payload?.expert || [] : payload?.general || [];
      if (!lane.length) throw new Error("empty lane");
      const pick = lane[Math.floor(Math.random() * Math.min(5, lane.length))];
      return { ...pick, __lane: state.lane };
    } catch {
      const fallback = await fetch(`${API_BASE}/api/latest`, { mode: "cors", cache: "no-cache" });
      if (!fallback.ok) throw new Error("daemon offline");
      const arts = await fallback.json();
      if (!Array.isArray(arts) || !arts.length) throw new Error("no articles");
      return { ...arts[0], __lane: state.lane };
    }
  }

  function renderArticle(article) {
    titleEl.textContent = article.Title || "No headline";
    linkEl.href = article.Link || "#";
    const source = (article.SourceName || "Recon").toUpperCase();
    metaEl.textContent = `${state.lane.toUpperCase()} · ${source} · ${relTime(article.Published)}`;
    statusEl.hidden = true;
  }

  async function refresh() {
    try {
      const article = await fetchLaneHeadline();
      renderArticle(article);
    } catch (err) {
      statusEl.hidden = false;
      statusEl.textContent = "Recon daemon offline";
      titleEl.textContent = "Run recon daemon to load signals.";
      metaEl.textContent = "WIDGET READY";
      linkEl.removeAttribute("href");
    }
  }

  laneGeneralBtn.addEventListener("click", () => {
    state.lane = "general";
    updateButtons();
    setStorage(state);
    refresh();
  });
  laneExpertBtn.addEventListener("click", () => {
    state.lane = "expert";
    updateButtons();
    setStorage(state);
    refresh();
  });
  lockBtn.addEventListener("click", () => {
    state.locked = !state.locked;
    updateButtons();
    setStorage(state);
  });

  let dragging = false;
  let dragDX = 0;
  let dragDY = 0;
  headerEl.addEventListener("pointerdown", (ev) => {
    if (state.locked) return;
    dragging = true;
    const rect = root.getBoundingClientRect();
    dragDX = ev.clientX - rect.left;
    dragDY = ev.clientY - rect.top;
    headerEl.setPointerCapture(ev.pointerId);
  });
  headerEl.addEventListener("pointermove", (ev) => {
    if (!dragging || state.locked) return;
    const nextX = Math.max(0, Math.min(window.innerWidth - rectWidth(), ev.clientX - dragDX));
    const nextY = Math.max(0, Math.min(window.innerHeight - rectHeight(), ev.clientY - dragDY));
    state.x = nextX;
    state.y = nextY;
    applyPosition();
  });
  headerEl.addEventListener("pointerup", () => {
    if (!dragging) return;
    dragging = false;
    setStorage(state);
  });

  function rectWidth() {
    return root.getBoundingClientRect().width;
  }
  function rectHeight() {
    return root.getBoundingClientRect().height;
  }

  window.addEventListener("resize", () => {
    if (!Number.isFinite(state.x) || !Number.isFinite(state.y)) return;
    state.x = Math.max(0, Math.min(window.innerWidth - rectWidth(), state.x));
    state.y = Math.max(0, Math.min(window.innerHeight - rectHeight(), state.y));
    applyPosition();
    setStorage(state);
  });

  (async () => {
    const saved = await getStorage();
    if (saved && typeof saved === "object") {
      state.lane = saved.lane === "expert" ? "expert" : "general";
      state.locked = saved.locked !== false;
      state.x = Number.isFinite(saved.x) ? saved.x : null;
      state.y = Number.isFinite(saved.y) ? saved.y : null;
    }
    updateButtons();
    applyPosition();
    refresh();
  })();
})();
