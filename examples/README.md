# Example output

One report in each of the four formats, all for the same account and period:
`ivuorinen`, 2026-09-01 to 2026-09-30.

| File                         | Format                                                       |
|------------------------------|--------------------------------------------------------------|
| [report.md](report.md)       | `--format markdown`, the default                             |
| [report.txt](report.txt)     | `--format text`, terminal layout (`GH_FORCE_TTY=100`)        |
| [report.json](report.json)   | `--format json`: the summary plus every event, the calendar and GitHub's totals |
| [report.html](report.html)   | `--format html`: open it in a browser; the charts load Plotly from its CDN |

## How they were generated

```bash
gh history ivuorinen --from 2026-09-01 --to 2026-09-30 --format markdown > report.md
GH_FORCE_TTY=100 gh history ivuorinen --from 2026-09-01 --to 2026-09-30 --format text > report.txt
gh history ivuorinen --from 2026-09-01 --to 2026-09-30 --format json > report.json
BROWSER=true gh history ivuorinen --from 2026-09-01 --to 2026-09-30 --format html -o report.html
```

`BROWSER=true` stops the HTML run from opening a browser. GitHub's figures for a
past month can change after the fact (September's commit count did), so a re-run
will not necessarily match these files exactly.

## Private repositories are renamed

The reports were run with a token that can see the account's private
repositories, so the raw output named them. Before committing:

- every private repository was renamed: those owned by `ivuorinen` became
  `ivuorinen/private-repo-1` to `-4`, and those owned by other accounts became
  `example-org/private-repo-1` to `-4`;
- in `report.json`, the pull request and issue titles and the repository
  descriptions of events in those repositories were replaced with
  "Private pull request", "Private issue" and "Private repository";
- in `report.json`, the IDs of comment events in those repositories were
  replaced with `gql-comment-private-N`: a GitHub comment node ID encodes the
  repository's database ID, which would identify the renamed repository.

Public repositories, and the titles of public pull requests and issues, are
unchanged. All counts are unchanged.
