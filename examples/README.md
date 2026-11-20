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

`BROWSER=true` stops the HTML run from opening a browser. The runs had no `TZ`
and no `--time-zone`, so their days and hours are in UTC, as each report records.
GitHub's figures for a past month can change after the fact (September's commit
count did), so a re-run will not necessarily match these files exactly.
`generated_at` in `report.json` records when the reports were made.

## Private repositories are renamed

The reports were run with a token that can see the account's private
repositories, so the raw output named them. Before committing, every repository
the report itself marks private (`repo_private` on its events, `private` in
`commits_by_repo`) was anonymised:

- each was renamed, numbered by event count: those owned by `ivuorinen` became
  `ivuorinen/private-repo-1` to `-5`, and those owned by other accounts became
  `example-org/private-repo-1` to `-4`. Event IDs that end in the repository
  name carry the new name;
- in `report.json`, the pull request and issue titles and the repository
  descriptions of events in those repositories were replaced with
  "Private pull request", "Private issue" and "Private repository";
- in `report.json`, the IDs of comment events in those repositories were
  replaced with `gql-comment-private-N`: a GitHub comment node ID encodes the
  repository's database ID, which would identify the renamed repository;
- in `report.json`, the database IDs of those repositories (`id` on their
  `top_repos` and `commits_by_repo` entries, `repo_id` on their events) were
  replaced with 0, for the same reason;
- every list and table of repositories was re-sorted as the CLI sorts it, by
  count and then by name, so repositories with equal counts stay in name order
  under their new names. This also re-cuts the top 15 the other formats show,
  and in `report.json` the keys of every object are back in sorted order;
- in `report.txt`, the top-repositories table was re-padded so its columns line
  up with the new names.

Public repositories, and the titles of public pull requests and issues, are
unchanged. All counts are unchanged.
