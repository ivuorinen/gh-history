---
id: audit-715af4b6
auditor: audit
severity: low
category: conventions
area: .git
status: open
found: 2026-10-04
---

# Commit 8ac080d claims a 1.25.0 language floor; its diff sets go 1.26.5

## Problem

The subject of a published commit contradicts its own diff about the minimum Go version, a user-facing contract.

## Evidence

`8ac080d chore(go): set language floor to 1.25.0 and track toolchain go1.27.0 (#22)`. `git show 8ac080d -- go.mod`:

    -go 1.26.6
    +go 1.26.5
    +toolchain go1.27.0

CLAUDE.md and `go.mod` agree on 1.26.5. Only the history says 1.25.0.

## Impact

Anyone using `git log` to find when the floor changed, or reading the release changelog that goreleaser builds from subjects, is told 1.25.0 works. Building with 1.25 fails on the `go 1.26.5` directive.

## Fix

The commit is published on main, so do not rewrite it. If 1.25.0 was the intent, set `go 1.25.0` in go.mod in a new commit and update CLAUDE.md. If 1.26.5 was the intent, no code change: note the correction in the next release's notes. Going forward, check the subject against `git diff --cached go.mod` before committing a toolchain change.

## Resolution

README Environment table lists GH_HOST, token vars, GH_CONFIG_DIR/XDG_CONFIG_HOME, BROWSER, GH_FORCE_TTY, TZ.
