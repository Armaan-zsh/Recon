package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	accentColor   = lipgloss.Color("#7D56F4") // Purple for Nexus
	dimColor      = lipgloss.Color("#78716c")
	panelColor    = lipgloss.Color("#1c1917")
	borderColor   = lipgloss.Color("#44403c")
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5f5f4"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(accentColor).PaddingLeft(1).Foreground(accentColor)
	metaStyle     = lipgloss.NewStyle().Foreground(dimColor)
	sourceStyle   = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Transform(strings.ToUpper)
	tuiHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Padding(1, 2)
	statusStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#292524")).Foreground(dimColor).Padding(0, 1)
)

type fetchCompleteMsg struct {
	result FetchResult
}

type tuiModel struct {
	articles     []Article
	result       FetchResult
	cfg          *AppConfig
	db           *IntelligenceDB
	cursor       int
	height       int
	width        int
	scroll       int
	viewport     viewport.Model
	vpReady      bool
	loadingInit  bool
	activePane   int // 0 = list, 1 = reader
	nexusView    bool
	nexusText    string
	spinner      spinner.Model
	isSyncing    bool
	syncStatus   string
}

type syncCompleteMsg struct {
	articles []Article
	newCount int
}

func performBackgroundSync(db *IntelligenceDB, cfg *AppConfig, currentHashes map[string]bool) tea.Cmd {
	return func() tea.Msg {
		if db == nil {
			return nil
		}

		lastSync := db.GetLastSyncTime()
		if time.Since(lastSync) < 15*time.Minute {
			return nil // Debounced
		}

		// Perform silent background engine sweep
		res, err := FetchAll(context.Background(), cfg, db)
		if err != nil || len(res.Articles) == 0 {
			return nil
		}
		_ = db.SetLastSyncTime(time.Now())

		// Fetch the newly merged state
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

func RunTUI(articles []Article, cfg *AppConfig) error {
	db, _ := InitDB()
	s := spinner.New()
	s.Spinner = spinner.Globe
	s.Style = lipgloss.NewStyle().Foreground(accentColor)

	m := tuiModel{
		articles:    articles,
		cfg:         cfg,
		db:          db,
		loadingInit: false,
		spinner:     s,
		isSyncing:   true,
		syncStatus:  "⏳ Syncing Intelligence Nexus...",
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
	return tea.Batch(m.spinner.Tick, performBackgroundSync(m.db, m.cfg, currentHashes))
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case syncCompleteMsg:
		m.isSyncing = false
		if msg.newCount > 0 {
			m.articles = msg.articles
			// UX Zero Layout Shift: push down cursor so scrolling focus isn't lost
			if m.cursor > 0 {
				m.cursor += msg.newCount
				m.scroll += msg.newCount
			}
			m.syncStatus = fmt.Sprintf("✓ Intelligence Nexus Updated: +%d signals", msg.newCount)
		} else {
			m.syncStatus = "✓ Intelligence Nexus Up-to-Date"
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
			m.viewport = viewport.New(m.width/2, m.height-4)
			m.vpReady = true
		}
		m.viewport.Width = m.width / 2
		m.viewport.Height = m.height - 4

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
				if m.cursor-m.scroll >= m.height-10 {
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
			// Trigger Nexus Relationship View
			if m.cursor < len(m.articles) && m.db != nil {
				art := m.articles[m.cursor]
				extractor := NewExtractor()
				ents := extractor.ExtractEntities(art)
				if len(ents) > 0 {
					// Pull historical timeline for the first high-signal entity
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
		return m.viewport.View() + "\n\n [ESC/X] Back to Intelligence List"
	}

	doc := strings.Builder{}
	
	// Header
	doc.WriteString(tuiHeaderStyle.Render("RECON INTELLIGENCE NEXUS") + "\n")

	// List
	var listLines []string
	start := m.scroll
	end := start + (m.height - 6)
	if end > len(m.articles) {
		end = len(m.articles)
	}

	for i := start; i < end; i++ {
		art := m.articles[i]
		title := art.Title
		if len(title) > m.width/2-10 {
			title = title[:m.width/2-13] + "..."
		}

		line := fmt.Sprintf("[%d] %s", art.Score, title)
		if i == m.cursor {
			listLines = append(listLines, selectedStyle.Render(line))
		} else {
			listLines = append(listLines, "  "+line)
		}
	}

	// Simple 2-column layout
	listCol := lipgloss.NewStyle().Width(m.width / 2).Render(strings.Join(listLines, "\n"))
	
	var readerContent string
	if m.cursor < len(m.articles) {
		art := m.articles[m.cursor]
		readerContent = fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", 
			sourceStyle.Render(art.SourceName),
			titleStyle.Render(art.Title),
			metaStyle.Render(art.Published.Format("2006-01-02 15:04")),
			art.Description)
	}
	readerCol := lipgloss.NewStyle().
		Width(m.width/2).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(borderColor).
		PaddingLeft(2).
		Render(readerContent)

	doc.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listCol, readerCol))
	
	// Footer
	spinnerStr := ""
	if m.isSyncing {
		spinnerStr = m.spinner.View() + " "
	}
	
	footerContent := statusStyle.Render(fmt.Sprintf(" %s%s ", spinnerStr, m.syncStatus))
	footerControls := statusStyle.Render(fmt.Sprintf(" %d SOURCES • %d ARTICLES • [X] NEXUS EVOLUTION • [O] OPEN • [Q] QUIT ", 1865, len(m.articles)))
	
	doc.WriteString("\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, footerContent, " • ", footerControls))

	return doc.String()
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
