package daterange

import (
	"strings"
	"testing"
	"time"
)

func d(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestNew(t *testing.T) {
	dr, err := New(d(2024, 1, 1), d(2024, 1, 31))
	if err != nil {
		t.Fatal(err)
	}
	if dr.Days() != 31 {
		t.Errorf("expected 31 days, got %d", dr.Days())
	}
}

func TestNewInvalid(t *testing.T) {
	_, err := New(d(2024, 2, 1), d(2024, 1, 1))
	if err == nil {
		t.Error("expected error for invalid range")
	}
}

func TestYear(t *testing.T) {
	dr := Year(2024)
	if dr.Days() != 366 { // 2024 is a leap year
		t.Errorf("expected 366 days, got %d", dr.Days())
	}
}

func TestYearNonLeap(t *testing.T) {
	dr := Year(2023)
	if dr.Days() != 365 {
		t.Errorf("expected 365 days, got %d", dr.Days())
	}
}

func TestYearCurrentCapsToToday(t *testing.T) {
	now := time.Now().UTC()
	dr := Year(now.Year())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if dr.End.After(today) {
		t.Errorf("expected end <= today (%v), got %v", today, dr.End)
	}
	if dr.Start != d(now.Year(), 1, 1) {
		t.Errorf("expected start Jan 1, got %v", dr.Start)
	}
}

func TestContains(t *testing.T) {
	dr := DateRange{Start: d(2024, 1, 1), End: d(2024, 1, 31)}
	if !dr.Contains(d(2024, 1, 15)) {
		t.Error("should contain Jan 15")
	}
	if dr.Contains(d(2024, 2, 1)) {
		t.Error("should not contain Feb 1")
	}
	if !dr.Contains(d(2024, 1, 1)) {
		t.Error("should contain start date")
	}
	if !dr.Contains(d(2024, 1, 31)) {
		t.Error("should contain end date")
	}
}

func TestOverlaps(t *testing.T) {
	a := DateRange{Start: d(2024, 1, 1), End: d(2024, 1, 31)}
	b := DateRange{Start: d(2024, 1, 15), End: d(2024, 2, 15)}
	if !a.Overlaps(b) {
		t.Error("should overlap")
	}

	c := DateRange{Start: d(2024, 3, 1), End: d(2024, 3, 31)}
	if a.Overlaps(c) {
		t.Error("should not overlap")
	}
}

func TestSubtract(t *testing.T) {
	a := DateRange{Start: d(2024, 1, 1), End: d(2024, 1, 31)}

	// No overlap
	b := DateRange{Start: d(2024, 3, 1), End: d(2024, 3, 31)}
	result := a.Subtract(b)
	if len(result) != 1 || result[0] != a {
		t.Errorf("no overlap: expected [%v], got %v", a, result)
	}

	// Full overlap
	c := DateRange{Start: d(2024, 1, 1), End: d(2024, 1, 31)}
	result = a.Subtract(c)
	if len(result) != 0 {
		t.Errorf("full overlap: expected empty, got %v", result)
	}

	// Partial overlap (middle)
	e := DateRange{Start: d(2024, 1, 10), End: d(2024, 1, 20)}
	result = a.Subtract(e)
	if len(result) != 2 {
		t.Fatalf("middle overlap: expected 2 ranges, got %d", len(result))
	}
	if result[0].End != d(2024, 1, 9) {
		t.Errorf("expected first range end Jan 9, got %v", result[0].End)
	}
	if result[1].Start != d(2024, 1, 21) {
		t.Errorf("expected second range start Jan 21, got %v", result[1].Start)
	}
}

func TestParseDateRange(t *testing.T) {
	// Year
	dr, err := ParseDateRange("", "", 2024, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Days() != 366 {
		t.Errorf("expected 366, got %d", dr.Days())
	}

	// Explicit from/to
	dr, err = ParseDateRange("2024-01-01", "2024-06-30", 0, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Start != d(2024, 1, 1) {
		t.Errorf("expected Jan 1, got %v", dr.Start)
	}

	// Conflicting options
	_, err = ParseDateRange("2024-01-01", "", 2024, false, false)
	if err == nil {
		t.Error("expected error for conflicting options")
	}
}

func TestParseDateInput_ValidDate(t *testing.T) {
	got, err := parseDateInput("start", "2024-03-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := d(2024, 3, 15)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDateInput_EmptyString(t *testing.T) {
	got, err := parseDateInput("start", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time for empty string, got %v", got)
	}
}

func TestParseDateInput_InvalidFormat(t *testing.T) {
	_, err := parseDateInput("end", "15-03-2024")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "end") {
		t.Errorf("error should contain label 'end': %v", err)
	}
}
