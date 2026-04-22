package tui

import (
	"fmt"
	"news-cli/internal/models"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderNexusTimeline(entityName string, articles []models.Article) string {
	if len(articles) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("No historical intelligence found for: " + entityName)
	}

	doc := strings.Builder{}

	tlTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Underline(true)

	doc.WriteString(tlTitleStyle.Render("EVOLUTION OF: "+strings.ToUpper(entityName)) + "\n\n")

	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Width(12)
	nodeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("112")).Render(" ● ")
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(" │ ")

	for i, a := range articles {
		dateStr := a.Published.Format("Jan 2006")

		doc.WriteString(dateStyle.Render(dateStr) + nodeStyle + lipgloss.NewStyle().Bold(true).Render(a.Title) + "\n")
		doc.WriteString(strings.Repeat(" ", 12) + lineStyle + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(fmt.Sprintf("%s • %s", a.SourceName, a.Link)) + "\n")

		if i < len(articles)-1 {
			doc.WriteString(strings.Repeat(" ", 12) + lineStyle + "\n")
		}
	}

	return lipgloss.NewStyle().Padding(1, 4).Render(doc.String())
}

func GenerateNexusSummary(entityName string, articles []models.Article) string {
	if len(articles) < 2 {
		return "Single sighting. No evolution identified yet."
	}

	first := articles[0].Published
	last := articles[len(articles)-1].Published
	span := last.Sub(first)

	days := int(span.Hours() / 24)
	return fmt.Sprintf("Threat active for %d days with %d major incidents/research posts recorded in Nexus memory.", days, len(articles))
}
