# gh-history

A GitHub CLI extension that analyzes user activity and generates statistics and visualizations for any timeframe.

## Features

- **Activity Tracking** — Commits, pull requests, issues, code reviews, comments, and repository creation
- **Statistics** — Streaks, event distributions, top repositories, activity patterns
- **Multiple Formats** — Text, JSON, Markdown, and interactive HTML reports with Plotly charts
- **Flexible Date Ranges** — Query any timeframe with year, month, and custom date range options

Commit counts come from GitHub's own contribution totals and streaks from its
contribution calendar. Everything else (pull requests, issues, reviews, comments
and the per-repository breakdown) covers every contribution your token can see.
When you report on yourself with a token that has access, that includes private
repositories: their names can appear in any format (text, Markdown and HTML list
the top 15 repositories, JSON lists all of them), and their pull request and
issue titles appear in JSON. Check a report before publishing it.

## Installation

```bash
gh extension install ivuorinen/gh-history
```

Or build from source:

```bash
git clone https://github.com/ivuorinen/gh-history.git
cd gh-history
mise install
mise run build
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
exclusive, and a year that has not started yet is rejected. A start date before
2008 is rejected, and an end date after today is treated as today.

### Time zone

Every day and hour in a report is counted in one time zone: `--time-zone` if
given, otherwise `TZ` if it names an IANA zone, otherwise UTC. The date flags,
"today", GitHub's contribution calendar, the streaks and the weekday and hour
charts all use it, and every format records it next to the date range.

```bash
gh history --last-month --time-zone Europe/Berlin
TZ=America/New_York gh history --last-month
```

`--time-zone` takes an IANA name such as `Europe/Berlin` or `UTC`, and anything
else is an error. A `TZ` that is not an IANA name, such as the POSIX form
`CET-1CEST,M3.5.0`, falls back to UTC with a warning. The machine's own zone
setting is not read, so the same command gives the same report on every machine.

### Output formats

```bash
gh history octocat --format text
gh history octocat --format json
gh history octocat --format markdown        # default
gh history octocat --format html            # writes a file and opens it in your browser
gh history octocat --format json -o stats.json
```

Markdown and JSON are written verbatim, so they pipe and redirect cleanly; only
`text` adapts its layout to the terminal. `--format text -o FILE` writes the
non-terminal, tab-separated layout.

[`examples/`](examples/) has one report in each format for the same month.

The four formats agree on every shared statistic — the summary never differs
between them. **JSON is the full offering**: on top of that summary it carries
detail the human-readable formats deliberately omit.

|                                                          | text / markdown / html | json                  |
|----------------------------------------------------------|------------------------|-----------------------|
| Summary (events, commits, PRs, issues, reviews, streaks) | yes                    | yes                   |
| Repositories by event count, marked public or private    | top 15                 | all, `top_repos`      |
| Category, weekday and hour distributions                 | yes                    | yes                   |
| Per-day event counts                                     | heatmap only           | `events_by_date`      |
| Contribution calendar, per day                           | heatmap only           | `calendar`            |
| GitHub's own totals, private repos included              | —                      | `contribution_totals` |
| Per-repository commit counts, private repos included     | —                      | `commits_by_repo`     |
| The event list, with titles, numbers and review states   | —                      | `events`              |
| When the report was made                                 | —                      | `generated_at`        |

[`docs/json.md`](docs/json.md) lists every JSON key with its type and meaning.

`contribution_totals` and `commits_by_repo` are GitHub's own counters, and
`summary.commits` is the same commit total. The other figures under `summary`
are counted from the contributions the token can list. Reporting on yourself
with full access, the two agree. They diverge when the token cannot see some of
the user's repositories.

GitHub lists at most 100 repositories in one query of per-repository commit
counts. When a period's list is cut off at 100 and falls short of its commit
total, gh-history fetches the period again in shorter periods, down to single
days, which recovers the repositories the limit left out. Whenever
`commits_by_repo` still adds up to less than `summary.commits`, gh-history
prints a warning on stderr. A single day with commits in more than 100
repositories is one cause: it cannot be split further.

Every repository is marked public or private. Text, Markdown and HTML show it as
a Visibility column in the top-repositories table; JSON carries `private` on
each `top_repos` and `commits_by_repo` entry and `repo_private` on each event.
The flag is GitHub's `isPrivate`, so an internal repository on GitHub Enterprise
shows as private.

JSON also gives every repository its GitHub database ID: `id` on each
`top_repos` and `commits_by_repo` entry and `repo_id` on each event. Unlike the
name, the ID stays the same when a repository is renamed, and looking it up
(`GET /repositories/{id}`) without access returns 404. So it can label a private
repository the same way in every report without naming it.

The summary's PR merge rate and issue close rate are the share of the PRs and
issues opened in the period that were merged or closed by the time the report
ran. GitHub lists PR and issue contributions by when the item was opened, so
only items opened in the period are collected: a PR or issue opened before the
period and merged or closed during it is not counted, neither in the rates nor
under "PRs Merged" and "Issues Closed". JSON carries each PR's and issue's state
at that time as `subject_state` on its events.

Every other summary figure counts what happened inside the period. The two rates
also depend on what happened after it, so fetching the same period again later
can change them. JSON records when the report was made as `generated_at`, an
RFC 3339 timestamp in the report's time zone.

`--format html` always writes to a file and opens it in your default browser. With
no `--output` the file is `<username>-report.html` in the current directory; with
`--output` a `.html` suffix is appended if missing. It is the only format that
cannot write to stdout.

### Options

| Flag             | Short | Description                                       |
|------------------|-------|---------------------------------------------------|
| `--username`     | `-u`  | User to report on (same as the positional form)   |
| `--hostname`     |       | GitHub host, for GitHub Enterprise                |
| `--from`         | `-f`  | Start date (YYYY-MM-DD)                           |
| `--to`           | `-t`  | End date (YYYY-MM-DD)                             |
| `--year`         | `-y`  | Full year shorthand                               |
| `--last-month`   |       | Previous calendar month                           |
| `--last-90-days` |       | Last 90 days                                      |
| `--time-zone`    |       | IANA zone days are counted in (default `TZ`, UTC) |
| `--output`       | `-o`  | Output file path                                  |
| `--format`       |       | `text`, `json`, `markdown` (default), or `html`   |
| `--verbose`      | `-v`  | Progress and diagnostics (written to stderr)      |
| `--version`      |       | Show version                                      |

Flags may appear before or after the username. Giving the username both
positionally and with `--username` is an error unless they match, as is giving
more than one username.

### GitHub Enterprise

```bash
gh history --hostname github.example.com --username octocat
```

`--hostname` sets both the API host and the host the token is looked up for, so
`GH_ENTERPRISE_TOKEN` / `GITHUB_ENTERPRISE_TOKEN` and the matching `hosts.yml`
entry are used. Without it the host comes from `GH_HOST`, then your `gh` config,
then `github.com`.

## Authentication

`gh-history` uses your existing GitHub CLI authentication. No separate setup is needed.

```bash
gh auth login
```

Token resolution order: `GH_TOKEN` env var, `GITHUB_TOKEN` env var, then `gh auth
token` (which covers the `gh` config file and any system keyring). For a GitHub
Enterprise host, `GH_ENTERPRISE_TOKEN` and `GITHUB_ENTERPRISE_TOKEN` are used
instead, so a github.com token is never sent to an internal instance.

## Environment

| Variable                                         | Effect                                                                 |
|--------------------------------------------------|------------------------------------------------------------------------|
| `GH_HOST`                                        | Host to query when `--hostname` is not given                           |
| `GH_TOKEN`, `GITHUB_TOKEN`                       | Token for github.com and `*.ghe.com`, before `gh auth token`           |
| `GH_ENTERPRISE_TOKEN`, `GITHUB_ENTERPRISE_TOKEN` | Token for a GitHub Enterprise Server host                              |
| `GH_CONFIG_DIR`, `XDG_CONFIG_HOME`               | Where `hosts.yml` is read from (default `~/.config/gh`)                |
| `BROWSER`                                        | Command that opens the HTML report instead of the platform opener      |
| `GH_FORCE_TTY`                                   | Force the terminal `text` layout; a value of `100` or `80%` sets width |
| `TZ`                                             | Time zone days are counted in when `--time-zone` is not given          |

## Development

Tasks and tool versions live in `mise.toml`; Go's version comes from `go.mod`. Install [mise](https://mise.jdx.dev/), then:

```bash
mise install            # Install Go, staticcheck, govulncheck and gh
mise run build          # Build for current platform
mise run test           # Run tests
mise run lint           # Run gofmt check, go vet + staticcheck
mise run vuln           # Run govulncheck
mise run test-race      # Run tests with race detector
mise run test-cov       # Run tests with coverage
mise run build-all      # Cross-compile for all platforms
mise run docs           # Regenerate docs/json.md from examples/report.json
mise run clean          # Remove build artifacts
mise run update         # Update Go modules and mise tools to their latest versions
mise tasks              # List every task
```

## Releasing

Releases use [CalVer](https://calver.org/) tags and are built automatically by GitHub Actions with [GoReleaser](https://goreleaser.com/). Binaries are signed with [cosign](https://github.com/sigstore/cosign).

```bash
mise run release        # Tag and push a new CalVer release (requires clean main branch)
```

## Contributing

```bash
mise run all            # Runs lint, race-enabled tests, govulncheck and build: the same gates as CI
```

## License

MIT
