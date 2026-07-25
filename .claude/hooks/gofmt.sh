#!/bin/bash
# Formats Go files after Claude edits them.
#
# Fails loudly rather than open: a hook that exits 0 when jq is missing or gofmt
# errors is indistinguishable from one that worked, so the convention it exists
# to enforce silently stops being enforced.
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "gofmt hook: jq is not installed, cannot read the tool input" >&2
  exit 1
fi

FILE=$(jq -r '.tool_input.file_path // empty')

# Only process Go files that exist on disk.
[[ -n "$FILE" && "$FILE" =~ \.go$ ]] || exit 0
[[ -f "$FILE" ]] || exit 0

if ! gofmt -w "$FILE"; then
  echo "gofmt hook: failed to format $FILE (syntax error?)" >&2
  exit 1
fi
