package testutil

import (
	"time"

	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/models"
)

// MakeEvent creates a minimal event on the given date, for tests that care only
// about the date. IssueCommentEvent is used because it carries no action.
func MakeEvent(year, month, day int) models.Event {
	return MakeTypedEvent("IssueCommentEvent", year, month, day, "")
}

// MakeTypedEvent creates an event of the given type and action on the given date.
func MakeTypedEvent(eventType string, year, month, day int, action string) models.Event {
	d := time.Date(year, time.Month(month), day, 12, 0, 0, 0, time.UTC)
	return models.Event{
		ID:        d.Format("20060102") + "-" + eventType,
		Type:      eventType,
		Actor:     "user",
		Repo:      "user/repo",
		Action:    action,
		CreatedAt: d,
	}
}

// SampleEvents returns a standard 5-event set for testing statistics calculation.
func SampleEvents() []models.Event {
	return []models.Event{
		{
			ID: "1", Type: "IssueCommentEvent", Actor: "user", Repo: "user/repo1",
			CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			ID: "2", Type: "PullRequestEvent", Actor: "user", Repo: "user/repo1",
			Action:    models.ActionOpened,
			CreatedAt: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			ID: "3", Type: "PullRequestEvent", Actor: "user", Repo: "user/repo1",
			Action: models.ActionClosed, Merged: true,
			CreatedAt: time.Date(2024, 1, 16, 9, 0, 0, 0, time.UTC),
		},
		{
			ID: "4", Type: "IssuesEvent", Actor: "user", Repo: "user/repo2",
			Action:    models.ActionOpened,
			CreatedAt: time.Date(2024, 1, 17, 14, 0, 0, 0, time.UTC),
		},
		{
			ID: "5", Type: "PullRequestReviewEvent", Actor: "user", Repo: "user/repo1",
			CreatedAt: time.Date(2024, 1, 18, 16, 0, 0, 0, time.UTC),
		},
	}
}

// SampleDateRange returns January 2024 as a DateRange.
func SampleDateRange() daterange.DateRange {
	return daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
}

// SampleStats returns a complete Statistics struct suitable for formatter tests.
func SampleStats() models.Statistics {
	return models.Statistics{
		Username:    "testuser",
		DateRange:   SampleDateRange(),
		TotalEvents: 100,
		EventsByCategory: map[models.Category]int{
			models.CategoryPullRequests: 20,
			models.CategoryIssues:       15,
			models.CategoryReviews:      10,
			models.CategoryComments:     5,
			models.CategoryRepos:        50,
		},
		EventsByType: map[string]int{
			"CreateEvent":            50,
			"PullRequestEvent":       20,
			"IssuesEvent":            15,
			"PullRequestReviewEvent": 10,
			"IssueCommentEvent":      5,
		},
		EventsByRepo: map[string]int{
			"testuser/repo1": 60,
			"testuser/repo2": 30,
			"testuser/repo3": 10,
		},
		EventsByDate: map[string]int{
			"2024-01-15": 10,
			"2024-01-16": 5,
		},
		EventsByWeekday: map[int]int{0: 30, 1: 25, 2: 20, 3: 15, 4: 10},
		EventsByHour:    map[int]int{9: 20, 10: 30, 14: 25, 16: 15, 20: 10},
		Streaks: &models.StreakInfo{
			LongestStreak: 5,
			ActiveDays:    15,
			TotalDays:     31,
		},
		Calendar: &models.ContributionCalendar{
			TotalContributions: 24,
			Days:               SampleCalendarDays(),
		},
		CommitCount:  80,
		PROpened:     10,
		PRMerged:     8,
		PRClosed:     2,
		IssuesOpened: 15,
		IssuesClosed: 12,
		ReviewsCount: 10,
	}
}

// SampleCalendarDays returns contribution calendar days for testing.
func SampleCalendarDays() []models.ContributionDay {
	var days []models.ContributionDay
	counts := []int{3, 0, 5, 2, 1, 0, 4, 1, 0, 2, 6}
	for i, count := range counts {
		days = append(days, models.ContributionDay{
			Date:              time.Date(2024, 1, 10+i, 0, 0, 0, 0, time.UTC),
			ContributionCount: count,
		})
	}
	return days
}
