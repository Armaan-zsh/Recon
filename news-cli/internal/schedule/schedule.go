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
ExecStartPost=-/usr/bin/notify-send -u normal -i dialog-information "✅ Recon Ready" "Your daily intelligence digest has been synced."
Environment=DISPLAY=:0 DBUS_SESSION_BUS_ADDRESS=unix:path=%%t/bus
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
After=suspend.target hibernate.target hybrid-sleep.target suspend-then-hibernate.target
PartOf=suspend.target hibernate.target hybrid-sleep.target suspend-then-hibernate.target

[Service]
Type=oneshot
TimeoutStartSec=180
ExecStartPre=/bin/sleep 10
ExecStart=/bin/bash -c 'if [ "$(date +%%%%F)" != "$(cat %s/last_run.txt 2>/dev/null)" ]; then %s --json > /dev/null 2>&1 && date +%%%%F > %s/last_run.txt && /usr/bin/notify-send -u normal -i dialog-information "✅ Recon" "Fresh intelligence synced." || true; fi'
Environment=DISPLAY=:0 DBUS_SESSION_BUS_ADDRESS=unix:path=%%t/bus

[Install]
WantedBy=suspend.target hibernate.target hybrid-sleep.target suspend-then-hibernate.target
`

const startupServiceTemplate = `[Unit]
Description=Recon digest on user login/startup
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
TimeoutStartSec=180
ExecStart=/bin/bash -c 'if [ "$(date +%%%%F)" != "$(cat %s/last_run.txt 2>/dev/null)" ]; then %s --json > /dev/null 2>&1 && date +%%%%F > %s/last_run.txt && /usr/bin/notify-send -u normal -i dialog-information "✅ Recon" "Fresh intelligence synced." || true; fi'
Environment=DISPLAY=:0 DBUS_SESSION_BUS_ADDRESS=unix:path=%%t/bus

[Install]
WantedBy=default.target
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

	cfgDir, _ := os.UserConfigDir()
	reconCfgDir := filepath.Join(cfgDir, "recon")
	if err := os.MkdirAll(reconCfgDir, 0755); err != nil {
		return fmt.Errorf("failed to create recon config dir: %w", err)
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

	resumeContent := fmt.Sprintf(resumeServiceTemplate, reconCfgDir, binPath, reconCfgDir)
	resumePath := filepath.Join(systemdDir, "recon-resume.service")
	if err := os.WriteFile(resumePath, []byte(resumeContent), 0644); err != nil {
		return fmt.Errorf("failed to write resume service: %w", err)
	}

	startupContent := fmt.Sprintf(startupServiceTemplate, reconCfgDir, binPath, reconCfgDir)
	startupPath := filepath.Join(systemdDir, "recon-startup.service")
	if err := os.WriteFile(startupPath, []byte(startupContent), 0644); err != nil {
		return fmt.Errorf("failed to write startup service: %w", err)
	}

	cmds := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "recon-digest.timer"},
		{"systemctl", "--user", "enable", "--now", "recon-startup.service"},
		{"systemctl", "--user", "enable", "recon-resume.service"},
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
		{"systemctl", "--user", "disable", "--now", "recon-startup.service"},
		{"systemctl", "--user", "disable", "recon-resume.service"},
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
