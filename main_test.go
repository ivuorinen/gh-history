package main

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/api"
	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/models"
)

func d(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestResolveHost(t *testing.T) {
	// An explicit flag beats the environment; the environment beats the default.
	t.Run("flag wins over GH_HOST", func(t *testing.T) {
		t.Setenv("GH_HOST", "env.example.com")
		if got := resolveHost("flag.example.com"); got != "flag.example.com" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("GH_HOST used when no flag", func(t *testing.T) {
		t.Setenv("GH_HOST", "env.example.com")
		if got := resolveHost(""); got != "env.example.com" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("defaults to github.com", func(t *testing.T) {
		t.Setenv("GH_HOST", "")
		t.Setenv("GH_CONFIG_DIR", t.TempDir()) // no hosts.yml
		if got := resolveHost(""); got != "github.com" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("sole configured host is used", func(t *testing.T) {
		dir := t.TempDir()
		writeHosts(t, dir, "github.example.com:\n    users:\n        me:\n            oauth_token: x\n")
		t.Setenv("GH_HOST", "")
		t.Setenv("GH_CONFIG_DIR", dir)
		if got := resolveHost(""); got != "github.example.com" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("ambiguous config falls back to github.com", func(t *testing.T) {
		dir := t.TempDir()
		writeHosts(t, dir, "github.com:\n    a: 1\ngithub.example.com:\n    b: 2\n")
		t.Setenv("GH_HOST", "")
		t.Setenv("GH_CONFIG_DIR", dir)
		if got := resolveHost(""); got != "github.com" {
			t.Errorf("got %q", got)
		}
	})
}

func writeHosts(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "hosts.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestConfiguredHosts(t *testing.T) {
	dir := t.TempDir()
	writeHosts(t, dir, "# a comment\ngithub.com:\n    users:\n        me:\n            oauth_token: x\ngithub.example.com:\n    git_protocol: ssh\n")
	t.Setenv("GH_CONFIG_DIR", dir)

	got := configuredHosts()
	want := []string{"github.com", "github.example.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestConfiguredHosts_MissingFile(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	if got := configuredHosts(); len(got) != 0 {
		t.Errorf("expected none, got %v", got)
	}
}

// An Enterprise host must never be handed a github.com token, or vice versa.
func TestTokenEnvVars(t *testing.T) {
	if got := tokenEnvVars("github.com"); got[0] != "GH_TOKEN" || got[1] != "GITHUB_TOKEN" {
		t.Errorf("github.com -> %v", got)
	}
	if got := tokenEnvVars("acme.ghe.com"); got[0] != "GH_TOKEN" {
		t.Errorf("tenancy should use the github.com vars, got %v", got)
	}
	got := tokenEnvVars("github.example.com")
	if got[0] != "GH_ENTERPRISE_TOKEN" || got[1] != "GITHUB_ENTERPRISE_TOKEN" {
		t.Errorf("enterprise -> %v", got)
	}
}

func TestResolveToken(t *testing.T) {
	t.Run("GH_TOKEN wins for github.com", func(t *testing.T) {
		t.Setenv("GH_TOKEN", "a")
		t.Setenv("GITHUB_TOKEN", "b")
		if got := resolveToken("github.com"); got != "a" {
			t.Errorf("got %q, want a", got)
		}
	})
	t.Run("falls through to GITHUB_TOKEN", func(t *testing.T) {
		t.Setenv("GH_TOKEN", "")
		t.Setenv("GITHUB_TOKEN", "b")
		if got := resolveToken("github.com"); got != "b" {
			t.Errorf("got %q, want b", got)
		}
	})
	t.Run("enterprise ignores the github.com vars", func(t *testing.T) {
		t.Setenv("GH_TOKEN", "dotcom")
		t.Setenv("GITHUB_TOKEN", "dotcom")
		t.Setenv("GH_ENTERPRISE_TOKEN", "ent")
		if got := resolveToken("github.example.com"); got != "ent" {
			t.Errorf("got %q, want ent", got)
		}
	})
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config
		wantErr bool
	}{
		{"neither given", config{}, false},
		{"two positionals", config{positional: []string{"octocat", "torvalds"}}, true},
		{"flag only", config{username: "octocat"}, false},
		{"positional only", config{positional: []string{"octocat"}}, false},
		{"both, agreeing", config{username: "octocat", positional: []string{"octocat"}}, false},
		// Silently picking one would hide a typo in the other.
		{"both, disagreeing", config{username: "octocat", positional: []string{"torvalds"}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestConfigSubjectPrefersFlag(t *testing.T) {
	c := config{username: "octocat", positional: []string{"octocat"}}
	if got := c.subject(); got != "octocat" {
		t.Errorf("subject() = %q, want octocat", got)
	}
	c = config{positional: []string{"torvalds"}}
	if got := c.subject(); got != "torvalds" {
		t.Errorf("subject() = %q, want torvalds", got)
	}
}

func TestBrowserCommand(t *testing.T) {
	tests := []struct {
		name     string
		launcher string
		goos     string
		wantName string
		wantArgN int
	}{
		{"linux default", "", "linux", "xdg-open", 1},
		{"macos default", "", "darwin", "open", 1},
		{"windows default", "", "windows", "rundll32", 2},
		{"BROWSER wins over the platform default", "firefox", "linux", "firefox", 1},
		{"BROWSER with its own flags", "firefox --new-tab", "linux", "firefox", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, args, err := browserCommand("report.html", tc.launcher, tc.goos)
			if err != nil {
				t.Fatal(err)
			}
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			if len(args) != tc.wantArgN {
				t.Errorf("args = %v, want %d of them", args, tc.wantArgN)
			}
			if !filepath.IsAbs(args[len(args)-1]) {
				t.Errorf("path argument must be absolute, got %q", args[len(args)-1])
			}
		})
	}
}

// A relative path beginning with "-" would be read as a flag by the opener;
// making it absolute is what prevents that.
func TestBrowserCommand_LeadingDashPathIsNotAFlag(t *testing.T) {
	_, args, err := browserCommand("-rf.html", "", "linux")
	if err != nil {
		t.Fatal(err)
	}
	got := args[len(args)-1]
	if strings.HasPrefix(got, "-") {
		t.Errorf("path argument still looks like a flag: %q", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected an absolute path, got %q", got)
	}
}

func TestSortedRepoCounts(t *testing.T) {
	// Counts accumulated across year chunks: repo1 appears in two chunks.
	got := sortedRepoCounts(map[string]models.RepoCount{
		"u/repo1": {Repo: "u/repo1", Count: 30 + 20},
		"u/repo2": {Repo: "u/repo2", Count: 40},
		"u/repo3": {Repo: "u/repo3", Count: 40},
	})
	want := []models.RepoCount{
		{Repo: "u/repo1", Count: 50},
		{Repo: "u/repo2", Count: 40}, // tie broken by name, so output is stable
		{Repo: "u/repo3", Count: 40},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSortedRepoCounts_Empty(t *testing.T) {
	if got := sortedRepoCounts(map[string]models.RepoCount{}); len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
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
				if c.subject() != "octocat" {
					t.Errorf("subject = %q", c.subject())
				}
			},
		},
		{
			name: "no username leaves it empty for resolution",
			args: []string{},
			check: func(t *testing.T, c *config) {
				if c.subject() != "" {
					t.Errorf("subject = %q, want empty", c.subject())
				}
			},
		},
		{
			name: "--username flag",
			args: []string{"--username", "octocat"},
			check: func(t *testing.T, c *config) {
				if c.subject() != "octocat" {
					t.Errorf("subject = %q", c.subject())
				}
			},
		},
		{
			name: "-u short form",
			args: []string{"-u", "octocat"},
			check: func(t *testing.T, c *config) {
				if c.subject() != "octocat" {
					t.Errorf("subject = %q", c.subject())
				}
			},
		},
		{
			name: "--hostname",
			args: []string{"--hostname", "github.example.com"},
			check: func(t *testing.T, c *config) {
				if c.hostname != "github.example.com" {
					t.Errorf("hostname = %q", c.hostname)
				}
			},
		},
		{
			name: "hostname defaults to empty so resolveHost picks it",
			args: []string{},
			check: func(t *testing.T, c *config) {
				if c.hostname != "" {
					t.Errorf("hostname = %q, want empty", c.hostname)
				}
			},
		},
		{
			name: "--time-zone",
			args: []string{"octocat", "--time-zone", "Europe/Helsinki"},
			check: func(t *testing.T, c *config) {
				if c.timeZone != "Europe/Helsinki" {
					t.Errorf("timeZone = %q", c.timeZone)
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
			// Go's flag package stops at the first non-flag argument, so this
			// form silently ignored every flag after the username. It is the
			// form the README documents throughout.
			name: "flags AFTER the positional username are still parsed",
			args: []string{"octocat", "--format", "json", "--from", "2024-01-01", "--verbose"},
			check: func(t *testing.T, c *config) {
				if c.subject() != "octocat" {
					t.Errorf("subject = %q", c.subject())
				}
				if c.format != "json" {
					t.Errorf("format = %q, want json — the flag after the username was dropped", c.format)
				}
				if c.fromDate != "2024-01-01" {
					t.Errorf("fromDate = %q, want 2024-01-01", c.fromDate)
				}
				if !c.verbose {
					t.Error("verbose flag after the username was dropped")
				}
			},
		},
		{
			name: "flags interleaved around the username",
			args: []string{"--verbose", "octocat", "--format", "text"},
			check: func(t *testing.T, c *config) {
				if c.subject() != "octocat" || !c.verbose || c.format != "text" {
					t.Errorf("subject=%q verbose=%v format=%q", c.subject(), c.verbose, c.format)
				}
			},
		},
		{
			name: "two positional usernames are both captured for validate to reject",
			args: []string{"octocat", "torvalds"},
			check: func(t *testing.T, c *config) {
				if len(c.positional) != 2 {
					t.Errorf("positional = %v, want both captured", c.positional)
				}
			},
		},
		{
			name: "flags before the username still leave it positional",
			args: []string{"--verbose", "octocat"},
			check: func(t *testing.T, c *config) {
				if c.posUsername() != "octocat" || !c.verbose {
					t.Errorf("posUsername = %q verbose = %v", c.posUsername(), c.verbose)
				}
				if c.subject() != "octocat" {
					t.Errorf("subject = %q", c.subject())
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

// The zone comes from --time-zone, then TZ when it names an IANA zone, then
// UTC, never from the machine's /etc/localtime, so the same command counts the
// same days everywhere. A bad --time-zone is an error, since it was asked for;
// a TZ that is not an IANA name falls back to UTC with a warning, since GitHub
// would not understand it.
func TestResolveTimeZone(t *testing.T) {
	tests := []struct {
		name     string
		flag, tz string
		want     string // the zone's name; empty when an error is wanted
		warns    bool
	}{
		{name: "flag", flag: "Europe/Helsinki", tz: "America/Los_Angeles", want: "Europe/Helsinki"},
		{name: "flag wins over a bad TZ, unwarned", flag: "UTC", tz: "CET-1CEST,M3.5.0", want: "UTC"},
		{name: "bad flag", flag: "Mars/Olympus_Mons"},
		{name: "POSIX flag", flag: "CET-1CEST,M3.5.0"},
		{name: "Local is not a zone name", flag: "Local"},
		// Debian's tzdata links zoneinfo/localtime to /etc/localtime, so
		// time.LoadLocation would load the machine's zone under this name.
		{name: "localtime is not a zone name", flag: "localtime"},
		{name: "TZ", tz: "America/Los_Angeles", want: "America/Los_Angeles"},
		{name: "TZ with the POSIX leading colon", tz: ":Europe/Berlin", want: "Europe/Berlin"},
		{name: "POSIX TZ falls back to UTC", tz: "CET-1CEST,M3.5.0,M10.5.0/3", want: "UTC", warns: true},
		{name: "TZ=localtime falls back to UTC", tz: "localtime", want: "UTC", warns: true},
		{name: "unset TZ is UTC", want: "UTC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, warning, err := resolveTimeZone(tt.flag, tt.tz)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("--time-zone %q resolved to %v, want an error", tt.flag, loc)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if loc.String() != tt.want {
				t.Errorf("zone = %v, want %s", loc, tt.want)
			}
			if (warning != "") != tt.warns {
				t.Errorf("warning = %q, want one: %v", warning, tt.warns)
			}
		})
	}
}

// Every year chunk keeps the range's zone, so each chunk's from/to fall on
// the zone's midnights like the range's own.
func TestSplitIntoYearChunks_KeepsZone(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		t.Fatal(err)
	}
	dr := daterange.DateRange{Start: d(2023, 6, 1), End: d(2024, 12, 31), Loc: loc}
	chunks := splitIntoYearChunks(dr)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if c.Location() != loc {
			t.Errorf("chunk %d is in %v, want %v", i, c.Location(), loc)
		}
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

// fakeFetcher serves one canned ContributionResult per chunk, in call order,
// and records the report range each chunk was fetched with.
type fakeFetcher struct {
	chunks      []api.ContributionResult
	calls       int
	reports     []daterange.DateRange
	comments    []models.Event
	commentsErr error
}

func (f *fakeFetcher) FetchContributions(_ string, _, report daterange.DateRange) (api.ContributionResult, error) {
	f.reports = append(f.reports, report)
	r := f.chunks[f.calls]
	f.calls++
	return r, nil
}

func (f *fakeFetcher) FetchIssueComments(string, daterange.DateRange) ([]models.Event, error) {
	return f.comments, f.commentsErr
}

func TestFetchEvents_AggregatesChunks(t *testing.T) {
	dr := daterange.DateRange{Start: d(2023, 1, 1), End: d(2024, 12, 31)}
	shared := models.Event{ID: "dup", Type: "IssueCommentEvent", CreatedAt: d(2023, 6, 1)}
	f := &fakeFetcher{
		chunks: []api.ContributionResult{
			{
				Events:        []models.Event{shared, {ID: "a", CreatedAt: d(2023, 3, 1)}},
				Totals:        models.ContributionTotals{Commits: 10, Reviews: 1},
				CommitsByRepo: []models.RepoCount{{Repo: "u/x", Count: 4}, {Repo: "u/y", Count: 6}},
				CalendarTotal: 11,
			},
			{
				Events:        []models.Event{{ID: "b", CreatedAt: d(2024, 3, 1)}},
				Totals:        models.ContributionTotals{Commits: 5, Reviews: 2},
				CommitsByRepo: []models.RepoCount{{Repo: "u/x", Count: 5}},
				CalendarTotal: 7,
			},
		},
		comments: []models.Event{shared},
	}

	got, err := fetchEvents(&config{}, f, dr, "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.Totals.Commits != 15 || got.Totals.Reviews != 3 {
		t.Errorf("totals not summed across chunks: %+v", got.Totals)
	}
	if got.CalendarTotal != 18 {
		t.Errorf("CalendarTotal = %d, want 18", got.CalendarTotal)
	}
	want := []models.RepoCount{{Repo: "u/x", Count: 9}, {Repo: "u/y", Count: 6}}
	if len(got.CommitsByRepo) != 2 || got.CommitsByRepo[0] != want[0] || got.CommitsByRepo[1] != want[1] {
		t.Errorf("CommitsByRepo = %+v, want %+v", got.CommitsByRepo, want)
	}
	if len(got.Events) != 3 {
		t.Errorf("expected 3 events after dedup, got %d", len(got.Events))
	}
	if got.Events[0].ID != "b" {
		t.Errorf("events must be newest first, got %q first", got.Events[0].ID)
	}
	for i, r := range f.reports {
		if r != dr {
			t.Errorf("chunk %d fetched with report range %v, want the whole range %v", i, r, dr)
		}
	}
}

// Merging per-repository commit counts across year chunks keeps each
// repository's visibility and database ID; summing counts alone dropped them. A
// repository one chunk saw private stays private when a later chunk sees it
// public, rather than taking whichever chunk came last.
func TestFetchEvents_MergedCommitsKeepVisibilityAndID(t *testing.T) {
	dr := daterange.DateRange{Start: d(2023, 1, 1), End: d(2024, 12, 31)}
	f := &fakeFetcher{
		chunks: []api.ContributionResult{
			{CommitsByRepo: []models.RepoCount{
				{Repo: "u/priv", Count: 4, Private: true, ID: 1},
				{Repo: "u/pub", Count: 6, ID: 2},
				{Repo: "u/flipped", Count: 1, Private: true, ID: 3},
			}},
			{CommitsByRepo: []models.RepoCount{
				{Repo: "u/priv", Count: 5, Private: true, ID: 1},
				{Repo: "u/flipped", Count: 2, ID: 3},
			}},
		},
	}

	got, err := fetchEvents(&config{}, f, dr, "u")
	if err != nil {
		t.Fatal(err)
	}
	want := []models.RepoCount{
		{Repo: "u/priv", Count: 9, Private: true, ID: 1},
		{Repo: "u/pub", Count: 6, ID: 2},
		{Repo: "u/flipped", Count: 3, Private: true, ID: 3},
	}
	if !slices.Equal(got.CommitsByRepo, want) {
		t.Errorf("CommitsByRepo = %+v, want %+v", got.CommitsByRepo, want)
	}
}

// --output was silently dropped for the text format.
func TestWriteOutput_TextHonoursOutputFile(t *testing.T) {
	dir := t.TempDir()
	stats := models.Statistics{
		Username:  "octocat",
		DateRange: daterange.DateRange{Start: d(2024, 1, 1), End: d(2024, 1, 31)},
	}
	writeOutput(&config{format: "text", outputFile: filepath.Join(dir, "report.txt")}, stats)

	data, err := fs.ReadFile(os.DirFS(dir), "report.txt")
	if err != nil {
		t.Fatalf("text report was not written to --output: %v", err)
	}
	if !strings.Contains(string(data), "GitHub Activity Report: octocat") {
		t.Errorf("unexpected file content:\n%s", data)
	}
}

// A report ends with exactly one newline wherever it goes. --output wrote JSON
// with none, while stdout's Println gave Markdown, which already ends in one, a
// second.
func TestWriteToFileOrStdout_EndsWithOneNewline(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no trailing newline", `{"a": 1}`, "{\"a\": 1}\n"},
		{"one trailing newline", "# Report\n", "# Report\n"},
		{"empty", "", "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name+" to a file", func(t *testing.T) {
			dir := t.TempDir()
			writeToFileOrStdout([]byte(tt.in), filepath.Join(dir, "report"))
			got, err := fs.ReadFile(os.DirFS(dir), "report")
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("file = %q, want %q", got, tt.want)
			}
		})
		t.Run(tt.name+" to stdout", func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			stdout := os.Stdout
			os.Stdout = w
			writeToFileOrStdout([]byte(tt.in), "")
			os.Stdout = stdout
			w.Close()
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

// A comment failure keeps whatever was fetched; it warns rather than failing.
func TestFetchEvents_KeepsCommentsOnError(t *testing.T) {
	dr := daterange.DateRange{Start: d(2024, 1, 1), End: d(2024, 1, 31)}
	f := &fakeFetcher{
		chunks:      []api.ContributionResult{{}},
		comments:    []models.Event{{ID: "c", CreatedAt: d(2024, 1, 2)}},
		commentsErr: errors.New("network blip"),
	}
	got, err := fetchEvents(&config{}, f, dr, "u")
	if err != nil {
		t.Fatalf("a comment failure must not fail the fetch: %v", err)
	}
	if len(got.Events) != 1 {
		t.Errorf("partial comments must be kept, got %d events", len(got.Events))
	}
}
