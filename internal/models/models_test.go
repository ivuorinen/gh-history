package models

import (
	"testing"
	"time"
)

func TestTopRepos(t *testing.T) {
	tests := []struct {
		name      string
		byRepo    map[string]int
		n         int
		wantLen   int
		wantFirst string
		wantCount int
	}{
		{
			name:      "fewer repos than n",
			byRepo:    map[string]int{"a/one": 5, "a/two": 9},
			n:         15,
			wantLen:   2,
			wantFirst: "a/two",
			wantCount: 9,
		},
		{
			name:      "exactly n repos",
			byRepo:    map[string]int{"a/one": 1, "a/two": 2, "a/three": 3},
			n:         3,
			wantLen:   3,
			wantFirst: "a/three",
			wantCount: 3,
		},
		{
			name:      "more repos than n truncates to the highest counts",
			byRepo:    map[string]int{"a/one": 1, "a/two": 2, "a/three": 3, "a/four": 4},
			n:         2,
			wantLen:   2,
			wantFirst: "a/four",
			wantCount: 4,
		},
		{
			name:    "empty",
			byRepo:  map[string]int{},
			n:       5,
			wantLen: 0,
		},
		{
			name:    "n of zero returns nothing",
			byRepo:  map[string]int{"a/one": 1},
			n:       0,
			wantLen: 0,
		},
		{
			// repos[:n] would panic on a negative bound.
			name:    "negative n returns nothing rather than panicking",
			byRepo:  map[string]int{"a/one": 1},
			n:       -1,
			wantLen: 0,
		},
		{
			name:    "negative n on an empty map",
			byRepo:  map[string]int{},
			n:       -5,
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Statistics{EventsByRepo: tc.byRepo}
			got := s.TopRepos(tc.n)
			if len(got) != tc.wantLen {
				t.Fatalf("expected %d repos, got %d (%v)", tc.wantLen, len(got), got)
			}
			if tc.wantLen == 0 {
				return
			}
			if got[0].Repo != tc.wantFirst {
				t.Errorf("expected first repo %q, got %q", tc.wantFirst, got[0].Repo)
			}
			if got[0].Count != tc.wantCount {
				t.Errorf("expected first count %d, got %d", tc.wantCount, got[0].Count)
			}
			// Result must be sorted descending by count.
			for i := 1; i < len(got); i++ {
				if got[i-1].Count < got[i].Count {
					t.Errorf("not sorted descending at %d: %v", i, got)
				}
			}
		})
	}
}

func TestPRMergeRate(t *testing.T) {
	tests := []struct {
		name           string
		merged, opened int
		want           float64
	}{
		{"half", 5, 10, 50},
		{"all", 10, 10, 100},
		{"none merged", 0, 10, 0},
		{"no PRs opened does not divide by zero", 0, 0, 0},
		{"merged without opened in range", 3, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Statistics{PRMerged: tc.merged, PROpened: tc.opened}
			if got := s.PRMergeRate(); got != tc.want {
				t.Errorf("PRMergeRate() = %.1f, want %.1f", got, tc.want)
			}
		})
	}
}

func TestIssueCloseRate(t *testing.T) {
	tests := []struct {
		name           string
		closed, opened int
		want           float64
	}{
		{"half", 5, 10, 50},
		{"none opened does not divide by zero", 0, 0, 0},
		{"closed without opened in range", 4, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Statistics{IssuesClosed: tc.closed, IssuesOpened: tc.opened}
			if got := s.IssueCloseRate(); got != tc.want {
				t.Errorf("IssueCloseRate() = %.1f, want %.1f", got, tc.want)
			}
		})
	}
}

func TestStreakInfoActivityRate(t *testing.T) {
	tests := []struct {
		name              string
		active, totalDays int
		want              float64
	}{
		{"ten percent", 10, 100, 10},
		{"every day", 31, 31, 100},
		{"zero total does not divide by zero", 5, 0, 0},
		{"zero value struct", 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := StreakInfo{ActiveDays: tc.active, TotalDays: tc.totalDays}
			if got := s.ActivityRate(); got != tc.want {
				t.Errorf("ActivityRate() = %.1f, want %.1f", got, tc.want)
			}
		})
	}
}

// Streak arithmetic compares dates for an exact 24h delta, which only holds if
// every date is normalized to UTC midnight. Asserting the exact day matters:
// a local timestamp can fall on a different UTC date than it appears to.
func TestEventDateTruncatesToUTCDay(t *testing.T) {
	east5 := time.FixedZone("east5", 5*3600)
	west8 := time.FixedZone("west8", -8*3600)

	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "already UTC midnight",
			in:   time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			want: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "late UTC evening stays on the same day",
			in:   time.Date(2024, 3, 15, 23, 59, 59, 999, time.UTC),
			want: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			// 23:59 +05:00 is 18:59 UTC — still March 15.
			name: "east of UTC, no boundary crossed",
			in:   time.Date(2024, 3, 15, 23, 59, 59, 999, east5),
			want: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			// 01:00 +05:00 is 20:00 UTC the previous day.
			name: "east of UTC, crosses back a day",
			in:   time.Date(2024, 3, 15, 1, 0, 0, 0, east5),
			want: time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC),
		},
		{
			// 20:00 -08:00 is 04:00 UTC the next day.
			name: "west of UTC, crosses forward a day",
			in:   time.Date(2024, 3, 15, 20, 0, 0, 0, west8),
			want: time.Date(2024, 3, 16, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Event{CreatedAt: tc.in}.Date()
			if !got.Equal(tc.want) {
				t.Errorf("Date() = %v, want %v", got, tc.want)
			}
			if got.Location() != time.UTC {
				t.Errorf("expected UTC location, got %v", got.Location())
			}
		})
	}
}
