---
id: audit-a90c454a
auditor: audit
severity: medium
category: maintainability
area: .claude/hooks/gofmt.sh
status: open
found: 2026-07-25
---

# The gofmt hook is bypassable, has no jq preflight, and reports success unconditionally

## Problem

The only automated enforcement in `.claude/` is a `PostToolUse` hook that runs
`gofmt -w`. It fails open in three independent ways.

## Evidence

`.claude/hooks/gofmt.sh` in full:

    #!/bin/bash
    FILE=$(jq -r '.tool_input.file_path')

    if [[ ! "$FILE" =~ \.go$ ]]; then
      exit 0
    fi

    gofmt -w "$FILE"
    exit 0

and `.claude/settings.json`:

    "matcher": "Edit|Write"

1. **No `jq` preflight.** If `jq` is not installed, `FILE` is empty, the regex test
   fails, and the hook exits 0 having formatted nothing — indistinguishable from
   success.
2. **`gofmt` failures are discarded.** The exit status is never checked and the
   script ends in an unconditional `exit 0`. A file with a syntax error makes
   `gofmt` fail; the hook still reports success and the malformed file stands.
3. **Matcher does not cover Bash.** Only `Edit` and `Write` are matched, so a `.go`
   file created or rewritten through a Bash heredoc, `sed -i`, or `cat >` is never
   formatted.

`.claude/settings.json` also declares no `permissions` block at all.

## Impact

`CLAUDE.md` states "Standard `go vet` and `gofmt` formatting" as a project
convention and `.claude/agents/code-reviewer.md` lists "`gofmt` formatted" and
"`go vet` clean" in its review checklist. The hook is the only mechanism intended to
enforce that, and it can silently do nothing. Combined with `make lint` masking
staticcheck failures and no CI running either, this project currently has zero
enforced quality gates — three separate mechanisms that all fail open.

## Fix

Make the hook fail loudly and cover the paths that matter:

    #!/bin/bash
    set -euo pipefail

    command -v jq >/dev/null 2>&1 || { echo "gofmt hook: jq not installed" >&2; exit 1; }

    FILE=$(jq -r '.tool_input.file_path // empty')
    [[ -n "$FILE" && "$FILE" =~ \.go$ ]] || exit 0
    [[ -f "$FILE" ]] || exit 0

    if ! gofmt -l -w "$FILE"; then
      echo "gofmt hook: failed to format $FILE" >&2
      exit 1
    fi

Add `Bash` to the matcher, or accept that shell-authored Go files are covered by the
CI `gofmt -l` check proposed in the CI finding — the CI gate is the more reliable of
the two and should exist regardless.
