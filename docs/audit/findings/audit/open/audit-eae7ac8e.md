---
id: audit-eae7ac8e
auditor: audit
severity: low
category: reliability
area: internal/output/text.go
status: open
found: 2026-07-25
---

# Unchecked writer errors let the text report fail silently

## Problem

`golangci-lint` reports nine unchecked error returns, all in the output path. Most
are cosmetic, but two are `tableprinter.Render()` calls whose failure produces no
output and no signal.

## Evidence

    $ golangci-lint run ./...
    internal/output/text.go:41:13: Error return value of `fmt.Fprintf` is not checked (errcheck)
    internal/output/text.go:42:13: Error return value of `fmt.Fprintf` is not checked (errcheck)
    internal/output/text.go:43:14: Error return value of `fmt.Fprintln` is not checked (errcheck)
    internal/output/text.go:46:14: Error return value of `fmt.Fprintln` is not checked (errcheck)
    internal/output/text.go:47:14: Error return value of `fmt.Fprintln` is not checked (errcheck)
    internal/output/text.go:79:11: Error return value of `tp.Render` is not checked (errcheck)
    internal/output/text.go:86:14: Error return value of `fmt.Fprintf` is not checked (errcheck)
    internal/output/text.go:120:13: Error return value of `tp2.Render` is not checked (errcheck)
    main.go:73:10: Error return value of `fs.Parse` is not checked (errcheck)
    9 issues:
    * errcheck: 9

`go vet` and `staticcheck` are both clean; only errcheck flags these.

The two that matter are `tp.Render()` (text.go:79, the entire Summary table) and
`tp2.Render()` (text.go:120, the Top Repositories table). `FormatText` returns
nothing, so `main.go:175` has no way to learn that a render failed.

`main.go:73`'s `fs.Parse` is benign — the FlagSet is constructed with
`flag.ExitOnError` (main.go:57), so parse failures exit before returning.

## Impact

`FormatTextTo` writes to an arbitrary `io.Writer` (text.go:39). When that writer is
a closed pipe or a full disk — `gh history user --format text | head -5` closes the
pipe early — the summary table silently vanishes from the output while the
surrounding section headers, which are also unchecked, appear to succeed. The user
gets a partial report and exit code 0.

## Fix

Have `FormatTextTo` return an error and check the two `Render` calls, which are the
only ones that can fail in a way the caller could act on:

    func FormatTextTo(w io.Writer, isTTY bool, width int, stats models.Statistics) error {
        ...
        if err := tp.Render(); err != nil {
            return fmt.Errorf("render summary table: %w", err)
        }
        ...
        if err := tp2.Render(); err != nil {
            return fmt.Errorf("render repositories table: %w", err)
        }
        return nil
    }

`FormatText` propagates it and `writeOutput` (main.go:174-175) calls `fatal` on a
non-nil result, matching how the JSON and HTML branches already handle errors. Add
`errcheck` to a `.golangci.yml` and to the CI lint step so this class does not
regress.
