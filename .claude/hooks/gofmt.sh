#!/bin/bash
FILE=$(jq -r '.tool_input.file_path')

# Only process Go files (skip test generation artifacts, etc.)
if [[ ! "$FILE" =~ \.go$ ]]; then
  exit 0
fi

# Format the file
gofmt -w "$FILE"
exit 0
