package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const serviceTemplate = `[Unit]
Description=Recon - Daily Tech & CyberSec News Digest

[Service]
Type=oneshot
ExecStart=%s --browser
Environment=DISPLAY=:0
`

const timerTemplate = `[Unit]
Description=Recon Daily Digest Timer

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
ExecStart=/bin/bash -c 'if [ "$(date +%%F)" != "$(cat %s/last_run.txt 2>/dev/null)" ]; then %s --browser; fi'
`

// ScheduleInstall creates systemd user timer and service files.
func ScheduleInstall(scheduleTime string) error {
	// Get the binary path
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}
	binPath, _ = filepath.EvalSymlinks(binPath)

	// Get systemd user directory
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

	// Write service file
	serviceContent := fmt.Sprintf(serviceTemplate, binPath)
	servicePath := filepath.Join(systemdDir, "recon-digest.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Write timer file
	timerContent := fmt.Sprintf(timerTemplate, scheduleTime)
	timerPath := filepath.Join(systemdDir, "recon-digest.timer")
	if err := os.WriteFile(timerPath, []byte(timerContent), 0644); err != nil {
		return fmt.Errorf("failed to write timer file: %w", err)
	}

	// Write resume service
	resumeContent := fmt.Sprintf(resumeServiceTemplate, cfgDir, binPath)
	resumePath := filepath.Join(systemdDir, "recon-resume.service")
	if err := os.WriteFile(resumePath, []byte(resumeContent), 0644); err != nil {
		return fmt.Errorf("failed to write resume service: %w", err)
	}

	// Reload systemd and enable/start the timer
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
		_ = cmd.Run() // Ignore errors if already disabled
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
