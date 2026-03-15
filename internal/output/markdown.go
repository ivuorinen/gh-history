package output

import (
	"fmt"
	"sort"
	"strings"

	ghText "github.com/cli/go-gh/v2/pkg/text"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// FormatMarkdown returns a Markdown-formatted report.
func FormatMarkdown(stats models.Statistics) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# GitHub Activity Report: %s\n\n", stats.Username)
	fmt.Fprintf(&b, "**Period:** %s to %s\n\n",
		stats.DateRange.Start.Format(ghutil.DateFormat),
		stats.DateRange.End.Format(ghutil.DateFormat))

	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(&b, "| Total Events | %d |\n", stats.TotalEvents)
	fmt.Fprintf(&b, "| Commits | %d |\n", stats.CommitCount)
	fmt.Fprintf(&b, "| PRs Opened | %d |\n", stats.PROpened)
	fmt.Fprintf(&b, "| PRs Merged | %d |\n", stats.PRMerged)
	fmt.Fprintf(&b, "| Code Reviews | %d |\n", stats.ReviewsCount)

	if stats.Streaks != nil {
		s := stats.Streaks
		fmt.Fprintf(&b, "| Active Days | %d / %d |\n", s.ActiveDays, s.TotalDays)
		fmt.Fprintf(&b, "| Longest Streak | %s |\n", ghText.Pluralize(s.LongestStreak, "day"))
		fmt.Fprintf(&b, "| Current Streak | %s |\n", ghText.Pluralize(s.CurrentStreak, "day"))
	}

	b.WriteString("\n## Activity by Category\n\n")
	b.WriteString("| Category | Count | Bar | Percentage |\n")
	b.WriteString("|----------|-------|-----|------------|\n")

	entries := BuildCategoryBars(stats, 20, AllCategories)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	for _, entry := range entries {
		fmt.Fprintf(&b, "| %s | %d | %s | %.1f%% |\n", entry.Label, entry.Count, entry.Bar, entry.Percent)
	}

	if weekdayEntries := BuildWeekdayBars(stats, 20); len(weekdayEntries) > 0 {
		b.WriteString("\n## Activity by Day of Week\n\n")
		b.WriteString("| Day | Count | Bar | Percentage |\n")
		b.WriteString("|-----|-------|-----|------------|\n")
		for _, entry := range weekdayEntries {
			fmt.Fprintf(&b, "| %s | %d | %s | %.1f%% |\n", entry.Label, entry.Count, entry.Bar, entry.Percent)
		}
	}

	if hourlyEntries := BuildHourlyBars(stats, 20); len(hourlyEntries) > 0 {
		b.WriteString("\n## Activity by Hour (UTC)\n\n")
		b.WriteString("| Hour | Count | Bar | Percentage |\n")
		b.WriteString("|------|-------|-----|------------|\n")
		for _, entry := range hourlyEntries {
			fmt.Fprintf(&b, "| %s | %d | %s | %.1f%% |\n", entry.Label, entry.Count, entry.Bar, entry.Percent)
		}
	}

	topRepos := stats.TopRepos(15)
	if len(topRepos) > 0 {
		b.WriteString("\n## Top Repositories\n\n")
		b.WriteString("| # | Repository | Events |\n")
		b.WriteString("|---|------------|--------|\n")
		for i, rc := range topRepos {
			fmt.Fprintf(&b, "| %d | %s | %d |\n", i+1, rc.Repo, rc.Count)
		}
	}

	return b.String()
}
