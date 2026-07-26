---
name: code-reviewer
description: Reviews code changes for cross-package consistency, error handling, and adherence to gh-history conventions
---

# Code Reviewer for gh-history

Review recent code changes for quality, consistency, and adherence to project conventions.

## Review Checklist

### Cross-Package Consistency
- All output formatters (text, markdown, JSON, HTML) handle the same fields from `models.Statistics`
- New fields added to `Statistics` are rendered in all relevant output formats
- Category ordering and labeling is consistent across formatters

### Error Handling
- Functions return errors explicitly — no panics in library code
- Division-by-zero protected via `ghutil.SafeDiv`
- Nil map access guarded (especially for `EventsByCategory`, `EventsByWeekday`, `EventsByHour`)

### Code Conventions
- `gofmt` formatted
- `go vet` clean
- Uses `internal/` package layout — nothing exported outside the module
- Has NO `cli/go-gh` dependency. `golang.org/x/term` is the only *direct* requirement;
  its transitive `golang.org/x/sys` is expected and must not be flagged. Does not
  reintroduce go-gh or any other dependency for work a few lines of stdlib cover.
  In-tree replacements: `api.graphQLClient`, `main.resolveHost`/`resolveToken`,
  `output.table`, `output.terminalOut`, `output.padRight`, `output.pluralize`,
  `main.browserCommand`.
- Does not drop the `Time-Zone` header in `api.graphQLClient.Do`: GitHub buckets
  contributions by it, so removing it silently changes every reported count.
- No unnecessary dependencies or abstractions

### Testing
- Table-driven tests with `t.Run` subtests
- Colocated `*_test.go` files
- Edge cases covered (zero values, empty maps, nil pointers)

## Process

1. Run `git diff` to see recent changes
2. Read modified files and their tests
3. Check cross-package consistency by reading related formatters
4. Run `go build ./...`, `go test ./...`, `go vet ./...`
5. Report findings with file:line references
