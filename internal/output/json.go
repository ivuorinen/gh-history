package output

import (
	"encoding/json"

	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// FormatJSON returns the statistics as a JSON byte slice.
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
		"events_by_category": stats.EventsByCategory,
		"events_by_type":     stats.EventsByType,
		"events_by_repo":     stats.EventsByRepo,
		"top_repos":          stats.TopRepos(15),
		"events_by_date":     stats.EventsByDate,
		"events_by_weekday":  stats.EventsByWeekday,
		"events_by_hour":     stats.EventsByHour,
	}

	// The contribution calendar includes private-repository activity, so it is
	// not derivable from events_by_date. Emit it rather than computing it and
	// dropping it on the floor.
	if stats.Calendar != nil {
		days := make([]map[string]any, 0, len(stats.Calendar.Days))
		for _, d := range stats.Calendar.Days {
			days = append(days, map[string]any{
				"date":  d.Date.Format(ghutil.DateFormat),
				"count": d.ContributionCount,
			})
		}
		data["calendar"] = map[string]any{
			"total_contributions": stats.Calendar.TotalContributions,
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
