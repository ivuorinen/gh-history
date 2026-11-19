package analysis

import (
	"slices"

	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// Calculator computes statistics from GitHub events.
type Calculator struct {
	Username      string
	DateRange     daterange.DateRange
	CalendarDays  []models.ContributionDay
	Totals        models.ContributionTotals
	CommitsByRepo []models.RepoCount
	CalendarTotal int
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
		RepoPrivate:      make(map[string]bool),
		EventsByDate:     make(map[string]int),
		EventsByWeekday:  make(map[int]int),
		EventsByHour:     make(map[int]int),
	}

	// No early return for an empty event slice: CalendarDays and
	// TotalCommitContributions are independent of the event list (they include
	// commits and contributions the token cannot list), so bailing out here
	// would zero the report for a user whose activity the token cannot see.
	for _, event := range events {
		cat := CategorizeEvent(event.Type)
		stats.EventsByCategory[cat]++
		stats.EventsByType[event.Type]++
		stats.EventsByRepo[event.Repo]++
		// GitHub reports visibility as of each query, so a repository whose
		// visibility changes mid-run is seen both ways. One private
		// observation makes it private: the rule is order-independent, so the
		// result no longer depends on which source wrote last.
		stats.RepoPrivate[event.Repo] = stats.RepoPrivate[event.Repo] || event.RepoPrivate
		stats.EventsByDate[event.Date().Format(ghutil.DateFormat)]++

		// Convert Go weekday (0=Sunday) to Python-style (0=Monday)
		wd := int(event.CreatedAt.Weekday())
		wd = (wd + 6) % 7 // Sunday=6, Monday=0, ...
		stats.EventsByWeekday[wd]++
		stats.EventsByHour[event.CreatedAt.Hour()]++

		trackDetailedStats(&stats, event)
	}

	// Filter calendar days to the requested date range. GitHub returns exactly
	// the days from `from` through the day `to` falls on, so a caller that sends
	// a later `to` gets extra days that would inflate active days and streaks.
	// The API client sends the matching boundary; this keeps the guarantee here.
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
	stats.CommitCount = c.Totals.Commits

	// Detail carried through for the JSON format.
	stats.Events = events
	stats.Totals = c.Totals
	// A repository can have commits and no listed event, so the commit counts
	// are a visibility source of their own, merged by the same rule as events.
	for _, rc := range c.CommitsByRepo {
		stats.RepoPrivate[rc.Repo] = stats.RepoPrivate[rc.Repo] || rc.Private
	}
	// The commit rows then take the merged visibility, so commits_by_repo and
	// the top-repositories tables cannot disagree about a repository. They are
	// a copy: the caller's slice stays as fetched.
	stats.CommitsByRepo = slices.Clone(c.CommitsByRepo)
	for i := range stats.CommitsByRepo {
		stats.CommitsByRepo[i].Private = stats.RepoPrivate[stats.CommitsByRepo[i].Repo]
	}

	// Build calendar on stats
	if len(filteredDays) > 0 {
		cal := &models.ContributionCalendar{
			Days:          filteredDays,
			ReportedTotal: c.CalendarTotal,
		}
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
			if event.SubjectState == "MERGED" {
				stats.PROpenedMerged++
			}
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
			if event.SubjectState == "CLOSED" {
				stats.IssuesOpenedClosed++
			}
		case models.ActionClosed:
			stats.IssuesClosed++
		}
	case "PullRequestReviewEvent":
		stats.ReviewsCount++
	}
}
