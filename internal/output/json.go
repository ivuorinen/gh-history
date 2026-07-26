package output

import (
	"encoding/json"
	"time"

	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// FormatJSON returns the statistics as a JSON byte slice.
//
// JSON is the full offering: it carries everything the tool knows, including
// detail the human-readable formats deliberately omit (the event list, GitHub's
// own private-inclusive totals, and the per-repository commit breakdown). The
// shared summary still matches the other formats exactly — JSON only adds.
func FormatJSON(stats models.Statistics) ([]byte, error) {
	data := map[string]any{
		"username": stats.Username,
		"date_range": map[string]string{
			"start": stats.DateRange.Start.Format(ghutil.DateFormat),
			"end":   stats.DateRange.End.Format(ghutil.DateFormat),
		},
		"summary": map[string]int{
			"total_events":  stats.TotalEvents,
			"commits":       stats.CommitCount,
			"prs_opened":    stats.PROpened,
			"prs_merged":    stats.PRMerged,
			"prs_closed":    stats.PRClosed,
			"issues_opened": stats.IssuesOpened,
			"issues_closed": stats.IssuesClosed,
			"reviews":       stats.ReviewsCount,
		},
		// GitHub's own counts for the period. These include private
		// repositories, so they are generally higher than the event-derived
		// figures in "summary", which can only see public activity.
		"contribution_totals": map[string]int{
			"commits":       stats.Totals.Commits,
			"issues":        stats.Totals.Issues,
			"pull_requests": stats.Totals.PullRequests,
			"reviews":       stats.Totals.Reviews,
			"repositories":  stats.Totals.Repositories,
		},
		"events_by_category": stats.EventsByCategory,
		"events_by_type":     stats.EventsByType,
		"events_by_repo":     stats.EventsByRepo,
		"top_repos":          repoCounts(stats.TopRepos(15)),
		"events_by_date":     stats.EventsByDate,
		"events_by_weekday":  stats.EventsByWeekday,
		"events_by_hour":     stats.EventsByHour,
		"commits_by_repo":    repoCounts(stats.CommitsByRepo),
		"events":             jsonEvents(stats.Events),
	}

	// The contribution calendar includes private-repository activity, so it is
	// not derivable from events_by_date. Emit it rather than computing it and
	// dropping it on the floor.
	if stats.Calendar != nil {
		days := make([]map[string]any, 0, len(stats.Calendar.Days))
		for _, d := range stats.Calendar.Days {
			days = append(days, map[string]any{
				"date":    d.Date.Format(ghutil.DateFormat),
				"count":   d.ContributionCount,
				"weekday": d.Weekday(), // 0=Monday, matching events_by_weekday
			})
		}
		data["calendar"] = map[string]any{
			"total_contributions": stats.Calendar.TotalContributions,
			"reported_total":      stats.Calendar.ReportedTotal,
			"days":                days,
		}
	} else {
		data["calendar"] = nil
	}

	if stats.Streaks != nil {
		s := stats.Streaks
		streaks := map[string]any{
			"longest":       s.LongestStreak,
			"longest_start": nil,
			"longest_end":   nil,
			"current":       s.CurrentStreak,
			"current_start": nil,
			"active_days":   s.ActiveDays,
			"total_days":    s.TotalDays,
			"activity_rate": s.ActivityRate(),
		}
		if s.LongestStreakStart != nil {
			streaks["longest_start"] = s.LongestStreakStart.Format(ghutil.DateFormat)
		}
		if s.LongestStreakEnd != nil {
			streaks["longest_end"] = s.LongestStreakEnd.Format(ghutil.DateFormat)
		}
		if s.CurrentStreakStart != nil {
			streaks["current_start"] = s.CurrentStreakStart.Format(ghutil.DateFormat)
		}
		data["streaks"] = streaks
	} else {
		data["streaks"] = nil
	}

	return json.MarshalIndent(data, "", "  ")
}

// repoCounts renders a RepoCount slice with explicit keys rather than relying on
// Go field names, so the JSON contract does not move when the struct is renamed.
// Every RepoCount in the document goes through here, so the casing is uniform.
func repoCounts(counts []models.RepoCount) []map[string]any {
	out := make([]map[string]any, 0, len(counts))
	for _, rc := range counts {
		out = append(out, map[string]any{"repo": rc.Repo, "count": rc.Count})
	}
	return out
}

// jsonEvents renders the event list. Fields that do not apply to an event type
// are omitted rather than emitted as zero values, so a consumer can distinguish
// "no title" from "empty title".
func jsonEvents(events []models.Event) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		m := map[string]any{
			"id":         e.ID,
			"type":       e.Type,
			"repo":       e.Repo,
			"created_at": e.CreatedAt.UTC().Format(time.RFC3339),
		}
		if e.Action != "" {
			m["action"] = e.Action
			if e.Type == "PullRequestEvent" && e.Action == models.ActionClosed {
				m["merged"] = e.Merged
			}
		}
		if e.Number != 0 {
			m["number"] = e.Number
		}
		if e.Title != "" {
			m["title"] = e.Title
		}
		if e.Description != "" {
			m["description"] = e.Description
		}
		if e.ReviewState != "" {
			m["review_state"] = e.ReviewState
		}
		if !e.SubjectCreatedAt.IsZero() {
			m["subject_created_at"] = e.SubjectCreatedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, m)
	}
	return out
}
