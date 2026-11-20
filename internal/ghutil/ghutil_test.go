package ghutil

import (
	"testing"
	"time"
)

// TruncateToDay returns the day an instant falls on in the given zone, as
// midnight UTC. Late on the 15th in UTC is already the 16th in Helsinki and
// still the 15th in Los Angeles; early on the 15th in UTC is the 14th there.
func TestTruncateToDay(t *testing.T) {
	helsinki, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		t.Fatal(err)
	}
	losAngeles, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	late := time.Date(2024, 3, 15, 23, 30, 45, 123, time.UTC)
	early := time.Date(2024, 3, 15, 5, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		in   time.Time
		loc  *time.Location
		want time.Time
	}{
		{"UTC", late, time.UTC, time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
		{"ahead of UTC", late, helsinki, time.Date(2024, 3, 16, 0, 0, 0, 0, time.UTC)},
		{"behind UTC", early, losAngeles, time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC)},
		{"input carries another zone", late.In(losAngeles), time.UTC, time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)},
	} {
		if got := TruncateToDay(tc.in, tc.loc); got != tc.want {
			t.Errorf("%s: TruncateToDay(%v, %v) = %v, want %v", tc.name, tc.in, tc.loc, got, tc.want)
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

func TestDateFormat(t *testing.T) {
	if DateFormat != "2006-01-02" {
		t.Errorf("DateFormat = %q, want %q", DateFormat, "2006-01-02")
	}
}
