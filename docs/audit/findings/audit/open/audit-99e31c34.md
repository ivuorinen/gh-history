---
id: audit-99e31c34
auditor: audit
severity: high
category: conventions
area: Makefile
status: open
found: 2026-07-25
---

# make lint reports success even when staticcheck fails, so lint can never fail the build

## Problem

The `lint` recipe chains `command -v` and the tool invocation with `&&`, then falls
back to `echo` with `||`. Shell precedence makes `A && B || C` evaluate `C` whenever
*either* `A` or `B` fails — so a staticcheck run that reports real problems is
indistinguishable from staticcheck not being installed, and both exit 0.

## Evidence

`Makefile:35`:

    @command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

Reproduced with a stub that exists on PATH and exits 1:

    $ printf '#!/bin/sh\nexit 1\n' > faketool && chmod +x faketool
    $ command -v faketool >/dev/null 2>&1 && faketool || echo "not installed, skipping"
    not installed, skipping
    recipe exit status when tool exists and FAILS: 0

The tool ran, failed, and the recipe printed "not installed, skipping" and returned
success.

## Impact

`make lint` and therefore `make all` (which the README documents as the contributor
gate: "make all — Runs lint, test, and build") can never fail because of
staticcheck. Every staticcheck finding is silently swallowed, and the misleading
"not installed" message actively hides that the tool ran at all. Combined with the
absence of any CI lint job, nothing in this project enforces static analysis.

## Fix

Separate the availability probe from the run so failures propagate:

    .PHONY: lint
    lint:
    	go vet ./...
    	@if command -v staticcheck >/dev/null 2>&1; then \
    		staticcheck ./...; \
    	else \
    		echo "staticcheck not installed, skipping"; \
    	fi

The `if` body's exit status becomes the recipe's, so a staticcheck failure fails
`make lint`. Apply the same shape to the `gh calver` probe in the `release` target
(Makefile:81), which has the identical `&&`/`||` structure.
