---
id: audit-856a2b2f
auditor: audit
severity: medium
category: docs
area: README.md
status: open
found: 2026-07-25
---

# README advertises release tracking the tool cannot produce

## Problem

The Features list promises activity types that no code path emits.

## Evidence

`README.md:7`:

    - **Activity Tracking** — Commits, pull requests, issues, reviews, releases, and more

`api.FetchContributions` synthesizes exactly five event types — `PullRequestEvent`,
`IssuesEvent`, `PullRequestReviewEvent`, `CreateEvent`, `IssueCommentEvent`
(client.go:407-498) — plus `IssueCommentEvent` from `FetchIssueComments`. No code
anywhere produces `ReleaseEvent`. Consequently `models.CategoryReleases` is mapped
in `EventCategories` (categories.go:17) and rendered in `AllCategories`
(text.go:23), but its count is structurally always zero, so
`BuildCategoryBars` skips it (barchart.go:43-45) and "Releases" never appears in any
report.

"and more" is similarly unbacked: forks, stars, and watches are all mapped but never
produced.

## Impact

A user installs the extension specifically to count their releases, runs it, sees no
Releases row, and reasonably concludes the tool is broken or that they published
nothing. The documentation describes an event-type surface (the REST Events API)
that this GraphQL implementation does not have.

## Fix

Correct the claim to what the GraphQL contributions API actually returns:

    - **Activity Tracking** — Commits, pull requests, issues, reviews, comments, and repository creation

If release tracking is wanted, it needs implementing — GraphQL exposes releases via
`user.repositories.releases`, not via `contributionsCollection`, so it would be a
separate query rather than a mapping tweak. Also document that commit counts come
from `totalCommitContributions` (which includes private repositories) while the
per-repository breakdown covers public activity only.
