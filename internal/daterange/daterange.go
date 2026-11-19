package daterange

import (
	"fmt"
	"time"

	"github.com/ivuorinen/gh-history/internal/ghutil"
)

// FirstGitHubYear is the year GitHub launched; no activity can predate it.
const FirstGitHubYear = 2008

// DateRange represents a date range for querying GitHub activity: the days
// Start through End, inclusive, as days in Loc.
//
// Start and End are dates, held as midnight UTC like every date in a report.
// Loc decides where those days begin and end as instants, which StartTime and
// EndDateTime give: GitHub's from/to and every filter on an instant cut there.
// One zone per range keeps every figure in a report on the same days.
type DateRange struct {
	Start time.Time
	End   time.Time
	Loc   *time.Location // nil is UTC
}

// Location returns the zone the range's days are days in.
func (dr DateRange) Location() *time.Location {
	if dr.Loc == nil {
		return time.UTC
	}
	return dr.Loc
}

// StartTime returns the instant the range starts: the first instant of Start
// in the range's zone.
func (dr DateRange) StartTime() time.Time {
	return dayStart(dr.Start.Year(), dr.Start.Month(), dr.Start.Day(), dr.Location())
}

// dayStart returns the first instant of the date y-m-d in loc.
//
// That is midnight except where a clock change moves midnight, and there
// time.Date does not guarantee which instant it returns. Go's choice depends on
// the zone's side of UTC: measured over 2000-2030, a missing midnight west of
// UTC (Havana, São Paulo, Asunción) came back as 23:00 the day before, and a
// repeated midnight east of UTC (Amman, Gaza) as the second 00:00. Either is
// an hour off, and every boundary of a range is one of these instants.
func dayStart(y int, m time.Month, d int, loc *time.Location) time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, loc)
	if t.Day() != d {
		// Midnight fell in a spring-forward gap: the day starts as the gap ends.
		_, end := t.ZoneBounds()
		return end
	}
	if start, _ := t.ZoneBounds(); start.Equal(t) {
		// t opens the zone a fall-back switched to. If the instant before is
		// still the same day, midnight came first under the previous offset.
		if before := t.Add(-time.Nanosecond).In(loc); before.Day() == d {
			_, offset := before.Zone()
			return time.Date(y, m, d, 0, 0, 0, 0, time.FixedZone("", offset)).In(loc)
		}
	}
	return t
}

// New creates a DateRange of the dates start through end, as days in loc,
// validating that start is not after end. start and end are dates: their UTC
// day is taken.
func New(start, end time.Time, loc *time.Location) (DateRange, error) {
	start = ghutil.TruncateToDay(start, time.UTC)
	end = ghutil.TruncateToDay(end, time.UTC)
	if start.After(end) {
		return DateRange{}, fmt.Errorf("start date %s is after end date %s", start.Format(ghutil.DateFormat), end.Format(ghutil.DateFormat))
	}
	return DateRange{Start: start, End: end, Loc: loc}, nil
}

// Today returns today's date in loc. Every "today" a range is built or judged
// against comes from here, so a range ends on the day it is in its own zone.
func Today(loc *time.Location) time.Time {
	return ghutil.TruncateToDay(time.Now(), loc)
}

// Year creates a DateRange spanning the given year.
// If the year is in progress, the end date is capped to today. A year that has
// not started yet is an error: capping its end to today would otherwise yield a
// range whose start is after its end.
func Year(year int, loc *time.Location) (DateRange, error) {
	// GitHub did not exist before 2008, and an unbounded lower end is not just
	// useless: the range is split into one-year GraphQL chunks, so --year 1
	// would issue two thousand requests before returning anything.
	if year < FirstGitHubYear {
		return DateRange{}, fmt.Errorf("year %d predates GitHub; use %d or later", year, FirstGitHubYear)
	}
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	today := Today(loc)
	if start.After(today) {
		return DateRange{}, fmt.Errorf("year %d has not started yet", year)
	}
	if end.After(today) {
		end = today
	}
	return New(start, end, loc)
}

// LastMonth creates a DateRange for the previous calendar month.
func LastMonth(loc *time.Location) DateRange {
	today := Today(loc)
	firstOfThisMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastOfPrevMonth := firstOfThisMonth.AddDate(0, 0, -1)
	firstOfPrevMonth := time.Date(lastOfPrevMonth.Year(), lastOfPrevMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	return DateRange{Start: firstOfPrevMonth, End: lastOfPrevMonth, Loc: loc}
}

// LastNDays creates a DateRange for the last n days (inclusive of today).
func LastNDays(n int, loc *time.Location) DateRange {
	today := Today(loc)
	start := today.AddDate(0, 0, -(n - 1))
	return DateRange{Start: start, End: today, Loc: loc}
}

// Days returns the total number of days in the range (inclusive).
func (dr DateRange) Days() int {
	return int(dr.End.Sub(dr.Start).Hours()/24) + 1
}

// EndDateTime returns the instant the range ends: the first instant of the day
// after End in the range's zone. Callers should use < (not <=) when comparing
// against it.
func (dr DateRange) EndDateTime() time.Time {
	// End is a midnight-UTC date, so AddDate steps exactly one calendar day and
	// carries over month and year ends before dayStart compares the day.
	next := dr.End.AddDate(0, 0, 1)
	return dayStart(next.Year(), next.Month(), next.Day(), dr.Location())
}

// ParseDateRange parses CLI flags into a DateRange of days in loc. The dates
// given are days in loc, and "today", which caps the end, is today there.
func ParseDateRange(fromDate, toDate string, year int, lastMonth, last90 bool, loc *time.Location) (DateRange, error) {
	options := 0
	if fromDate != "" || toDate != "" {
		options++
	}
	if year != 0 {
		options++
	}
	if lastMonth {
		options++
	}
	if last90 {
		options++
	}
	if options > 1 {
		return DateRange{}, fmt.Errorf("cannot combine --from/--to with --year, --last-month, or --last-90-days")
	}

	if year != 0 {
		return Year(year, loc)
	}
	if lastMonth {
		return LastMonth(loc), nil
	}
	if last90 {
		return LastNDays(90, loc), nil
	}

	if fromDate == "" && toDate == "" {
		return LastNDays(90, loc), nil
	}

	today := Today(loc)
	var start, end time.Time

	start, err := parseDateInput("start", fromDate)
	if err != nil {
		return DateRange{}, err
	}
	if fromDate == "" {
		start = today.AddDate(-1, 0, 0)
	}
	// Same bounds Year applies: a pre-GitHub start issues one request per
	// year back to it, and a future end inflates the day count the activity
	// rate is divided by.
	if start.Year() < FirstGitHubYear {
		return DateRange{}, fmt.Errorf("start date %s predates GitHub; use %d or later",
			start.Format(ghutil.DateFormat), FirstGitHubYear)
	}

	end, err = parseDateInput("end", toDate)
	if err != nil {
		return DateRange{}, err
	}
	if toDate == "" || end.After(today) {
		end = today
	}

	return New(start, end, loc)
}

func parseDateInput(label, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(ghutil.DateFormat, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s date format, use YYYY-MM-DD: %w", label, err)
	}
	return t, nil
}
