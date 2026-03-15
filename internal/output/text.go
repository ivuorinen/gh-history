package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/cli/go-gh/v2/pkg/term"
	ghText "github.com/cli/go-gh/v2/pkg/text"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// AllCategories lists categories in display order.
var AllCategories = []models.Category{
	models.CategoryCommits,
	models.CategoryPullRequests,
	models.CategoryIssues,
	models.CategoryReviews,
	models.CategoryComments,
	models.CategoryRepos,
	models.CategoryReleases,
	models.CategoryOther,
}

// FormatText writes a plain text report to stdout using terminal-aware table formatting.
func FormatText(stats models.Statistics) {
	t := term.FromEnv()
	isTTY := t.IsTerminalOutput()
	width := 80
	if w, _, err := t.Size(); err == nil && w > 0 {
		width = w
	}
	FormatTextTo(t.Out(), isTTY, width, stats)
}

// FormatTextTo writes a plain text report to w. This is the testable core of FormatText.
func FormatTextTo(w io.Writer, isTTY bool, width int, stats models.Statistics) {
	// Header
	fmt.Fprintf(w, "GitHub Activity Report: %s\n", stats.Username)
	fmt.Fprintf(w, "%s to %s\n", stats.DateRange.Start.Format(ghutil.DateFormat), stats.DateRange.End.Format(ghutil.DateFormat))
	fmt.Fprintln(w, strings.Repeat("-", 60))

	// Summary table
	fmt.Fprintln(w, "\nSummary")
	fmt.Fprintln(w, strings.Repeat("-", 50))

	tp := tableprinter.New(w, isTTY, width)
	tp.AddField("Total Events")
	tp.AddField(fmtInt(stats.TotalEvents))
	tp.EndRow()

	if stats.Streaks != nil {
		s := stats.Streaks
		tp.AddField("Active Days")
		tp.AddField(fmt.Sprintf("%d / %d (%.1f%%)", s.ActiveDays, s.TotalDays, s.ActivityRate()))
		tp.EndRow()
		tp.AddField("Longest Streak")
		tp.AddField(ghText.Pluralize(s.LongestStreak, "day"))
		tp.EndRow()
		tp.AddField("Current Streak")
		tp.AddField(ghText.Pluralize(s.CurrentStreak, "day"))
		tp.EndRow()
	}

	tp.AddField("Commits")
	tp.AddField(fmtInt(stats.CommitCount))
	tp.EndRow()
	tp.AddField("PRs Opened")
	tp.AddField(fmt.Sprintf("%d", stats.PROpened))
	tp.EndRow()
	tp.AddField("PRs Merged")
	tp.AddField(fmt.Sprintf("%d", stats.PRMerged))
	tp.EndRow()
	tp.AddField("Reviews")
	tp.AddField(fmt.Sprintf("%d", stats.ReviewsCount))
	tp.EndRow()
	tp.Render()

	// Categories — keep manual formatting for the Unicode bar chart
	fmt.Fprintln(w, "\nActivity by Category")
	fmt.Fprintln(w, strings.Repeat("-", 50))

	for _, entry := range BuildCategoryBars(stats, 20, AllCategories) {
		fmt.Fprintf(w, "  %s %6s  %s  %5.1f%%\n", ghText.PadRight(18, entry.Label), fmtInt(entry.Count), entry.Bar, entry.Percent)
	}

	// Weekday distribution
	if weekdayEntries := BuildWeekdayBars(stats, 20); len(weekdayEntries) > 0 {
		fmt.Fprintln(w, "\nActivity by Day of Week")
		fmt.Fprintln(w, strings.Repeat("-", 50))
		for _, entry := range weekdayEntries {
			fmt.Fprintf(w, "  %s %6s  %s  %5.1f%%\n", ghText.PadRight(18, entry.Label), fmtInt(entry.Count), entry.Bar, entry.Percent)
		}
	}

	// Hourly distribution
	if hourlyEntries := BuildHourlyBars(stats, 20); len(hourlyEntries) > 0 {
		fmt.Fprintln(w, "\nActivity by Hour (UTC)")
		fmt.Fprintln(w, strings.Repeat("-", 50))
		for _, entry := range hourlyEntries {
			fmt.Fprintf(w, "  %s %6s  %s  %5.1f%%\n", ghText.PadRight(18, entry.Label), fmtInt(entry.Count), entry.Bar, entry.Percent)
		}
	}

	// Top repos table
	topRepos := stats.TopRepos(15)
	if len(topRepos) > 0 {
		fmt.Fprintln(w, "\nTop Repositories")
		fmt.Fprintln(w, strings.Repeat("-", 50))

		tp2 := tableprinter.New(w, isTTY, width)
		for i, rc := range topRepos {
			tp2.AddField(fmt.Sprintf("%d.", i+1))
			tp2.AddField(rc.Repo)
			tp2.AddField(fmt.Sprintf("%s events", fmtInt(rc.Count)))
			tp2.EndRow()
		}
		tp2.Render()
	}
}

func fmtInt(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}
