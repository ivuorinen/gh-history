---
id: audit-61cfa9b4
auditor: audit
severity: high
category: reliability
area: internal/api/client.go
status: open
found: 2026-07-25
---

# paginateGQL swallows page errors and returns partial data as if complete

## Problem

The generic paginator breaks out of its loop on any error from `doPage` and returns
whatever it accumulated so far. The error is neither returned nor logged. Callers
have no way to distinguish "there were no more pages" from "the next page failed".

## Evidence

`internal/api/client.go:530-549`:

    nodes, nextPI, err := doPage(vars)
    if err != nil {
        break
    }
    all = append(all, nodes...)

`paginateGQL` returns only `[]T` — there is no error channel. All four callers
(`paginatePRs`, `paginateIssues`, `paginateReviews`, `paginateRepos`, lines
551-589) discard the possibility entirely, and `FetchContributions` appends the
truncated slices at lines 393-402 and returns `nil` error.

## Impact

A user with more than 100 pull requests in the range whose second page hits a rate
limit, a transient 502, or a token-scope error gets a report built from the first
100 PRs only — presented as a complete, authoritative result with exit code 0.
Statistics like "PRs Opened", "Top Repositories", and the category distribution are
all silently understated. The same applies to issues, reviews, and repositories.

Note that `MaxPaginationPages = 10` also caps each collection at ~1100 nodes with
no signal; see the separate finding on issue-comment truncation.

## Fix

Propagate the error and let the caller decide:

    func paginateGQL[T any](..., doPage func(...) ([]T, pageInfo, error)) ([]T, error) {
        var all []T
        for i := 0; i < ghutil.MaxPaginationPages && pi.HasNextPage && pi.EndCursor != nil; i++ {
            nodes, nextPI, err := doPage(vars)
            if err != nil {
                return all, fmt.Errorf("pagination page %d: %w", i+1, err)
            }
            all = append(all, nodes...)
            pi = nextPI
        }
        if pi.HasNextPage {
            return all, fmt.Errorf("results truncated at %d pages", ghutil.MaxPaginationPages)
        }
        return all, nil
    }

Have `FetchContributions` return the error (it already returns one), or at minimum
carry a `Truncated bool` on `ContributionResult` and print a warning to stderr
unconditionally — not only under `--verbose`.
