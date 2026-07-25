---
id: audit-aba25842
auditor: audit
severity: low
category: conventions
area: .git
status: open
found: 2026-07-25
---

# CI-only dependency bumps are marked as breaking changes

## Problem

Two commits carry the Conventional Commits `!` breaking-change marker for changes
that alter one line of a CI workflow and nothing a user consumes.

## Evidence

    e97edba chore(deps)!: update actions/setup-go action (v6.5.0 → v7.0.0) (#16)
             .github/workflows/release.yml | 2 +-
             1 file changed, 1 insertion(+), 1 deletion(-)

    b24c246 chore(deps)!: update actions/checkout action (v6.0.3 → v7.0.0) (#12)
             .github/workflows/release.yml | 2 +-
             1 file changed, 1 insertion(+), 1 deletion(-)

Each diff is a single pinned-SHA swap in the release workflow. Neither touches
`go.mod`, any `internal/` package, the CLI flag surface, or the output formats.

The `!` appears to have been propagated mechanically from the upstream action's own
major-version bump rather than describing the effect on this project.

## Impact

Low, because this project releases on CalVer (`.github/workflows/release.yml:6`
triggers on `20*` tags), so a false breaking marker cannot mis-version a release the
way it would under SemVer. But `.goreleaser.yaml:35-40` builds the changelog from
commit subjects, so these land in released notes announcing breaking changes to
users whose experience of the tool is unchanged. That erodes the signal for the day
a genuine breaking change ships — a removed flag or a changed JSON schema.

Scope naming is also inconsistent for one class of change: `chore(actions)` in
commits 6695932, 60d501a, 01d8e72, ed680b1, aab6c38, 0e52c63 versus `chore(deps)`
in b24c246 and e97edba, for identical action bumps.

## Fix

Reserve `!` for changes that break this project's contract — CLI flags, output
schema, minimum Go version, extension install path. A CI action bump is
`chore(ci):` or `chore(deps):` without the marker.

Settle on one scope for GitHub Action updates and encode it in the Renovate config
(`renovate.json` extends `local>ivuorinen/renovate-config`, so the
`commitMessageTopic`/`semanticCommitScope` for the `github-actions` manager belongs
there) so the choice is applied automatically rather than per-PR.
