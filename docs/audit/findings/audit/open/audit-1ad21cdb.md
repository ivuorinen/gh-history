---
id: audit-1ad21cdb
auditor: audit
severity: medium
category: tests
area: internal
status: open
found: 2026-07-25
---

# Coverage holes on live paths: models at 0%, main at 7.2%, FetchIssueComments untested

## Problem

Measured coverage is concentrated in the pure-computation packages while several
paths that shape user-visible output have none.

## Evidence

    $ go test -cover ./...
    github.com/ivuorinen/gh-history                    coverage:  7.2% of statements
    github.com/ivuorinen/gh-history/internal/analysis  coverage: 90.7%
    github.com/ivuorinen/gh-history/internal/api       coverage: 56.7%
    github.com/ivuorinen/gh-history/internal/daterange coverage: 73.2%
    github.com/ivuorinen/gh-history/internal/ghutil    coverage: 94.4%
    github.com/ivuorinen/gh-history/internal/models    coverage:  0.0% of statements
    github.com/ivuorinen/gh-history/internal/output    coverage: 90.5%

Specific untested behaviour:

- `internal/models` has no test file at all. `TopRepos` (tie-breaking and the
  `n`-truncation boundary), `PRMergeRate`, `IssueCloseRate`, `ActivityRate`, and
  `Event.Date()` are all unverified. These feed every formatter.
- `FetchIssueComments` (client.go:617-664) has no test — not its date filtering, not
  its pagination, not its error path. It is the source of the entire Comments
  category.
- `main` covers only `splitIntoYearChunks` (main_test.go). `parseFlags`,
  `resolveUser`, `fetchEvents` deduplication, and `writeOutput`'s format dispatch —
  including the `default: fatal("unknown format %q")` branch — are untested.
- `paginateGQL`'s error path is untested, which is how the silent-truncation bug
  filed separately survived.

## Impact

Three of the defects in this audit sit squarely in these gaps: the empty-events
calendar discard (analysis, in a branch no test enters), the `fmtInt` overflow
(output, no case above 999,999), and the pagination error swallow (api, error path
never exercised). Coverage is high where bugs are cheap and absent where they are
expensive.

## Fix

Add, in priority order:

1. `internal/models/models_test.go` — table-driven cases for `TopRepos` with fewer
   than / exactly / more than `n` repos, and the three rate helpers at zero
   denominator (they route through `SafeDiv`, so assert 0 rather than NaN).
2. `TestFetchIssueComments` in `internal/api` using the existing `mockGQLClient`:
   in-range and out-of-range `createdAt`, a `hasNextPage` walk, and a mid-walk error
   asserting that partial events are returned alongside the error.
3. `TestParseFlags` and a `writeOutput` format-dispatch test in `main`, extracting
   the `fatal` call behind a variable so the unknown-format branch is assertable.

Wire `make test` into CI (filed separately) so these actually gate merges.
