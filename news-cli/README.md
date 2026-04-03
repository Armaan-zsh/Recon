# Recon: Intelligence Discovery Engine

Recon is a high-performance, terminal-based intelligence discovery engine that aggregates, scores, and maps technical security data from over 2,500 elite feeds.

## Core Features

- **Zero-Latency Persistence**: Instant startup (0.02s) using a local-first SQLite WAL-mode architecture.
- **The Motherlode**: Ingests 2,500+ technical RSS feeds including research labs, threat intel blogs, and niche advisories.
- **Dragnet Engine**: Real-time boolean tracking of Google News RSS for emerging zero-days and supply chain attacks.
- **Nexus Evolution**: ASCI threat mapping (press `X`) to visualize relational history between entities and incidents.
- **Stale-While-Revalidate**: Automatic background synchronization updates the nexus while you read, with zero layout shift.

## Installation

Requires Go 1.24+:

```bash
git clone https://github.com/spacecadet/recon.git
cd recon
go build -o recon .
mv recon ~/.local/bin/
```

## Usage

```bash
# Initialize the configuration
recon init

# Launch the interactive researcher TUI
recon

# Launch the browser-based sidebar view
recon --browser

# Output raw JSON for pipeline integration
recon --json
```

## TUI Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move selected article down |
| `k` / `↑` | Move selected article up |
| `o` | Open selected article in default browser |
| `x` | Toggle Nexus Evolution (ASCII relationship graph) |
| `q` / `Ctrl+C` | Exit Recon |

## Technical Architecture

- **Database**: SQLite with Write-Ahead Logging (WAL) for high-concurrency background ingestion.
- **Concurrency**: 500-worker parallel fetcher with adaptive scoring.
- **UI Framework**: Built with Bubble Tea, Lipgloss, and Cobra.

## License

GPL-3.0
