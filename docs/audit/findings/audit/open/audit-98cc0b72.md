---
id: audit-98cc0b72
auditor: audit
severity: medium
category: correctness
area: internal/output/text.go
status: open
found: 2026-07-25
---

# fmtInt produces malformed thousands separators above 999,999

## Problem

`fmtInt` inserts a single separator by dividing by 1000 once. That is correct only
for values below 1,000,000; above that the quotient itself needs grouping and does
not get it.

## Evidence

`internal/output/text.go:124-129`:

    func fmtInt(n int) string {
        if n < 1000 { return fmt.Sprintf("%d", n) }
        return fmt.Sprintf("%d,%03d", n/1000, n%1000)
    }

Rendered through the real formatter with `TotalEvents: 1234567, CommitCount: 1000000`:

    Total Events	1234,567
    Commits	1000,000

Expected `1,234,567` and `1,000,000`.

## Impact

Wrong on its face for any account with a seven-figure count. `Commits` is the
realistic trigger: it is set from `TotalCommitContributions` summed across every
year chunk (stats.go:73-75, main.go:122), so a long multi-year range on a busy
account crosses 1,000,000 and prints `1000,000`. The malformed grouping also
appears in the "Top Repositories" event counts (text.go:117). Negative values skip
grouping entirely because `n < 1000` matches.

## Fix

Group all digits, and handle the sign:

    func fmtInt(n int) string {
        s := strconv.Itoa(n)
        sign := ""
        if strings.HasPrefix(s, "-") {
            sign, s = "-", s[1:]
        }
        for i := len(s) - 3; i > 0; i -= 3 {
            s = s[:i] + "," + s[i:]
        }
        return sign + s
    }

Add table-driven cases for 999, 1000, 999999, 1000000, 1234567, and -1500. Note
also that `fmtInt` is applied inconsistently — text.go:71-77 formats PRs Opened,
PRs Merged, and Reviews with bare `%d` while Total Events and Commits use `fmtInt`.
Route all of them through the same helper.
