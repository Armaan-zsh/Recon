package tui

import (
	"context"
	"fmt"
	"news-cli/internal/database"
	"news-cli/internal/extractor"
	"news-cli/internal/fetcher"
	"news-cli/internal/models"
	"news-cli/internal/notifier"
	"news-cli/internal/textutil"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	pageStyle        = lipgloss.NewStyle().Background(lipgloss.Color("#0b1016")).Foreground(lipgloss.Color("#edf2f7"))
	panelStyle       = lipgloss.NewStyle().Background(lipgloss.Color("#111827")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#243244")).Padding(1, 2)
	headerStyle      = lipgloss.NewStyle().Background(lipgloss.Color("#111827")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#155e75")).Padding(1, 2)
	brandStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f59e0b"))
	subtitleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#93c5fd"))
	sectionTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#67e8f9"))
	selectedCard     = lipgloss.NewStyle().Background(lipgloss.Color("#172033")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#f59e0b")).Padding(1, 1)
	cardStyle        = lipgloss.NewStyle().Background(lipgloss.Color("#0f172a")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#1f2937")).Padding(1, 1)
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f8fafc"))
	selectedTitle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#fde68a"))
	metaStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))
	readerTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#dbe4ee"))
	sourceStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22d3ee"))
	keyStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f59e0b"))
	statusMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#cbd5e1")).Background(lipgloss.Color("#13202f")).Padding(0, 1)
	statusHotStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#fff7ed")).Background(lipgloss.Color("#9a3412")).Padding(0, 1)
)

type tuiModel struct {
	articles     []models.Article
	keywords     []string
	torProxy     string
	feedData     []byte
	db           *database.IntelligenceDB
	cursor       int
	height       int
	width        int
	scroll       int
	viewport     viewport.Model
	vpReady      bool
	nexusView    bool
	nexusText    string
	spinner      spinner.Model
	isSyncing    bool
	syncStatus   string
	totalSources int
}

type syncCompleteMsg struct {
	articles []models.Article
	newCount int
}

func performBackgroundSync(db *database.IntelligenceDB, keywords []string, torProxy string, feedData []byte, currentHashes map[string]bool) tea.Cmd {
	return func() tea.Msg {
		if db == nil {
			return syncCompleteMsg{}
		}

		lastSync := db.GetLastSyncTime()
		if time.Since(lastSync) < 15*time.Minute {
			return syncCompleteMsg{}
		}

		res, err := fetcher.FetchAll(context.Background(), keywords, torProxy, db, feedData)
		if err != nil || len(res.Articles) == 0 {
			return syncCompleteMsg{}
		}
		_ = db.SetLastSyncTime(time.Now())

		newArticles, _ := db.GetRecentArticles(200)
		newCount := 0
		for _, a := range newArticles {
			if !currentHashes[a.Hash()] {
				newCount++
			}
		}

		return syncCompleteMsg{articles: newArticles, newCount: newCount}
	}
}

func RunTUI(articles []models.Article, keywords []string, torProxy string, feedData []byte) error {
	db, _ := database.InitDB()
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#22d3ee"))

	totalSources := 0
	if feeds, err := fetcher.LoadFeeds(feedData); err == nil {
		totalSources = len(feeds)
	}

	m := tuiModel{
		articles:     articles,
		keywords:     keywords,
		torProxy:     torProxy,
		feedData:     feedData,
		db:           db,
		spinner:      s,
		isSyncing:    true,
		syncStatus:   "Scanning for fresh signals",
		totalSources: totalSources,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	if db != nil {
		db.Close()
	}
	return err
}

func (m tuiModel) Init() tea.Cmd {
	currentHashes := make(map[string]bool)
	for _, a := range m.articles {
		currentHashes[a.Hash()] = true
	}
	return tea.Batch(m.spinner.Tick, performBackgroundSync(m.db, m.keywords, m.torProxy, m.feedData, currentHashes))
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case syncCompleteMsg:
		m.isSyncing = false
		if msg.newCount > 0 {
			m.articles = msg.articles
			m.syncStatus = fmt.Sprintf("%d new signals landed", msg.newCount)
			go notifier.NotifyNewArticles(msg.newCount, msg.articles[0])
		} else {
			m.syncStatus = "No new signals in the last sync window"
		}
		return m, nil

	case spinner.TickMsg:
		if m.isSyncing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.vpReady {
			m.viewport = viewport.New(max(40, m.width/2), max(10, m.height-8))
			m.vpReady = true
		}
		m.viewport.Width = max(40, m.width-8)
		m.viewport.Height = max(10, m.height-8)

	case tea.KeyMsg:
		key := msg.String()
		if m.nexusView {
			if key == "esc" || key == "q" || key == "x" {
				m.nexusView = false
				return m, nil
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.articles)-1 {
				m.cursor++
				if m.cursor-m.scroll >= m.visibleItems() {
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
		case "x", "X":
			if m.cursor < len(m.articles) && m.db != nil {
				art := m.articles[m.cursor]
				ext := extractor.NewExtractor()
				ents := ext.ExtractEntities(art)
				if len(ents) > 0 {
					hist, _ := m.db.GetEntityTimeline(ents[0])
					m.nexusText = RenderNexusTimeline(ents[0], hist)
					m.viewport.SetContent(m.nexusText)
					m.nexusView = true
				}
			}
		case "o":
			if m.cursor < len(m.articles) {
				_ = openInBrowser(m.articles[m.cursor].Link)
			}
		}
	}
	return m, nil
}

func (m tuiModel) View() string {
	if m.nexusView {
		nexusChrome := headerStyle.Width(max(60, m.width-4)).Render(
			lipgloss.JoinVertical(lipgloss.Left,
				brandStyle.Render("RECON NEXUS"),
				subtitleStyle.Render("Entity timeline view"),
			),
		)
		return pageStyle.Render(nexusChrome + "\n" + m.viewport.View() + "\n" + statusMutedStyle.Render("[esc] back"))
	}

	if m.width == 0 || m.height == 0 {
		return "Loading Recon..."
	}

	header := m.renderHeader()
	body := m.renderBody()
	footer := m.renderFooter()
	return pageStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
}

func (m tuiModel) renderHeader() string {
	articleCount := fmt.Sprintf("%d live articles", len(m.articles))
	sourceCount := fmt.Sprintf("%d sources", m.totalSources)
	dateLabel := time.Now().Format("Mon Jan 02 15:04")
	status := m.syncStatus
	if m.isSyncing {
		status = m.spinner.View() + " " + status
	}

	chips := lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusMutedStyle.Render(articleCount),
		" ",
		statusMutedStyle.Render(sourceCount),
		" ",
		statusMutedStyle.Render(dateLabel),
	)

	return headerStyle.Width(max(60, m.width-4)).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			brandStyle.Render("RECON // INTELLIGENCE NEXUS"),
			subtitleStyle.Render("High-signal security, infra, exploit, and AI tracking"),
			"",
			lipgloss.JoinHorizontal(lipgloss.Left, chips, "  ", m.statusPill(status)),
		),
	)
}

func (m tuiModel) renderBody() string {
	listWidth := max(44, m.width*5/12)
	readerWidth := max(44, m.width-listWidth-6)

	left := m.renderList(listWidth)
	right := m.renderReader(readerWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m tuiModel) renderList(width int) string {
	if len(m.articles) == 0 {
		return panelStyle.Width(width).Render(
			lipgloss.JoinVertical(lipgloss.Left,
				sectionTitle.Render("SIGNAL BOARD"),
				"",
				metaStyle.Render("No articles are loaded yet."),
				metaStyle.Render("Run a fresh sync and this panel will populate."),
			),
		)
	}

	start := m.scroll
	end := min(len(m.articles), start+m.visibleItems())
	var cards []string
	cardWidth := width - 6
	for i := start; i < end; i++ {
		art := m.articles[i]
		title := textutil.Truncate(art.Title, max(24, cardWidth-4))
		summary := textutil.Truncate(textutil.PlainText(art.Description), max(30, cardWidth-6))
		metaLine := lipgloss.JoinHorizontal(
			lipgloss.Left,
			scorePill(art.Score),
			" ",
			sourceStyle.Render(strings.ToUpper(textutil.Truncate(art.SourceName, 20))),
			" ",
			metaStyle.Render(ageLabel(art.Published)),
		)

		cardBody := lipgloss.JoinVertical(
			lipgloss.Left,
			metaLine,
			titleStyle.Width(cardWidth).Render(title),
			metaStyle.Width(cardWidth).Render(summary),
		)

		style := cardStyle.Width(cardWidth)
		if i == m.cursor {
			style = selectedCard.Width(cardWidth)
			cardBody = lipgloss.JoinVertical(
				lipgloss.Left,
				metaLine,
				selectedTitle.Width(cardWidth).Render(title),
				metaStyle.Width(cardWidth).Render(summary),
			)
		}
		cards = append(cards, style.Render(cardBody))
	}

	listHeader := lipgloss.JoinHorizontal(
		lipgloss.Left,
		sectionTitle.Render("SIGNAL BOARD"),
		"  ",
		metaStyle.Render(fmt.Sprintf("%d-%d of %d", start+1, end, len(m.articles))),
	)

	return panelStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, listHeader, "", strings.Join(cards, "\n")),
	)
}

func (m tuiModel) renderReader(width int) string {
	if len(m.articles) == 0 || m.cursor >= len(m.articles) {
		return panelStyle.Width(width).Render(
			lipgloss.JoinVertical(lipgloss.Left,
				sectionTitle.Render("READER"),
				"",
				metaStyle.Render("Fresh articles will appear here after the next successful sync."),
			),
		)
	}

	art := m.articles[m.cursor]
	host := art.Link
	if idx := strings.Index(host, "://"); idx >= 0 {
		host = host[idx+3:]
	}
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}

	reader := lipgloss.JoinVertical(
		lipgloss.Left,
		sectionTitle.Render("READER"),
		"",
		lipgloss.JoinHorizontal(lipgloss.Left, sourceStyle.Render(strings.ToUpper(art.SourceName)), " ", scorePill(art.Score), " ", metaStyle.Render(art.Published.Local().Format("2006-01-02 15:04"))),
		"",
		titleStyle.Width(width-6).Render(art.Title),
		"",
		readerTextStyle.Width(width-6).Render(textutil.PlainText(art.Description)),
		"",
		metaStyle.Render("Host: "+host),
		metaStyle.Render("Link: "+textutil.Truncate(art.Link, max(24, width-12))),
		"",
		keyStyle.Render("[o] open")+metaStyle.Render(" in browser   ")+keyStyle.Render("[x] nexus")+metaStyle.Render(" entity timeline   ")+keyStyle.Render("[q] quit"),
	)

	return panelStyle.Width(width).Render(reader)
}

func (m tuiModel) renderFooter() string {
	footer := lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusMutedStyle.Render("j/k move"),
		" ",
		statusMutedStyle.Render("o open"),
		" ",
		statusMutedStyle.Render("x nexus"),
		" ",
		statusMutedStyle.Render("q quit"),
	)
	return footer
}

func (m tuiModel) visibleItems() int {
	return max(1, (m.height-10)/5)
}

func (m tuiModel) statusPill(text string) string {
	if strings.Contains(strings.ToLower(text), "new") || strings.Contains(text, "landed") {
		return statusHotStyle.Render(text)
	}
	return statusMutedStyle.Render(text)
}

func scorePill(score int) string {
	bg := lipgloss.Color("#1e293b")
	fg := lipgloss.Color("#cbd5e1")
	if score >= 80 {
		bg = lipgloss.Color("#7c2d12")
		fg = lipgloss.Color("#ffedd5")
	} else if score >= 65 {
		bg = lipgloss.Color("#854d0e")
		fg = lipgloss.Color("#fef3c7")
	} else if score >= 40 {
		bg = lipgloss.Color("#164e63")
		fg = lipgloss.Color("#cffafe")
	}
	return lipgloss.NewStyle().Background(bg).Foreground(fg).Padding(0, 1).Render(fmt.Sprintf("%d", score))
}

func ageLabel(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func openInBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
