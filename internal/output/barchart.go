package output

import (
	"fmt"
	"strings"

	"github.com/ivuorinen/gh-history/internal/analysis"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

var weekdayLabels = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

// BarChartEntry represents one row of a Unicode bar chart.
type BarChartEntry struct {
	Label   string
	Count   int
	Bar     string
	Percent float64
}

// BuildCategoryBars computes bar chart entries for the given categories, ordered as provided.
// barWidth controls the total character width of the bar. Zero-count categories are omitted.
func BuildCategoryBars(stats models.Statistics, barWidth int, categories []models.Category) []BarChartEntry {
	total := stats.TotalEvents
	if total == 0 {
		total = 1
	}

	maxCount := 0
	for _, cat := range categories {
		if c := stats.EventsByCategory[cat]; c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		maxCount = 1
	}

	var entries []BarChartEntry
	for _, cat := range categories {
		count := stats.EventsByCategory[cat]
		if count == 0 {
			continue
		}
		pct := ghutil.SafeDiv(count, total) * 100
		filled := int(ghutil.SafeDiv(count, maxCount) * float64(barWidth))
		bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", barWidth-filled)
		entries = append(entries, BarChartEntry{
			Label:   analysis.CategoryLabels[cat],
			Count:   count,
			Bar:     bar,
			Percent: pct,
		})
	}
	return entries
}

// BuildWeekdayBars computes bar chart entries for activity by day of week (0=Monday–6=Sunday).
// Zero-count days are omitted.
func BuildWeekdayBars(stats models.Statistics, barWidth int) []BarChartEntry {
	total := stats.TotalEvents
	if total == 0 {
		total = 1
	}

	maxCount := 0
	for day := range 7 {
		if c := stats.EventsByWeekday[day]; c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		maxCount = 1
	}

	var entries []BarChartEntry
	for day := range 7 {
		count := stats.EventsByWeekday[day]
		if count == 0 {
			continue
		}
		pct := ghutil.SafeDiv(count, total) * 100
		filled := int(ghutil.SafeDiv(count, maxCount) * float64(barWidth))
		bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", barWidth-filled)
		entries = append(entries, BarChartEntry{
			Label:   weekdayLabels[day],
			Count:   count,
			Bar:     bar,
			Percent: pct,
		})
	}
	return entries
}

// BuildHourlyBars computes bar chart entries for activity by hour (0–23 UTC).
// Zero-count hours are omitted.
func BuildHourlyBars(stats models.Statistics, barWidth int) []BarChartEntry {
	total := stats.TotalEvents
	if total == 0 {
		total = 1
	}

	maxCount := 0
	for hour := range 24 {
		if c := stats.EventsByHour[hour]; c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		maxCount = 1
	}

	var entries []BarChartEntry
	for hour := range 24 {
		count := stats.EventsByHour[hour]
		if count == 0 {
			continue
		}
		pct := ghutil.SafeDiv(count, total) * 100
		filled := int(ghutil.SafeDiv(count, maxCount) * float64(barWidth))
		bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", barWidth-filled)
		entries = append(entries, BarChartEntry{
			Label:   fmt.Sprintf("%02d", hour),
			Count:   count,
			Bar:     bar,
			Percent: pct,
		})
	}
	return entries
}
