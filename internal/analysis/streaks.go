package analysis

import (
	"sort"
	"time"

	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// CalculateStreaks computes streak information from events within a date range.
func CalculateStreaks(events []models.Event, start, end time.Time) models.StreakInfo {
	dateSet := make(map[string]time.Time)
	for _, e := range events {
		d := e.Date()
		dateSet[d.Format(time.DateOnly)] = d
	}
	return calculateStreaksFromDates(sortedDates(dateSet), start, end)
}

// CalculateStreaksFromCalendar computes streak information from contribution calendar days.
func CalculateStreaksFromCalendar(days []models.ContributionDay, start, end time.Time) models.StreakInfo {
	dateSet := make(map[string]time.Time)
	for _, d := range days {
		if d.ContributionCount > 0 {
			dateSet[d.Date.Format(time.DateOnly)] = d.Date
		}
	}
	return calculateStreaksFromDates(sortedDates(dateSet), start, end)
}

// sortedDates extracts and sorts dates from a date set.
func sortedDates(dateSet map[string]time.Time) []time.Time {
	dates := make([]time.Time, 0, len(dateSet))
	for _, d := range dateSet {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})
	return dates
}

// calculateStreaksFromDates computes streaks from a sorted slice of active dates.
func calculateStreaksFromDates(activeDates []time.Time, start, end time.Time) models.StreakInfo {
	totalDays := int(end.Sub(start).Hours()/24) + 1

	if len(activeDates) == 0 {
		return models.StreakInfo{TotalDays: totalDays}
	}

	activeDays := len(activeDates)

	type streak struct {
		start  time.Time
		end    time.Time
		length int
	}

	// Build streaks
	streaks := []streak{}
	sStart := activeDates[0]
	sLen := 1

	for i := 1; i < len(activeDates); i++ {
		diff := activeDates[i].Sub(activeDates[i-1])
		if diff == 24*time.Hour {
			sLen++
		} else {
			streaks = append(streaks, streak{start: sStart, end: activeDates[i-1], length: sLen})
			sStart = activeDates[i]
			sLen = 1
		}
	}
	streaks = append(streaks, streak{start: sStart, end: activeDates[len(activeDates)-1], length: sLen})

	// Find longest
	longest := streaks[0]
	for _, s := range streaks[1:] {
		if s.length > longest.length {
			longest = s
		}
	}

	info := models.StreakInfo{
		LongestStreak:      longest.length,
		LongestStreakStart: &longest.start,
		LongestStreakEnd:   &longest.end,
		ActiveDays:         activeDays,
		TotalDays:          totalDays,
	}

	// Current streak: check from today backwards
	today := ghutil.TruncateToDay(ghutil.NowUTC())
	if !end.Before(today) {
		lastActive := activeDates[len(activeDates)-1]
		yesterday := today.AddDate(0, 0, -1)
		if !lastActive.Before(yesterday) {
			for i := len(streaks) - 1; i >= 0; i-- {
				if !streaks[i].end.Before(yesterday) {
					info.CurrentStreak = streaks[i].length
					info.CurrentStreakStart = &streaks[i].start
					break
				}
			}
		}
	}

	return info
}
