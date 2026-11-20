# Dependencies

- **No `cli/go-gh` dependency.** The only requirement is `golang.org/x/term` (plus its
  `golang.org/x/sys`). Do not reintroduce go-gh; in-tree replacements are
  `api.graphQLClient` (GraphQL over `net/http`), `main.resolveHost` / `resolveToken`
  (auth), `output.table`, `output.terminalOut`, `output.padRight`, `output.pluralize`
  and `main.browserCommand`. Dropping go-gh shrank the binary from 15.3 to 8.1
  megabytes.
