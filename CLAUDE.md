# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Role

You are a Go engineer maintaining `gh-history`: keep each change small, and keep
every reported figure traceable to the data GitHub returned.

## Project

`gh-history` is a Go CLI tool, designed as a `gh` extension, that analyzes a GitHub
user's activity. It fetches data only through the GitHub GraphQL API and computes
streaks, distributions and top repositories. It writes the results as text, JSON,
Markdown, or an interactive HTML report with Plotly charts.

## Development Commands

```bash
mise install             # Install Go (from go.mod), staticcheck, govulncheck, gh
mise run build           # Build for current platform
mise run test            # Run full test suite
mise run lint            # gofmt check + go vet + staticcheck
mise run vuln            # govulncheck
mise run test-race       # Tests with race detector
mise run test-cov        # Tests with coverage
mise run build-all       # Cross-compile all platforms
mise run docs            # Regenerate docs/json.md from examples/report.json
mise run clean           # Remove build artifacts
mise run update          # go get -u, go mod tidy, bump mise.toml tools
mise run all             # lint + test-race + vuln + build (the CI gates)
go test ./internal/analysis/...  # Run tests for one package
go run . [username]     # Run locally
```

## Architecture

Entry point: `main.go` (flag-based CLI via `flag.NewFlagSet`).

`parseFlags` loops over `fs.Parse` rather than calling it once: Go's `flag` package
stops at the first non-flag argument, so a single call silently drops every flag
written after the positional username (`gh history octocat --format json`). Keep the
loop, and keep `TestParseFlags`'s case for flags written after the username.

Source lives in `internal/` with seven packages:

- **api/** — GitHub GraphQL client over `net/http` (`graphQLClient`). Host and token are resolved in `main` (`resolveHost`/`resolveToken`) via `--hostname`/`GH_HOST` and the token env vars or `gh auth token`. Pagination via cursor-based GraphQL.
- **analysis/** — `Calculator` processes events into a `Statistics` struct. Streak calculation, event categorization into the six values of the `Category` type in `internal/models`, activity rate computation.
- **daterange/** — Date range types and parsing. Supports `--year`, `--last-month`, `--last-90-days`, `--from`/`--to`. The current year and a `--to` after today are capped to today; a future `--year` and a start before 2008 are rejected.
- **ghutil/** — Shared utilities: the date format constant, `TruncateToDay`, `SafeDiv`, and `WriteOwnerOnly`, which every report file is written through.
- **models/** — Core data types: `Event`, `Statistics`, `Streaks`, `Category`, `ContributionDay`.
- **output/** — Formatters (text via the in-tree `table`, JSON, Markdown, HTML) and Plotly chart generation. Terminal detection lives in `terminalOut`. HTML report embeds charts inline.
- **testutil/** — Test helpers and sample data fixtures.

`tools/jsondoc` writes `docs/json.md`, the JSON field reference, from
`examples/report.json`. Each key's type and meaning live in its `fields` list, and
the run fails without writing when the report and the list disagree. After a JSON
key is added, renamed or removed, regenerate `examples/report.json`, make `fields`
match it (add the new key, delete the old one), then run `mise run docs`.

## Code Conventions

- **Go 1.26.5+** required (`go.mod` is the single source of truth: mise reads its
  `toolchain` line through `idiomatic_version_file_enable_tools`, and the release job
  reads it via `go-version-file`, so bump it there and nowhere else)
- Standard `go vet` and `gofmt` formatting
- All functions return errors explicitly — no panics in library code
- `internal/` package layout — nothing exported outside the module
- Two sets of constraints live in `.claude/rules/`, which Claude Code loads with this
  file: `dependencies.md` (no go-gh) and `github-api.md` (the `Time-Zone` header, the
  inclusive `to`, pagination, the 100-repository limit).
- Markdown and JSON print verbatim, ending in exactly one newline whether they go to
  stdout or `--output`; only `text` adapts to the terminal.

## Build & Release

- **`mise.toml`** — primary build interface: tasks and pinned tool versions
- **`mise.toml` tool versions** — Renovate's mise manager bumps them
- **staticcheck and govulncheck** — `go:` backend, built with the go.mod toolchain
- **`.github/workflows/ci.yml`** — `jdx/mise-action`, then the same `mise run` tasks
- **`.goreleaser.yaml`** — GoReleaser v2 config: CGO_ENABLED=0, trimpath, cosign-signed checksums
- **`.github/workflows/release.yml`** — triggered on CalVer tags (`20*`), runs GoReleaser
- **`.github/workflows/codeql.yml`** — CodeQL analysis on push/PR to main
- `mise run release` — tags with `gh calver next`, pushes tag, GitHub Actions builds release

## Testing

- Standard `go test` with table-driven tests
- Test files colocated with source (`*_test.go`)
- Helper function `d(year, month, day)` in daterange tests for concise date construction
