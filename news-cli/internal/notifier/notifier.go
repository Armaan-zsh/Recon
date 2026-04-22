package notifier

import (
	"fmt"
	"news-cli/internal/models"
	"os/exec"
	"runtime"
)

func NotifyNewArticles(newCount int, lead models.Article) {
	if newCount <= 0 || runtime.GOOS != "linux" {
		return
	}

	title := fmt.Sprintf("Recon: %d new signal(s)", newCount)
	body := lead.Title
	if newCount > 1 {
		body = fmt.Sprintf("%s\n+ %d more", lead.Title, newCount-1)
	}

	urgency := "normal"
	icon := "dialog-information"
	if lead.Score >= 80 {
		urgency = "critical"
		icon = "dialog-warning"
	}

	_ = exec.Command("notify-send", "-u", urgency, "-i", icon, title, body).Run()
}
