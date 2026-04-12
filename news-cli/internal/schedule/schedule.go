package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const serviceTemplate = `[Unit]
Description=Recon - Daily Tech & CyberSec Intelligence Digest
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
TimeoutStartSec=180
ExecStart=%s --json
Environment=DISPLAY=:0 DBUS_SESSION_BUS_ADDRESS=unix:path=%%t/bus
`

const timerTemplate = `[Unit]
Description=Recon Daily Intelligence Timer

[Timer]
OnCalendar=*-*-* %s:00
OnBootSec=2min
OnUnitActiveSec=30min
AccuracySec=5min
Persistent=true

[Install]
WantedBy=timers.target
`

func Install(scheduleTime string) error {
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

	_ = os.Remove(filepath.Join(systemdDir, "recon-startup.service"))
	_ = os.Remove(filepath.Join(systemdDir, "recon-resume.service"))

	cmds := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "recon-digest.timer"},
		{"systemctl", "--user", "start", "recon-digest.service"},
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

func Disable() error {
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

func Status() error {
	cmd := exec.Command("systemctl", "--user", "status", "recon-digest.timer")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
