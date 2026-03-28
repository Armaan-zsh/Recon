package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os/exec"
	"runtime"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Recon — Daily Digest</title>
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
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-primary);
            margin: 0;
            padding: 0;
            line-height: 1.6;
            display: flex;
            height: 100vh;
            overflow: hidden;
        }
        #sidebar {
            width: 38%;
            min-width: 350px;
            max-width: 500px;
            height: 100vh;
            overflow-y: auto;
            border-right: 1px solid var(--border);
            background-color: var(--bg-panel);
        }
        #content {
            flex-grow: 1;
            height: 100vh;
            background-color: var(--bg-color);
            display: flex;
            flex-direction: column;
        }
        #content iframe {
            flex-grow: 1;
            width: 100%;
            border: none;
            background-color: #fff;
        }
        .empty-state {
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100%;
            color: var(--text-secondary);
            font-size: 1.25rem;
            text-align: center;
            padding: 2rem;
            flex-direction: column;
        }
        .article {
            padding: 1.5rem 2rem;
            border-bottom: 1px solid var(--border);
            transition: all 0.2s;
            cursor: pointer;
        }
        .article:hover, .article.active {
            background-color: var(--bg-hover);
            border-left: 5px solid var(--accent);
            padding-left: calc(2rem - 5px);
        }
        .title {
            font-size: 1.15rem;
            font-weight: 700;
            display: block;
            text-decoration: none;
            color: var(--text-primary);
            margin-bottom: 0.5rem;
            line-height: 1.4;
        }
        .title:hover { color: var(--accent); }
        .meta {
            font-size: 0.85rem;
            color: var(--text-secondary);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .source {
            color: var(--accent);
            font-weight: 700;
            text-transform: uppercase;
        }
        .external-link {
            color: var(--text-secondary);
            text-decoration: none;
            font-size: 0.8rem;
            border: 1px solid var(--border);
            padding: 2px 6px;
            border-radius: 4px;
        }
        .external-link:hover {
            color: var(--text-primary);
            border-color: var(--text-secondary);
        }
        ::-webkit-scrollbar { width: 8px; }
        ::-webkit-scrollbar-track { background: var(--bg-panel); }
        ::-webkit-scrollbar-thumb { background: var(--border); border-radius: 4px; }
        ::-webkit-scrollbar-thumb:hover { background: var(--text-secondary); }
    </style>
</head>
<body>
    <div id="sidebar">
        {{if not .Articles}}
        <div class="empty-state">No articles found matching the given keywords.</div>
        {{else}}
        {{range $index, $element := .Articles}}
        <div class="article" onclick="loadArticle('{{.Link}}', this)">
            <a class="title" href="{{.Link}}" onclick="event.preventDefault()">{{.Title}}</a>
            <div class="meta">
                <span class="source">{{.SourceName}}</span>
                <a href="{{.Link}}" target="_blank" class="external-link" onclick="event.stopPropagation()">Open ↗</a>
            </div>
            <div class="meta" style="margin-top: 8px; font-size: 0.75rem;">
                <span>{{.Published.Format "Jan 02, 2006"}}</span>
                <span>Score: {{.Score}}</span>
            </div>
        </div>
        {{end}}
        {{end}}
    </div>
    <div id="content">
        <div id="empty-state" class="empty-state">
            <svg style="margin-bottom: 1rem; color: var(--border);" width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path></svg>
            Select an article from the sidebar to start reading.
            <div style="font-size: 0.85rem; margin-top: 1rem; max-width: 300px;">If a site blocks iframes, click "Open ↗" to read in new tab.</div>
        </div>
        <iframe id="article-frame" style="display: none;" sandbox="allow-same-origin allow-scripts allow-popups allow-forms"></iframe>
    </div>
    <script>
        function loadArticle(url, element) {
            document.querySelectorAll('.article').forEach(el => el.classList.remove('active'));
            element.classList.add('active');
            document.getElementById('empty-state').style.display = 'none';
            const frame = document.getElementById('article-frame');
            frame.style.display = 'block';
            frame.src = url;
        }
    </script>
</body>
</html>
`

func renderHTML(articles []Article) (string, error) {
	funcMap := template.FuncMap{
		"add": func(i, j int) int { return i + j },
	}
	t, err := template.New("index").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, struct{ Articles []Article }{Articles: articles}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func serveAndOpen(htmlContent string) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Failed to start local server: %v\n", err)
		return
	}
	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(htmlContent))
	})

	go func() {
		_ = http.Serve(listener, nil)
	}()

	openBrowser(url)

	fmt.Printf("\n  Serving at %s\n", url)
	fmt.Printf("  Press Ctrl+C to exit.\n\n")

	select {}
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		fmt.Printf("Could not open browser: %v\n", err)
	}
}
