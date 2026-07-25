---
id: audit-87130fdc
auditor: audit
severity: medium
category: conventions
area: internal/output
status: open
found: 2026-07-25
---

# Computed statistics reach only some output formats; JSON and the others disagree on what a report contains

## Problem

The four formatters expose four different subsets of `models.Statistics`. Several
computed fields reach exactly one format, and three reach none at all.

## Evidence

Field by field across the formatters:

| Field | text | markdown | html | json |
| --- | --- | --- | --- | --- |
| `PRClosed` | no | no | no | yes (json.go:23) |
| `IssuesOpened` | no | no | no | yes (json.go:24) |
| `IssuesClosed` | no | no | no | yes (json.go:25) |
| `EventsByRepo` (beyond top 15) | no | no | no | no |
| `EventsByDate` | no | no | heatmap only | no |
| `Calendar` | no | no | no | no |
| `Streaks.CurrentStreakStart` | no | no | no | no |
| `Streaks.LongestStreakEnd` | no | no | no | yes (json.go:50) |
| `ActivityRate()` | yes (text.go:57) | no | no | yes (json.go:44) |

`stats.Calendar` is built at stats.go:78-84 with a summed `TotalContributions` and
read by nothing. `CurrentStreakStart` is set at streaks.go:102 and read by nothing —
`json.go:41` emits `"current"` but no corresponding start date, while `"longest"`
gets both ends.

## Impact

`--format` changes what the report *contains*, not just how it looks, which is not
what a format flag implies and is not documented anywhere. A user comparing the
Markdown report against the JSON output sees different issue statistics and
reasonably concludes one is buggy. Issue tracking in particular is invisible in the
three human-readable formats despite being computed for all of them — `IssuesOpened`
and `IssuesClosed` are collected on every run and shown only if you happen to pick
JSON.

## Fix

Define one canonical field set and render all of it in every format, omitting rows
only when a value is genuinely absent (e.g. `Streaks == nil`):

- Add Issues Opened / Issues Closed / PRs Closed rows to `FormatText`
  (text.go:70-78), `FormatMarkdown` (markdown.go:25-29), and `buildHTML`'s card list
  (html.go:310-316).
- Add `"current_start"` to the JSON streaks object beside `"longest_start"`.
- Either emit `stats.Calendar` in the JSON payload and drive the HTML heatmap from
  it, or drop the field — carrying a computed structure no consumer reads is what
  let the heatmap/streak inconsistency go unnoticed.

A shared `[]struct{Label string; Value string}` summary builder consumed by all four
formatters would make future drift structurally impossible.
