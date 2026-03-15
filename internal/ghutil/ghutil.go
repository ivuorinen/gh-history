package ghutil

import (
	"strings"
	"time"
)

// DateFormat is the standard date layout used throughout the application.
const DateFormat = time.DateOnly

// MaxPaginationPages is the maximum number of pages to fetch in pagination loops.
const MaxPaginationPages = 10

// NowUTC returns the current time in UTC.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// TruncateToDay returns t with hour, minute, second, and nanosecond set to zero in UTC.
func TruncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ParseRFC3339Fallback attempts to parse s as RFC3339. It first tries replacing
// a trailing "Z" with "+00:00" (as GitHub sometimes returns), then falls back
// to standard RFC3339 parsing. Returns the zero time if both fail.
func ParseRFC3339Fallback(s string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.Replace(s, "Z", "+00:00", 1))
	if err == nil {
		return t
	}
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	return time.Time{}
}

// ParseDateFallback parses a date string that may be either "2006-01-02" or
// an RFC3339 timestamp (as returned by modernc.org/sqlite DATE handling).
// Returns the zero time if both formats fail.
func ParseDateFallback(s string) time.Time {
	if t, err := time.Parse(DateFormat, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	return time.Time{}
}

// NormalizeUser lowercases a username for consistent cache key usage.
func NormalizeUser(user string) string {
	return strings.ToLower(user)
}

// SafeDiv returns numerator/denominator as a float64, returning 0 if denominator is 0.
func SafeDiv(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
