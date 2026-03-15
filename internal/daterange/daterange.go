package daterange

import (
	"fmt"
	"time"

	"github.com/ivuorinen/gh-history/internal/ghutil"
)

// DateRange represents a date range for querying GitHub activity.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// New creates a DateRange, validating that start is not after end.
func New(start, end time.Time) (DateRange, error) {
	start = ghutil.TruncateToDay(start)
	end = ghutil.TruncateToDay(end)
	if start.After(end) {
		return DateRange{}, fmt.Errorf("start date %s is after end date %s", start.Format(ghutil.DateFormat), end.Format(ghutil.DateFormat))
	}
	return DateRange{Start: start, End: end}, nil
}

// Year creates a DateRange spanning the given year.
// If the year is the current year or later, the end date is capped to today.
func Year(year int) DateRange {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	today := ghutil.TruncateToDay(ghutil.NowUTC())
	if end.After(today) {
		end = today
	}
	return DateRange{Start: start, End: end}
}

// LastMonth creates a DateRange for the previous calendar month.
func LastMonth() DateRange {
	now := ghutil.NowUTC()
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastOfPrevMonth := firstOfThisMonth.AddDate(0, 0, -1)
	firstOfPrevMonth := time.Date(lastOfPrevMonth.Year(), lastOfPrevMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	return DateRange{Start: firstOfPrevMonth, End: lastOfPrevMonth}
}

// LastNDays creates a DateRange for the last n days (inclusive of today).
func LastNDays(n int) DateRange {
	today := ghutil.TruncateToDay(ghutil.NowUTC())
	start := today.AddDate(0, 0, -(n - 1))
	return DateRange{Start: start, End: today}
}

// Days returns the total number of days in the range (inclusive).
func (dr DateRange) Days() int {
	return int(dr.End.Sub(dr.Start).Hours()/24) + 1
}

// Contains checks if a time falls within this range (date-only comparison).
func (dr DateRange) Contains(t time.Time) bool {
	d := ghutil.TruncateToDay(t)
	return !d.Before(dr.Start) && !d.After(dr.End)
}

// Overlaps checks if this range overlaps with another.
func (dr DateRange) Overlaps(other DateRange) bool {
	return !dr.Start.After(other.End) && !other.Start.After(dr.End)
}

// Subtract removes another range from this one, returning remaining ranges.
func (dr DateRange) Subtract(other DateRange) []DateRange {
	if !dr.Overlaps(other) {
		return []DateRange{dr}
	}
	var result []DateRange
	if dr.Start.Before(other.Start) {
		result = append(result, DateRange{
			Start: dr.Start,
			End:   other.Start.AddDate(0, 0, -1),
		})
	}
	if dr.End.After(other.End) {
		result = append(result, DateRange{
			Start: other.End.AddDate(0, 0, 1),
			End:   dr.End,
		})
	}
	return result
}

// StartDateTime returns the start as beginning of day UTC.
func (dr DateRange) StartDateTime() time.Time {
	return dr.Start
}

// EndDateTime returns the start of the day after End in UTC.
// Callers should use < (not <=) when comparing against this value.
func (dr DateRange) EndDateTime() time.Time {
	return time.Date(dr.End.Year(), dr.End.Month(), dr.End.Day()+1, 0, 0, 0, 0, time.UTC)
}

// ParseDateRange parses CLI flags into a DateRange.
func ParseDateRange(fromDate, toDate string, year int, lastMonth, last90 bool) (DateRange, error) {
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
		return Year(year), nil
	}
	if lastMonth {
		return LastMonth(), nil
	}
	if last90 {
		return LastNDays(90), nil
	}

	if fromDate == "" && toDate == "" {
		return LastNDays(90), nil
	}

	today := ghutil.TruncateToDay(ghutil.NowUTC())
	var start, end time.Time

	start, err := parseDateInput("start", fromDate)
	if err != nil {
		return DateRange{}, err
	}
	if fromDate == "" {
		start = today.AddDate(-1, 0, 0)
	}

	end, err = parseDateInput("end", toDate)
	if err != nil {
		return DateRange{}, err
	}
	if toDate == "" {
		end = today
	}

	return New(start, end)
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
