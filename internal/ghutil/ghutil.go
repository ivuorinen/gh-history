package ghutil

import (
	"time"
)

// DateFormat is the standard date layout used throughout the application.
const DateFormat = time.DateOnly

// MaxPaginationPages is the maximum number of pages to fetch in pagination loops.
const MaxPaginationPages = 10

// TruncateToDay returns t with hour, minute, second, and nanosecond set to zero in UTC.
func TruncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// SafeDiv returns numerator/denominator as a float64, returning 0 if denominator is 0.
func SafeDiv(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
