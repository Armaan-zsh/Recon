package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	accentColor   = lipgloss.Color("#f97316")
	dimColor      = lipgloss.Color("#78716c")
	panelColor    = lipgloss.Color("#1c1917")
	borderColor   = lipgloss.Color("#44403c")
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5f5f4"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(accentColor).PaddingLeft(1).Foreground(accentColor)
	metaStyle     = lipgloss.NewStyle().Foreground(dimColor)
	sourceStyle   = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Transform(strings.ToUpper)
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Padding(1, 2)
	statusStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#292524")).Foreground(dimColor).Padding(0, 1)
	cachedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")) // Emerald for cached
)

// Messages
type articleContentMsg struct {
	url     string
	content string
	err     error
}

type fetchCompleteMsg struct {
	result FetchResult
}

type tuiModel struct {
	articles     []Article
	result       FetchResult
	sources      []FeedSource
	keywords     []string
	strictFilter bool
	cfg          *AppConfig
	cursor       int
	height       int
	width        int
	scroll       int
	viewport     viewport.Model
	vpReady      bool
	vpContent    string
	loading      bool
	loadingInit  bool             // TRUE during the initial huge fetch
	activePane   int              // 0 = list, 1 = reader
	lastLoaded   int              // index of last loaded article (-1 = none)
	cache        map[string]string // URL -> rendered markdown
	fetching     map[string]bool   // URL -> is currently being fetched in background
	spinner      spinner.Model
}

func runTUI(sources []FeedSource, keywords []string, strictFilter bool, cfg *AppConfig) error {
	s := spinner.New()
	s.Spinner = spinner.Globe
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	m := tuiModel{
		sources:      sources,
		keywords:     keywords,
		strictFilter: strictFilter,
		cfg:          cfg,
		loadingInit:  true,
		lastLoaded:   -1,
		cache:        make(map[string]string),
		fetching:     make(map[string]bool),
		spinner:      s,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			res := FetchFeeds(context.Background(), m.sources, m.keywords, m.strictFilter, m.cfg)
			return fetchCompleteMsg{result: res}
		},
	)
}

func (m *tuiModel) startPreloading() tea.Cmd {
	var cmds []tea.Cmd
	
	limit := 5
	if len(m.articles) < limit {
		limit = len(m.articles)
	}
	
	for i := 0; i < limit; i++ {
		url := m.articles[i].Link
		m.fetching[url] = true
		cmds = append(cmds, fetchArticleContent(url, m.width/2-4))
	}
	
	return tea.Batch(cmds...)
}

// fetchArticleContent fetches URL and extracts readable text.
func fetchArticleContent(url string, width int) tea.Cmd {
	return func() tea.Msg {
		article, err := readability.FromURL(url, 15*time.Second)
		if err != nil {
			return articleContentMsg{url: url, err: err}
		}

		// Build markdown from extracted content
		var md strings.Builder
		md.WriteString(fmt.Sprintf("# %s\n\n", article.Title))
		if article.Byline != "" {
			md.WriteString(fmt.Sprintf("*%s*\n\n", article.Byline))
		}
		md.WriteString("---\n\n")
		md.WriteString(article.TextContent)

		// Render with glamour
		renderWidth := width
		if renderWidth < 40 {
			renderWidth = 40
		}

		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(renderWidth),
		)
		if err != nil {
			return articleContentMsg{url: url, content: md.String()}
		}

		rendered, err := renderer.Render(md.String())
		if err != nil {
			return articleContentMsg{url: url, content: md.String()}
		}

		return articleContentMsg{url: url, content: rendered}
	}
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchCompleteMsg:
		m.result = msg.result
		sort.Slice(m.result.Articles, func(i, j int) bool {
			if m.result.Articles[i].Score == m.result.Articles[j].Score {
				return m.result.Articles[i].Published.After(m.result.Articles[j].Published)
			}
			return m.result.Articles[i].Score > m.result.Articles[j].Score
		})
		
		topN := 50
		if len(m.result.Articles) < topN {
			topN = len(m.result.Articles)
		}
		m.articles = m.result.Articles[:topN]
		m.loadingInit = false
		_ = RecordLastRun(m.cfg)
		return m, m.startPreloading()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		vpWidth := m.width/2 - 4
		if vpWidth < 30 {
			vpWidth = 30
		}
		vpHeight := m.height - 4
		if vpHeight < 5 {
			vpHeight = 5
		}

		if !m.vpReady {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.viewport.SetContent("  Select an article to read.\n\n  j/k: navigate\n  Enter: read full article\n  Tab: scroll reader\n  o: open in browser\n  q: quit")
			m.vpReady = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case articleContentMsg:
		delete(m.fetching, msg.url)
		if msg.err == nil {
			m.cache[msg.url] = msg.content
			// If this is the article we're currently trying to show, update viewport
			if m.lastLoaded != -1 && m.articles[m.lastLoaded].Link == msg.url {
				m.loading = false
				m.viewport.SetContent(msg.content)
				m.viewport.GotoTop()
			}
		} else if m.lastLoaded != -1 && m.articles[m.lastLoaded].Link == msg.url {
			m.loading = false
			m.viewport.SetContent(fmt.Sprintf("\n  ⚠ Could not fetch article: %v\n\n  Press 'o' to open in browser instead.", msg.err))
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Global keys
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activePane = 1 - m.activePane
			return m, nil
		case "o":
			if m.cursor < len(m.articles) {
				openInBrowser(m.articles[m.cursor].Link)
			}
			return m, nil
		}

		if m.activePane == 0 {
			switch key {
			case "j", "down":
				if m.cursor < len(m.articles)-1 {
					m.cursor++
					visibleItems := (m.height - 4) / 2
					if m.cursor-m.scroll >= visibleItems {
						m.scroll++
					}
					// Pre-fetch the next article
					next := m.cursor + 1
					if next < len(m.articles) {
						url := m.articles[next].Link
						if _, ok := m.cache[url]; !ok && !m.fetching[url] {
							m.fetching[url] = true
							return m, fetchArticleContent(url, m.viewport.Width)
						}
					}
				}
			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.scroll {
						m.scroll--
					}
				}
			case "g":
				m.cursor = 0
				m.scroll = 0
			case "G":
				m.cursor = len(m.articles) - 1
				visibleItems := (m.height - 4) / 2
				if m.cursor >= visibleItems {
					m.scroll = m.cursor - visibleItems + 1
				}
			case "enter":
				if m.cursor < len(m.articles) {
					url := m.articles[m.cursor].Link
					m.lastLoaded = m.cursor
					
					if content, ok := m.cache[url]; ok {
						m.loading = false
						m.viewport.SetContent(content)
						m.viewport.GotoTop()
						m.activePane = 1
					} else {
						m.loading = true
						a := m.articles[m.cursor]
						header := titleStyle.Render(a.Title) + "\n" + sourceStyle.Render(a.SourceName) + "\n\n"
						desc := metaStyle.Render(a.Description) + "\n\n"
						m.viewport.SetContent(header + desc + "  " + m.spinner.View() + " Fetching full article...")
						m.viewport.GotoTop()
						m.activePane = 1
						
						if !m.fetching[url] {
							m.fetching[url] = true
							return m, fetchArticleContent(url, m.viewport.Width)
						}
					}
				}
			}
		} else {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m tuiModel) View() string {
	if m.width == 0 || !m.vpReady {
		return "Starting Recon..."
	}

	var b strings.Builder

	// Header
	header := headerStyle.Render("RECON 2.0")
	if m.loadingInit {
		header += " " + m.spinner.View() + lipgloss.NewStyle().Foreground(dimColor).Render(fmt.Sprintf(" GATHERING INTELLIGENCE (Scanning %d Feeds)", len(m.sources)))
	} else if len(m.fetching) > 0 {
		header += " " + m.spinner.View()
	}
	b.WriteString(header)
	b.WriteString("\n")

	if m.loadingInit {
		// Just render the empty skeleton while loading
		listPane := lipgloss.NewStyle().Width(m.width/2 - 2).Height(m.height - 4).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(borderColor).Render("\n  Connecting to secure channels...")
		readerPane := lipgloss.NewStyle().Width(m.width/2 - 2).Height(m.height - 4).Render("")
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listPane, readerPane))
		return b.String()
	}

	if len(m.articles) == 0 {
		b.WriteString("\n  No articles found matching your categories.\n")
		return b.String()
	}

	listWidth := m.width / 2
	visibleItems := (m.height - 4) / 2

	// Build article list
	var listLines []string
	end := m.scroll + visibleItems
	if end > len(m.articles) {
		end = len(m.articles)
	}

	for i := m.scroll; i < end; i++ {
		a := m.articles[i]
		title := a.Title
		maxLen := listWidth - 10
		if len(title) > maxLen {
			title = title[:maxLen-3] + "..."
		}

		prefix := "  "
		if i == m.cursor {
			if m.activePane == 0 {
				line := selectedStyle.Render(title)
				listLines = append(listLines, line)
			} else {
				line := titleStyle.Render(prefix + title)
				listLines = append(listLines, line)
			}
		} else {
			line := prefix + title
			listLines = append(listLines, line)
		}

		// Metadata line
		cachedIcon := ""
		if _, ok := m.cache[a.Link]; ok {
			cachedIcon = cachedStyle.Render(" ●")
		} else if m.fetching[a.Link] {
			cachedIcon = metaStyle.Render(" " + m.spinner.View())
		}
		
		src := sourceStyle.Render(a.SourceName)
		score := metaStyle.Render(fmt.Sprintf(" · %d", a.Score))
		listLines = append(listLines, "  "+src+score+cachedIcon)
	}

	listContent := strings.Join(listLines, "\n")
	listLineCount := len(listLines)
	targetHeight := m.height - 4
	for listLineCount < targetHeight {
		listContent += "\n"
		listLineCount++
	}

	// Reader Pane
	readerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(m.width - listWidth - 2).
		Height(m.height - 4)

	if m.activePane == 1 {
		readerStyle = readerStyle.BorderForeground(accentColor)
	}

	reader := readerStyle.Render(m.viewport.View())

	// Combine
	layout := lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", reader)
	b.WriteString(layout)
	b.WriteString("\n")

	// Status Bar
	paneLabel := "[LIST]"
	if m.activePane == 1 {
		paneLabel = "[READER]"
	}
	
	fetchInfo := fmt.Sprintf("%d feeds scanned in %.1fs", m.result.TotalFeeds, m.result.Duration.Seconds())
	if m.result.Duration.Seconds() > 10 {
		fetchInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Render(fetchInfo)
	} else {
		fetchInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Render(fetchInfo)
	}

	status := statusStyle.Width(m.width).Render(
		fmt.Sprintf(" %s | %d articles | %s | j/k:nav Ent:read Tab:focus o:browser q:quit",
			paneLabel, len(m.articles), fetchInfo),
	)
	b.WriteString(status)

	return b.String()
}

func openInBrowser(url string) {
	switch runtime.GOOS {
	case "linux":
		_ = exec.Command("xdg-open", url).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	}
}
