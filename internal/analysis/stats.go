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

	if len(events) == 0 {
		streaks := CalculateStreaks(nil, c.DateRange.Start, c.DateRange.End)
		stats.Streaks = &streaks
		return stats
	}

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
		streaks := CalculateStreaksFromCalendar(filteredDays, c.DateRange.Start, c.DateRange.End)
		stats.Streaks = &streaks
	} else {
		streaks := CalculateStreaks(events, c.DateRange.Start, c.DateRange.End)
		stats.Streaks = &streaks
	}

	// Use GraphQL commit total if higher than event-based count
	if c.TotalCommitContributions > stats.CommitCount {
		stats.CommitCount = c.TotalCommitContributions
	}

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
	case "PushEvent":
		stats.CommitCount += countCommits(event.Payload)
	case "PullRequestEvent":
		trackActionCount(event, &stats.PROpened, &stats.PRClosed, func(pr map[string]any) bool {
			if merged, ok := pr["merged"].(bool); ok && merged {
				stats.PRMerged++
				return true
			}
			return false
		})
	case "IssuesEvent":
		trackActionCount(event, &stats.IssuesOpened, &stats.IssuesClosed, nil)
	case "PullRequestReviewEvent":
		stats.ReviewsCount++
	}
}

// countCommits extracts the commit count from a PushEvent payload.
// It tries the "commits" array first, then "size" (as float64 from JSON or int from Go),
// and falls back to 1 since every PushEvent represents at least one commit.
func countCommits(payload map[string]any) int {
	if commits, ok := payload["commits"].([]any); ok && len(commits) > 0 {
		return len(commits)
	}
	if size, ok := payload["size"].(float64); ok && size > 0 {
		return int(size)
	}
	if size, ok := payload["size"].(int); ok && size > 0 {
		return size
	}
	return 1
}

// trackActionCount handles the common opened/closed action pattern for PRs and Issues.
// For "closed" events, if checkMerged is non-nil it is called with the payload to
// allow distinguishing merged PRs from closed ones. If checkMerged returns true,
// *closed is NOT incremented (the caller handled it as a merge).
func trackActionCount(event models.Event, opened, closed *int, checkMerged func(map[string]any) bool) {
	action, _ := event.Payload["action"].(string)
	switch action {
	case "opened":
		*opened++
	case "closed":
		if checkMerged != nil {
			if pr, ok := event.Payload["pull_request"].(map[string]any); ok {
				if checkMerged(pr) {
					return
				}
			}
		}
		*closed++
	}
}
