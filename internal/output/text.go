package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// AllCategories lists categories in display order. It matches the categories
// analysis.EventCategories can actually produce.
var AllCategories = []models.Category{
	models.CategoryPullRequests,
	models.CategoryIssues,
	models.CategoryReviews,
	models.CategoryComments,
	models.CategoryRepos,
	models.CategoryOther,
}

// FormatText writes a plain text report to stdout using terminal-aware table formatting.
func FormatText(stats models.Statistics) error {
	t := term.FromEnv()
	isTTY := t.IsTerminalOutput()
	width := 80
	if w, _, err := t.Size(); err == nil && w > 0 {
		width = w
	}
	return FormatTextTo(t.Out(), isTTY, width, stats)
}

// errWriter records the first write error. go-gh's non-TTY table printer writes
// eagerly and its Render always returns nil, so checking Render alone would let
// a truncated report be reported as a success.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) Write(p []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	n, err := e.w.Write(p)
	if err != nil {
		e.err = err
	}
	return n, err
}

// FormatTextTo writes a plain text report to w. This is the testable core of FormatText.
func FormatTextTo(out io.Writer, isTTY bool, width int, stats models.Statistics) error {
	w := &errWriter{w: out}

	// Header
	fmt.Fprintf(w, "GitHub Activity Report: %s\n", stats.Username)
	fmt.Fprintf(w, "%s to %s\n", stats.DateRange.Start.Format(ghutil.DateFormat), stats.DateRange.End.Format(ghutil.DateFormat))
	fmt.Fprintln(w, strings.Repeat("-", 60))

	// Summary table
	fmt.Fprintln(w, "\nSummary")
	fmt.Fprintln(w, strings.Repeat("-", 50))

	tp := tableprinter.New(w, isTTY, width)
	for _, row := range BuildSummary(stats) {
		tp.AddField(row.Label)
		tp.AddField(row.Value)
		tp.EndRow()
	}
	if err := tp.Render(); err != nil {
		return fmt.Errorf("render summary table: %w", err)
	}

	// Categories — keep manual formatting for the Unicode bar chart
	fmt.Fprintln(w, "\nActivity by Category")
	fmt.Fprintln(w, strings.Repeat("-", 50))

	for _, entry := range BuildCategoryBars(stats, 20, AllCategories) {
		fmt.Fprintf(w, "  %s %6s  %s  %5.1f%%\n", padRight(18, entry.Label), fmtInt(entry.Count), entry.Bar, entry.Percent)
	}

	// Weekday distribution
	if weekdayEntries := BuildWeekdayBars(stats, 20); len(weekdayEntries) > 0 {
		fmt.Fprintln(w, "\nActivity by Day of Week")
		fmt.Fprintln(w, strings.Repeat("-", 50))
		for _, entry := range weekdayEntries {
			fmt.Fprintf(w, "  %s %6s  %s  %5.1f%%\n", padRight(18, entry.Label), fmtInt(entry.Count), entry.Bar, entry.Percent)
		}
	}

	// Hourly distribution
	if hourlyEntries := BuildHourlyBars(stats, 20); len(hourlyEntries) > 0 {
		fmt.Fprintln(w, "\nActivity by Hour (UTC)")
		fmt.Fprintln(w, strings.Repeat("-", 50))
		for _, entry := range hourlyEntries {
			fmt.Fprintf(w, "  %s %6s  %s  %5.1f%%\n", padRight(18, entry.Label), fmtInt(entry.Count), entry.Bar, entry.Percent)
		}
	}

	// Top repos table
	topRepos := stats.TopRepos(15)
	if len(topRepos) > 0 {
		fmt.Fprintln(w, "\nTop Repositories")
		fmt.Fprintln(w, strings.Repeat("-", 50))

		tp2 := tableprinter.New(w, isTTY, width)
		for i, rc := range topRepos {
			tp2.AddField(fmt.Sprintf("%d.", i+1))
			tp2.AddField(rc.Repo)
			tp2.AddField(fmt.Sprintf("%s events", fmtInt(rc.Count)))
			tp2.EndRow()
		}
		if err := tp2.Render(); err != nil {
			return fmt.Errorf("render repositories table: %w", err)
		}
	}
	if w.err != nil {
		return fmt.Errorf("write text report: %w", w.err)
	}
	return nil
}

// padRight pads s on the right with spaces to reach width, and never truncates.
//
// Width is measured in runes. Every label passed here is a fixed ASCII string
// (category names, weekday names, zero-padded hours), so this is exact; a label
// containing double-width runes would under-pad by one column per such rune.
func padRight(width int, s string) string {
	if pad := width - utf8.RuneCountInString(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// pluralize renders "1 day" / "2 days". The naive "s" suffix matches every noun
// this is called with.
func pluralize(n int, thing string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, thing)
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// fmtInt renders n with thousands separators. It groups every three digits, not
// just the first: a single divide by 1000 renders 1234567 as "1234,567".
func fmtInt(n int) string {
	s := strconv.Itoa(n)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return sign + s
}
