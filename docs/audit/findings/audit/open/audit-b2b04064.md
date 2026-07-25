---
id: audit-b2b04064
auditor: audit
severity: low
category: correctness
area: internal/output/html.go
status: open
found: 2026-07-25
---

# Heatmap renders a trailing empty week beyond the data range

## Problem

The week-grid loop extends a full week past the last day that has data, so the
rightmost column of the heatmap is always empty.

## Evidence

`internal/output/html.go:455`:

    for !current.After(end.AddDate(0, 0, 7)) {

`end` is already the latest date present in `stats.EventsByDate` (line 437). Adding
seven days guarantees at least one — and depending on weekday alignment, two —
iterations past any date that can have a count, since `dateMap` lookups for those
days return the zero value (line 460).

## Impact

Cosmetic but visible: every generated report shows a trailing blank column, and the
x-axis carries a week label ("Jan 02" style, line 456) for a week containing no
data. On a short range — `--last-month` produces four to five columns — one blank
column out of five is a noticeable distortion of the chart's shape.

## Fix

Iterate to the end of the week containing `end`, not a week beyond it:

    for !current.After(end) {

`current` advances a week at a time from a Monday-aligned start (lines 452-453), so
the loop already emits the full week that contains `end`; the `AddDate(0, 0, 7)`
adds an extra one on top of that.
