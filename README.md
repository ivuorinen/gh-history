# gh-history

A GitHub CLI extension that analyzes user activity and generates statistics and visualizations for any timeframe.

## Features

- **Activity Tracking** — Commits, pull requests, issues, reviews, releases, and more
- **Statistics** — Streaks, event distributions, top repositories, activity patterns
- **Multiple Formats** — Text, JSON, Markdown, and interactive HTML reports with Plotly charts
- **Flexible Date Ranges** — Query any timeframe with year, month, and custom date range options

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

### Output formats

```bash
gh history octocat --format text
gh history octocat --format json
gh history octocat --format markdown        # default
gh history octocat --format html             # generates and opens an interactive report
gh history octocat --format json -o stats.json
```

### Additional options

```bash
gh history octocat --verbose         # show progress and debug info
gh history --version                 # show version
```

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
