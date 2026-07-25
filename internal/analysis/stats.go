package analysis

import (
	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// Calculator computes statistics from GitHub events.
type Calculator struct {
	Username                 string
	DateRange                daterange.DateRange
	CalendarDays             []models.ContributionDay
	TotalCommitContributions int
}

// Calculate processes events and returns computed statistics.
func (c *Calculator) Calculate(events []models.Event) models.Statistics {
	stats := models.Statistics{
		Username:         c.Username,
		DateRange:        c.DateRange,
		TotalEvents:      len(events),
		EventsByCategory: make(map[models.Category]int),
		EventsByType:     make(map[string]int),
		EventsByRepo:     make(map[string]int),
		EventsByDate:     make(map[string]int),
		EventsByWeekday:  make(map[int]int),
		EventsByHour:     make(map[int]int),
	}

	// No early return for an empty event slice: CalendarDays and
	// TotalCommitContributions are independent of the public event list (they
	// include private-repository activity), so bailing out here would zero the
	// report for users whose contributions are all private.
	for _, event := range events {
		cat := CategorizeEvent(event.Type)
		stats.EventsByCategory[cat]++
		stats.EventsByType[event.Type]++
		stats.EventsByRepo[event.Repo]++
		stats.EventsByDate[event.Date().Format(ghutil.DateFormat)]++

		// Convert Go weekday (0=Sunday) to Python-style (0=Monday)
		wd := int(event.CreatedAt.Weekday())
		wd = (wd + 6) % 7 // Sunday=6, Monday=0, ...
		stats.EventsByWeekday[wd]++
		stats.EventsByHour[event.CreatedAt.Hour()]++

		trackDetailedStats(&stats, event)
	}

	// Filter calendar days to the requested date range.
	// GitHub's contributionCalendar returns week-aligned data that can include
	// days outside the range, which would inflate active day counts and streaks.
	filteredDays := make([]models.ContributionDay, 0, len(c.CalendarDays))
	for _, d := range c.CalendarDays {
		if !d.Date.Before(c.DateRange.Start) && !d.Date.After(c.DateRange.End) {
			filteredDays = append(filteredDays, d)
		}
	}

	// Prefer calendar-based streaks (includes private repos)
	if len(filteredDays) > 0 {
		streaks := CalculateStreaksFromCalendar(filteredDays, c.DateRange)
		stats.Streaks = &streaks
	} else {
		streaks := CalculateStreaks(events, c.DateRange)
		stats.Streaks = &streaks
	}

	// Commit counts come from the GraphQL contributions total, which covers
	// private repositories. No synthesized event carries commit data.
	stats.CommitCount = c.TotalCommitContributions

	// Build calendar on stats
	if len(filteredDays) > 0 {
		cal := &models.ContributionCalendar{Days: filteredDays}
		for _, d := range filteredDays {
			cal.TotalContributions += d.ContributionCount
		}
		stats.Calendar = cal
	}

	return stats
}

func trackDetailedStats(stats *models.Statistics, event models.Event) {
	switch event.Type {
	case "PullRequestEvent":
		switch event.Action {
		case models.ActionOpened:
			stats.PROpened++
		case models.ActionClosed:
			// Merged and closed-without-merge are mutually exclusive.
			if event.Merged {
				stats.PRMerged++
			} else {
				stats.PRClosed++
			}
		}
	case "IssuesEvent":
		switch event.Action {
		case models.ActionOpened:
			stats.IssuesOpened++
		case models.ActionClosed:
			stats.IssuesClosed++
		}
	case "PullRequestReviewEvent":
		stats.ReviewsCount++
	}
}
