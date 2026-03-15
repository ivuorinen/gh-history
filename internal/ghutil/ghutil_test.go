package ghutil

import (
	"testing"
	"time"
)

func TestTruncateToDay(t *testing.T) {
	input := time.Date(2024, 3, 15, 14, 30, 45, 123, time.UTC)
	got := TruncateToDay(input)
	want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("TruncateToDay(%v) = %v, want %v", input, got, want)
	}
}

func TestParseRFC3339Fallback(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"standard RFC3339", "2024-01-15T10:00:00+00:00", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"Z suffix", "2024-01-15T10:00:00Z", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{"invalid", "not-a-date", time.Time{}},
		{"empty", "", time.Time{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseRFC3339Fallback(tc.input)
			if !got.Equal(tc.want) {
				t.Errorf("ParseRFC3339Fallback(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseDateFallback(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"date only", "2024-01-15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"RFC3339", "2024-01-15T10:30:00Z", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"invalid", "bad", time.Time{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseDateFallback(tc.input)
			if !got.Equal(tc.want) {
				t.Errorf("ParseDateFallback(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeUser(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"TestUser", "testuser"},
		{"ALLCAPS", "allcaps"},
		{"already", "already"},
	}
	for _, tc := range tests {
		got := NormalizeUser(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeUser(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSafeDiv(t *testing.T) {
	tests := []struct {
		num, den int
		want     float64
	}{
		{10, 2, 5.0},
		{1, 3, 1.0 / 3.0},
		{0, 5, 0.0},
		{5, 0, 0.0},
		{0, 0, 0.0},
	}
	for _, tc := range tests {
		got := SafeDiv(tc.num, tc.den)
		if got != tc.want {
			t.Errorf("SafeDiv(%d, %d) = %f, want %f", tc.num, tc.den, got, tc.want)
		}
	}
}

func TestNowUTC(t *testing.T) {
	now := NowUTC()
	if now.Location() != time.UTC {
		t.Errorf("NowUTC() location = %v, want UTC", now.Location())
	}
}

func TestDateFormat(t *testing.T) {
	if DateFormat != "2006-01-02" {
		t.Errorf("DateFormat = %q, want %q", DateFormat, "2006-01-02")
	}
}
