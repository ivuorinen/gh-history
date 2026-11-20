package daterange

import (
	"strings"
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/ghutil"
)

func d(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestNew(t *testing.T) {
	dr, err := New(d(2024, 1, 1), d(2024, 1, 31), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Days() != 31 {
		t.Errorf("expected 31 days, got %d", dr.Days())
	}
}

func TestNewInvalid(t *testing.T) {
	_, err := New(d(2024, 2, 1), d(2024, 1, 1), time.UTC)
	if err == nil {
		t.Error("expected error for invalid range")
	}
}

func TestYear(t *testing.T) {
	dr, err := Year(2024, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Days() != 366 { // 2024 is a leap year
		t.Errorf("expected 366 days, got %d", dr.Days())
	}
}

func TestYearNonLeap(t *testing.T) {
	dr, err := Year(2023, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Days() != 365 {
		t.Errorf("expected 365 days, got %d", dr.Days())
	}
}

func TestYearCurrentCapsToToday(t *testing.T) {
	now := time.Now().UTC()
	dr, err := Year(now.Year(), time.UTC)
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
	dr, err := Year(future, time.UTC)
	if err == nil {
		t.Fatalf("expected error for future year %d, got range %v..%v", future, dr.Start, dr.End)
	}
	if _, err := ParseDateRange("", "", future, false, false, time.UTC); err == nil {
		t.Errorf("ParseDateRange should propagate the future-year error")
	}
}

// An unbounded lower end is not merely useless: the range is split into
// one-year GraphQL chunks, so --year 1 would issue two thousand requests.
func TestYearBeforeGitHubIsRejected(t *testing.T) {
	for _, y := range []int{1, -5, 0, FirstGitHubYear - 1} {
		if _, err := Year(y, time.UTC); err == nil {
			t.Errorf("Year(%d) should be rejected", y)
		}
		if _, err := ParseDateRange("", "", y, false, false, time.UTC); y != 0 && err == nil {
			t.Errorf("ParseDateRange(year=%d) should propagate the rejection", y)
		}
	}
	if _, err := Year(FirstGitHubYear, time.UTC); err != nil {
		t.Errorf("Year(%d) should be accepted, got %v", FirstGitHubYear, err)
	}
}

func TestYearBoundaryIsInclusiveOfToday(t *testing.T) {
	// The current year must still be accepted: start is Jan 1, never after today.
	now := time.Now().UTC()
	if _, err := Year(now.Year(), time.UTC); err != nil {
		t.Errorf("current year should be valid, got %v", err)
	}
	if _, err := Year(now.Year()+1, time.UTC); err == nil {
		t.Error("next year should be rejected")
	}
}

func TestParseDateRange(t *testing.T) {
	// Year
	dr, err := ParseDateRange("", "", 2024, false, false, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Days() != 366 {
		t.Errorf("expected 366, got %d", dr.Days())
	}

	// Explicit from/to
	dr, err = ParseDateRange("2024-01-01", "2024-06-30", 0, false, false, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Start != d(2024, 1, 1) {
		t.Errorf("expected Jan 1, got %v", dr.Start)
	}

	// Conflicting options
	_, err = ParseDateRange("2024-01-01", "", 2024, false, false, time.UTC)
	if err == nil {
		t.Error("expected error for conflicting options")
	}
}

// Each range flag gives its constructor's range, no flag gives the last 90
// days, and a --from/--to missing one side fills it in: a missing --from is a
// year before today, and a missing --to is today.
func TestParseDateRange_EachFlagPicksItsRange(t *testing.T) {
	today := Today(time.UTC)
	tenDaysAgo := today.AddDate(0, 0, -10)
	for _, tc := range []struct {
		name              string
		from, to          string
		lastMonth, last90 bool
		want              DateRange
	}{
		{name: "last month", lastMonth: true, want: LastMonth(time.UTC)},
		{name: "last 90 days", last90: true, want: LastNDays(90, time.UTC)},
		{name: "no flag", want: LastNDays(90, time.UTC)},
		{name: "to only", to: tenDaysAgo.Format(ghutil.DateFormat), want: DateRange{Start: today.AddDate(-1, 0, 0), End: tenDaysAgo}},
		{name: "from only", from: tenDaysAgo.Format(ghutil.DateFormat), want: DateRange{Start: tenDaysAgo, End: today}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dr, err := ParseDateRange(tc.from, tc.to, 0, tc.lastMonth, tc.last90, time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			if !dr.Start.Equal(tc.want.Start) || !dr.End.Equal(tc.want.End) {
				t.Errorf("got %v to %v, want %v to %v", dr.Start, dr.End, tc.want.Start, tc.want.End)
			}
		})
	}
}

// Any two of --from/--to, --year, --last-month and --last-90-days conflict.
func TestParseDateRange_RangeFlagsConflict(t *testing.T) {
	for _, tc := range []struct {
		name              string
		from              string
		year              int
		lastMonth, last90 bool
	}{
		{name: "year and last month", year: 2024, lastMonth: true},
		{name: "last month and last 90 days", lastMonth: true, last90: true},
		{name: "from and last 90 days", from: "2024-01-01", last90: true},
	} {
		if _, err := ParseDateRange(tc.from, "", tc.year, tc.lastMonth, tc.last90, time.UTC); err == nil {
			t.Errorf("%s: expected a conflict error", tc.name)
		}
	}
}

// A malformed date is an error naming the flag it came from.
func TestParseDateRange_MalformedDateNamesItsFlag(t *testing.T) {
	for _, tc := range []struct{ from, to, want string }{
		{from: "2024-13-01", want: "invalid start date"},
		{to: "2024/01/31", want: "invalid end date"},
	} {
		_, err := ParseDateRange(tc.from, tc.to, 0, false, false, time.UTC)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ParseDateRange(%q, %q) error = %v, want %q", tc.from, tc.to, err, tc.want)
		}
	}
}

// --from/--to get the bounds --year has: --from 0001-01-01 would otherwise
// issue about two thousand chunk requests.
func TestParseDateRange_FromBeforeGitHubIsRejected(t *testing.T) {
	if _, err := ParseDateRange("0001-01-01", "2024-01-31", 0, false, false, time.UTC); err == nil {
		t.Error("a start before GitHub existed must be rejected")
	}
	if _, err := ParseDateRange("2008-01-01", "2008-01-31", 0, false, false, time.UTC); err != nil {
		t.Errorf("a start in %d must be accepted: %v", FirstGitHubYear, err)
	}
}

// A future --to would inflate TotalDays and so understate the activity rate.
func TestParseDateRange_FutureToIsCappedToToday(t *testing.T) {
	today := ghutil.TruncateToDay(time.Now(), time.UTC)
	future := today.AddDate(0, 3, 0).Format(ghutil.DateFormat)
	from := today.AddDate(0, 0, -3).Format(ghutil.DateFormat)

	dr, err := ParseDateRange(from, future, 0, false, false, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if !dr.End.Equal(today) {
		t.Errorf("End = %v, want today %v", dr.End, today)
	}
	if dr.Days() != 4 {
		t.Errorf("Days() = %d, want 4", dr.Days())
	}
}

func loadZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// A range's days are days in its zone: it starts at the zone's midnight on
// Start and ends at the zone's midnight after End, the instants GitHub's
// from/to and the close and comment filters cut at. Start and End stay dates,
// so Days counts calendar days. A range without a zone is in UTC.
func TestDateRange_BoundariesFollowZone(t *testing.T) {
	loc := loadZone(t, "Europe/Helsinki")
	// March has 31 days but, with the switch to summer time, 743 hours.
	dr, err := New(d(2024, 3, 1), d(2024, 3, 31), loc)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Location() != loc {
		t.Errorf("Location() = %v, want %v", dr.Location(), loc)
	}
	if got, want := dr.StartTime(), time.Date(2024, 3, 1, 0, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("StartTime() = %v, want %v", got, want)
	}
	if got, want := dr.EndDateTime(), time.Date(2024, 4, 1, 0, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("EndDateTime() = %v, want %v", got, want)
	}
	if dr.Start != d(2024, 3, 1) || dr.End != d(2024, 3, 31) {
		t.Errorf("Start, End = %v, %v; want the dates themselves", dr.Start, dr.End)
	}
	if dr.Days() != 31 {
		t.Errorf("Days() = %d, want 31", dr.Days())
	}

	zoneless := DateRange{Start: d(2024, 3, 1), End: d(2024, 3, 31)}
	if zoneless.Location() != time.UTC {
		t.Errorf("a range without a zone is in %v, want UTC", zoneless.Location())
	}
	if got, want := zoneless.EndDateTime(), d(2024, 4, 1); !got.Equal(want) {
		t.Errorf("zoneless EndDateTime() = %v, want %v", got, want)
	}
}

// A day starts at its first instant even where its midnight is missing or
// repeated, which time.Date does not guarantee: west of UTC it answered a
// missing midnight with 23:00 the day before, and east of UTC a repeated
// midnight with the second 00:00, an hour off either way. StartTime on the day
// and EndDateTime on the day before must both give that first instant.
func TestDateRange_BoundariesAtFirstInstantOfDay(t *testing.T) {
	for _, tc := range []struct {
		name, zone string
		day        time.Time
		want       time.Time // the first instant of day, in UTC
	}{
		{"missing midnight, west of UTC", "America/Sao_Paulo", d(2018, 11, 4), time.Date(2018, 11, 4, 3, 0, 0, 0, time.UTC)},
		{"repeated midnight, west of UTC", "America/Havana", d(2023, 11, 5), time.Date(2023, 11, 5, 4, 0, 0, 0, time.UTC)},
		{"repeated midnight, east of UTC", "Asia/Amman", d(2000, 9, 29), time.Date(2000, 9, 28, 21, 0, 0, 0, time.UTC)},
		{"ordinary midnight", "Europe/Helsinki", d(2024, 3, 31), time.Date(2024, 3, 30, 22, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loc := loadZone(t, tc.zone)
			starting := DateRange{Start: tc.day, End: tc.day, Loc: loc}
			if got := starting.StartTime(); !got.Equal(tc.want) {
				t.Errorf("StartTime() = %v, want %v", got, tc.want.In(loc))
			}
			ending := DateRange{Start: tc.day.AddDate(0, 0, -1), End: tc.day.AddDate(0, 0, -1), Loc: loc}
			if got := ending.EndDateTime(); !got.Equal(tc.want) {
				t.Errorf("EndDateTime() = %v, want %v", got, tc.want.In(loc))
			}
		})
	}
}

// Every constructor records the zone, and "today" is today in it. UTC+14 and
// UTC-11 are 25 hours apart, so their dates differ at every moment: reading
// today in UTC, or in the machine's zone, would give both the same End.
func TestConstructors_TodayFollowsZone(t *testing.T) {
	ahead := loadZone(t, "Pacific/Kiritimati")
	behind := loadZone(t, "Pacific/Pago_Pago")

	a, b := LastNDays(1, ahead), LastNDays(1, behind)
	if !a.End.After(b.End) {
		t.Errorf("today is %v at UTC+14 and %v at UTC-11; want the first later", a.End, b.End)
	}
	future := a.End.AddDate(0, 0, 5).Format(ghutil.DateFormat)
	for _, tc := range []struct {
		loc   *time.Location
		today time.Time
	}{{ahead, a.End}, {behind, b.End}} {
		parsed, err := ParseDateRange("2024-01-01", future, 0, false, false, tc.loc)
		if err != nil {
			t.Fatal(err)
		}
		if !parsed.End.Equal(tc.today) || parsed.Start != d(2024, 1, 1) {
			t.Errorf("%v: --from 2024-01-01 --to %s = %v..%v, want 2024-01-01..%v",
				tc.loc, future, parsed.Start, parsed.End, tc.today)
		}
		year, err := Year(2024, tc.loc)
		if err != nil {
			t.Fatal(err)
		}
		for name, dr := range map[string]DateRange{
			"LastNDays": LastNDays(1, tc.loc), "LastMonth": LastMonth(tc.loc),
			"Year": year, "ParseDateRange": parsed,
		} {
			if dr.Location() != tc.loc {
				t.Errorf("%s(%v) is in %v", name, tc.loc, dr.Location())
			}
		}
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
