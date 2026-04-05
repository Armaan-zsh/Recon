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
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #1c1917;
            --bg-panel: #292524;
            --bg-hover: #44403c;
            --text-primary: #f5f5f4;
            --text-secondary: #a8a29e;
            --accent: #f97316;
            --border: #44403c;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-primary);
            line-height: 1.6;
            display: flex;
            height: 100vh;
            overflow: hidden;
        }
        #sidebar {
            width: 340px;
            min-width: 340px;
            height: 100vh;
            overflow-y: auto;
            border-right: 1px solid var(--border);
            background-color: var(--bg-panel);
        }
        #content {
            flex-grow: 1;
            height: 100vh;
            overflow-y: auto;
            background-color: var(--bg-color);
            padding: 2rem 3rem;
        }
        #content.empty {
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .empty-msg {
            color: var(--text-secondary);
            font-size: 1.1rem;
            text-align: center;
        }
        .loading {
            color: var(--accent);
            font-size: 1.1rem;
            text-align: center;
            padding: 4rem;
        }
        .article {
            padding: 1rem 1.25rem;
            border-bottom: 1px solid var(--border);
            cursor: pointer;
            transition: background 0.15s;
        }
        .article:hover, .article.active {
            background-color: var(--bg-hover);
            border-left: 4px solid var(--accent);
            padding-left: calc(1.25rem - 4px);
        }
        .article .title {
            font-size: 0.95rem;
            font-weight: 700;
            color: var(--text-primary);
            line-height: 1.35;
            margin-bottom: 0.3rem;
        }
        .article:hover .title { color: var(--accent); }
        .meta {
            font-size: 0.75rem;
            color: var(--text-secondary);
        }
        .source { color: var(--accent); font-weight: 700; text-transform: uppercase; }
        #reader-title { font-size: 1.6rem; font-weight: 700; line-height: 1.3; margin-bottom: 0.5rem; }
        #reader-meta { color: var(--text-secondary); font-size: 0.85rem; margin-bottom: 1.5rem; padding-bottom: 1rem; border-bottom: 1px solid var(--border); }
        #reader-meta a { color: var(--accent); text-decoration: none; }
        #reader-meta a:hover { text-decoration: underline; }
        #reader-body { font-size: 1rem; line-height: 1.8; color: #d6d3d1; }
        #reader-body p { margin-bottom: 1rem; }
        #reader-body h1, #reader-body h2, #reader-body h3 { color: var(--text-primary); margin: 1.5rem 0 0.75rem; }
        #reader-body a { color: var(--accent); }
        #reader-body img { max-width: 100%; border-radius: 6px; margin: 1rem 0; }
        #reader-body pre, #reader-body code { background: var(--bg-panel); padding: 0.5rem; border-radius: 4px; overflow-x: auto; font-size: 0.9rem; }
        ::-webkit-scrollbar { width: 8px; }
        ::-webkit-scrollbar-track { background: var(--bg-panel); }
        ::-webkit-scrollbar-thumb { background: var(--border); border-radius: 4px; }
    </style>
</head>
<body>
    <div id="sidebar">
        {{range $index, $el := .Articles}}
        <div class="article" onclick="loadArticle({{$index}}, '{{.Link}}', this)" data-idx="{{$index}}">
            <div class="title">{{.Title}}</div>
            <div class="meta">
                <span class="source">{{.SourceName}}</span> · {{.Published.Format "Jan 02"}} · {{.Score}}
            </div>
        </div>
        {{end}}
    </div>
    <div id="content" class="empty">
        <div class="empty-msg">
            <div style="font-size: 2rem; margin-bottom: 1rem;">📰</div>
            Click an article to read it here.<br>
            <span style="font-size: 0.85rem;">Content is fetched and rendered server-side — no iframe blocking.</span>
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
                    let html = '<div id="reader-title">' + data.title + '</div>';
                    html += '<div id="reader-meta">';
                    if (data.byline) html += data.byline + ' · ';
                    html += '<a href="' + url + '" target="_blank">Open original ↗</a></div>';
                    html += '<div id="reader-body">' + data.content + '</div>';
                    content.innerHTML = html;
                    content.scrollTop = 0;
                })
                .catch(err => {
                    content.innerHTML = '<div class="loading">⚠ Failed to fetch<br><br><a href="' + url + '" target="_blank" style="color:#f97316">Open in new tab ↗</a></div>';
                });
        }
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
