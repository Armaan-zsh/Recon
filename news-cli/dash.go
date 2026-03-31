package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

	divider = lipgloss.NewStyle().
		SetString("•").
		Padding(0, 1).
		Foreground(subtle).
		String()

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2).
			MarginTop(1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2).
			MarginRight(2).
			Width(40)
)

type dashboardModel struct {
	viewport viewport.Model
	ready    bool
	articles []Article
}

func runGridDashboard() error {
	p := tea.NewProgram(&dashboardModel{}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *dashboardModel) Init() tea.Cmd {
	return nil
}

func (m *dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *dashboardModel) View() string {
	doc := strings.Builder{}

	// Header
	header := headerStyle.Render("RECON INTELLIGENCE NEXUS DASHBOARD")
	doc.WriteString(header + "\n\n")

	// Grid Row 1
	row1 := lipgloss.JoinHorizontal(lipgloss.Top,
		cardStyle.Render(fmt.Sprintf("%s\n\n%s", lipgloss.NewStyle().Bold(true).Foreground(special).Render("📡 FRESH SIGNALS"), "No new signals in last 30m.")),
		cardStyle.Render(fmt.Sprintf("%s\n\n%s", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("🔴 CRITICAL VULNS"), "CVE-2026-3055: RCE in Linux Kernel")),
	)
	doc.WriteString(row1 + "\n\n")

	// Grid Row 2
	row2 := lipgloss.JoinHorizontal(lipgloss.Top,
		cardStyle.Render(fmt.Sprintf("%s\n\n%s", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).Render("🕵️ APT TRACKER"), "Volt Typhoon: Targeting US Power Grid")),
		cardStyle.Render(fmt.Sprintf("%s\n\n%s", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")).Render("📚 DEEP RESEARCH"), "The Internals of Zero-Click iMessage Exploits")),
	)
	doc.WriteString(row2 + "\n\n")

	// Footer
	doc.WriteString("\n" + lipgloss.NewStyle().Foreground(subtle).Render("Press 'q' to exit dashboard • 's' to sync feeds"))

	return lipgloss.NewStyle().Padding(1, 4).Render(doc.String())
}
