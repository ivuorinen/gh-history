#!/bin/bash
# Stop gate: the session may not end with unformatted code, vet errors or
# failing tests.
#
# Claude Code blocks a Stop and shows stderr to the agent only on exit 2; any
# other non-zero status is a non-blocking notice to the user. The previous
# inline `go build && go vet && go test` exited 1 on failure, so it reported
# and never stopped anything.
#
# stop_hook_active is true when the agent is already continuing because of this
# hook. Exiting 0 then keeps a failure the agent cannot fix from looping forever;
# the failure was already shown once.
set -uo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "verify hook: jq is not installed, cannot read the hook input" >&2
  exit 2
fi

if [[ "$(jq -r '.stop_hook_active // false')" == "true" ]]; then
  exit 0
fi

fail=0

# A gofmt that fails (a file it cannot parse, gofmt missing) can print nothing
# on stdout, which would read as "everything formatted".
if ! unformatted=$(gofmt -l .); then
  echo "gofmt: failed to run" >&2
  fail=1
fi
if [[ -n "$unformatted" ]]; then
  echo "gofmt: unformatted files:" >&2
  echo "$unformatted" >&2
  fail=1
fi

go vet ./... >&2 || fail=1
go test ./... >&2 || fail=1

[[ $fail -eq 0 ]] || exit 2
