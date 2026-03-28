# Recon

> Daily tech & cybersecurity intelligence — straight from 570+ feeds, right in your terminal.

Recon fetches, scores, and surfaces the best cybersecurity and tech news from 570+ RSS feeds including Qualys, CrowdStrike, Unit 42, Mandiant, CISA, SANS ISC, Krebs on Security, Hacker News, and hundreds more. No AI fluff. No sponsored content. Just signal.

## Features

- **570+ curated feeds** — vulnerability research labs, CVE trackers, threat intel blogs, tech engineering blogs
- **Smart scoring** — multi-signal: title weighting, CVE pattern detection, zero-day bonuses, source authority
- **Terminal UI** — split-pane TUI with keyboard navigation (j/k/Enter/q)
- **Browser mode** — warm dark sidebar view with inline article reader
- **Auto-scheduling** — systemd timer that opens your digest every morning, even if your laptop was asleep
- **First-run wizard** — pick your timezone, focus categories, and schedule time once
- **Zero file artifacts** — serves HTML in-memory, no files left on disk
- **Single binary** — all 570+ feeds baked in via `go:embed`

## Install

### Option 1: Build from source

```bash
git clone https://github.com/Armaan-zsh/Recon.git
cd Recon/news-cli
make install
```

### Option 2: Go install (requires Go 1.24+)

```bash
go install github.com/Armaan-zsh/Recon/news-cli@latest
```

### Option 3: One-liner (download pre-built binary)

```bash
curl -sL https://github.com/Armaan-zsh/Recon/releases/latest/download/recon-linux-amd64 -o ~/.local/bin/recon && chmod +x ~/.local/bin/recon
```

> **Note:** Make sure `~/.local/bin` is in your PATH:
> ```bash
> echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc
> ```

## Usage

```bash
# First run — interactive setup wizard
recon init

# Fetch your daily digest (TUI mode)
recon

# Open in browser sidebar view
recon --browser

# Override keywords for a one-off search
recon --tags vulnerability,zero-day

# Output raw JSON (for piping)
recon --json

# Schedule auto-run every morning
recon schedule set --time 07:00

# Check schedule status
recon schedule status

# Disable auto-run
recon schedule disable

# Re-run setup wizard anytime
recon init
```

## TUI Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Open article in browser |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `q` / `Ctrl+C` | Quit |

## How Scoring Works

Articles are ranked by a multi-signal score:

| Signal | Points |
|--------|--------|
| Keyword in **title** | +10 |
| Keyword in **description** | +5 |
| Contains `CVE-YYYY-NNNNN` | +8 |
| Mentions "zero-day" / "0day" | +10 |
| Mentions "breach" / "leak" | +8 |
| Mentions "critical" / "RCE" | +7 |
| From a high-authority source | +5 |
| Contains anti-keywords (politics) | Score = 0 |

## Auto-Schedule (Linux)

Recon uses systemd user timers with three layers of protection:

1. **`Persistent=true`** — if laptop was off at scheduled time, runs on next boot
2. **Resume hook** — if laptop was sleeping, runs 10s after wake
3. **Missed-run detection** — warns you if >24h since last digest

## Categories

During setup, pick from:

- Vulnerabilities & CVEs
- Malware & Threat Intel
- Zero-Days & Exploits
- Data Breaches & Leaks
- Cloud Security
- Cryptography
- AI & Machine Learning
- Web Development
- DevOps & Infrastructure
- Privacy & Surveillance

## Config

Preferences are saved to `~/.config/recon/config.json`. Edit manually or re-run `recon init`.

## Feed Sources

Some of the 570+ sources include:

**Vulnerability Research:** Qualys, Rapid7, CrowdStrike, Unit 42, Mandiant, Zero Day Initiative, Cisco Talos, Microsoft MSRC, SentinelOne, Check Point Research, Elastic Security Labs, PortSwigger, SANS ISC, Tenable, Volexity

**Advisories:** CISA, NVD

**News:** Krebs on Security, BleepingComputer, The Hacker News, Dark Reading, TechCrunch Security, Ars Technica, The Register, CyberScoop, Hacker News (100+ points)

**Engineering:** Google, Cloudflare, Netflix, Stripe, Shopify, Discord, GitHub, and 400+ more

## Project Structure

```
news-cli/
├── main.go           # CLI entry, Cobra subcommands
├── config.go         # XDG config, categories, timezones
├── fetcher.go        # Concurrent fetcher + multi-signal scorer
├── setup.go          # First-run interactive wizard (huh)
├── schedule.go       # systemd timer management
├── renderer.go       # HTML browser view + ephemeral server
├── tui.go            # Bubble Tea terminal UI
├── links.json        # 570+ embedded feeds
├── Makefile           # build / install / release
├── go.mod
└── go.sum
```

## Tech Stack

- **Language:** Go
- **Feeds:** [gofeed](https://github.com/mmcdole/gofeed)
- **CLI:** [Cobra](https://github.com/spf13/cobra)
- **TUI:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Setup wizard:** [Huh](https://github.com/charmbracelet/huh)
- **Concurrency:** [errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup)

## License

MIT
