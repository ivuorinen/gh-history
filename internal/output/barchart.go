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

// buildBars is the one bar-chart algorithm behind every Build*Bars function.
// labels[i] names counts[i]; zero counts are omitted. Percentages are of total
// (all events), bar lengths are relative to the largest count shown.
func buildBars(total, barWidth int, labels []string, counts []int) []BarChartEntry {
	if total == 0 {
		total = 1
	}
	maxCount := 1
	for _, c := range counts {
		maxCount = max(maxCount, c)
	}

	var entries []BarChartEntry
	for i, count := range counts {
		if count == 0 {
			continue
		}
		filled := int(ghutil.SafeDiv(count, maxCount) * float64(barWidth))
		entries = append(entries, BarChartEntry{
			Label:   labels[i],
			Count:   count,
			Bar:     strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled),
			Percent: ghutil.SafeDiv(count, total) * 100,
		})
	}
	return entries
}

// BuildCategoryBars computes bar chart entries for the given categories, ordered as provided.
// barWidth controls the total character width of the bar. Zero-count categories are omitted.
func BuildCategoryBars(stats models.Statistics, barWidth int, categories []models.Category) []BarChartEntry {
	labels := make([]string, len(categories))
	counts := make([]int, len(categories))
	for i, cat := range categories {
		labels[i], counts[i] = analysis.CategoryLabels[cat], stats.EventsByCategory[cat]
	}
	return buildBars(stats.TotalEvents, barWidth, labels, counts)
}

// BuildWeekdayBars computes bar chart entries for activity by day of week (0=Monday–6=Sunday).
// Zero-count days are omitted.
func BuildWeekdayBars(stats models.Statistics, barWidth int) []BarChartEntry {
	counts := make([]int, 7)
	for day := range counts {
		counts[day] = stats.EventsByWeekday[day]
	}
	return buildBars(stats.TotalEvents, barWidth, weekdayLabels, counts)
}

// BuildHourlyBars computes bar chart entries for activity by hour (0–23 UTC).
// Zero-count hours are omitted.
func BuildHourlyBars(stats models.Statistics, barWidth int) []BarChartEntry {
	labels := make([]string, 24)
	counts := make([]int, 24)
	for hour := range counts {
		labels[hour], counts[hour] = fmt.Sprintf("%02d", hour), stats.EventsByHour[hour]
	}
	return buildBars(stats.TotalEvents, barWidth, labels, counts)
}
