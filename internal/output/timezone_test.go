package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/models"
	"github.com/ivuorinen/gh-history/internal/testutil"
)

// zoneStats is a report whose days are days in Helsinki.
func zoneStats(t *testing.T) models.Statistics {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		t.Fatal(err)
	}
	stats := testutil.SampleStats()
	stats.DateRange.Loc = loc
	return stats
}

// Every format records the zone the report's days are counted in, next to the
// date range, so a saved report can be checked and reproduced. The hour charts
// name it too: their hours are hours in that zone, not in UTC.
func TestFormats_RecordTimeZone(t *testing.T) {
	stats := zoneStats(t)

	var text bytes.Buffer
	if err := FormatTextTo(&text, false, 80, stats); err != nil {
		t.Fatal(err)
	}
	html, err := buildHTML(stats)
	if err != nil {
		t.Fatal(err)
	}
	for format, tc := range map[string]struct {
		out  string
		want []string
	}{
		"text": {text.String(), []string{
			"2024-01-01 to 2024-01-31\nTime zone: Europe/Helsinki\n",
			"\nActivity by Hour (Europe/Helsinki)\n",
		}},
		"markdown": {FormatMarkdown(stats), []string{
			"**Period:** 2024-01-01 to 2024-01-31\n\n**Time zone:** Europe/Helsinki\n",
			"\n## Activity by Hour (Europe/Helsinki)\n",
		}},
		"html": {html, []string{
			"2024-01-01 to 2024-01-31 &middot; Time zone: Europe/Helsinki",
			`aria-label="Activity by hour (Europe/Helsinki): `,
			// html/template escapes the slash inside a script's string.
			`title: 'Hour (Europe\/Helsinki)'`,
		}},
	} {
		for _, want := range tc.want {
			if !strings.Contains(tc.out, want) {
				t.Errorf("%s report is missing %q:\n%s", format, want, tc.out)
			}
		}
		if strings.Contains(tc.out, "UTC") {
			t.Errorf("%s report in Europe/Helsinki still names UTC", format)
		}
	}

	dateRange := decodeJSON(t, stats)["date_range"].(map[string]any)
	if dateRange["time_zone"] != "Europe/Helsinki" {
		t.Errorf("JSON date_range.time_zone = %v, want Europe/Helsinki", dateRange["time_zone"])
	}
}

// A range built without a zone is in UTC, and says so.
func TestFormatJSON_ZonelessRangeIsUTC(t *testing.T) {
	dateRange := decodeJSON(t, testutil.SampleStats())["date_range"].(map[string]any)
	if dateRange["time_zone"] != "UTC" {
		t.Errorf("date_range.time_zone = %v, want UTC", dateRange["time_zone"])
	}
}

// The PR merge and issue close rates count each item by its state when the data
// was fetched, so the same period fetched later can give other rates. JSON
// records that moment as generated_at, in the report's zone.
func TestFormatJSON_GeneratedAtInReportZone(t *testing.T) {
	stats := zoneStats(t)
	stats.GeneratedAt = time.Date(2026, 11, 20, 12, 30, 5, 0, time.UTC)
	if got := decodeJSON(t, stats)["generated_at"]; got != "2026-11-20T14:30:05+02:00" {
		t.Errorf("generated_at = %v, want 2026-11-20T14:30:05+02:00", got)
	}
}
