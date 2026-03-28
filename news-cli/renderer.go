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
            padding: 2rem 3rem;
            line-height: 1.6;
            max-width: 960px;
            margin: auto;
        }
        .article {
            padding: 1.25rem 0;
            border-bottom: 1px solid var(--border);
        }
        .title {
            font-size: 1.15rem;
            font-weight: 700;
            text-decoration: none;
            color: var(--text-primary);
            line-height: 1.4;
        }
        .title:hover { color: var(--accent); }
        .meta {
            font-size: 0.85rem;
            color: var(--text-secondary);
            margin-top: 0.4rem;
        }
        .source {
            color: var(--accent);
            font-weight: 700;
            text-transform: uppercase;
        }
        ::-webkit-scrollbar { width: 8px; }
        ::-webkit-scrollbar-track { background: var(--bg-color); }
        ::-webkit-scrollbar-thumb { background: var(--border); border-radius: 4px; }
    </style>
</head>
<body>
    {{if not .Articles}}
    <p style="font-size: 1.25rem; margin-top: 2rem;">No articles found matching the given keywords.</p>
    {{else}}
    {{range .Articles}}
    <div class="article">
        <a class="title" href="{{.Link}}" target="_blank">{{.Title}}</a>
        <div class="meta">
            <span class="source">{{.SourceName}}</span> · {{.Published.Format "Jan 02, 2006"}} · Score: {{.Score}}
        </div>
    </div>
    {{end}}
    {{end}}
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

	openBrowserURL(url)

	fmt.Printf("\n  Serving at %s\n", url)
	fmt.Printf("  Press Ctrl+C to exit.\n\n")

	select {}
}

func openBrowserURL(url string) {
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
