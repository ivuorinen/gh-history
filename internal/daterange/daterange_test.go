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
	dr, err := Year(2024)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Days() != 366 { // 2024 is a leap year
		t.Errorf("expected 366 days, got %d", dr.Days())
	}
}

func TestYearNonLeap(t *testing.T) {
	dr, err := Year(2023)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Days() != 365 {
		t.Errorf("expected 365 days, got %d", dr.Days())
	}
}

func TestYearCurrentCapsToToday(t *testing.T) {
	now := time.Now().UTC()
	dr, err := Year(now.Year())
	if err != nil {
		t.Fatal(err)
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if dr.End.After(today) {
		t.Errorf("expected end <= today (%v), got %v", today, dr.End)
	}
	if dr.Start != d(now.Year(), 1, 1) {
		t.Errorf("expected start Jan 1, got %v", dr.Start)
	}
}

// A year that has not started must be rejected. Capping its end to today would
// otherwise produce a range whose start is after its end, which yields zero
// fetch chunks and a silently empty report with a negative day count.
func TestYearFutureIsRejected(t *testing.T) {
	future := time.Now().UTC().Year() + 5
	dr, err := Year(future)
	if err == nil {
		t.Fatalf("expected error for future year %d, got range %v..%v", future, dr.Start, dr.End)
	}
	if _, err := ParseDateRange("", "", future, false, false); err == nil {
		t.Errorf("ParseDateRange should propagate the future-year error")
	}
}

func TestYearBoundaryIsInclusiveOfToday(t *testing.T) {
	// The current year must still be accepted: start is Jan 1, never after today.
	now := time.Now().UTC()
	if _, err := Year(now.Year()); err != nil {
		t.Errorf("current year should be valid, got %v", err)
	}
	if _, err := Year(now.Year() + 1); err == nil {
		t.Error("next year should be rejected")
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
