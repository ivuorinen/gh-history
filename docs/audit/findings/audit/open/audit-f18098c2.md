---
id: audit-f18098c2
auditor: audit
severity: medium
category: reliability
area: main.go
status: open
found: 2026-07-25
---

# resolveUser reports every authentication failure as 'username required'

## Problem

`resolveUser` discards both errors it can encounter — client construction and the
`viewer` query — and falls through to a single fixed message that names neither.

## Evidence

`main.go:83-96`:

    client, err := newAPIClient()
    if err == nil {
        if username, err := client.GetAuthenticatedUser(); err == nil && username != "" {
            ...
            return username
        }
    }
    fatal("username required. Usage: gh history <username> [options]\nOr authenticate with: gh auth login")

Both `err` values are shadowed and dropped. `logVerbose` is not called on either
path, so `--verbose` reveals nothing either.

## Impact

An expired or revoked token, a token missing the `read:user` scope, a network
outage, a proxy rejection, and genuinely having no credentials all produce the
identical message telling the user to supply a username or run `gh auth login`. For
the expired-token case that advice is right by accident; for a network failure it
sends the user to re-authenticate a working credential. Diagnosing the real cause is
impossible from the CLI output because the underlying error is never printed at any
verbosity.

## Fix

Keep the errors and report what actually happened:

    func resolveUser(cfg *config) string {
        if cfg.username != "" {
            return cfg.username
        }
        client, err := newAPIClient()
        if err != nil {
            fatal("no username given and GitHub client unavailable: %v\nRun: gh auth login", err)
        }
        username, err := client.GetAuthenticatedUser()
        if err != nil {
            fatal("no username given and could not resolve authenticated user: %v\nRun: gh auth login", err)
        }
        if username == "" {
            fatal("username required. Usage: gh history <username> [options]\nOr authenticate with: gh auth login")
        }
        logVerbose(cfg.verbose, "Using authenticated user: %s", username)
        return username
    }
