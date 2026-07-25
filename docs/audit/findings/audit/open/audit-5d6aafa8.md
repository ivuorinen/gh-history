---
id: audit-5d6aafa8
auditor: audit
severity: high
category: correctness
area: internal/api/client.go
status: open
found: 2026-07-25
---

# Verbose diagnostics are written to stdout, corrupting JSON and Markdown output

## Problem

The API client prints its verbose progress lines with `fmt.Printf`, which writes to
stdout — the same stream the JSON and Markdown formatters write the report to.
`main.go` gets this right (`logVerbose` uses `os.Stderr`); the client does not.

## Evidence

`internal/api/client.go:515-518`:

    if c.Verbose {
        fmt.Printf("  GraphQL: %d PRs, %d issues, %d reviews, %d repos, %d calendar days\n", ...)
    }

and `internal/api/client.go:659-661`:

    if c.Verbose {
        fmt.Printf("  GraphQL comments: %d\n", len(events))
    }

Compare `main.go:49-53`, which correctly targets stderr:

    func logVerbose(verbose bool, format string, args ...any) {
        if verbose { fmt.Fprintf(os.Stderr, format+"\n", args...) }
    }

`writeToFileOrStdout` (main.go:169) writes the report with `fmt.Println` to stdout
whenever no `-o` is given.

## Impact

`gh history someone --verbose --format json > stats.json` produces a file that
begins with `  GraphQL: 12 PRs, 3 issues, ...` and is therefore not valid JSON —
`jq` and every other consumer reject it. The same corruption hits `--format
markdown` piped to a file. Because one year-chunk emits one line, a multi-year
range interleaves several diagnostic lines through the output. Users combining
`--verbose` with a pipe get silently unusable data.

## Fix

Route both through stderr, matching the rest of the codebase:

    fmt.Fprintf(os.Stderr, "  GraphQL: %d PRs, %d issues, %d reviews, %d repos, %d calendar days\n", ...)
    fmt.Fprintf(os.Stderr, "  GraphQL comments: %d\n", len(events))

Add `"os"` to the imports. Better still, give `Client` a `logf` method (or an
`io.Writer` field defaulting to `os.Stderr`) so the destination is decided in one
place and can be asserted in tests.
