---
id: audit-34072b6a
auditor: audit
severity: low
category: maintainability
area: internal/output/html.go
status: open
found: 2026-07-25
---

# HTML report charts have no accessible alternative, duplicate a heading, and omit landmarks

## Problem

The generated report is the tool's primary visual output, and its accessibility
layer is thin: chart containers are unlabelled empty divs until JavaScript runs, one
heading is duplicated verbatim, and the page has no landmark structure.

## Evidence

- Chart containers, e.g. `html.go:155`, `:178`, `:204`, `:225`, `:251`:

      <div class="chart" id="chart-categories"></div>

  No `role`, no `aria-label`, no text alternative. Four of the five charts —
  Activity Distribution, by Day of Week, by Hour, and the Heatmap — have no
  equivalent data anywhere else in the document, so their content is unavailable to
  a screen-reader user. Only "Top Repositories" has a table equivalent
  (html.go:274-281).
- Duplicate heading, `html.go:250` and `html.go:273`:

      <h2 class="chart-title">Top Repositories</h2>
      ...
      <h2 class="chart-title">Top Repositories</h2>

  Two sibling sections carry the identical accessible name, so heading-list
  navigation cannot distinguish the chart from the table.
- No `<main>`, `<section>`-with-name, or any landmark; content sits in a bare
  `<div class="container">` (html.go:135).

Colour contrast does pass: `--text-secondary` #8b949e on `--bg-secondary` #161b22
is 5.6:1 and `--accent` #58a6ff on the same is 6.9:1, both above the WCAG 2.2 AA
thresholds. `<html lang="en">` is set correctly (html.go:62).

## Impact

WCAG 2.2 AA 1.1.1 (Non-text Content) is not met for four of five charts. A report
generated to share activity data — the stated purpose of the HTML format — conveys
none of its distribution or heatmap data to assistive technology.

## Fix

1. Give each chart container an accessible name and a summary:

       <div class="chart" id="chart-weekly" role="img"
            aria-label="Activity by day of week: Monday 30 events, Tuesday 25, ..."></div>

   The `BuildWeekdayBars` / `BuildHourlyBars` / `BuildCategoryBars` helpers in
   `barchart.go` already produce exactly these label/count pairs for the text
   formatter — reuse them to build the label string rather than duplicating the
   logic.
2. Rename the second heading to "Top Repositories (table)", or merge the chart and
   table into one `<section>` under a single heading.
3. Wrap the content in `<main>` and give each `.chart-section` an
   `aria-labelledby` pointing at its `<h2>`.
