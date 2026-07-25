---
id: audit-e18a420f
auditor: audit
severity: medium
category: maintainability
area: internal/ghutil/ghutil.go
status: open
found: 2026-07-25
---

# Dead helpers left over from a removed caching layer are still exported and tested

## Problem

Several exported helpers have no non-test caller anywhere in the module. Because
everything lives under `internal/`, nothing outside the module can use them either,
so they are unambiguously dead — not "public API kept for consumers".

## Evidence

Grep across all non-test `.go` files finds only the declarations themselves:

    ghutil.ParseRFC3339Fallback  — declared ghutil.go:27, no caller
    ghutil.ParseDateFallback     — declared ghutil.go:42, no caller
    ghutil.NormalizeUser         — declared ghutil.go:53, no caller
    daterange.Days()             — declared daterange.go:55, callers: tests only
    daterange.Contains()         — declared daterange.go:60, callers: tests only
    daterange.Overlaps()         — declared daterange.go:66, called only by Subtract
    daterange.Subtract()         — declared daterange.go:71, callers: tests only
    testutil.SampleCacheEvents() — declared testutil.go:131, no caller at all

The provenance is documented in the code itself. `ghutil.go:40-41`:

    // ParseDateFallback parses a date string that may be either "2006-01-02" or
    // an RFC3339 timestamp (as returned by modernc.org/sqlite DATE handling).

`modernc.org/sqlite` is not in `go.mod` and never appears in the source.
`NormalizeUser`'s comment says "for consistent cache key usage" (ghutil.go:52) and
`SampleCacheEvents` says "for cache tests" (testutil.go:130) — there is no cache
package and no cache tests.

## Impact

Roughly 60 lines of source plus their tests describe a persistence layer that does
not exist. The comments actively mislead: a reader encountering `ParseDateFallback`
reasonably concludes this project has a SQLite cache and goes looking for it. The
tests covering these functions inflate `ghutil` to 94.4% coverage, which overstates
how much of the *live* code is verified. `daterange.Subtract`/`Overlaps`/`Contains`
are range-algebra primitives that only a caching layer (computing which sub-ranges
still need fetching) would need.

## Fix

Delete `ParseRFC3339Fallback`, `ParseDateFallback`, `NormalizeUser`,
`SampleCacheEvents`, and `DateRange.Subtract`/`Overlaps`/`Contains`, together with
their tests. Keep `DateRange.Days()` only if a formatter is wired to it — otherwise
delete it too; `calculateStreaksFromDates` computes its own `totalDays` inline
(streaks.go:46) rather than calling it, which is itself duplication worth collapsing
onto `Days()` if the method stays.

Reinstate any of these when the cache actually lands; recovering them from git
history is cheaper than carrying misleading dead code.
