package renderer

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"news-cli/internal/models"
	"os/exec"
	"runtime"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Recon — Daily Digest</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;700&family=IBM+Plex+Mono:wght@400;600&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #09111a;
            --bg-panel: rgba(15, 23, 42, 0.92);
            --bg-card: rgba(17, 24, 39, 0.92);
            --bg-hover: #172233;
            --text-primary: #eef2ff;
            --text-secondary: #94a3b8;
            --accent: #f59e0b;
            --accent-cool: #22d3ee;
            --border: #223247;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: 'Space Grotesk', sans-serif;
            background:
                radial-gradient(circle at top left, rgba(34, 211, 238, 0.12), transparent 28%),
                radial-gradient(circle at top right, rgba(245, 158, 11, 0.10), transparent 24%),
                var(--bg-color);
            color: var(--text-primary);
            line-height: 1.6;
            display: flex;
            height: 100vh;
            overflow: hidden;
        }
        #sidebar {
            width: 380px;
            min-width: 380px;
            height: 100vh;
            overflow-y: auto;
            border-right: 1px solid var(--border);
            background-color: var(--bg-panel);
            backdrop-filter: blur(14px);
        }
        #content {
            flex-grow: 1;
            height: 100vh;
            overflow-y: auto;
            padding: 2rem 3rem 3rem;
        }
        #content.empty {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .empty-msg, .sidebar-empty {
            color: var(--text-secondary);
            font-size: 1.1rem;
            text-align: center;
        }
        .sidebar-empty {
            padding: 3rem 2rem;
        }
        .loading {
            color: var(--accent);
            font-size: 1.1rem;
            text-align: center;
            padding: 4rem;
        }
        .sidebar-head {
            position: sticky;
            top: 0;
            z-index: 2;
            padding: 1.25rem 1.25rem 1rem;
            border-bottom: 1px solid var(--border);
            background: linear-gradient(180deg, rgba(9, 17, 26, 0.96), rgba(15, 23, 42, 0.9));
            backdrop-filter: blur(14px);
        }
        .eyebrow {
            font-family: 'IBM Plex Mono', monospace;
            font-size: 0.82rem;
            color: var(--accent-cool);
            letter-spacing: 0.08em;
            text-transform: uppercase;
        }
        .headline {
            font-size: 1.4rem;
            font-weight: 700;
            margin-top: 0.35rem;
        }
        .subhead {
            color: var(--text-secondary);
            font-size: 0.92rem;
            margin-top: 0.35rem;
        }
        .article-list {
            padding: 0.8rem;
        }
        .article {
            padding: 1rem 1rem 1rem 1.1rem;
            margin-bottom: 0.75rem;
            border: 1px solid var(--border);
            border-radius: 18px;
            background: var(--bg-card);
            cursor: pointer;
            transition: transform 0.15s, background 0.15s, border-color 0.15s;
        }
        .article:hover, .article.active {
            background-color: var(--bg-hover);
            border-color: rgba(245, 158, 11, 0.55);
            transform: translateY(-1px);
        }
        .article .title {
            font-size: 1rem;
            font-weight: 700;
            color: var(--text-primary);
            line-height: 1.35;
            margin: 0.55rem 0 0.45rem;
        }
        .article:hover .title { color: var(--accent); }
        .meta {
            font-size: 0.78rem;
            color: var(--text-secondary);
            font-family: 'IBM Plex Mono', monospace;
        }
        .source { color: var(--accent-cool); font-weight: 700; text-transform: uppercase; }
        .score {
            display: inline-block;
            margin-left: 0.5rem;
            padding: 0.12rem 0.48rem;
            border-radius: 999px;
            background: rgba(245, 158, 11, 0.14);
            color: #fcd34d;
        }
        .summary {
            color: var(--text-secondary);
            font-size: 0.88rem;
            line-height: 1.5;
        }
        .reader-shell {
            max-width: 920px;
            margin: 0 auto;
        }
        .reader-top {
            display: flex;
            gap: 0.8rem;
            flex-wrap: wrap;
            margin-bottom: 1rem;
            font-family: 'IBM Plex Mono', monospace;
            color: var(--text-secondary);
        }
        .reader-chip {
            border: 1px solid var(--border);
            border-radius: 999px;
            padding: 0.25rem 0.7rem;
            background: rgba(17, 24, 39, 0.85);
        }
        #reader-title { font-size: 2.3rem; font-weight: 700; line-height: 1.1; margin-bottom: 0.75rem; max-width: 18ch; }
        #reader-meta { color: var(--text-secondary); font-size: 0.95rem; margin-bottom: 1.5rem; padding-bottom: 1rem; border-bottom: 1px solid var(--border); }
        #reader-meta a { color: var(--accent); text-decoration: none; }
        #reader-meta a:hover { text-decoration: underline; }
        #reader-body { font-size: 1.04rem; line-height: 1.85; color: #d6dbe3; }
        #reader-body p { margin-bottom: 1rem; }
        #reader-body h1, #reader-body h2, #reader-body h3 { color: var(--text-primary); margin: 1.5rem 0 0.75rem; }
        #reader-body a { color: var(--accent); }
        #reader-body img { max-width: 100%; border-radius: 14px; margin: 1rem 0; }
        #reader-body pre, #reader-body code { background: rgba(15, 23, 42, 0.92); padding: 0.5rem; border-radius: 8px; overflow-x: auto; font-size: 0.9rem; }
        ::-webkit-scrollbar { width: 8px; }
        ::-webkit-scrollbar-track { background: var(--bg-panel); }
        ::-webkit-scrollbar-thumb { background: var(--border); border-radius: 4px; }
        @media (max-width: 960px) {
            body { flex-direction: column; height: auto; overflow: auto; }
            #sidebar { width: 100%; min-width: 100%; height: auto; }
            #content { height: auto; min-height: 65vh; padding: 1.4rem; }
            #reader-title { font-size: 1.7rem; }
        }
    </style>
</head>
<body>
    <div id="sidebar">
        <div class="sidebar-head">
            <div class="eyebrow">Recon</div>
            <div class="headline">Live Signal Board</div>
            <div class="subhead">{{len .Articles}} ranked articles ready to read</div>
        </div>
        <div class="article-list">
            {{if .Articles}}
                {{range $index, $el := .Articles}}
                <div class="article" onclick="loadArticle({{$index}}, '{{.Link}}', this)" data-idx="{{$index}}">
                    <div class="meta">
                        <span class="source">{{.SourceName}}</span>
                        <span class="score">{{.Score}}</span>
                        · {{.Published.Format "Jan 02 15:04"}}
                    </div>
                    <div class="title">{{.Title}}</div>
                    <div class="summary">{{.Description}}</div>
                </div>
                {{end}}
            {{else}}
                <div class="sidebar-empty">No articles are available yet. Run a sync and reload this view.</div>
            {{end}}
        </div>
    </div>
    <div id="content" class="empty">
        <div class="empty-msg">
            <div style="font-size: 2rem; margin-bottom: 1rem;">◎</div>
            Loading first article…<br>
            <span style="font-size: 0.85rem;">Recon fetches and renders article content server-side.</span>
        </div>
    </div>
    <script>
        function loadArticle(idx, url, el) {
            document.querySelectorAll('.article').forEach(a => a.classList.remove('active'));
            el.classList.add('active');

            const content = document.getElementById('content');
            content.className = '';
            content.innerHTML = '<div class="loading">⏳ Fetching article...</div>';

            fetch('/read?url=' + encodeURIComponent(url))
                .then(r => r.json())
                .then(data => {
                    if (data.error) {
                        content.innerHTML = '<div class="loading">⚠ ' + data.error + '<br><br><a href="' + url + '" target="_blank" style="color:#f97316">Open in new tab ↗</a></div>';
                        return;
                    }
                    let html = '<div class="reader-shell">';
                    html += '<div class="reader-top"><div class="reader-chip">Article ' + (idx + 1) + '</div><div class="reader-chip">Live render</div></div>';
                    html += '<div id="reader-title">' + data.title + '</div>';
                    html += '<div id="reader-meta">';
                    if (data.byline) html += data.byline + ' · ';
                    html += '<a href="' + url + '" target="_blank">Open original ↗</a></div>';
                    html += '<div id="reader-body">' + data.content + '</div>';
                    html += '</div>';
                    content.innerHTML = html;
                    content.scrollTop = 0;
                })
                .catch(err => {
                    content.innerHTML = '<div class="loading">⚠ Failed to fetch<br><br><a href="' + url + '" target="_blank" style="color:#f97316">Open in new tab ↗</a></div>';
                });
        }

        window.addEventListener('DOMContentLoaded', () => {
            const first = document.querySelector('.article');
            if (first) first.click();
        });
    </script>
</body>
</html>
`

func RenderHTML(articles []models.Article) (string, error) {
	funcMap := template.FuncMap{
		"add": func(i, j int) int { return i + j },
	}
	t, err := template.New("index").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, struct{ Articles []models.Article }{Articles: articles}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ServeAndOpen(htmlContent string) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Failed to start local server: %v\n", err)
		return
	}
	port := listener.Addr().(*net.TCPAddr).Port
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(htmlContent))
	})

	mux.HandleFunc("/read", func(w http.ResponseWriter, r *http.Request) {
		articleURL := r.URL.Query().Get("url")
		if articleURL == "" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error":"no url provided"}`))
			return
		}
		if models.IsOnionURL(articleURL) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error":"Onion content is intel-only in browser mode. Open via a Tor-configured environment."}`))
			return
		}

		client := &http.Client{Timeout: 15 * time.Second}
		req, _ := http.NewRequest("GET", articleURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"130\", \"Google Chrome\";v=\"130\", \"Not?A_Brand\";v=\"99\"")
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", "\"Linux\"")

		respFetch, err := client.Do(req)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"error":"Network failure: %s"}`, html.EscapeString(err.Error()))
			return
		}
		defer respFetch.Body.Close()

		if respFetch.StatusCode == 403 || respFetch.StatusCode == 503 {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"error":"Blocked by Cloudflare/Anti-Bot (%d). Try opening original."}`, respFetch.StatusCode)
			return
		}

		u, _ := url.Parse(articleURL)
		article, err := readability.FromReader(respFetch.Body, u)
		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			fmt.Fprintf(w, `{"error":"Extraction failed: %s"}`, html.EscapeString(err.Error()))
			return
		}

		titleJSON := strings.ReplaceAll(article.Title, `"`, `\"`)
		titleJSON = strings.ReplaceAll(titleJSON, "\n", " ")
		bylineJSON := strings.ReplaceAll(article.Byline, `"`, `\"`)
		bylineJSON = strings.ReplaceAll(bylineJSON, "\n", " ")

		contentHTML := article.Content
		if contentHTML == "" {
			contentHTML = "<p>" + html.EscapeString(article.TextContent) + "</p>"
		}

		fmt.Fprintf(w, `{"title":"%s","byline":"%s","content":%q}`, titleJSON, bylineJSON, contentHTML)
	})

	go func() {
		_ = http.Serve(listener, mux)
	}()

	openBrowserURL(serverURL)

	fmt.Printf("\n  Serving at %s\n", serverURL)
	fmt.Printf("  Press Ctrl+C to exit.\n\n")

	select {}
}

func openBrowserURL(u string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", u).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	case "darwin":
		err = exec.Command("open", u).Start()
	}
	if err != nil {
		fmt.Printf("Could not open browser: %v\n", err)
	}
}
