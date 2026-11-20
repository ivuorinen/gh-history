package output

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/ivuorinen/gh-history/internal/analysis"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// htmlReportData holds all data needed by the HTML template.
//
// The *JSON fields are plain Go values, not strings. The template writes them
// inside <script>, where html/template JSON-encodes a value itself, escaping
// anything that could end the script. A pre-marshalled string would need a
// template.JS cast, which tells the template to trust it unescaped.
type htmlReportData struct {
	Username  string
	DateStart string
	DateEnd   string
	TimeZone  string
	Cards     []SummaryRow

	HasCategories  bool
	CategoriesJSON any
	CategoriesAlt  string

	WeeklyJSON any
	WeeklyAlt  string
	HourlyJSON any
	HourlyAlt  string

	HasHeatmap   bool
	HeatmapJSON  any
	HeatmapTitle string
	HeatmapAlt   string

	HasTopRepos  bool
	TopReposJSON any
	TopReposAlt  string
	ReposTable   []htmlRepoRow
}

// barsAlt renders bar-chart entries as a screen-reader alternative. Charts are
// canvas/SVG drawn by Plotly and carry no text of their own, so without this the
// data is unavailable to assistive technology.
func barsAlt(prefix string, entries []BarChartEntry) string {
	if len(entries) == 0 {
		return prefix + ": no data"
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf("%s %d", e.Label, e.Count))
	}
	return prefix + ": " + strings.Join(parts, ", ")
}

type htmlRepoRow struct {
	Rank       int
	Repo       string
	Visibility string // visibilityLabel, so the table words it as text and Markdown do
	Count      int
}

var reportTemplate = template.Must(template.New("report").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GitHub Activity Report: {{.Username}}</title>
    <!-- Pinned version + Subresource Integrity: "plotly-latest" is a moving
         target and an unpinned, unverified CDN script executes whatever the CDN
         serves. The hash is the sha384 of plotly-3.0.1.min.js. -->
    <script src="https://cdn.plot.ly/plotly-3.0.1.min.js"
            integrity="sha384-8cEu0XVLh4s92OG4Ua4ZS75MN//b+0KqyCrhQqaXgHMVHnKC3DNVhwUyH5spa1J2"
            crossorigin="anonymous"></script>
    <style>
        :root {
            --bg-primary: #0d1117;
            --bg-secondary: #161b22;
            --text-primary: #c9d1d9;
            --text-secondary: #8b949e;
            --accent: #58a6ff;
            --green: #3fb950;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            line-height: 1.6;
            padding: 2rem;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        header {
            text-align: center;
            margin-bottom: 2rem;
            padding: 2rem;
            background: var(--bg-secondary);
            border-radius: 8px;
        }
        h1 { color: var(--accent); margin-bottom: 0.5rem; }
        .date-range { color: var(--text-secondary); }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: var(--bg-secondary);
            padding: 1.5rem;
            border-radius: 8px;
            text-align: center;
        }
        .stat-value {
            font-size: 2rem;
            font-weight: bold;
            color: var(--green);
        }
        .stat-label { color: var(--text-secondary); font-size: 0.9rem; }
        .chart-section {
            background: var(--bg-secondary);
            padding: 1.5rem;
            border-radius: 8px;
            margin-bottom: 1.5rem;
        }
        .chart-title { margin-bottom: 1rem; color: var(--accent); }
        .chart { width: 100%; min-height: 300px; }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        th, td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid var(--bg-primary);
        }
        th { color: var(--text-secondary); }
        .chart-fallback {
            display: none;
            padding: 1rem;
            border: 1px solid var(--accent);
            border-radius: 6px;
        }
        .no-plotly .chart-fallback { display: block; }
        .no-plotly .chart { display: none; }
    </style>
</head>
<body>
    <main class="container">
        <header>
            <h1>GitHub Activity Report</h1>
            <div class="date-range">
                {{.Username}} &middot; {{.DateStart}} to {{.DateEnd}} &middot; Time zone: {{.TimeZone}}
            </div>
        </header>

        <div class="stats-grid">
            {{range .Cards}}
            <div class="stat-card">
                <div class="stat-value">{{.Value}}</div>
                <div class="stat-label">{{.Label}}</div>
            </div>
            {{end}}
        </div>

        <p class="chart-fallback">Charts could not be loaded: the Plotly library
        is unavailable (no network connection, or the script failed its integrity
        check). The summary above and the repository table below are unaffected;
        each chart's figures are in its description, which screen readers announce
        and which the page source shows as the chart's <code>aria-label</code>.</p>

        {{if .HasCategories}}
        <section class="chart-section" aria-labelledby="h-categories">
            <h2 class="chart-title" id="h-categories">Activity Distribution</h2>
            <div class="chart" id="chart-categories" role="img" aria-label="{{.CategoriesAlt}}"></div>
            <script>
                (function() {
                    var d = {{.CategoriesJSON}};
                    Plotly.newPlot('chart-categories', [{
                        labels: d.labels,
                        values: d.values,
                        type: 'pie',
                        hole: 0.4
                    }], {
                        paper_bgcolor: 'rgba(0,0,0,0)',
                        plot_bgcolor: 'rgba(0,0,0,0)',
                        font: {color: '#c9d1d9'},
                        showlegend: true,
                        margin: {t: 20, b: 20, l: 20, r: 20}
                    }, {responsive: true});
                })();
            </script>
        </section>
        {{end}}

        <section class="chart-section" aria-labelledby="h-weekly">
            <h2 class="chart-title" id="h-weekly">Activity by Day of Week</h2>
            <div class="chart" id="chart-weekly" role="img" aria-label="{{.WeeklyAlt}}"></div>
            <script>
                (function() {
                    var d = {{.WeeklyJSON}};
                    Plotly.newPlot('chart-weekly', [{
                        x: d.x,
                        y: d.y,
                        type: 'bar',
                        marker: {color: '#3fb950'}
                    }], {
                        paper_bgcolor: 'rgba(0,0,0,0)',
                        plot_bgcolor: 'rgba(0,0,0,0)',
                        font: {color: '#c9d1d9'},
                        xaxis: {title: 'Day of Week'},
                        yaxis: {title: 'Events'},
                        margin: {t: 20, b: 40, l: 50, r: 20}
                    }, {responsive: true});
                })();
            </script>
        </section>

        <section class="chart-section" aria-labelledby="h-hourly">
            <h2 class="chart-title" id="h-hourly">Activity by Hour</h2>
            <div class="chart" id="chart-hourly" role="img" aria-label="{{.HourlyAlt}}"></div>
            <script>
                (function() {
                    var d = {{.HourlyJSON}};
                    Plotly.newPlot('chart-hourly', [{
                        x: d.x,
                        y: d.y,
                        type: 'bar',
                        marker: {color: '#58a6ff'}
                    }], {
                        paper_bgcolor: 'rgba(0,0,0,0)',
                        plot_bgcolor: 'rgba(0,0,0,0)',
                        font: {color: '#c9d1d9'},
                        xaxis: {title: 'Hour ({{.TimeZone}})', dtick: 2},
                        yaxis: {title: 'Events'},
                        margin: {t: 20, b: 40, l: 50, r: 20}
                    }, {responsive: true});
                })();
            </script>
        </section>

        {{if .HasHeatmap}}
        <section class="chart-section" aria-labelledby="h-heatmap">
            <h2 class="chart-title" id="h-heatmap">{{.HeatmapTitle}}</h2>
            <div class="chart" id="chart-heatmap" role="img" aria-label="{{.HeatmapAlt}}"></div>
            <script>
                (function() {
                    var d = {{.HeatmapJSON}};
                    Plotly.newPlot('chart-heatmap', [{
                        y: d.y,
                        x: d.x,
                        z: d.z,
                        type: 'heatmap',
                        colorscale: [[0, '#161b22'], [0.5, '#0e4429'], [1, '#39d353']],
                        showscale: false
                    }], {
                        paper_bgcolor: 'rgba(0,0,0,0)',
                        plot_bgcolor: 'rgba(0,0,0,0)',
                        font: {color: '#c9d1d9'},
                        margin: {t: 20, b: 40, l: 50, r: 20},
                        yaxis: {autorange: 'reversed'}
                    }, {responsive: true});
                })();
            </script>
        </section>
        {{end}}

        {{if .HasTopRepos}}
        <section class="chart-section" aria-labelledby="h-repos-chart">
            <h2 class="chart-title" id="h-repos-chart">Top Repositories</h2>
            <div class="chart" id="chart-repos" role="img" aria-label="{{.TopReposAlt}}"></div>
            <script>
                (function() {
                    var d = {{.TopReposJSON}};
                    Plotly.newPlot('chart-repos', [{
                        y: d.y,
                        x: d.x,
                        type: 'bar',
                        orientation: 'h',
                        marker: {color: '#58a6ff'}
                    }], {
                        paper_bgcolor: 'rgba(0,0,0,0)',
                        plot_bgcolor: 'rgba(0,0,0,0)',
                        font: {color: '#c9d1d9'},
                        xaxis: {title: 'Events'},
                        margin: {t: 20, b: 40, l: 200, r: 20}
                    }, {responsive: true});
                })();
            </script>
        </section>

        <section class="chart-section" aria-labelledby="h-repos-table">
            <h2 class="chart-title" id="h-repos-table">Top Repositories (table)</h2>
            <table>
                <caption class="stat-label">Repositories by number of events in the reporting period</caption>
                <thead><tr><th scope="col">#</th><th scope="col">Repository</th><th scope="col">Visibility</th><th scope="col">Events</th></tr></thead>
                <tbody>
                    {{range .ReposTable}}
                    <tr><td>{{.Rank}}</td><td>{{.Repo}}</td><td>{{.Visibility}}</td><td>{{.Count}}</td></tr>
                    {{end}}
                </tbody>
            </table>
        </section>
        {{end}}
    </main>
    <script>
        // Surface a readable message instead of blank boxes when the CDN script
        // is missing or fails its integrity check.
        if (typeof Plotly === 'undefined') {
            document.documentElement.classList.add('no-plotly');
        }
    </script>
</body>
</html>`))

// GenerateHTML creates an HTML report with embedded Plotly charts. It creates
// no directory, as --output does for every other format, and the file is
// readable by its owner only (see ghutil.WriteOwnerOnly).
func GenerateHTML(stats models.Statistics, outputPath string) error {
	html, err := buildHTML(stats)
	if err != nil {
		return err
	}
	return ghutil.WriteOwnerOnly(outputPath, []byte(html))
}

func buildHTML(stats models.Statistics) (string, error) {
	data := htmlReportData{
		Username:  stats.Username,
		DateStart: stats.DateRange.Start.Format(ghutil.DateFormat),
		DateEnd:   stats.DateRange.End.Format(ghutil.DateFormat),
		TimeZone:  stats.DateRange.Location().String(),
		// Same source as every other format, so no statistic is format-specific.
		Cards: BuildSummary(stats),
	}
	addCategoryChart(&data, stats)
	addTimeCharts(&data, stats)
	addHeatmap(&data, stats)
	addTopRepos(&data, stats)

	var buf bytes.Buffer
	if err := reportTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render HTML template: %w", err)
	}
	return buf.String(), nil
}

// addCategoryChart fills the category chart, in AllCategories order. The
// template leaves the chart out when no event has a category.
func addCategoryChart(data *htmlReportData, stats models.Statistics) {
	if len(stats.EventsByCategory) == 0 {
		return
	}
	var labels []string
	var values []int
	for _, cat := range AllCategories {
		if count, ok := stats.EventsByCategory[cat]; ok && count > 0 {
			labels = append(labels, analysis.CategoryLabels[cat])
			values = append(values, count)
		}
	}
	data.HasCategories = true
	data.CategoriesJSON = map[string]any{"labels": labels, "values": values}
	data.CategoriesAlt = barsAlt("Activity distribution by category",
		BuildCategoryBars(stats, 1, AllCategories))
}

// addTimeCharts fills the weekday and hour charts. Every weekday and hour gets
// a bar, a zero one included, so the axes are the same in every report.
func addTimeCharts(data *htmlReportData, stats models.Statistics) {
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	weekCounts := make([]int, 7)
	for i := range 7 {
		weekCounts[i] = stats.EventsByWeekday[i]
	}
	data.WeeklyJSON = map[string]any{"x": days, "y": weekCounts}
	data.WeeklyAlt = barsAlt("Activity by day of week", BuildWeekdayBars(stats, 1))

	hours := make([]int, 24)
	hourCounts := make([]int, 24)
	for i := range 24 {
		hours[i] = i
		hourCounts[i] = stats.EventsByHour[i]
	}
	data.HourlyJSON = map[string]any{"x": hours, "y": hourCounts}
	data.HourlyAlt = barsAlt("Activity by hour ("+data.TimeZone+")", BuildHourlyBars(stats, 1))
}

// addHeatmap fills the heatmap and names its source in the title: the calendar
// counts commits and contributions the token cannot list, event dates do not,
// and the two give visibly different pictures.
func addHeatmap(data *htmlReportData, stats models.Statistics) {
	payload, summary, fromCalendar := buildHeatmapData(stats)
	if payload == nil {
		return
	}
	data.HasHeatmap = true
	data.HeatmapJSON = payload
	if fromCalendar {
		data.HeatmapTitle = "Contribution Heatmap (all repositories)"
	} else {
		data.HeatmapTitle = "Contribution Heatmap (events)"
	}
	data.HeatmapAlt = summary
}

// addTopRepos fills the top-15 chart and table. The chart's lists run from the
// 15th repository to the first: a horizontal bar chart draws its first category
// at the bottom, so the busiest repository ends up on top.
func addTopRepos(data *htmlReportData, stats models.Statistics) {
	topRepos := stats.TopRepos(15)
	if len(topRepos) == 0 {
		return
	}
	var repoNames []string
	var repoCounts []int
	for i := len(topRepos) - 1; i >= 0; i-- {
		repoNames = append(repoNames, topRepos[i].Repo)
		repoCounts = append(repoCounts, topRepos[i].Count)
	}
	data.HasTopRepos = true
	data.TopReposJSON = map[string]any{"y": repoNames, "x": repoCounts}
	altParts := make([]string, 0, len(topRepos))
	for _, rc := range topRepos {
		altParts = append(altParts, fmt.Sprintf("%s %d", rc.Repo, rc.Count))
	}
	data.TopReposAlt = "Top repositories by events: " + strings.Join(altParts, ", ")

	for i, rc := range topRepos {
		data.ReposTable = append(data.ReposTable, htmlRepoRow{
			Rank:       i + 1,
			Repo:       rc.Repo,
			Visibility: visibilityLabel(rc.Private),
			Count:      rc.Count,
		})
	}
}

// heatmapSource returns the per-day counts the heatmap draws and whether they
// come from the contribution calendar. The calendar wins over event dates
// because it also counts commits. analysis.Calculate makes the same choice for
// streaks, and a different source here would make the heatmap contradict the
// streak figures in the same report.
func heatmapSource(stats models.Statistics) (map[string]int, bool) {
	if stats.Calendar == nil || len(stats.Calendar.Days) == 0 {
		return stats.EventsByDate, false
	}
	dateMap := make(map[string]int, len(stats.Calendar.Days))
	for _, d := range stats.Calendar.Days {
		dateMap[d.Date.Format(ghutil.DateFormat)] = d.ContributionCount
	}
	return dateMap, true
}

// buildHeatmapData returns the heatmap's chart data (nil when there is no day
// to draw), a text alternative, and whether the data came from the
// contribution calendar rather than event dates.
func buildHeatmapData(stats models.Statistics) (payload any, alt string, fromCalendar bool) {
	dateMap, fromCalendar := heatmapSource(stats)
	if len(dateMap) == 0 {
		return nil, "", fromCalendar
	}

	type dateCount struct {
		date  time.Time
		count int
	}
	var dates []dateCount
	total := 0
	// The calendar source includes every day in the window, zero-count days
	// among them, so active days must be counted rather than taken from len().
	activeDays := 0
	for ds, count := range dateMap {
		d, err := time.Parse(ghutil.DateFormat, ds)
		if err != nil {
			continue
		}
		dates = append(dates, dateCount{d, count})
		total += count
		if count > 0 {
			activeDays++
		}
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].date.Before(dates[j].date)
	})

	if len(dates) == 0 {
		return nil, "", fromCalendar
	}

	start := dates[0].date
	end := dates[len(dates)-1].date

	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	var weekLabels []string
	var z [][]int
	for range 7 {
		z = append(z, []int{})
	}

	current := start
	wd := (int(current.Weekday()) + 6) % 7
	current = current.AddDate(0, 0, -wd)

	// current is Monday-aligned and steps a week at a time, so this already
	// emits the whole week containing end. Extending past end would render a
	// trailing all-zero column.
	for !current.After(end) {
		// The full date, not "Jan 02": Plotly merges identical category labels,
		// and a Monday's month and day recur in later years of a long range.
		weekLabels = append(weekLabels, current.Format(ghutil.DateFormat))
		for d := range 7 {
			day := current.AddDate(0, 0, d)
			z[d] = append(z[d], dateMap[day.Format(ghutil.DateFormat)])
		}
		current = current.AddDate(0, 0, 7)
	}

	payload = map[string]any{"y": dayNames, "x": weekLabels, "z": z}
	source := "events"
	if fromCalendar {
		source = "all repositories"
	}
	alt = fmt.Sprintf("Contribution heatmap from %s: %d contributions across %d active days, %s to %s",
		source, total, activeDays,
		start.Format(ghutil.DateFormat), end.Format(ghutil.DateFormat))
	return payload, alt, fromCalendar
}
