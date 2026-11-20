# GitHub API

How gh-history talks to GitHub's GraphQL API. Each rule below records a measured
failure; keep the tests each one names.

- **The `Time-Zone` request header is load-bearing.** GitHub buckets
  `contributionsCollection` by it, so omitting it changes every count in the report —
  measured on a one-day query: 1 review without the header, 2 with the local zone.
  `main.resolveTimeZone` picks the zone: `--time-zone`, then the `TZ` environment
  variable when it names an IANA zone, then UTC. It does not read the machine's local
  zone, so two machines given the same flags and environment produce the same report.
  Every request sends the zone, and the `DateRange` carries it to every day boundary:
  GitHub's `from`/`to`, the close and comment filters, the date, weekday and hour
  buckets, "today" and the streaks. Dates themselves stay midnight-UTC values, so they
  compare and step by 24 hours with no daylight-saving day; only
  `DateRange.StartTime`, `EndDateTime` and `ghutil.TruncateToDay` turn instants into
  days and back. `time/tzdata` is embedded for machines without a zone database
  (Windows without Go). Keep the `InRangeZone`, `FollowsZone`, `ReportZone` and
  `TimeZone` tests.
- **`contributionsCollection`'s `to` is inclusive by day.** GitHub counts the whole day
  `to` falls on, so `FetchContributions` sends the last second of `End`
  (`EndDateTime() - 1s`). Sending the next midnight added a day to every total —
  measured: 41 commits reported for a one-day range whose day held 25. Keep
  `TestFetchContributions_ToIsLastSecondOfEnd`.
- **Pagination has no page limit.** `paginateGQL` follows `hasNextPage` to the last
  page, as `gh api --paginate` does. `FetchIssueComments` does too, but also stops
  early, with `hasNextPage` still true, at the first comment whose `updatedAt`
  predates the range start: the walk is newest-updated first and `createdAt <=
  updatedAt`, so nothing after it can be in range. A 10-page limit capped every
  collection at 1,100 nodes (a year's reviews read 1,100 where GitHub returned 2,594
  across 26 pages) and stopped the comment walk at 1,000 comments, so a range older
  than the 1,000 most recently updated came back short or empty. With the limit gone,
  a response that repeats a cursor would loop the walk, so both walks treat a cursor
  that comes back a second time as an error. Keep the `FollowsEveryPage` and
  `RepeatedCursorIsAnError` tests.
- **`commitContributionsByRepository` lists at most 100 repositories** and has no
  pagination. `repoCommitCounts` fetches a window that lists 100 and falls short of
  `totalCommitContributions` again in halves, down to single days; a window counts
  only its own days, so the halves add up. It splits only at the limit: a shorter
  list that falls short has another cause, and splitting it would query every day of
  the year. `fetchEvents` warns on stderr when the merged list still falls short.
  Keep the `PastRepositoryLimit`, `ShortListUnderLimitIsNotSplit` and
  `WarnsWhenCommitsByRepoFallShort` tests.
