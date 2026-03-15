package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	ghText "github.com/cli/go-gh/v2/pkg/text"
	"github.com/ivuorinen/gh-history/internal/analysis"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// htmlReportData holds all data needed by the HTML template.
type htmlReportData struct {
	Username  string
	DateStart string
	DateEnd   string
	Cards     []htmlCard

	HasCategories  bool
	CategoriesJSON template.JS

	WeeklyJSON template.JS
	HourlyJSON template.JS

	HasHeatmap  bool
	HeatmapJSON template.JS

	HasTopRepos  bool
	TopReposJSON template.JS
	ReposTable   []htmlRepoRow
}

type htmlCard struct {
	Label string
	Value string
}

type htmlRepoRow struct {
	Rank  int
	Repo  string
	Count int
}

// mustJSON marshals v to JSON for embedding in the template.
// Returns an error instead of silently discarding marshal failures.
func mustJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "null", fmt.Errorf("json marshal: %w", err)
	}
	return string(b), nil
}

var reportTemplate = template.Must(template.New("report").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GitHub Activity Report: {{.Username}}</title>
    <script src="https://cdn.plot.ly/plotly-latest.min.js"></script>
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
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>GitHub Activity Report</h1>
            <div class="date-range">
                {{.Username}} &middot; {{.DateStart}} to {{.DateEnd}}
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

        {{if .HasCategories}}
        <div class="chart-section">
            <h2 class="chart-title">Activity Distribution</h2>
            <div class="chart" id="chart-categories"></div>
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
        </div>
        {{end}}

        <div class="chart-section">
            <h2 class="chart-title">Activity by Day of Week</h2>
            <div class="chart" id="chart-weekly"></div>
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
        </div>

        <div class="chart-section">
            <h2 class="chart-title">Activity by Hour</h2>
            <div class="chart" id="chart-hourly"></div>
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
                        xaxis: {title: 'Hour (UTC)', dtick: 2},
                        yaxis: {title: 'Events'},
                        margin: {t: 20, b: 40, l: 50, r: 20}
                    }, {responsive: true});
                })();
            </script>
        </div>

        {{if .HasHeatmap}}
        <div class="chart-section">
            <h2 class="chart-title">Contribution Heatmap</h2>
            <div class="chart" id="chart-heatmap"></div>
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
        </div>
        {{end}}

        {{if .HasTopRepos}}
        <div class="chart-section">
            <h2 class="chart-title">Top Repositories</h2>
            <div class="chart" id="chart-repos"></div>
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
        </div>

        <div class="chart-section">
            <h2 class="chart-title">Top Repositories</h2>
            <table>
                <thead><tr><th>#</th><th>Repository</th><th>Events</th></tr></thead>
                <tbody>
                    {{range .ReposTable}}
                    <tr><td>{{.Rank}}</td><td>{{.Repo}}</td><td>{{.Count}}</td></tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}
    </div>
</body>
</html>`))

// GenerateHTML creates an HTML report with embedded Plotly charts.
func GenerateHTML(stats models.Statistics, outputPath string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	html, err := buildHTML(stats)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(html), 0o644)
}

func buildHTML(stats models.Statistics) (string, error) {
	data := htmlReportData{
		Username:  stats.Username,
		DateStart: stats.DateRange.Start.Format(ghutil.DateFormat),
		DateEnd:   stats.DateRange.End.Format(ghutil.DateFormat),
	}

	// Cards
	data.Cards = []htmlCard{
		{"Total Events", fmt.Sprintf("%d", stats.TotalEvents)},
		{"Commits", fmt.Sprintf("%d", stats.CommitCount)},
		{"PRs Opened", fmt.Sprintf("%d", stats.PROpened)},
		{"PRs Merged", fmt.Sprintf("%d", stats.PRMerged)},
		{"Code Reviews", fmt.Sprintf("%d", stats.ReviewsCount)},
	}
	if stats.Streaks != nil {
		s := stats.Streaks
		data.Cards = append(data.Cards,
			htmlCard{"Active Days", fmt.Sprintf("%d / %d", s.ActiveDays, s.TotalDays)},
			htmlCard{"Longest Streak", ghText.Pluralize(s.LongestStreak, "day")},
			htmlCard{"Current Streak", ghText.Pluralize(s.CurrentStreak, "day")},
		)
	}

	// Categories chart
	if len(stats.EventsByCategory) > 0 {
		var labels []string
		var values []int
		for _, cat := range AllCategories {
			if count, ok := stats.EventsByCategory[cat]; ok && count > 0 {
				labels = append(labels, analysis.CategoryLabels[cat])
				values = append(values, count)
			}
		}
		catJSON, err := mustJSON(map[string]any{"labels": labels, "values": values})
		if err != nil {
			return "", err
		}
		data.HasCategories = true
		data.CategoriesJSON = template.JS(catJSON)
	}

	// Weekly chart
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	weekCounts := make([]int, 7)
	for i := range 7 {
		weekCounts[i] = stats.EventsByWeekday[i]
	}
	weeklyJSON, err := mustJSON(map[string]any{"x": days, "y": weekCounts})
	if err != nil {
		return "", err
	}
	data.WeeklyJSON = template.JS(weeklyJSON)

	// Hourly chart
	hours := make([]int, 24)
	hourCounts := make([]int, 24)
	for i := range 24 {
		hours[i] = i
		hourCounts[i] = stats.EventsByHour[i]
	}
	hourlyJSON, err := mustJSON(map[string]any{"x": hours, "y": hourCounts})
	if err != nil {
		return "", err
	}
	data.HourlyJSON = template.JS(hourlyJSON)

	// Heatmap
	heatmapData, err := buildHeatmapData(stats)
	if err != nil {
		return "", err
	}
	if heatmapData != "" {
		data.HasHeatmap = true
		data.HeatmapJSON = template.JS(heatmapData)
	}

	// Top repos
	topRepos := stats.TopRepos(15)
	if len(topRepos) > 0 {
		var repoNames []string
		var repoCounts []int
		for i := len(topRepos) - 1; i >= 0; i-- {
			repoNames = append(repoNames, topRepos[i].Repo)
			repoCounts = append(repoCounts, topRepos[i].Count)
		}
		reposJSON, err := mustJSON(map[string]any{"y": repoNames, "x": repoCounts})
		if err != nil {
			return "", err
		}
		data.HasTopRepos = true
		data.TopReposJSON = template.JS(reposJSON)

		for i, rc := range topRepos {
			data.ReposTable = append(data.ReposTable, htmlRepoRow{
				Rank:  i + 1,
				Repo:  rc.Repo,
				Count: rc.Count,
			})
		}
	}

	var buf bytes.Buffer
	if err := reportTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render HTML template: %w", err)
	}
	return buf.String(), nil
}

func buildHeatmapData(stats models.Statistics) (string, error) {
	if len(stats.EventsByDate) == 0 {
		return "", nil
	}

	type dateCount struct {
		date  time.Time
		count int
	}
	var dates []dateCount
	for ds, count := range stats.EventsByDate {
		d, err := time.Parse(ghutil.DateFormat, ds)
		if err != nil {
			continue
		}
		dates = append(dates, dateCount{d, count})
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].date.Before(dates[j].date)
	})

	if len(dates) == 0 {
		return "", nil
	}

	start := dates[0].date
	end := dates[len(dates)-1].date

	dateMap := make(map[string]int)
	for _, dc := range dates {
		dateMap[dc.date.Format(ghutil.DateFormat)] = dc.count
	}

	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	var weekLabels []string
	var z [][]int
	for range 7 {
		z = append(z, []int{})
	}

	current := start
	wd := (int(current.Weekday()) + 6) % 7
	current = current.AddDate(0, 0, -wd)

	for !current.After(end.AddDate(0, 0, 7)) {
		weekLabel := current.Format("Jan 02")
		weekLabels = append(weekLabels, weekLabel)
		for d := range 7 {
			day := current.AddDate(0, 0, d)
			count := dateMap[day.Format(ghutil.DateFormat)]
			z[d] = append(z[d], count)
		}
		current = current.AddDate(0, 0, 7)
	}

	return mustJSON(map[string]any{"y": dayNames, "x": weekLabels, "z": z})
}
