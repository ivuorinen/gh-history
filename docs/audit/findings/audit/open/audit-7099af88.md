---
id: audit-7099af88
auditor: audit
severity: medium
category: security
area: go.mod
status: open
found: 2026-07-25
---

# Two known-vulnerable indirect dependencies with fixes available

## Problem

`trivy fs --scanners vuln` reports two CVEs in transitive dependencies pinned below
their fixed versions.

## Evidence

    $ trivy fs --scanners vuln,secret,misconfig .
    go.mod (gomod)  Total: 2

    golang.org/x/net  CVE-2026-46600  installed v0.55.0  fixed 0.56.0
      Parsing an invalid SVCB or HTTPS RR can panic when the size...
    golang.org/x/text CVE-2026-56852  installed v0.37.0  fixed 0.39.0
      A norm.Iter can enter an infinite loop when handling input containing ...

Both arrive via `github.com/cli/go-gh/v2` (x/net through the markdown sanitizer
chain `glamour → bluemonday`; x/text through `glamour`/`go-runewidth`). Neither is
a direct requirement. `gitleaks` and `semgrep` (p/golang, p/security-audit) reported
nothing; `staticcheck` is clean.

Reachability was not established — `govulncheck`, which answers that question by
call-graph analysis, is not installed on this machine and this audit does not
install tooling. The x/net advisory concerns DNS resource-record parsing, which this
code does not exercise; the x/text `norm.Iter` path is plausibly reachable through
Markdown rendering of untrusted repository names and titles, but that is unproven.

## Impact

Whatever the reachability, this is a released, signed binary distributed via `gh
extension install`, so its dependency set is what users get. The x/text infinite
loop is the more concerning of the two: repository names and PR titles are
attacker-influenced strings that flow into `markdown.Render` (main.go:191) when the
Markdown report is displayed in a terminal.

## Fix

Both are one command, since the fixed versions are backward compatible:

    go get golang.org/x/net@v0.56.0
    go get golang.org/x/text@v0.39.0
    go mod tidy
    go test ./...

Then confirm with `trivy fs --scanners vuln --exit-code 1 .`.

Longer term, install `govulncheck` and add it to CI so reachability is answered
automatically rather than guessed:

    - run: go run golang.org/x/vuln/cmd/govulncheck@latest ./...

Renovate is already configured and has been landing security bumps (commits f743c48,
946d903, 408a77f), so these will likely be offered — the CI gate is what makes them
safe to merge.
