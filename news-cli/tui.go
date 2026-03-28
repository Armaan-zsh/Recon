package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	accentColor   = lipgloss.Color("#f97316")
	dimColor      = lipgloss.Color("#a8a29e")
	panelColor    = lipgloss.Color("#292524")
	borderColor   = lipgloss.Color("#44403c")
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5f5f4"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	metaStyle     = lipgloss.NewStyle().Foreground(dimColor)
	sourceStyle   = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Transform(strings.ToUpper)
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Padding(1, 2)
	statusStyle   = lipgloss.NewStyle().Background(panelColor).Foreground(dimColor).Padding(0, 1)
)

// Messages
type articleContentMsg struct {
	content string
	err     error
}

type tuiModel struct {
	articles    []Article
	result      FetchResult
	cursor      int
	height      int
	width       int
	scroll      int
	viewport    viewport.Model
	vpReady     bool
	vpContent   string
	loading     bool
	activePane  int // 0 = list, 1 = reader
	lastLoaded  int // index of last loaded article (-1 = none)
	cache       map[string]string // URL -> rendered markdown
}

func runTUI(articles []Article, result FetchResult) error {
	m := tuiModel{
		articles:   articles,
		result:     result,
		lastLoaded: -1,
		cache:      make(map[string]string),
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

// fetchArticleContent fetches URL and extracts readable text.
func fetchArticleContent(url string, width int) tea.Cmd {
	return func() tea.Msg {
		article, err := readability.FromURL(url, 15*time.Second)
		if err != nil {
			return articleContentMsg{err: err}
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
		renderWidth := width - 6
		if renderWidth < 40 {
			renderWidth = 40
		}

		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(renderWidth),
		)
		if err != nil {
			return articleContentMsg{content: md.String()}
		}

		rendered, err := renderer.Render(md.String())
		if err != nil {
			return articleContentMsg{content: md.String()}
		}

		return articleContentMsg{content: rendered}
	}
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		vpWidth := m.width/2 - 2
		if vpWidth < 30 {
			vpWidth = 30
		}
		vpHeight := m.height - 4
		if vpHeight < 5 {
			vpHeight = 5
		}

		if !m.vpReady {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.viewport.SetContent("  Press Enter on an article to read it here.\n\n  Tab to switch panes. q to quit.")
			m.vpReady = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
		}

	case articleContentMsg:
		m.loading = false
		if msg.err != nil {
			m.viewport.SetContent(fmt.Sprintf("\n  ⚠ Could not fetch article: %v\n\n  Press 'o' to open in browser instead.", msg.err))
		} else {
			m.cache[m.articles[m.lastLoaded].Link] = msg.content
			m.vpContent = msg.content
			m.viewport.SetContent(msg.content)
			m.viewport.GotoTop()
		}

	case tea.KeyMsg:
		key := msg.String()

		// Global keys
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activePane = 1 - m.activePane // Toggle 0 <-> 1
			return m, nil
		case "o":
			// Open current article in browser
			if m.cursor < len(m.articles) {
				openInBrowser(m.articles[m.cursor].Link)
			}
			return m, nil
		}

		if m.activePane == 0 {
			// List pane controls
			switch key {
			case "j", "down":
				if m.cursor < len(m.articles)-1 {
					m.cursor++
					visibleItems := (m.height - 4) / 2
					if visibleItems < 3 {
						visibleItems = 3
					}
					if m.cursor-m.scroll >= visibleItems {
						m.scroll++
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
				if m.cursor < len(m.articles) && !m.loading {
					// Check cache first
					if content, ok := m.cache[m.articles[m.cursor].Link]; ok {
						m.viewport.SetContent(content)
						m.viewport.GotoTop()
						m.lastLoaded = m.cursor
						m.activePane = 1
						return m, nil
					}

					m.loading = true
					m.lastLoaded = m.cursor
					
					// Show immediate metadata/description
					a := m.articles[m.cursor]
					header := titleStyle.Render(a.Title) + "\n" + sourceStyle.Render(a.SourceName) + "\n\n"
					desc := metaStyle.Render(a.Description) + "\n\n"
					m.viewport.SetContent(header + desc + "  ⏳ Fetching full article...")
					m.viewport.GotoTop()
					m.activePane = 1

					vpWidth := m.width/2 - 4
					return m, fetchArticleContent(m.articles[m.cursor].Link, vpWidth)
				}
			}
		} else {
			// Reader pane — pass keys to viewport for scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m tuiModel) View() string {
	if m.width == 0 || !m.vpReady {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	header := headerStyle.Render("RECON")
	b.WriteString(header)
	b.WriteString("\n")

	if len(m.articles) == 0 {
		b.WriteString("\n  No articles found. Try: recon --tags vulnerability,malware\n")
		return b.String()
	}

	listWidth := m.width / 2
	if listWidth < 30 {
		listWidth = 30
	}

	visibleItems := (m.height - 4) / 2
	if visibleItems < 3 {
		visibleItems = 3
	}

	// Build article list
	var listLines []string
	end := m.scroll + visibleItems
	if end > len(m.articles) {
		end = len(m.articles)
	}

	for i := m.scroll; i < end; i++ {
		a := m.articles[i]
		title := a.Title
		maxTitleLen := listWidth - 8
		if maxTitleLen < 20 {
			maxTitleLen = 20
		}
		if len(title) > maxTitleLen {
			title = title[:maxTitleLen-3] + "..."
		}

		prefix := "  "
		if m.activePane == 0 && i == m.cursor {
			prefix = "▸ "
			line := selectedStyle.Render(prefix + title)
			listLines = append(listLines, line)
		} else if i == m.cursor {
			line := titleStyle.Render(prefix + title)
			listLines = append(listLines, line)
		} else {
			line := metaStyle.Render(prefix + title)
			listLines = append(listLines, line)
		}

		src := sourceStyle.Render(a.SourceName)
		score := metaStyle.Render(fmt.Sprintf(" · %d", a.Score))
		listLines = append(listLines, "  "+src+score)
	}

	listContent := strings.Join(listLines, "\n")

	// Pad list to fill height
	listLineCount := len(listLines)
	targetHeight := m.height - 4
	for listLineCount < targetHeight {
		listContent += "\n"
		listLineCount++
	}

	// Build right pane with border
	readerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(m.width - listWidth - 4).
		Height(m.height - 4)

	if m.activePane == 1 {
		readerStyle = readerStyle.BorderForeground(accentColor)
	}

	reader := readerStyle.Render(m.viewport.View())

	// Combine columns
	layout := lipgloss.JoinHorizontal(lipgloss.Top, listContent, " ", reader)
	b.WriteString(layout)
	b.WriteString("\n")

	// Status bar
	paneIndicator := "[LIST]"
	if m.activePane == 1 {
		paneIndicator = "[READER]"
	}
	loadingIndicator := ""
	if m.loading {
		loadingIndicator = " ⏳"
	}

	status := statusStyle.Width(m.width).Render(
		fmt.Sprintf(" %d articles | %d/%d feeds | %.1fs | %s%s | j/k:nav Enter:read Tab:switch o:browser q:quit",
			len(m.articles), m.result.FetchedFeeds, m.result.TotalFeeds, m.result.Duration.Seconds(),
			paneIndicator, loadingIndicator),
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
