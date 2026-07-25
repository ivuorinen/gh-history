---
id: audit-e77f9833
auditor: audit
severity: high
category: security
area: internal/output/html.go
status: open
found: 2026-07-25
---

# HTML report loads Plotly from a remote CDN with no version pin and no SRI hash

## Problem

The generated report pulls its charting library from a third-party CDN at render
time, using a floating "latest" URL and no Subresource Integrity attribute.

## Evidence

`internal/output/html.go:67`:

    <script src="https://cdn.plot.ly/plotly-latest.min.js"></script>

No `integrity=`, no `crossorigin=`, no pinned version. `GenerateHTML` writes this
file to disk (line 299) and `main.go:209-210` immediately opens it in the user's
default browser.

## Impact

Three distinct problems from one line:

1. **Supply chain.** Every time the user opens a report — including old reports
   saved months ago — the browser fetches and executes whatever JavaScript
   `cdn.plot.ly` serves at that moment, in the user's browser, with no integrity
   check. A CDN compromise or DNS hijack executes attacker JavaScript with access
   to the report's contents and the browser's origin.
2. **Unpinned dependency.** `plotly-latest.min.js` is a moving target that Plotly
   has explicitly frozen at the v1.x line and advises against; the charts can
   change behaviour or break without any change to this repository.
3. **Offline breakage.** With no network, `Plotly` is undefined, every
   `Plotly.newPlot` call throws, and all five charts render as blank boxes with no
   error shown to the user — the report silently degrades to empty divs.

## Fix

Pin the version and add SRI at minimum:

    <script src="https://cdn.plot.ly/plotly-3.0.1.min.js"
            integrity="sha384-<hash>"
            crossorigin="anonymous"></script>

Better, and consistent with the tool writing a self-contained artifact the user
keeps: vendor the minified library and embed it with `go:embed`, so the report has
no network dependency at all. Either way, add a `<noscript>`/undefined-Plotly guard
that surfaces a visible message instead of blank chart containers.
