package ghutil

import (
	"time"
)

// DateFormat is the standard date layout used throughout the application.
const DateFormat = time.DateOnly

// TruncateToDay returns the calendar day instant t falls on in loc, as midnight
// UTC.
//
// Every date in a report is a midnight UTC value, so dates compare and step by
// 24 hours without a daylight-saving day in between. The zone decides only
// which day an instant belongs to: t is read in loc, whatever zone it carries,
// because reading Year/Month/Day in t's own zone gives that zone's day instead.
func TruncateToDay(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}

// SafeDiv returns numerator/denominator as a float64, returning 0 if denominator is 0.
func SafeDiv(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
