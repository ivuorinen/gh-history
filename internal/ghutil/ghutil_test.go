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
