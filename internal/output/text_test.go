package output

import (
	"strings"
	"testing"

	"github.com/ivuorinen/gh-history/internal/models"
	"github.com/ivuorinen/gh-history/internal/testutil"
)

func TestFmtInt(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{7, "7"},
		{999, "999"},
		{1000, "1,000"},
		{9999, "9,999"},
		{999999, "999,999"},
		// A single divide by 1000 renders these as "1000,000" and "1234,567".
		{1000000, "1,000,000"},
		{1234567, "1,234,567"},
		{1000000000, "1,000,000,000"},
		{-1500, "-1,500"},
		{-1234567, "-1,234,567"},
	}
	for _, tc := range tests {
		if got := fmtInt(tc.in); got != tc.want {
			t.Errorf("fmtInt(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Every format renders the same statistics; --format must change presentation
// only, never which numbers a report contains.
func TestSummaryParityAcrossFormats(t *testing.T) {
	stats := testutil.SampleStats()

	var text strings.Builder
	if err := FormatTextTo(&text, false, 200, stats); err != nil {
		t.Fatal(err)
	}
	md := FormatMarkdown(stats)
	html, err := buildHTML(stats)
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range BuildSummary(stats) {
		for name, out := range map[string]string{"text": text.String(), "markdown": md, "html": html} {
			if !strings.Contains(out, row.Label) {
				t.Errorf("%s output is missing the %q row", name, row.Label)
			}
		}
	}

	// Specifically the statistics that used to reach only the JSON formatter.
	for _, label := range []string{"PRs Closed", "Issues Opened", "Issues Closed"} {
		if !strings.Contains(md, label) {
			t.Errorf("markdown should include %q", label)
		}
	}
}

func TestFormatTextTo_ReportsWriterErrors(t *testing.T) {
	if err := FormatTextTo(failingWriter{}, false, 80, testutil.SampleStats()); err == nil {
		t.Error("expected a write failure to be reported, not silently swallowed")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

var errWrite = &writeError{}

type writeError struct{}

func (*writeError) Error() string { return "disk full" }

func TestBuildSummary_NilStreaks(t *testing.T) {
	rows := BuildSummary(models.Statistics{TotalEvents: 3})
	for _, r := range rows {
		if strings.Contains(r.Label, "Streak") || r.Label == "Active Days" {
			t.Errorf("streak rows must be omitted when Streaks is nil, got %q", r.Label)
		}
	}
	if len(rows) == 0 {
		t.Error("expected the non-streak rows to still be present")
	}
}
