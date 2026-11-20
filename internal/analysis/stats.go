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
		RepoID:           make(map[string]int64),
		EventsByDate:     make(map[string]int),
		EventsByWeekday:  make(map[int]int),
		EventsByHour:     make(map[int]int),
	}

	// No early return for an empty event slice: CalendarDays and
	// TotalCommitContributions are independent of the event list (they include
	// commits and contributions the token cannot list), so bailing out here
	// would zero the report for a user whose activity the token cannot see.
	//
	// The date, weekday and hour buckets read each event in the report's zone,
	// the zone GitHub's calendar uses, so every figure counts the same days.
	loc := c.DateRange.Location()
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
		// Every source reports the same ID for a repository, or 0 when it has
		// none. max keeps a 0 from erasing a real ID, in any order.
		stats.RepoID[event.Repo] = max(stats.RepoID[event.Repo], event.RepoID)
		stats.EventsByDate[event.Date(loc).Format(ghutil.DateFormat)]++

		local := event.CreatedAt.In(loc)
		// Convert Go weekday (0=Sunday) to Python-style (0=Monday)
		wd := int(local.Weekday())
		wd = (wd + 6) % 7 // Sunday=6, Monday=0, ...
		stats.EventsByWeekday[wd]++
		stats.EventsByHour[local.Hour()]++

		trackDetailedStats(&stats, event)
	}

	filteredDays := daysInRange(c.CalendarDays, c.DateRange)

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
	stats.CommitsByRepo = mergeCommitRows(&stats, c.CommitsByRepo)
	stats.Calendar = calendarOf(filteredDays, c.CalendarTotal)
	return stats
}

// daysInRange returns the calendar days inside dr. GitHub returns exactly the
// days from `from` through the day `to` falls on, so a caller that sends a
// later `to` gets extra days that would inflate active days and streaks. The
// API client sends the matching boundary; this keeps the guarantee here.
func daysInRange(days []models.ContributionDay, dr daterange.DateRange) []models.ContributionDay {
	inRange := make([]models.ContributionDay, 0, len(days))
	for _, d := range days {
		if !d.Date.Before(dr.Start) && !d.Date.After(dr.End) {
			inRange = append(inRange, d)
		}
	}
	return inRange
}

// mergeCommitRows folds the commit rows' visibility and IDs into stats and
// returns a copy of the rows carrying the merged values. A repository can have
// commits and no listed event, so the rows are a source of visibility and ID of
// their own, merged by the same rules as events. The rows then take the merged
// values, so commits_by_repo and the top-repositories tables cannot disagree
// about a repository. The caller's slice stays as fetched.
func mergeCommitRows(stats *models.Statistics, rows []models.RepoCount) []models.RepoCount {
	for _, rc := range rows {
		stats.RepoPrivate[rc.Repo] = stats.RepoPrivate[rc.Repo] || rc.Private
		stats.RepoID[rc.Repo] = max(stats.RepoID[rc.Repo], rc.ID)
	}
	merged := slices.Clone(rows)
	for i := range merged {
		merged[i].Private = stats.RepoPrivate[merged[i].Repo]
		merged[i].ID = stats.RepoID[merged[i].Repo]
	}
	return merged
}

// calendarOf builds the report's calendar from the days in range, or returns
// nil when there are none. TotalContributions is summed from those days;
// reportedTotal is GitHub's own figure, kept beside it.
func calendarOf(days []models.ContributionDay, reportedTotal int) *models.ContributionCalendar {
	if len(days) == 0 {
		return nil
	}
	cal := &models.ContributionCalendar{Days: days, ReportedTotal: reportedTotal}
	for _, d := range days {
		cal.TotalContributions += d.ContributionCount
	}
	return cal
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
