---
id: audit-982ede47
auditor: audit
severity: low
category: conventions
area: LICENSE
status: open
found: 2026-07-25
---

# Released binaries ship without the MIT attribution notices their dependencies require

## Problem

Every dependency in the tree is permissively licensed, and most are MIT — which
requires the copyright notice and permission text to accompany "copies or
substantial portions of the Software". GoReleaser ships bare binaries with no
accompanying license material.

## Evidence

License survey across the full module graph — no copyleft, no source-available, no
unlicensed modules:

    MIT      github.com/cli/go-gh/v2, shurcooL-graphql, glamour, lipgloss, termenv,
             goldmark, goldmark-emoji, colorprofile, httpretty, reflow, uniseg,
             go-runewidth, go-isatty, douceur, terminfo, ansi, cellbuf, x/term, ...
    BSD-2    github.com/cli/safeexec
    BSD-3    golang.org/x/* (exp, net, sys, term, text), gorilla/css
    Apache-2 github.com/google/shlex
    ISC      go-spew

The project's own `LICENSE` is MIT (added in 241d6cb, "Copyright (c) 2026 Ismo
Vuorinen") and `README.md:97` agrees. No conflicts, no contamination.

`.goreleaser.yaml:29-34` produces binary-only archives:

    archives:
      - formats: [binary]

`formats: [binary]` emits the compiled executable alone — no `files:` block, so no
`LICENSE` and no third-party notices are included.

## Impact

Low and purely a compliance formality — nothing here restricts use or
redistribution, and binary-only distribution without notices is extremely common in
the Go ecosystem. But the statically linked binary does contain substantial portions
of MIT-licensed code, and the letter of those licenses asks for the notice to travel
with it.

## Fix

Generate a combined notice file at build time and attach it to the release. Simplest
route with existing tooling:

    go install github.com/google/go-licenses@latest
    go-licenses save ./... --save_path=third_party/

then reference it from the release rather than the binary archive, since
`formats: [binary]` cannot carry extra files:

    release:
      extra_files:
        - glob: LICENSE
        - glob: third_party/**

Alternatively switch archives to `formats: [tar.gz]` with a `files:` block listing
`LICENSE` and the generated notices — at the cost of changing the install artifact
shape that `gh extension install` expects.
