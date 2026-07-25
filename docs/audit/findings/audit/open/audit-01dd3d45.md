---
id: audit-01dd3d45
auditor: audit
severity: high
category: tests
area: .github/workflows
status: open
found: 2026-07-25
---

# No CI workflow runs the test suite or the linter

## Problem

The repository has exactly two workflows — `codeql.yml` and `release.yml`. Neither
runs `go test`, `go vet`, `make test`, or `make lint`. Nothing gates a pull request
on the suite passing.

## Evidence

    $ ls .github/workflows/
    codeql.yml  release.yml
    $ grep -rln 'go test\|make test\|make lint\|go vet' .github/
    NONE: no workflow runs tests or lint

`release.yml` triggers only on `push: tags: ["20*"]` and runs GoReleaser.
`codeql.yml` runs CodeQL static analysis; it compiles the Go code but never
executes tests.

## Impact

Every merge to `main` is unverified. This is not hypothetical for this repo: 13 of
the last 15 commits are Renovate dependency bumps merged through PRs (#1–#16), each
one merged without the test suite ever running. A dependency update that breaks
`go-gh`'s tableprinter, markdown renderer, or GraphQL client would reach `main` and
then a signed release with no signal. The `make test` / `make lint` targets exist
and pass locally, so the gap is purely that nothing invokes them.

## Fix

Add `.github/workflows/ci.yml`:

    name: CI
    on:
      push:
        branches: [main]
      pull_request:
        branches: [main]
      merge_group:
    permissions: {}
    jobs:
      test:
        runs-on: ubuntu-latest
        permissions:
          contents: read
        steps:
          - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
          - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
            with:
              go-version-file: go.mod
          - run: go vet ./...
          - run: go test -race ./...
          - run: gofmt -l . | tee /dev/stderr | (! read)

Then mark the job required in branch protection, so PRs cannot merge red.
