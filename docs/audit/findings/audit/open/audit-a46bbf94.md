---
id: audit-a46bbf94
auditor: audit
severity: medium
category: correctness
area: internal/api/client.go
status: open
found: 2026-07-25
---

# Issue comments are paged by UPDATED_AT with a 10-page cap, so comment counts are wrong for older ranges

## Problem

`FetchIssueComments` walks the user's comments ordered by `UPDATED_AT DESC`, filters
each page client-side by `createdAt`, and stops after `MaxPaginationPages` (10)
pages. The sort key and the filter key are different fields, so pages cannot be
reasoned about: a comment created two years ago but edited yesterday sorts first,
while a comment created last week but never edited can sort arbitrarily far down.

## Evidence

`internal/api/client.go:591-602`:

    issueComments(first: 100, after: $after, orderBy: {field: UPDATED_AT, direction: DESC})

`internal/api/client.go:623` caps the walk:

    for range ghutil.MaxPaginationPages {

and `ghutil.go:12` sets `MaxPaginationPages = 10`. The in-range test at line 638
filters on `n.CreatedAt`. There is no early exit and no signal when the cap is hit.

## Impact

The walk sees at most 1000 comments, ordered by a field unrelated to the filter. A
user with more than 1000 issue comments gets an arbitrary subset of their requested
range — and for any range older than their most recent 1000 comments, typically zero
comments, silently. The "Comments" category and total event count are understated
with no warning. Unlike the other collections, this query is not even scoped to the
date range server-side, so it also wastes up to 10 round trips fetching comments
that are then discarded.

## Fix

Sort by the field being filtered so the walk can terminate correctly, and report
truncation:

    issueComments(first: 100, after: $after, orderBy: {field: UPDATED_AT, direction: DESC})

becomes a `CREATED_AT`-ordered walk (or, better, drop this query entirely and use
`contributionsCollection`'s issue/PR comment data alongside the other
sub-collections, which is already date-scoped server-side).

With a `createdAt DESC` ordering, break as soon as a node predates `startDT` — the
remainder is guaranteed older. If the loop exits because the page cap was reached
rather than because `hasNextPage` is false, return an error or set a truncation flag
so `main.go` can warn unconditionally.
