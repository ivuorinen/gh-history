package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	got := sortedRepoCounts(map[string]int{
		"u/repo1": 30 + 20,
		"u/repo2": 40,
		"u/repo3": 40,
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
	if got := sortedRepoCounts(map[string]int{}); len(got) != 0 {
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
			name: "hostname defaults to empty so go-gh resolves it",
			args: []string{},
			check: func(t *testing.T, c *config) {
				if c.hostname != "" {
					t.Errorf("hostname = %q, want empty", c.hostname)
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
