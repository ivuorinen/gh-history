package main

import (
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/daterange"
)

func d(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T, c *config)
	}{
		{
			name: "positional username",
			args: []string{"octocat"},
			check: func(t *testing.T, c *config) {
				if c.username != "octocat" {
					t.Errorf("username = %q", c.username)
				}
			},
		},
		{
			name: "no username leaves it empty for resolution",
			args: []string{},
			check: func(t *testing.T, c *config) {
				if c.username != "" {
					t.Errorf("username = %q, want empty", c.username)
				}
			},
		},
		{
			name: "format is lowercased",
			args: []string{"--format", "JSON"},
			check: func(t *testing.T, c *config) {
				if c.format != "json" {
					t.Errorf("format = %q, want json", c.format)
				}
			},
		},
		{
			name: "format defaults to markdown",
			args: []string{},
			check: func(t *testing.T, c *config) {
				if c.format != "markdown" {
					t.Errorf("format = %q, want markdown", c.format)
				}
			},
		},
		{
			name: "short flags match long flags",
			args: []string{"-f", "2024-01-01", "-t", "2024-06-30", "-o", "out.json", "-y", "2024", "-v"},
			check: func(t *testing.T, c *config) {
				if c.fromDate != "2024-01-01" || c.toDate != "2024-06-30" {
					t.Errorf("dates = %q..%q", c.fromDate, c.toDate)
				}
				if c.outputFile != "out.json" {
					t.Errorf("output = %q", c.outputFile)
				}
				if c.year != 2024 {
					t.Errorf("year = %d", c.year)
				}
				if !c.verbose {
					t.Error("verbose should be set")
				}
			},
		},
		{
			name: "flags before the username still leave it positional",
			args: []string{"--verbose", "octocat"},
			check: func(t *testing.T, c *config) {
				if c.username != "octocat" || !c.verbose {
					t.Errorf("username = %q verbose = %v", c.username, c.verbose)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, parseFlags(tc.args))
		})
	}
}

func TestSplitIntoYearChunks_SingleDay(t *testing.T) {
	dr := daterange.DateRange{Start: d(2024, 6, 15), End: d(2024, 6, 15)}
	chunks := splitIntoYearChunks(dr)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Start != dr.Start || chunks[0].End != dr.End {
		t.Errorf("chunk mismatch: got %v to %v", chunks[0].Start, chunks[0].End)
	}
}

func TestSplitIntoYearChunks_ExactlyOneYear(t *testing.T) {
	dr := daterange.DateRange{Start: d(2024, 1, 1), End: d(2024, 12, 31)}
	chunks := splitIntoYearChunks(dr)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Start != dr.Start || chunks[0].End != dr.End {
		t.Errorf("chunk mismatch: got %v to %v", chunks[0].Start, chunks[0].End)
	}
}

func TestSplitIntoYearChunks_TwoYears(t *testing.T) {
	dr := daterange.DateRange{Start: d(2023, 1, 1), End: d(2024, 12, 31)}
	chunks := splitIntoYearChunks(dr)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Start != d(2023, 1, 1) {
		t.Errorf("first chunk start: got %v", chunks[0].Start)
	}
	if chunks[0].End != d(2023, 12, 31) {
		t.Errorf("first chunk end: got %v", chunks[0].End)
	}
	if chunks[1].Start != d(2024, 1, 1) {
		t.Errorf("second chunk start: got %v", chunks[1].Start)
	}
	if chunks[1].End != d(2024, 12, 31) {
		t.Errorf("second chunk end: got %v", chunks[1].End)
	}
}

func TestSplitIntoYearChunks_CrossYearBoundary(t *testing.T) {
	dr := daterange.DateRange{Start: d(2023, 6, 1), End: d(2024, 6, 30)}
	chunks := splitIntoYearChunks(dr)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	// First chunk: 2023-06-01 to 2024-05-31 (1 year - 1 day from start)
	if chunks[0].Start != d(2023, 6, 1) {
		t.Errorf("first chunk start: got %v", chunks[0].Start)
	}
	// Second chunk should start the day after first chunk ends
	if !chunks[1].End.Equal(d(2024, 6, 30)) {
		t.Errorf("second chunk end: got %v, want %v", chunks[1].End, d(2024, 6, 30))
	}
	// Chunks should be contiguous
	expectedSecondStart := chunks[0].End.AddDate(0, 0, 1)
	if !chunks[1].Start.Equal(expectedSecondStart) {
		t.Errorf("chunks not contiguous: first ends %v, second starts %v", chunks[0].End, chunks[1].Start)
	}
}
