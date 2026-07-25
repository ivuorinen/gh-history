---
id: audit-5ba67597
auditor: audit
severity: medium
category: maintainability
area: internal/models/models.go
status: open
found: 2026-07-25
---

# Events round-trip through an untyped map[string]any that no external format requires

## Problem

`Event.Payload` is `map[string]any`. The api package builds these maps from data it
already has in typed form, and the analysis package immediately takes them apart
again with runtime type assertions. Nothing is serialized or deserialized in
between — the map exists purely as an internal handoff.

## Evidence

Construction, `internal/api/client.go:414-417`:

    Payload: map[string]any{
        "action":       "opened",
        "pull_request": map[string]any{"number": n.PullRequest.Number, "title": n.PullRequest.Title},
    },

Destructuring, `internal/analysis/stats.go:129` and `:135`:

    action, _ := event.Payload["action"].(string)
    if pr, ok := event.Payload["pull_request"].(map[string]any); ok {

and `stats.go:112-120`, which probes three possible types for one field because the
map has no schema:

    if commits, ok := payload["commits"].([]any); ok && len(commits) > 0 { ... }
    if size, ok := payload["size"].(float64); ok && size > 0 { ... }
    if size, ok := payload["size"].(int); ok && size > 0 { ... }

The `float64`-vs-`int` fallback exists because these payloads once came from JSON.
They no longer do: `FetchContributions` writes Go values directly, so the `float64`
branch is unreachable.

## Impact

Every field access is a runtime assertion that the compiler cannot check. A typo in
a payload key — `"pull_request"` vs `"pullRequest"` — compiles, passes vet, passes
staticcheck, and silently produces a zero count: `trackActionCount` would simply
never increment `PRMerged`. The merged-PR path is exactly one such assertion deep
(stats.go:135-137), so the PR merge rate can silently read 0% with no failure
anywhere. The three-way `size` probe is dead code preserved by the untyped shape.

## Fix

Give `Event` typed optional detail and delete the assertions:

    type Event struct {
        ID        string
        Type      string
        Actor     string
        Repo      string
        Action    string          // "opened" | "closed" | ""
        Merged    bool
        CreatedAt time.Time
    }

`trackDetailedStats` then switches on `event.Type` and reads `event.Action` /
`event.Merged` directly; `countCommits` and its `size` probes disappear (see the
related unreachable-code finding). If a payload escape hatch is genuinely wanted
later, add a typed struct per event type rather than restoring the map.
