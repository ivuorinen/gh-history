package ghutil

import (
	"time"
)

// DateFormat is the standard date layout used throughout the application.
const DateFormat = time.DateOnly

// MaxPaginationPages is the maximum number of pages to fetch in pagination loops.
const MaxPaginationPages = 10

// TruncateToDay returns the start of t's UTC day.
//
// t is converted to UTC first: reading Year/Month/Day off a non-UTC time yields
// that zone's calendar day, which would then be mislabelled as a UTC date. Every
// caller currently passes a UTC time, so this changes no existing behaviour — it
// makes the stated contract true for any caller that does not.
func TruncateToDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// SafeDiv returns numerator/denominator as a float64, returning 0 if denominator is 0.
func SafeDiv(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
