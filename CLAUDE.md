# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`gh-history` is a Go CLI tool (designed as a `gh` extension) that analyzes GitHub user activity and generates statistics and visualizations. It fetches data exclusively via the GitHub GraphQL API, computes statistics (streaks, distributions, top repos), and outputs results as text, JSON, Markdown, or interactive HTML reports with Plotly charts.

## Development Commands

```bash
make build              # Build for current platform
make test               # Run full test suite
make lint               # go vet + staticcheck
make test-race          # Tests with race detector
make test-cov           # Tests with coverage
make build-all          # Cross-compile all platforms
make clean              # Remove build artifacts
make all                # lint + test + build
go test ./internal/analysis/...  # Run tests for one package
go run . [username]     # Run locally
```

## Architecture

Entry point: `main.go` (flag-based CLI via `flag.NewFlagSet`).

Source lives in `internal/` with seven packages:

- **api/** — GitHub GraphQL client using `go-gh/v2`. Token resolution via `GH_TOKEN`, `GITHUB_TOKEN`, or `gh auth` config. Pagination via cursor-based GraphQL.
- **analysis/** — `Calculator` processes events into a `Statistics` struct. Streak calculation, event categorization (8 categories), activity rate computation.
- **daterange/** — Date range types and parsing. Supports `--year`, `--last-month`, `--last-90-days`, `--from`/`--to`. Current/future years cap end date to today.
- **ghutil/** — Shared utilities: date format constants, pagination limits, user normalization.
- **models/** — Core data types: `Event`, `Statistics`, `Streaks`, `Category`, `ContributionDay`.
- **output/** — Formatters (text via `go-gh` tableprinter, JSON, Markdown, HTML) and Plotly chart generation. HTML report embeds charts inline.
- **testutil/** — Test helpers and sample data fixtures.

## Code Conventions

- **Go 1.26+** required
- Standard `go vet` and `gofmt` formatting
- All functions return errors explicitly — no panics in library code
- `internal/` package layout — nothing exported outside the module
- `go-gh/v2` for terminal detection, table printing, auth, browser, markdown rendering

## Build & Release

- **Makefile** — primary build interface (`make build`, `make test`, `make release`)
- **`.goreleaser.yaml`** — GoReleaser v2 config: CGO_ENABLED=0, trimpath, cosign-signed checksums
- **`.github/workflows/release.yml`** — triggered on CalVer tags (`20*`), runs GoReleaser
- **`.github/workflows/codeql.yml`** — CodeQL analysis on push/PR to main
- `make release` — tags with `gh calver next`, pushes tag, GitHub Actions builds release

## Testing

- Standard `go test` with table-driven tests
- Test files colocated with source (`*_test.go`)
- Helper function `d(year, month, day)` in daterange tests for concise date construction
