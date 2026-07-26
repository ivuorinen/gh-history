package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ivuorinen/gh-history/internal/testutil"
)

func TestFormatText(t *testing.T) {
	var buf bytes.Buffer
	FormatTextTo(&buf, false, 80, testutil.SampleStats())
	out := buf.String()

	if !strings.Contains(out, "testuser") {
		t.Error("should contain username")
	}
	if !strings.Contains(out, "100") {
		t.Error("should contain total events")
	}
	if !strings.Contains(out, "Commits") {
		t.Error("should contain category label")
	}
	if !strings.Contains(out, "repo1") {
		t.Error("should contain top repo")
	}
}

func TestFormatJSON(t *testing.T) {
	data, err := FormatJSON(testutil.SampleStats())
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result["username"] != "testuser" {
		t.Error("should contain username")
	}
	summary := result["summary"].(map[string]any)
	if summary["total_events"].(float64) != 100 {
		t.Error("should contain total events")
	}
}

func TestFormatMarkdown(t *testing.T) {
	md := FormatMarkdown(testutil.SampleStats())

	if !strings.Contains(md, "# GitHub Activity Report: testuser") {
		t.Error("should contain markdown header")
	}
	if !strings.Contains(md, "| Total Events | 100 |") {
		t.Error("should contain total events row")
	}
	if !strings.Contains(md, "## Top Repositories") {
		t.Error("should contain top repos section")
	}
}

func TestGenerateHTMLContainsCharts(t *testing.T) {
	html, err := buildHTML(testutil.SampleStats())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(html, "chart-categories") {
		t.Error("should contain category chart")
	}
	if !strings.Contains(html, "chart-weekly") {
		t.Error("should contain weekly chart")
	}
	if !strings.Contains(html, "chart-hourly") {
		t.Error("should contain hourly chart")
	}
	if !strings.Contains(html, "testuser") {
		t.Error("should contain username")
	}
}

// The CDN script must be pinned and integrity-checked: "plotly-latest" is a
// moving target and an unverified script runs whatever the CDN serves.
func TestGenerateHTML_PlotlyIsPinnedWithIntegrity(t *testing.T) {
	html, err := buildHTML(testutil.SampleStats())
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(html, "plotly-latest") {
		t.Error("Plotly must not be loaded from an unpinned 'latest' URL")
	}
	if !strings.Contains(html, "cdn.plot.ly/plotly-3.0.1.min.js") {
		t.Error("expected a pinned Plotly version")
	}
	if !strings.Contains(html, `integrity="sha384-`) {
		t.Error("expected a Subresource Integrity hash on the Plotly script")
	}
	if !strings.Contains(html, `crossorigin="anonymous"`) {
		t.Error("SRI requires crossorigin on a cross-origin script")
	}
}

// Charts are drawn by Plotly and carry no text of their own, so each needs a
// text alternative for assistive technology (WCAG 2.2 AA 1.1.1).
func TestGenerateHTML_ChartsHaveTextAlternatives(t *testing.T) {
	html, err := buildHTML(testutil.SampleStats())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`id="chart-categories" role="img" aria-label="`,
		`id="chart-weekly" role="img" aria-label="`,
		`id="chart-hourly" role="img" aria-label="`,
		`id="chart-heatmap" role="img" aria-label="`,
		`id="chart-repos" role="img" aria-label="`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing accessible chart markup: %s", want)
		}
	}
	if !strings.Contains(html, "<main") {
		t.Error("expected a main landmark")
	}
	// Two sections previously shared the heading "Top Repositories".
	if strings.Count(html, ">Top Repositories<") > 1 {
		t.Error("duplicate 'Top Repositories' headings make heading navigation ambiguous")
	}
}

// The heatmap must use the same source as the streak figures, or the two
// contradict each other in the same report.
func TestGenerateHTML_HeatmapPrefersCalendar(t *testing.T) {
	stats := testutil.SampleStats() // has both Calendar and EventsByDate
	html, err := buildHTML(stats)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Contribution Heatmap (all repositories)") {
		t.Error("expected the heatmap to be sourced from the contribution calendar")
	}

	stats.Calendar = nil
	html, err = buildHTML(stats)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Contribution Heatmap (public events)") {
		t.Error("expected the heatmap to fall back to event dates and say so")
	}
}
