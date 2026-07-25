---
id: audit-11eaf015
auditor: audit
severity: medium
category: reliability
area: internal/api/client.go
status: open
found: 2026-07-25
---

# CheckUserExists detects a missing user by substring-matching GitHub's error prose

## Problem

Whether a username exists is decided by searching the returned error's text for the
English phrase "Could not resolve". Any change to GitHub's wording, a localized
response, or a differently-phrased error turns a "user not found" into a hard fatal.

## Evidence

`internal/api/client.go:61-67`:

    if err != nil {
        if strings.Contains(err.Error(), "Could not resolve") {
            return false, nil
        }
        return false, fmt.Errorf("GraphQL user check: %w", err)
    }
    return resp.User != nil, nil

`main.go:237-243` turns any error from this into `fatal("checking user: %v", err)`,
so the CLI exits 1 before fetching anything.

## Impact

The happy path already works without the string match: GraphQL returns `data.user =
null` with a `NOT_FOUND` error for an unknown login, and the `resp.User != nil`
check on line 67 covers it. The substring branch is a fallback whose failure mode is
severe — if GitHub rewords the message to, say, "Could not find a User with the
login", every lookup of a nonexistent user becomes `Error: checking user: GraphQL
user check: ...` instead of the intended `Error: user "x" not found`. The check is
also over-broad in the other direction: an unrelated error that happens to contain
"Could not resolve" (a DNS failure message, for instance) would be misreported as
"user not found", sending the user chasing a typo that isn't there.

## Fix

Match on the structured error type go-gh already provides rather than prose:

    var gqlErr *ghAPI.GraphQLError
    if errors.As(err, &gqlErr) {
        for _, e := range gqlErr.Errors {
            if e.Type == "NOT_FOUND" {
                return false, nil
            }
        }
    }
    return false, fmt.Errorf("GraphQL user check: %w", err)

Add a test that feeds a `NOT_FOUND` GraphQLError and one that feeds an unrelated
transport error, asserting `(false, nil)` and a non-nil error respectively.
