package analysis

import (
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/models"
	"github.com/ivuorinen/gh-history/internal/testutil"
)

func TestStreaksEmpty(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	info := CalculateStreaks(nil, start, end)

	if info.LongestStreak != 0 {
		t.Errorf("expected 0 longest streak, got %d", info.LongestStreak)
	}
	if info.ActiveDays != 0 {
		t.Errorf("expected 0 active days, got %d", info.ActiveDays)
	}
	if info.TotalDays != 31 {
		t.Errorf("expected 31 total days, got %d", info.TotalDays)
	}
}

func TestStreaksConsecutive(t *testing.T) {
	events := []models.Event{
		testutil.MakeEvent(2024, 1, 10),
		testutil.MakeEvent(2024, 1, 11),
		testutil.MakeEvent(2024, 1, 12),
		testutil.MakeEvent(2024, 1, 13),
		testutil.MakeEvent(2024, 1, 14),
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	info := CalculateStreaks(events, start, end)

	if info.LongestStreak != 5 {
		t.Errorf("expected 5 day streak, got %d", info.LongestStreak)
	}
	if info.ActiveDays != 5 {
		t.Errorf("expected 5 active days, got %d", info.ActiveDays)
	}
}

func TestStreaksWithGaps(t *testing.T) {
	events := []models.Event{
		testutil.MakeEvent(2024, 1, 1),
		testutil.MakeEvent(2024, 1, 2),
		testutil.MakeEvent(2024, 1, 3),
		// gap
		testutil.MakeEvent(2024, 1, 10),
		testutil.MakeEvent(2024, 1, 11),
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	info := CalculateStreaks(events, start, end)

	if info.LongestStreak != 3 {
		t.Errorf("expected longest streak 3, got %d", info.LongestStreak)
	}
	if info.ActiveDays != 5 {
		t.Errorf("expected 5 active days, got %d", info.ActiveDays)
	}
}

func TestStreaksDuplicateDates(t *testing.T) {
	events := []models.Event{
		testutil.MakeEvent(2024, 1, 5),
		testutil.MakeEvent(2024, 1, 5), // duplicate
		testutil.MakeEvent(2024, 1, 6),
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	info := CalculateStreaks(events, start, end)

	if info.ActiveDays != 2 {
		t.Errorf("expected 2 active days (deduped), got %d", info.ActiveDays)
	}
	if info.LongestStreak != 2 {
		t.Errorf("expected 2 day streak, got %d", info.LongestStreak)
	}
}

func TestCalculateStreaksFromCalendar_ConsecutiveDays(t *testing.T) {
	days := []models.ContributionDay{
		{Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), ContributionCount: 3},
		{Date: time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
		{Date: time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC), ContributionCount: 5},
		{Date: time.Date(2024, 1, 13, 0, 0, 0, 0, time.UTC), ContributionCount: 2},
		{Date: time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	info := CalculateStreaksFromCalendar(days, start, end)

	if info.LongestStreak != 5 {
		t.Errorf("expected 5 day streak, got %d", info.LongestStreak)
	}
	if info.ActiveDays != 5 {
		t.Errorf("expected 5 active days, got %d", info.ActiveDays)
	}
}

func TestCalculateStreaksFromCalendar_WithGaps(t *testing.T) {
	days := []models.ContributionDay{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), ContributionCount: 2},
		{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
		// gap
		{Date: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), ContributionCount: 3},
		{Date: time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	info := CalculateStreaksFromCalendar(days, start, end)

	if info.LongestStreak != 3 {
		t.Errorf("expected longest streak 3, got %d", info.LongestStreak)
	}
	if info.ActiveDays != 5 {
		t.Errorf("expected 5 active days, got %d", info.ActiveDays)
	}
}

func TestCalculateStreaksFromCalendar_ZeroCountDays(t *testing.T) {
	days := []models.ContributionDay{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), ContributionCount: 0}, // zero breaks streak
		{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), ContributionCount: 1},
	}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	info := CalculateStreaksFromCalendar(days, start, end)

	if info.LongestStreak != 1 {
		t.Errorf("expected longest streak 1 (zero-count breaks streak), got %d", info.LongestStreak)
	}
	if info.ActiveDays != 2 {
		t.Errorf("expected 2 active days, got %d", info.ActiveDays)
	}
}

func TestCalculateStreaksFromCalendar_Empty(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	info := CalculateStreaksFromCalendar(nil, start, end)

	if info.LongestStreak != 0 {
		t.Errorf("expected 0 longest streak, got %d", info.LongestStreak)
	}
	if info.ActiveDays != 0 {
		t.Errorf("expected 0 active days, got %d", info.ActiveDays)
	}
	if info.TotalDays != 31 {
		t.Errorf("expected 31 total days, got %d", info.TotalDays)
	}
}

func TestActivityRate(t *testing.T) {
	info := models.StreakInfo{ActiveDays: 10, TotalDays: 100}
	rate := info.ActivityRate()
	if rate != 10.0 {
		t.Errorf("expected 10.0%%, got %.1f%%", rate)
	}

	empty := models.StreakInfo{}
	if empty.ActivityRate() != 0.0 {
		t.Error("expected 0% for empty")
	}
}
