---
name: gen-test
description: Generate table-driven Go tests matching gh-history project conventions
disable-model-invocation: true
---

# Generate Go Tests

Generate table-driven Go tests for the specified Go source file, following this project's conventions.

## Conventions

- Tests live in colocated `*_test.go` files (same package, same directory)
- Use standard `testing` package — no external test frameworks
- Prefer table-driven tests with `t.Run` subtests for multiple cases
- Use `t.Fatal`/`t.Fatalf` for setup failures, `t.Error`/`t.Errorf` for assertion failures
- Helper functions are defined at the top of test files (e.g., `d(year, month, day)` for date construction in daterange tests)
- Test function names: `Test<FunctionName>_<Scenario>` (e.g., `TestBuildCategoryBars_Standard`)
- All functions return errors explicitly — test both success and error paths
- Zero-value and edge-case coverage (empty maps, nil pointers, zero counts)

## Steps

1. Read the target source file to understand the functions to test
2. Read any existing `*_test.go` file in the same directory for existing helpers and patterns
3. Generate tests covering:
   - Happy path with representative data
   - Edge cases (zero values, empty inputs, nil maps)
   - Error conditions (if the function returns errors)
4. Write or update the `*_test.go` file
5. Run `go test ./path/to/package/...` to verify

## Example Pattern

```go
func TestFunctionName_Scenario(t *testing.T) {
    tests := []struct {
        name string
        // inputs
        want // expected output
    }{
        {"happy path", /* ... */},
        {"empty input", /* ... */},
        {"edge case", /* ... */},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := FunctionName(tt.input)
            if got != tt.want {
                t.Errorf("FunctionName(%v) = %v, want %v", tt.input, got, tt.want)
            }
        })
    }
}
```
