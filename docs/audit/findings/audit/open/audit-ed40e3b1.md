---
id: audit-ed40e3b1
auditor: audit
severity: low
category: security
area: .github/workflows/release.yml
status: open
found: 2026-07-25
---

# Release job requests attestations: write but produces no attestations

## Problem

The release job grants a token permission that nothing in the pipeline uses.

## Evidence

`.github/workflows/release.yml:13-16`:

    permissions:
      contents: write
      id-token: write
      attestations: write

The job's only artifact-producing step is `goreleaser/goreleaser-action`. The
GoReleaser config contains no attestation configuration — `.goreleaser.yaml` has
`builds`, `archives`, `changelog`, `signs`, and `release` blocks only. There is no
`actions/attest-build-provenance` step and no `attestations:` key. `id-token: write`
*is* used, correctly, by cosign keyless signing (`.goreleaser.yaml:42-50`).

## Impact

Minor but real least-privilege drift: the `GITHUB_TOKEN` handed to GoReleaser and
every action in the job carries authority to write attestations to the repository
that no step needs. If any action in the chain were compromised, that authority
would be available to it. The workflow is otherwise well-hardened — top-level
`permissions: {}`, every action pinned to a full commit SHA — which makes the unused
grant the odd one out.

## Fix

Drop the unused permission:

    permissions:
      contents: write
      id-token: write

Or, if build provenance is actually wanted (it would pair naturally with the
existing cosign signing), add the step that uses it:

      - uses: actions/attest-build-provenance@<sha>
        with:
          subject-path: 'dist/*'
