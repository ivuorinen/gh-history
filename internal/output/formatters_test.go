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

	if !strings.Contains(html, "plotly-latest.min.js") {
		t.Error("should include Plotly CDN")
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
