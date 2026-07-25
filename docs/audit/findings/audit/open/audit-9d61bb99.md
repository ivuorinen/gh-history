---
id: audit-9d61bb99
auditor: audit
severity: medium
category: correctness
area: internal/output/html.go
status: open
found: 2026-07-25
---

# HTML report's heatmap and streak cards are computed from different data sources and contradict each other

## Problem

Within one report, the "Active Days" / "Longest Streak" / "Current Streak" cards are
derived from GitHub's contribution calendar (which includes private repositories),
while the "Contribution Heatmap" is derived from `stats.EventsByDate` (public
synthesized events only). The two panels describe the same period using different
universes of data.

## Evidence

`internal/analysis/stats.go:63-70` selects the calendar as the streak source when
available:

    // Prefer calendar-based streaks (includes private repos)
    if len(filteredDays) > 0 {
        streaks := CalculateStreaksFromCalendar(filteredDays, ...)

`internal/output/html.go:411-413` builds the heatmap from the event map instead:

    func buildHeatmapData(stats models.Statistics) (string, error) {
        if len(stats.EventsByDate) == 0 { return "", nil }

`stats.EventsByDate` is populated only from the event loop at stats.go:42, which
sees only the PR/issue/review/repo/comment events synthesized in
`FetchContributions`. `stats.Calendar` — which holds per-day counts for the same
window, including private activity — is built at stats.go:78-84 and then never read
by any formatter.

## Impact

A user with substantial private-repo activity sees, in the same report, "Active
Days: 180 / 365" beside a heatmap that is mostly empty. The two numbers cannot both
be right and nothing in the report explains the discrepancy. The heatmap is the most
visually prominent element of the HTML output, so the wrong one dominates the
reader's impression. `stats.Calendar` is computed and carried through for no
consumer at all.

## Fix

Feed the heatmap from the calendar when it is present, falling back to events only
when it is not — mirroring the choice `Calculate` already makes for streaks:

    func buildHeatmapData(stats models.Statistics) (string, error) {
        counts := map[string]int{}
        if stats.Calendar != nil && len(stats.Calendar.Days) > 0 {
            for _, d := range stats.Calendar.Days {
                counts[d.Date.Format(ghutil.DateFormat)] = d.ContributionCount
            }
        } else {
            counts = stats.EventsByDate
        }
        // ... existing sort/grid construction over `counts`
    }

Label the chart to say which source it used, so the two panels are never silently
inconsistent again.
