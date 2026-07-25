---
id: audit-5a19a8aa
auditor: audit
severity: high
category: correctness
area: internal/analysis/stats.go
status: open
found: 2026-07-25
---

# Empty-events early return discards calendar days and commit totals, zeroing private-repo users' reports

## Problem

`Calculate` returns early when `len(events) == 0`, before the blocks that filter
`CalendarDays`, build `stats.Calendar`, and apply `TotalCommitContributions`. Both
of those inputs are independent of the public event list — they come straight from
the GraphQL `contributionsCollection` and include private-repository activity.

## Evidence

`internal/analysis/stats.go:31-35`:

    if len(events) == 0 {
        streaks := CalculateStreaks(nil, c.DateRange.Start, c.DateRange.End)
        stats.Streaks = &streaks
        return stats
    }

Executed against the real code, same Calculator, only the event slice differs:

    no events + 3 calendar days + 42 commits => ActiveDays=0 LongestStreak=0 CommitCount=0 Calendar==nil:true
    1 event    + same calendar               => ActiveDays=3 LongestStreak=3 CommitCount=42 Calendar==nil:false

## Impact

A user whose activity is entirely in private repositories — the exact case the
calendar path exists to cover, per the comment at stats.go:63 "Prefer
calendar-based streaks (includes private repos)" — receives an all-zeros report:
0 active days, 0 longest streak, 0 commits, no calendar. Adding one single public
event flips the same data to a correct 3/3/42. The report is silently wrong for the
population it was designed to serve.

## Fix

Delete the early return. The remaining code already handles an empty event slice
correctly: the `for _, event := range events` loop is a no-op, `filteredDays`
falls back to `CalculateStreaks(events, ...)` when the calendar is also empty, and
the maps are already initialised.

    func (c *Calculator) Calculate(events []models.Event) models.Statistics {
        stats := models.Statistics{ ... }   // unchanged

        for _, event := range events { ... }   // no-op when empty

        // filter calendar, choose streak source, apply TotalCommitContributions,
        // build stats.Calendar — all as today, unconditionally
    }

Add a test asserting that a Calculator with calendar days and zero events still
reports the calendar-derived streaks and the GraphQL commit total.
