// Command jsondoc writes docs/json.md, the reference for every key in
// gh-history's JSON output.
//
// It reads a real report, examples/report.json, rather than a hand-made
// sample, so the documented keys are the keys the CLI writes. Each key's type
// and meaning are written once, in fields. The run fails and writes nothing
// when the report has a key fields does not document, fields documents a key
// the report lacks, or a value's JSON type is not its documented type. A key
// added to the output therefore cannot reach the regenerated example without a
// meaning.
//
// Every meaning describes gh-history's own output, most of it computed, so
// GitHub's GraphQL schema descriptions would cover only a few pass-through
// fields, at the cost of a token and a network call per run.
//
// Usage: go run ./tools/jsondoc examples/report.json docs/json.md
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// field documents one JSON key. path is dotted from the top level, with []
// for the entries of an array, as in "events[].title". typ starts with the
// JSON type, and ends in "or null" when the value can be null.
type field struct {
	path, typ, meaning string
}

// fields lists every key in document order. A table starts wherever the
// parent path changes, so the keys of one object stay together.
var fields = []field{
	{"username", "string", "The GitHub login the report is about, in the account's own spelling."},
	{"date_range", "object", "The period the report covers."},
	{"generated_at", "string", "When the report was made, just after its data was fetched, as an RFC 3339 timestamp in the report's time zone. `pr_merge_rate` and `issue_close_rate` count each item by its state as fetched, no later than this."},
	{"summary", "object", "The summary figures. Text, Markdown and HTML show the same figures."},
	{"contribution_totals", "object", "GitHub's own contribution counters for the period, private repositories included."},
	{"events_by_category", "object (category → number)", "Events per category: `pull_requests`, `issues`, `reviews`, `comments`, `repos` or `other`. Categories without events are left out."},
	{"events_by_type", "object (event type → number)", "Events per event type; the types are listed under `events[].type`."},
	{"events_by_date", "object (date → number)", "Events per day. Days without events are left out."},
	{"events_by_weekday", "object (weekday → number)", "Events per day of the week in the report's time zone, keyed `0` (Monday) to `6` (Sunday). Weekdays without events are left out."},
	{"events_by_hour", "object (hour → number)", "Events per hour of the day in the report's time zone, keyed `0` to `23`. Hours without events are left out."},
	{"calendar", "object or null", "GitHub's contribution calendar for the period, private repositories included; null when GitHub returned no days."},
	{"streaks", "object", "Runs of consecutive active days. A day is active when the calendar counts a contribution on it or, without a calendar, when it has an event."},
	{"top_repos", "array of objects", "Every repository with events, most events first, ties in name order. Text, Markdown and HTML show the top 15."},
	{"commits_by_repo", "array of objects", "GitHub's commit count for every repository with commits in the period, private repositories included, most commits first, ties in name order."},
	{"events", "array of objects", "The events the summary and the distributions are counted from, newest first."},

	{"date_range.start", "string", "The first day of the period."},
	{"date_range.end", "string", "The last day of the period, included."},
	{"date_range.time_zone", "string", "The IANA time zone every day and hour in the report is counted in, such as `UTC` or `Europe/Helsinki`."},

	{"summary.total_events", "number", "The number of entries in `events`."},
	{"summary.commits", "number", "Commits in the period, private repositories included: GitHub's count, the same as `contribution_totals.commits`."},
	{"summary.prs_opened", "number", "Pull requests opened in the period."},
	{"summary.prs_merged", "number", "Pull requests opened in the period and merged within it."},
	{"summary.prs_closed", "number", "Pull requests opened in the period and closed within it without being merged."},
	{"summary.pr_merge_rate", "number", "The percentage of the pull requests opened in the period that were merged when they were fetched, unrounded."},
	{"summary.issues_opened", "number", "Issues opened in the period."},
	{"summary.issues_closed", "number", "Issues opened in the period and closed within it."},
	{"summary.issue_close_rate", "number", "The percentage of the issues opened in the period that were closed when they were fetched, unrounded."},
	{"summary.reviews", "number", "Pull request reviews submitted in the period."},

	{"contribution_totals.commits", "number", "Commits."},
	{"contribution_totals.issues", "number", "Issues opened."},
	{"contribution_totals.pull_requests", "number", "Pull requests opened."},
	{"contribution_totals.reviews", "number", "Pull request reviews submitted."},
	{"contribution_totals.repositories", "number", "Repositories created."},

	{"calendar.total_contributions", "number", "The sum of `count` over `days`."},
	{"calendar.reported_total", "number", "The calendar total GitHub reported, summed over the year chunks the period is fetched in."},
	{"calendar.days", "array of objects", "One entry per day of the period, oldest first."},

	{"calendar.days[].date", "string", "The day."},
	{"calendar.days[].count", "number", "Contributions GitHub counted on the day."},
	{"calendar.days[].weekday", "number", "The day of the week, `0` (Monday) to `6` (Sunday)."},

	{"streaks.longest", "number", "The longest run of active days in the period."},
	{"streaks.longest_start", "string or null", "The first day of the longest run; null when no day was active."},
	{"streaks.longest_end", "string or null", "The last day of the longest run; null when no day was active."},
	{"streaks.current", "number", "The run that reaches today or yesterday; 0 when the period ends before today or no run reaches yesterday."},
	{"streaks.current_start", "string or null", "The first day of the current run; null when `current` is 0."},
	{"streaks.active_days", "number", "Active days in the period."},
	{"streaks.total_days", "number", "Days in the period."},
	{"streaks.activity_rate", "number", "`active_days` as a percentage of `total_days`, unrounded."},

	{"top_repos[].repo", "string", "The repository, as `owner/name`."},
	{"top_repos[].count", "number", "Events in the repository."},
	{"top_repos[].private", "boolean", "Whether GitHub reports the repository as private. Internal repositories on GitHub Enterprise count as private."},
	{"top_repos[].id", "number", "The repository's GitHub database ID, which stays the same through renames; 0 when GitHub reported none."},

	{"commits_by_repo[].repo", "string", "The repository, as `owner/name`."},
	{"commits_by_repo[].count", "number", "Commits in the repository in the period."},
	{"commits_by_repo[].private", "boolean", "Whether GitHub reports the repository as private. Internal repositories on GitHub Enterprise count as private."},
	{"commits_by_repo[].id", "number", "The repository's GitHub database ID, which stays the same through renames; 0 when GitHub reported none."},

	{"events[].id", "string", "The event's identifier, unique within the report."},
	{"events[].type", "string", "`PullRequestEvent`, `IssuesEvent`, `PullRequestReviewEvent`, `IssueCommentEvent` (a comment on an issue or pull request) or `CreateEvent` (a repository created)."},
	{"events[].repo", "string", "The repository, as `owner/name`."},
	{"events[].repo_private", "boolean", "Whether GitHub reports the repository as private."},
	{"events[].repo_id", "number", "The repository's GitHub database ID; 0 when GitHub reported none."},
	{"events[].created_at", "string", "When the event happened, as an RFC 3339 timestamp in UTC."},
	{"events[].action", "string", "`opened` or `closed`. On `PullRequestEvent` and `IssuesEvent` only."},
	{"events[].merged", "boolean", "Whether the pull request was merged. On a `PullRequestEvent` whose `action` is `closed` only."},
	{"events[].number", "number", "The pull request's or issue's number. On `PullRequestEvent`, `IssuesEvent` and `PullRequestReviewEvent` only."},
	{"events[].title", "string", "The pull request's or issue's title. On the same events as `number`."},
	{"events[].description", "string", "The repository's description. On a `CreateEvent` whose repository has one only."},
	{"events[].review_state", "string", "The review's state as GitHub reports it, such as `APPROVED`, `CHANGES_REQUESTED` or `COMMENTED`. On `PullRequestReviewEvent` only."},
	{"events[].subject_created_at", "string", "When the pull request or issue was created, as an RFC 3339 timestamp in UTC. On `PullRequestEvent` and `IssuesEvent` only."},
	{"events[].subject_state", "string", "The pull request's or issue's state when it was fetched: `OPEN`, `CLOSED`, or `MERGED` for a pull request. The merge and close rates count opened items by it. On `PullRequestEvent` and `IssuesEvent` only."},
}

const header = `# JSON output reference

Every key ` + "`gh history --format json`" + ` writes, with its JSON type and meaning.
Dates are ` + "`YYYY-MM-DD`" + ` days in the report's time zone, ` + "`date_range.time_zone`" + `.

This file is generated by ` + "`mise run docs`" + ` from
[` + "`examples/report.json`" + `](../examples/report.json). Edit the field list in
[` + "`tools/jsondoc/main.go`" + `](../tools/jsondoc/main.go), not this file.
`

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: jsondoc <report.json> <out.md>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "jsondoc:", err)
		os.Exit(1)
	}
}

// run checks the report against fields and writes the reference only when
// they agree, so a failed run leaves the previous file in place.
func run(in, out string) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}
	documented := make(map[string]string, len(fields))
	for _, f := range fields {
		documented[f.path] = f.typ
	}
	found := map[string]bool{}
	var problems []string
	walk("", report, documented, found, &problems)
	for _, f := range fields {
		if !found[f.path] {
			problems = append(problems, "documented key missing from the report: "+f.path)
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "\n"))
	}
	// Owner-only is enough: git records no permission but the executable bit.
	return os.WriteFile(out, []byte(render()), 0o600)
}

// walk records every key path under prefix and reports undocumented keys and
// type mismatches. It descends into an object only when its documented type
// is a plain object, so the keys of events_by_date and the other count maps
// (dates, hours, event types) are read as data, not as fields.
func walk(prefix string, obj map[string]any, documented map[string]string, found map[string]bool, problems *[]string) {
	for key, value := range obj {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		typ, ok := documented[path]
		if !ok {
			if !found[path] {
				*problems = append(*problems, "undocumented key: "+path)
			}
			found[path] = true
			continue
		}
		if !typeMatches(typ, value) && !found[path] {
			*problems = append(*problems, fmt.Sprintf("%s is a %s, documented as %s", path, jsonType(value), typ))
		}
		found[path] = true
		switch v := value.(type) {
		case map[string]any:
			if typ == "object" || typ == "object or null" {
				walk(path, v, documented, found, problems)
			}
		case []any:
			for _, item := range v {
				if entry, ok := item.(map[string]any); ok {
					walk(path+"[]", entry, documented, found, problems)
				}
			}
		}
	}
}

// jsonType names the JSON type of a value encoding/json decoded into any.
func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	default:
		return "array"
	}
}

// typeMatches reports whether v fits typ: typ's first word is the JSON type,
// and only a typ ending in "or null" admits null.
func typeMatches(typ string, v any) bool {
	if v == nil {
		return strings.HasSuffix(typ, "or null")
	}
	return strings.HasPrefix(typ, jsonType(v))
}

// render writes one table per object, the top level first, in fields' order.
func render() string {
	var b strings.Builder
	b.WriteString(header)
	section := "\x00"
	for _, f := range fields {
		parent, key := "", f.path
		if i := strings.LastIndex(f.path, "."); i >= 0 {
			parent, key = f.path[:i], f.path[i+1:]
		}
		if parent != section {
			section = parent
			title := "Top level"
			if parent != "" {
				title = "`" + parent + "`"
			}
			fmt.Fprintf(&b, "\n## %s\n\n| Key | Type | Meaning |\n| --- | --- | --- |\n", title)
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", key, f.typ, f.meaning)
	}
	return b.String()
}
