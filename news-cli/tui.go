package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	accentColor    = lipgloss.Color("#f97316")
	dimColor       = lipgloss.Color("#a8a29e")
	bgColor        = lipgloss.Color("#1c1917")
	panelColor     = lipgloss.Color("#292524")
	borderColor    = lipgloss.Color("#44403c")
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5f5f4"))
	selectedStyle  = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	metaStyle      = lipgloss.NewStyle().Foreground(dimColor)
	sourceStyle    = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Transform(strings.ToUpper)
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Padding(1, 2)
	statusStyle    = lipgloss.NewStyle().Background(panelColor).Foreground(dimColor).Padding(0, 2)
	previewBorder  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderColor).Padding(1, 2)
)

type tuiModel struct {
	articles []Article
	result   FetchResult
	cursor   int
	height   int
	width    int
	scroll   int // scroll offset for article list
}

func runTUI(articles []Article, result FetchResult) error {
	m := tuiModel{
		articles: articles,
		result:   result,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.articles)-1 {
				m.cursor++
				// Auto-scroll
				visibleItems := m.height - 6 // header + status bar
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
		case "enter":
			if m.cursor < len(m.articles) {
				openInBrowser(m.articles[m.cursor].Link)
			}
		case "g":
			m.cursor = 0
			m.scroll = 0
		case "G":
			m.cursor = len(m.articles) - 1
			visibleItems := m.height - 6
			if m.cursor >= visibleItems {
				m.scroll = m.cursor - visibleItems + 1
			}
		}
	}

	return m, nil
}

func (m tuiModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	header := headerStyle.Render("RECON — Daily Cyber & Tech Intelligence")
	b.WriteString(header)
	b.WriteString("\n")

	if len(m.articles) == 0 {
		b.WriteString("\n  No articles found matching your keywords.\n")
		b.WriteString("\n  Try: recon --tags vulnerability,malware\n")
		return b.String()
	}

	// Calculate visible area
	listWidth := m.width / 2
	if listWidth < 40 {
		listWidth = 40
	}
	previewWidth := m.width - listWidth - 4

	visibleItems := m.height - 6
	if visibleItems < 5 {
		visibleItems = 5
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
		if len(title) > listWidth-6 {
			title = title[:listWidth-9] + "..."
		}

		var line string
		if i == m.cursor {
			line = selectedStyle.Render(fmt.Sprintf("  ▸ %s", title))
		} else {
			line = titleStyle.Render(fmt.Sprintf("    %s", title))
		}
		listLines = append(listLines, line)

		meta := metaStyle.Render(fmt.Sprintf("      %s · Score: %d", sourceStyle.Render(a.SourceName), a.Score))
		listLines = append(listLines, meta)
	}

	listContent := strings.Join(listLines, "\n")

	// Build preview pane
	var previewContent string
	if m.cursor < len(m.articles) {
		a := m.articles[m.cursor]
		pTitle := titleStyle.Copy().Width(previewWidth - 4).Render(a.Title)
		pSource := sourceStyle.Render(a.SourceName) + metaStyle.Render(fmt.Sprintf(" · %s · Score: %d", a.Published.Format("Jan 02, 2006"), a.Score))
		pDesc := metaStyle.Copy().Width(previewWidth - 4).Render(a.Description)
		pLink := lipgloss.NewStyle().Foreground(accentColor).Underline(true).Render(a.Link)

		previewContent = fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", pTitle, pSource, pDesc, pLink)
	}

	preview := previewBorder.Width(previewWidth).Height(visibleItems * 2).Render(previewContent)

	// Combine columns
	layout := lipgloss.JoinHorizontal(lipgloss.Top, listContent, "  ", preview)
	b.WriteString(layout)
	b.WriteString("\n")

	// Status bar
	status := statusStyle.Width(m.width).Render(
		fmt.Sprintf(" %d articles | %d/%d feeds | %.1fs | j/k:navigate Enter:open q:quit",
			len(m.articles), m.result.FetchedFeeds, m.result.TotalFeeds, m.result.Duration.Seconds()),
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
