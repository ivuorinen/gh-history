---
id: audit-355a10ce
auditor: audit
severity: low
category: docs
area: README.md
status: open
found: 2026-07-25
---

# README omits short flags and the HTML format's file-writing and browser-opening behaviour

## Problem

Documented usage diverges from what the CLI actually does in ways a user only
discovers by running it.

## Evidence

1. **Short flags undocumented.** `parseFlags` registers `-f`, `-t`, `-y`, `-o`,
   `-v` alongside their long forms (main.go:58-70). The README documents only
   `--from`, `--to`, `--year`, `--output`/`-o` (shown once at line 49), and
   `--verbose`.
2. **HTML always writes a file and opens a browser.** README:48 says only:

       gh history octocat --format html             # generates and opens an interactive report

   It does not say that with no `-o` the file is written to the current directory as
   `<username>-report.html` (main.go:199-201), that `-o` gains a forced `.html`
   suffix if it lacks one (main.go:202-204), or that the browser opens
   unconditionally with no flag to suppress it (main.go:209-210). There is no
   `--no-browser` option.
3. **Default range undocumented.** With no date flags, `ParseDateRange` returns the
   last 90 days (daterange.go:131-133). The README's date-range section lists the
   explicit options but never states the default.
4. **`--format html` ignores `--output -`/stdout entirely** — it is the only format
   that cannot write to stdout.

## Impact

A user scripting the tool in CI hits the browser-open behaviour with no documented
way to avoid it, and `gh history user --format html` unexpectedly drops a file into
whatever directory they happened to be in. The undocumented 90-day default means
output that looks like "all activity" is silently a quarter of a year.

## Fix

In the README's "Output formats" section:

    gh history octocat --format html
    # Writes <username>-report.html in the current directory (or the --output path,
    # with .html appended if missing) and opens it in your default browser.

Add to "Date ranges": "With no date flags, the last 90 days are used." Add a short
flags table, or drop the short aliases if they are not intended to be part of the
supported surface — registering them and not documenting them is the worst of both.
