---
id: audit-0572eff0
auditor: audit
severity: high
category: correctness
area: internal/daterange/daterange.go
status: open
found: 2026-07-25
---

# Year() skips validation, so a future --year yields an inverted range and a silently empty report

## Problem

`Year(year)` builds its `DateRange` with a struct literal instead of going through
`New()`, so the `start.After(end)` check never runs. When the requested year is in
the future, `end` is capped to today while `start` stays at January 1 of that year,
producing an inverted range. `ParseDateRange` returns it with a nil error.

## Evidence

`internal/daterange/daterange.go:28-36` returns `DateRange{Start: start, End: end}`
directly; `New()` at line 17 is the only path that validates.

Executed against the real code (today = 2026-07-25):

    Year(2030) => Start=2030-01-01 End=2026-07-25 inverted=true Days=-1255
    splitIntoYearChunks => 0 chunks
    ParseDateRange(year=2030) => 2030-01-01..2026-07-25 err=<nil>

`splitIntoYearChunks` (main.go:281) loops while `chunkStart.Before(dr.End)`, which is
false immediately, so zero chunks are produced and `FetchContributions` is never
called. The user gets a report with 0 events and a negative `TotalDays`
(`calculateStreaksFromDates` at streaks.go:46 computes `end.Sub(start)` on the
inverted range).

## Impact

`gh history --year 2030` (or any typo'd future year) prints a clean-looking,
completely empty report with a nonsensical negative day count instead of rejecting
the input. The failure is silent — no error, no warning, exit code 0.

## Fix

Route `Year()` through the validating constructor and surface the error:

    func Year(year int) (DateRange, error) {
        start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
        end := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
        today := ghutil.TruncateToDay(ghutil.NowUTC())
        if start.After(today) {
            return DateRange{}, fmt.Errorf("year %d is in the future", year)
        }
        if end.After(today) {
            end = today
        }
        return New(start, end)
    }

Update `ParseDateRange` (line 121-123) to propagate the error. `LastMonth()` and
`LastNDays()` should likewise return `New(...)` results so no constructor can
produce an unvalidated range.
