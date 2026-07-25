package output

import (
	"fmt"

	ghText "github.com/cli/go-gh/v2/pkg/text"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// SummaryRow is one label/value pair of the report summary.
type SummaryRow struct {
	Label string
	Value string
}

// BuildSummary returns the canonical summary rows every output format renders.
// Formats share this list so that --format changes presentation only, never
// which statistics a report contains.
func BuildSummary(stats models.Statistics) []SummaryRow {
	rows := []SummaryRow{
		{"Total Events", fmtInt(stats.TotalEvents)},
		{"Commits", fmtInt(stats.CommitCount)},
		{"PRs Opened", fmtInt(stats.PROpened)},
		{"PRs Merged", fmtInt(stats.PRMerged)},
		{"PRs Closed", fmtInt(stats.PRClosed)},
		{"Issues Opened", fmtInt(stats.IssuesOpened)},
		{"Issues Closed", fmtInt(stats.IssuesClosed)},
		{"Code Reviews", fmtInt(stats.ReviewsCount)},
	}
	if s := stats.Streaks; s != nil {
		rows = append(rows,
			SummaryRow{"Active Days", fmt.Sprintf("%s / %s (%.1f%%)",
				fmtInt(s.ActiveDays), fmtInt(s.TotalDays), s.ActivityRate())},
			SummaryRow{"Longest Streak", ghText.Pluralize(s.LongestStreak, "day")},
			SummaryRow{"Current Streak", ghText.Pluralize(s.CurrentStreak, "day")},
		)
	}
	return rows
}

// contributionCounts returns per-day contribution counts for the heatmap,
// preferring the GraphQL contribution calendar (which includes private
// repositories) over public event dates. Streak statistics make the same choice
// in analysis.Calculate; using a different source here would make the heatmap
// contradict the streak figures in the same report.
func contributionCounts(stats models.Statistics) (counts map[string]int, fromCalendar bool) {
	if stats.Calendar != nil && len(stats.Calendar.Days) > 0 {
		counts = make(map[string]int, len(stats.Calendar.Days))
		for _, d := range stats.Calendar.Days {
			counts[d.Date.Format(ghutil.DateFormat)] = d.ContributionCount
		}
		return counts, true
	}
	return stats.EventsByDate, false
}
