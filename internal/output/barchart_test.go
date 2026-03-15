package output

import (
	"testing"
	"unicode/utf8"

	"github.com/ivuorinen/gh-history/internal/models"
)

func TestBuildCategoryBars_Standard(t *testing.T) {
	stats := models.Statistics{
		TotalEvents: 100,
		EventsByCategory: map[models.Category]int{
			models.CategoryCommits:      50,
			models.CategoryPullRequests: 30,
			models.CategoryIssues:       20,
		},
	}

	entries := BuildCategoryBars(stats, 20, AllCategories)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First entry should be commits (follows AllCategories order)
	if entries[0].Label != "Commits" {
		t.Errorf("expected first label Commits, got %q", entries[0].Label)
	}
	if entries[0].Count != 50 {
		t.Errorf("expected count 50, got %d", entries[0].Count)
	}
	if entries[0].Percent != 50.0 {
		t.Errorf("expected 50%%, got %.1f%%", entries[0].Percent)
	}
	if utf8.RuneCountInString(entries[0].Bar) != 20 {
		t.Errorf("expected bar length 20 runes, got %d", utf8.RuneCountInString(entries[0].Bar))
	}
}

func TestBuildCategoryBars_ZeroTotal(t *testing.T) {
	stats := models.Statistics{
		TotalEvents:      0,
		EventsByCategory: map[models.Category]int{},
	}
	entries := BuildCategoryBars(stats, 20, AllCategories)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for zero total, got %d", len(entries))
	}
}

func TestBuildCategoryBars_SingleCategory(t *testing.T) {
	stats := models.Statistics{
		TotalEvents: 10,
		EventsByCategory: map[models.Category]int{
			models.CategoryCommits: 10,
		},
	}
	entries := BuildCategoryBars(stats, 20, AllCategories)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Percent != 100.0 {
		t.Errorf("expected 100%%, got %.1f%%", entries[0].Percent)
	}
}

func TestBuildCategoryBars_ZeroBarWidth(t *testing.T) {
	stats := models.Statistics{
		TotalEvents: 10,
		EventsByCategory: map[models.Category]int{
			models.CategoryCommits: 10,
		},
	}
	// Should not panic with barWidth=0
	entries := BuildCategoryBars(stats, 0, AllCategories)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Bar != "" {
		t.Errorf("expected empty bar, got %q", entries[0].Bar)
	}
}

func TestBuildWeekdayBars_Standard(t *testing.T) {
	stats := models.Statistics{
		TotalEvents: 100,
		EventsByWeekday: map[int]int{
			0: 30, // Monday
			2: 20, // Wednesday
			4: 50, // Friday
		},
	}
	entries := BuildWeekdayBars(stats, 20)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Should be in weekday order
	if entries[0].Label != "Monday" {
		t.Errorf("expected first label Monday, got %q", entries[0].Label)
	}
	if entries[1].Label != "Wednesday" {
		t.Errorf("expected second label Wednesday, got %q", entries[1].Label)
	}
	if entries[2].Label != "Friday" {
		t.Errorf("expected third label Friday, got %q", entries[2].Label)
	}
	// Friday has max count (50), should get full bar
	if utf8.RuneCountInString(entries[2].Bar) != 20 {
		t.Errorf("expected bar length 20 runes, got %d", utf8.RuneCountInString(entries[2].Bar))
	}
	if entries[2].Percent != 50.0 {
		t.Errorf("expected 50%%, got %.1f%%", entries[2].Percent)
	}
}

func TestBuildWeekdayBars_Empty(t *testing.T) {
	stats := models.Statistics{
		TotalEvents:     0,
		EventsByWeekday: map[int]int{},
	}
	entries := BuildWeekdayBars(stats, 20)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestBuildHourlyBars_Standard(t *testing.T) {
	stats := models.Statistics{
		TotalEvents: 60,
		EventsByHour: map[int]int{
			9:  20,
			14: 30,
			22: 10,
		},
	}
	entries := BuildHourlyBars(stats, 20)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Label != "09" {
		t.Errorf("expected label 09, got %q", entries[0].Label)
	}
	if entries[1].Label != "14" {
		t.Errorf("expected label 14, got %q", entries[1].Label)
	}
	if entries[2].Label != "22" {
		t.Errorf("expected label 22, got %q", entries[2].Label)
	}
	if utf8.RuneCountInString(entries[1].Bar) != 20 {
		t.Errorf("expected bar length 20 runes, got %d", utf8.RuneCountInString(entries[1].Bar))
	}
}

func TestBuildHourlyBars_Empty(t *testing.T) {
	stats := models.Statistics{
		TotalEvents:  0,
		EventsByHour: map[int]int{},
	}
	entries := BuildHourlyBars(stats, 20)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestBuildHourlyBars_SkipsZeroCounts(t *testing.T) {
	stats := models.Statistics{
		TotalEvents: 10,
		EventsByHour: map[int]int{
			0:  5,
			12: 5,
			23: 0, // should be skipped
		},
	}
	entries := BuildHourlyBars(stats, 20)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (zero-count skipped), got %d", len(entries))
	}
}
