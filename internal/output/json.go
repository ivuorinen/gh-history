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
		"top_repos":          stats.TopRepos(15),
		"events_by_weekday":  stats.EventsByWeekday,
		"events_by_hour":     stats.EventsByHour,
	}

	if stats.Streaks != nil {
		s := stats.Streaks
		streaks := map[string]any{
			"longest":       s.LongestStreak,
			"longest_start": nil,
			"longest_end":   nil,
			"current":       s.CurrentStreak,
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
		data["streaks"] = streaks
	} else {
		data["streaks"] = nil
	}

	return json.MarshalIndent(data, "", "  ")
}
