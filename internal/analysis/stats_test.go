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
	if stats.CommitCount != 2 {
		t.Errorf("expected 2 commits, got %d", stats.CommitCount)
	}
	if stats.PROpened != 1 {
		t.Errorf("expected 1 PR opened, got %d", stats.PROpened)
	}
	if stats.PRMerged != 1 {
		t.Errorf("expected 1 PR merged, got %d", stats.PRMerged)
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
}

func TestCategorizeEvent(t *testing.T) {
	tests := []struct {
		eventType string
		expected  models.Category
	}{
		{"PushEvent", models.CategoryCommits},
		{"PullRequestEvent", models.CategoryPullRequests},
		{"IssueCommentEvent", models.CategoryComments},
		{"UnknownEvent", models.CategoryOther},
	}
	for _, tc := range tests {
		got := CategorizeEvent(tc.eventType)
		if got != tc.expected {
			t.Errorf("CategorizeEvent(%q) = %q, want %q", tc.eventType, got, tc.expected)
		}
	}
}

func TestTrackActionCount_PRClosedWithoutPullRequestPayload(t *testing.T) {
	// PR closed but no pull_request key in payload — should increment closed
	event := models.Event{
		ID: "1", Type: "PullRequestEvent", Actor: "user", Repo: "user/repo",
		Payload:   map[string]any{"action": "closed"},
		CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	var opened, closed int
	trackActionCount(event, &opened, &closed, func(pr map[string]any) bool {
		return false
	})
	if opened != 0 {
		t.Errorf("expected 0 opened, got %d", opened)
	}
	if closed != 1 {
		t.Errorf("expected 1 closed, got %d", closed)
	}
}

func TestTrackActionCount_PRClosedNotMerged(t *testing.T) {
	event := models.Event{
		ID: "1", Type: "PullRequestEvent", Actor: "user", Repo: "user/repo",
		Payload:   map[string]any{"action": "closed", "pull_request": map[string]any{"merged": false}},
		CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	var opened, closed int
	trackActionCount(event, &opened, &closed, func(pr map[string]any) bool {
		if merged, ok := pr["merged"].(bool); ok && merged {
			return true
		}
		return false
	})
	if closed != 1 {
		t.Errorf("expected 1 closed (not merged), got %d", closed)
	}
}

func TestTrackActionCount_UnknownAction(t *testing.T) {
	event := models.Event{
		ID: "1", Type: "IssuesEvent", Actor: "user", Repo: "user/repo",
		Payload:   map[string]any{"action": "labeled"},
		CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	var opened, closed int
	trackActionCount(event, &opened, &closed, nil)
	if opened != 0 || closed != 0 {
		t.Errorf("expected 0/0 for unknown action, got %d/%d", opened, closed)
	}
}

func TestTrackActionCount_NoActionField(t *testing.T) {
	event := models.Event{
		ID: "1", Type: "IssuesEvent", Actor: "user", Repo: "user/repo",
		Payload:   map[string]any{},
		CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}
	var opened, closed int
	trackActionCount(event, &opened, &closed, nil)
	if opened != 0 || closed != 0 {
		t.Errorf("expected 0/0 for no action, got %d/%d", opened, closed)
	}
}

func TestCountCommits(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		want    int
	}{
		{
			name:    "commits array with 2 items",
			payload: map[string]any{"commits": []any{map[string]any{"sha": "a"}, map[string]any{"sha": "b"}}},
			want:    2,
		},
		{
			name:    "size as float64 (from JSON unmarshal)",
			payload: map[string]any{"size": float64(3)},
			want:    3,
		},
		{
			name:    "size as int (from Go construction)",
			payload: map[string]any{"size": 5},
			want:    5,
		},
		{
			name:    "empty payload falls back to 1",
			payload: map[string]any{},
			want:    1,
		},
		{
			name:    "nil commits falls back to size",
			payload: map[string]any{"commits": nil, "size": float64(4)},
			want:    4,
		},
		{
			name:    "empty commits array falls back to size",
			payload: map[string]any{"commits": []any{}, "size": float64(2)},
			want:    2,
		},
		{
			name:    "zero size falls back to 1",
			payload: map[string]any{"size": float64(0)},
			want:    1,
		},
		{
			name:    "commits takes priority over size",
			payload: map[string]any{"commits": []any{map[string]any{"sha": "a"}}, "size": float64(5)},
			want:    1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := countCommits(tc.payload)
			if got != tc.want {
				t.Errorf("countCommits() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCountCommits_InCalculate(t *testing.T) {
	// Verify CommitCount is always >= PushEvent count
	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	events := []models.Event{
		{
			ID: "1", Type: "PushEvent", Actor: "user", Repo: "user/repo",
			Payload:   map[string]any{}, // empty payload
			CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			ID: "2", Type: "PushEvent", Actor: "user", Repo: "user/repo",
			Payload:   map[string]any{"size": float64(3)},
			CreatedAt: time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
		},
	}
	calc := &Calculator{Username: "user", DateRange: dr}
	stats := calc.Calculate(events)

	pushEvents := stats.EventsByCategory[models.CategoryCommits]
	if stats.CommitCount < pushEvents {
		t.Errorf("CommitCount (%d) should be >= PushEvent count (%d)", stats.CommitCount, pushEvents)
	}
	// 1 (fallback) + 3 (from size) = 4
	if stats.CommitCount != 4 {
		t.Errorf("expected CommitCount 4, got %d", stats.CommitCount)
	}
}

func TestWeekdayMapping(t *testing.T) {
	// Jan 15, 2024 is a Monday
	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	events := []models.Event{{
		ID: "1", Type: "PushEvent", Actor: "user", Repo: "user/repo",
		Payload:   map[string]any{},
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

func TestCalculate_UsesHigherCommitCount(t *testing.T) {
	dr := testutil.SampleDateRange()
	calc := &Calculator{
		Username:                 "user",
		DateRange:                dr,
		TotalCommitContributions: 500,
	}
	// SampleEvents has 2 commits from events
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
		{ID: "1", Type: "PushEvent", Actor: "user", Repo: "user/repo",
			Payload: map[string]any{}, CreatedAt: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
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
