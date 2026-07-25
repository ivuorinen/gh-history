package analysis

import (
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/models"
	"github.com/ivuorinen/gh-history/internal/testutil"
)

func TestCalculate(t *testing.T) {
	dr := testutil.SampleDateRange()
	calc := &Calculator{Username: "user", DateRange: dr}
	stats := calc.Calculate(testutil.SampleEvents())

	if stats.TotalEvents != 5 {
		t.Errorf("expected 5 total events, got %d", stats.TotalEvents)
	}
	if stats.PROpened != 1 {
		t.Errorf("expected 1 PR opened, got %d", stats.PROpened)
	}
	if stats.PRMerged != 1 {
		t.Errorf("expected 1 PR merged, got %d", stats.PRMerged)
	}
	if stats.PRClosed != 0 {
		t.Errorf("expected 0 PRs closed-without-merge, got %d", stats.PRClosed)
	}
	if stats.IssuesOpened != 1 {
		t.Errorf("expected 1 issue opened, got %d", stats.IssuesOpened)
	}
	if stats.ReviewsCount != 1 {
		t.Errorf("expected 1 review, got %d", stats.ReviewsCount)
	}

	if stats.Streaks == nil {
		t.Fatal("expected streaks to be set")
	}
	if stats.Streaks.ActiveDays != 4 {
		t.Errorf("expected 4 active days, got %d", stats.Streaks.ActiveDays)
	}
}

func TestCalculateEmpty(t *testing.T) {
	dr := testutil.SampleDateRange()
	calc := &Calculator{Username: "user", DateRange: dr}
	stats := calc.Calculate(nil)

	if stats.TotalEvents != 0 {
		t.Errorf("expected 0 events, got %d", stats.TotalEvents)
	}
	if stats.Streaks == nil {
		t.Fatal("expected streaks to be set even when empty")
	}
	if stats.Streaks.TotalDays != 31 {
		t.Errorf("expected 31 total days, got %d", stats.Streaks.TotalDays)
	}
}

// A user whose activity is entirely in private repositories has calendar days
// and a commit total but no public events. Those inputs must still reach the
// report rather than being skipped along with the empty event list.
func TestCalculate_NoEventsStillUsesCalendarAndCommitTotal(t *testing.T) {
	dr := testutil.SampleDateRange()
	calc := &Calculator{
		Username:  "user",
		DateRange: dr,
		CalendarDays: []models.ContributionDay{
			{Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), ContributionCount: 5},
			{Date: time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC), ContributionCount: 3},
			{Date: time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC), ContributionCount: 7},
		},
		Totals: models.ContributionTotals{Commits: 42},
	}
	stats := calc.Calculate(nil)

	if stats.Streaks == nil {
		t.Fatal("expected streaks to be set")
	}
	if stats.Streaks.ActiveDays != 3 {
		t.Errorf("expected 3 active days from calendar, got %d", stats.Streaks.ActiveDays)
	}
	if stats.Streaks.LongestStreak != 3 {
		t.Errorf("expected longest streak 3 from calendar, got %d", stats.Streaks.LongestStreak)
	}
	if stats.CommitCount != 42 {
		t.Errorf("expected CommitCount 42 from GraphQL total, got %d", stats.CommitCount)
	}
	if stats.Calendar == nil {
		t.Fatal("expected Calendar to be set")
	}
	if stats.Calendar.TotalContributions != 15 {
		t.Errorf("expected 15 total contributions, got %d", stats.Calendar.TotalContributions)
	}
}

func TestCategorizeEvent(t *testing.T) {
	tests := []struct {
		eventType string
		expected  models.Category
	}{
		{"PullRequestEvent", models.CategoryPullRequests},
		{"PullRequestReviewEvent", models.CategoryReviews},
		{"IssuesEvent", models.CategoryIssues},
		{"IssueCommentEvent", models.CategoryComments},
		{"CreateEvent", models.CategoryRepos},
		{"UnknownEvent", models.CategoryOther},
		// Not produced by the GraphQL client; must not silently map anywhere.
		{"PushEvent", models.CategoryOther},
	}
	for _, tc := range tests {
		got := CategorizeEvent(tc.eventType)
		if got != tc.expected {
			t.Errorf("CategorizeEvent(%q) = %q, want %q", tc.eventType, got, tc.expected)
		}
	}
}

func TestTrackDetailedStats(t *testing.T) {
	tests := []struct {
		name  string
		event models.Event
		check func(t *testing.T, s models.Statistics)
	}{
		{
			name:  "PR opened",
			event: models.Event{Type: "PullRequestEvent", Action: models.ActionOpened},
			check: func(t *testing.T, s models.Statistics) {
				if s.PROpened != 1 || s.PRMerged != 0 || s.PRClosed != 0 {
					t.Errorf("got opened=%d merged=%d closed=%d", s.PROpened, s.PRMerged, s.PRClosed)
				}
			},
		},
		{
			name:  "PR closed and merged counts as merged only",
			event: models.Event{Type: "PullRequestEvent", Action: models.ActionClosed, Merged: true},
			check: func(t *testing.T, s models.Statistics) {
				if s.PRMerged != 1 || s.PRClosed != 0 {
					t.Errorf("got merged=%d closed=%d", s.PRMerged, s.PRClosed)
				}
			},
		},
		{
			name:  "PR closed without merge counts as closed",
			event: models.Event{Type: "PullRequestEvent", Action: models.ActionClosed},
			check: func(t *testing.T, s models.Statistics) {
				if s.PRClosed != 1 || s.PRMerged != 0 {
					t.Errorf("got closed=%d merged=%d", s.PRClosed, s.PRMerged)
				}
			},
		},
		{
			name:  "issue opened",
			event: models.Event{Type: "IssuesEvent", Action: models.ActionOpened},
			check: func(t *testing.T, s models.Statistics) {
				if s.IssuesOpened != 1 || s.IssuesClosed != 0 {
					t.Errorf("got opened=%d closed=%d", s.IssuesOpened, s.IssuesClosed)
				}
			},
		},
		{
			name:  "issue closed",
			event: models.Event{Type: "IssuesEvent", Action: models.ActionClosed},
			check: func(t *testing.T, s models.Statistics) {
				if s.IssuesClosed != 1 {
					t.Errorf("got closed=%d", s.IssuesClosed)
				}
			},
		},
		{
			name:  "unknown action increments nothing",
			event: models.Event{Type: "IssuesEvent", Action: "labeled"},
			check: func(t *testing.T, s models.Statistics) {
				if s.IssuesOpened != 0 || s.IssuesClosed != 0 {
					t.Errorf("got opened=%d closed=%d", s.IssuesOpened, s.IssuesClosed)
				}
			},
		},
		{
			name:  "empty action increments nothing",
			event: models.Event{Type: "PullRequestEvent"},
			check: func(t *testing.T, s models.Statistics) {
				if s.PROpened != 0 || s.PRClosed != 0 || s.PRMerged != 0 {
					t.Errorf("got opened=%d closed=%d merged=%d", s.PROpened, s.PRClosed, s.PRMerged)
				}
			},
		},
		{
			name:  "review",
			event: models.Event{Type: "PullRequestReviewEvent"},
			check: func(t *testing.T, s models.Statistics) {
				if s.ReviewsCount != 1 {
					t.Errorf("got reviews=%d", s.ReviewsCount)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stats models.Statistics
			trackDetailedStats(&stats, tc.event)
			tc.check(t, stats)
		})
	}
}

func TestWeekdayMapping(t *testing.T) {
	// Jan 15, 2024 is a Monday
	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	events := []models.Event{{
		ID: "1", Type: "IssueCommentEvent", Repo: "user/repo",
		CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Monday
	}}
	calc := &Calculator{Username: "user", DateRange: dr}
	stats := calc.Calculate(events)

	if stats.EventsByWeekday[0] != 1 { // Monday should be 0
		t.Errorf("Monday event not mapped to weekday 0: %v", stats.EventsByWeekday)
	}
}

func TestCalculate_UsesCalendarForStreaks(t *testing.T) {
	dr := testutil.SampleDateRange()
	calendarDays := []models.ContributionDay{
		{Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), ContributionCount: 3},
		{Date: time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
		{Date: time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC), ContributionCount: 5},
		{Date: time.Date(2024, 1, 13, 0, 0, 0, 0, time.UTC), ContributionCount: 2},
		{Date: time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
	}
	calc := &Calculator{
		Username:     "user",
		DateRange:    dr,
		CalendarDays: calendarDays,
	}
	// Events only cover 4 days (15-18), but calendar covers 5 consecutive days (10-14)
	stats := calc.Calculate(testutil.SampleEvents())

	if stats.Streaks == nil {
		t.Fatal("expected streaks to be set")
	}
	// Calendar has 5 consecutive days → streak of 5
	if stats.Streaks.LongestStreak != 5 {
		t.Errorf("expected longest streak 5 from calendar, got %d", stats.Streaks.LongestStreak)
	}
	if stats.Streaks.ActiveDays != 5 {
		t.Errorf("expected 5 active days from calendar, got %d", stats.Streaks.ActiveDays)
	}
	if stats.Calendar == nil {
		t.Fatal("expected Calendar to be set")
	}
}

func TestCalculate_CommitCountFromGraphQLTotal(t *testing.T) {
	dr := testutil.SampleDateRange()
	calc := &Calculator{
		Username:  "user",
		DateRange: dr,
		Totals:    models.ContributionTotals{Commits: 500},
	}
	stats := calc.Calculate(testutil.SampleEvents())

	if stats.CommitCount != 500 {
		t.Errorf("expected CommitCount 500 (from GraphQL), got %d", stats.CommitCount)
	}
}

func TestCalculate_CalendarDaysFilteredToRange(t *testing.T) {
	dr := daterange.DateRange{
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	// Simulate week-aligned calendar data that extends outside the range
	calendarDays := []models.ContributionDay{
		{Date: time.Date(2024, 12, 29, 0, 0, 0, 0, time.UTC), ContributionCount: 1}, // outside range
		{Date: time.Date(2024, 12, 30, 0, 0, 0, 0, time.UTC), ContributionCount: 1}, // outside range
		{Date: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), ContributionCount: 1}, // outside range
		{Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
		{Date: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
	}
	events := []models.Event{
		{ID: "1", Type: "IssueCommentEvent", Repo: "user/repo",
			CreatedAt: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
	}
	calc := &Calculator{
		Username:     "user",
		DateRange:    dr,
		CalendarDays: calendarDays,
	}
	stats := calc.Calculate(events)

	// Only 2 calendar days should be counted (Jan 1-2), not the 3 from Dec 2024
	if stats.Streaks.ActiveDays != 2 {
		t.Errorf("expected 2 active days (filtered to range), got %d", stats.Streaks.ActiveDays)
	}
	if stats.Streaks.TotalDays != 365 {
		t.Errorf("expected 365 total days, got %d", stats.Streaks.TotalDays)
	}
	// Calendar should also be filtered
	if len(stats.Calendar.Days) != 2 {
		t.Errorf("expected 2 calendar days (filtered), got %d", len(stats.Calendar.Days))
	}
}

func TestCalculate_FallbackWithoutCalendar(t *testing.T) {
	dr := testutil.SampleDateRange()
	calc := &Calculator{Username: "user", DateRange: dr}
	stats := calc.Calculate(testutil.SampleEvents())

	if stats.Streaks == nil {
		t.Fatal("expected streaks to be set")
	}
	// Without calendar, uses events (4 active days from SampleEvents)
	if stats.Streaks.ActiveDays != 4 {
		t.Errorf("expected 4 active days from events, got %d", stats.Streaks.ActiveDays)
	}
	if stats.Calendar != nil {
		t.Error("expected Calendar to be nil without CalendarDays")
	}
}
