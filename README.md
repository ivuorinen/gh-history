# gh-history

A GitHub CLI extension that analyzes user activity and generates statistics and visualizations for any timeframe.

## Features

- **Activity Tracking** — Commits, pull requests, issues, code reviews, comments, and repository creation
- **Statistics** — Streaks, event distributions, top repositories, activity patterns
- **Multiple Formats** — Text, JSON, Markdown, and interactive HTML reports with Plotly charts
- **Flexible Date Ranges** — Query any timeframe with year, month, and custom date range options

Commit counts and streaks come from GitHub's contribution calendar, so they
include private repositories. The per-repository breakdown covers public
activity only.

## Installation

```bash
gh extension install ivuorinen/gh-history
```

Or build from source:

```bash
git clone https://github.com/ivuorinen/gh-history.git
cd gh-history
make build
```

## Usage

```bash
# Defaults to the authenticated user if no username is given
gh history [username] [options]
```

### Date ranges

```bash
gh history --year 2025
gh history --last-month
gh history --last-90-days
gh history --from 2024-01-01 --to 2024-12-31
```

With no date flags, the last 90 days are used. Date range options are mutually
exclusive, and a year that has not started yet is rejected.

### Output formats

```bash
gh history octocat --format text
gh history octocat --format json
gh history octocat --format markdown        # default
gh history octocat --format html            # writes a file and opens it in your browser
gh history octocat --format json -o stats.json
```

Markdown and JSON are written verbatim, so they pipe and redirect cleanly; only
`text` adapts its layout to the terminal.

The four formats agree on every shared statistic — the summary never differs
between them. **JSON is the full offering**: on top of that summary it carries
detail the human-readable formats deliberately omit.

|                                                          | text / markdown / html | json                  |
|----------------------------------------------------------|------------------------|-----------------------|
| Summary (events, commits, PRs, issues, reviews, streaks) | yes                    | yes                   |
| Top 15 repositories                                      | yes                    | yes                   |
| Category, weekday and hour distributions                 | yes                    | yes                   |
| Full per-repository event counts                         | —                      | `events_by_repo`      |
| Per-day event counts                                     | heatmap only           | `events_by_date`      |
| Contribution calendar, per day                           | heatmap only           | `calendar`            |
| GitHub's own totals, private repos included              | —                      | `contribution_totals` |
| Per-repository commit counts, private repos included     | —                      | `commits_by_repo`     |
| The event list, with titles, numbers and review states   | —                      | `events`              |

`contribution_totals` and `commits_by_repo` come straight from GitHub and count
private-repository activity, so they are normally higher than the event-derived
figures under `summary`, which can only see public events.

`--format html` always writes to a file and opens it in your default browser. With
no `--output` the file is `<username>-report.html` in the current directory; with
`--output` a `.html` suffix is appended if missing. It is the only format that
cannot write to stdout.

### Options

| Flag             | Short | Description                                     |
|------------------|-------|-------------------------------------------------|
| `--from`         | `-f`  | Start date (YYYY-MM-DD)                         |
| `--to`           | `-t`  | End date (YYYY-MM-DD)                           |
| `--year`         | `-y`  | Full year shorthand                             |
| `--last-month`   |       | Previous calendar month                         |
| `--last-90-days` |       | Last 90 days                                    |
| `--output`       | `-o`  | Output file path                                |
| `--format`       |       | `text`, `json`, `markdown` (default), or `html` |
| `--verbose`      | `-v`  | Progress and diagnostics (written to stderr)    |
| `--version`      |       | Show version                                    |

## Authentication

`gh-history` uses your existing GitHub CLI authentication. No separate setup is needed.

```bash
gh auth login
```

Token resolution order: `GH_TOKEN` env var, `GITHUB_TOKEN` env var, `gh auth` config.

## Development

```bash
make build          # Build for current platform
make test           # Run tests
make lint           # Run go vet + staticcheck
make test-race      # Run tests with race detector
make test-cov       # Run tests with coverage
make build-all      # Cross-compile for all platforms
make clean          # Remove build artifacts
```

## Releasing

Releases use [CalVer](https://calver.org/) tags and are built automatically by GitHub Actions with [GoReleaser](https://goreleaser.com/). Binaries are signed with [cosign](https://github.com/sigstore/cosign).

```bash
make release        # Tag and push a new CalVer release (requires clean main branch)
```

## Contributing

```bash
make all            # Runs lint, test, and build
```

## License

MIT
