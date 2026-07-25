---
id: audit-cde81805
auditor: audit
severity: medium
category: reliability
area: internal/api/client.go
status: open
found: 2026-07-25
---

# No HTTP timeout is configured, so a stalled GitHub connection hangs the CLI indefinitely

## Problem

Both client constructors leave `ClientOptions.Timeout` at its zero value, and go-gh
passes that straight into `http.Client`. A zero `http.Client.Timeout` means no
timeout at all.

## Evidence

`internal/api/client.go:35-41`:

    gqlClient, err := ghAPI.NewGraphQLClient(ghAPI.ClientOptions{AuthToken: token})

Only `AuthToken` is set. In go-gh v2.13.0:

    pkg/api/client_options.go:59  // Timeout specifies a time limit for each API request.
    pkg/api/client_options.go:61  Timeout time.Duration
    pkg/api/http_client.go:119    return &http.Client{Transport: transport, Timeout: opts.Timeout}, nil

`NewClient()` (line 26-32) uses `DefaultGraphQLClient()`, which likewise applies no
timeout. No `context.Context` is threaded through any call in this package.

## Impact

If the connection stalls after the TCP handshake — a captive portal, a hung proxy, a
GitHub incident that accepts connections but never responds — `gh history` blocks
forever with no output and no way to recover but Ctrl-C. A multi-year query makes
this worse: `fetchEvents` issues one chunk request per year sequentially
(main.go:113-123), so any one of them can wedge the whole run. There is no retry and
no cancellation path either.

## Fix

Set an explicit per-request timeout in both constructors:

    const requestTimeout = 30 * time.Second

    func NewClientWithToken(token string) (*Client, error) {
        gqlClient, err := ghAPI.NewGraphQLClient(ghAPI.ClientOptions{
            AuthToken: token,
            Timeout:   requestTimeout,
        })
        ...
    }

`NewClient()` should construct via `NewGraphQLClient` with the same options rather
than `DefaultGraphQLClient()` so the timeout applies on the `gh auth` path too.
