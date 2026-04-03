package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const serviceTemplate = `[Unit]
Description=Recon - Daily Tech & CyberSec Intelligence Digest

[Service]
Type=oneshot
ExecStartPre=/usr/bin/notify-send -u normal -i dialog-information "🔍 Recon Intelligence" "Fetching your daily cyber intelligence digest..."
ExecStart=%s --browser
ExecStartPost=/usr/bin/notify-send -u normal -i dialog-information "✅ Recon Ready" "Your daily intelligence digest is open in the browser."
Environment=DISPLAY=:0
`

const timerTemplate = `[Unit]
Description=Recon Daily Intelligence Timer

[Timer]
OnCalendar=*-*-* %s:00
Persistent=true

[Install]
WantedBy=timers.target
`

const resumeServiceTemplate = `[Unit]
Description=Recon digest after resume from suspend

[Service]
Type=oneshot
ExecStartPre=/bin/sleep 10
ExecStart=/bin/bash -c 'if [ "$(date +%%F)" != "$(cat %s/last_run.txt 2>/dev/null)" ]; then /usr/bin/notify-send -u normal -i dialog-information "🔍 Recon" "Catching up on missed intelligence..." && %s --browser && /usr/bin/notify-send -u normal -i dialog-information "✅ Recon Ready" "Digest loaded."; fi'
`

// ScheduleInstall creates systemd user timer and service files.
func ScheduleInstall(scheduleTime string) error {

	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}
	binPath, _ = filepath.EvalSymlinks(binPath)

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	systemdDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user dir: %w", err)
	}

	cfgDir, err := configDir()
	if err != nil {
		return err
	}

	serviceContent := fmt.Sprintf(serviceTemplate, binPath)
	servicePath := filepath.Join(systemdDir, "recon-digest.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	timerContent := fmt.Sprintf(timerTemplate, scheduleTime)
	timerPath := filepath.Join(systemdDir, "recon-digest.timer")
	if err := os.WriteFile(timerPath, []byte(timerContent), 0644); err != nil {
		return fmt.Errorf("failed to write timer file: %w", err)
	}

	resumeContent := fmt.Sprintf(resumeServiceTemplate, cfgDir, binPath)
	resumePath := filepath.Join(systemdDir, "recon-resume.service")
	if err := os.WriteFile(resumePath, []byte(resumeContent), 0644); err != nil {
		return fmt.Errorf("failed to write resume service: %w", err)
	}

	cmds := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "recon-digest.timer"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %s: %w", strings.Join(args, " "), err)
		}
	}

	return nil
}

// ScheduleDisable stops and removes the systemd timer.
func ScheduleDisable() error {
	cmds := [][]string{
		{"systemctl", "--user", "disable", "--now", "recon-digest.timer"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	return nil
}

// ScheduleStatus shows the current status of the timer.
func ScheduleStatus() error {
	cmd := exec.Command("systemctl", "--user", "status", "recon-digest.timer")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
