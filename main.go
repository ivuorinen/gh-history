package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/ivuorinen/gh-history/internal/analysis"
	"github.com/ivuorinen/gh-history/internal/api"
	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
	"github.com/ivuorinen/gh-history/internal/output"
)

var version = "dev"

// config holds all parsed CLI flags.
type config struct {
	fromDate    string
	toDate      string
	year        int
	lastMonth   bool
	last90      bool
	outputFile  string
	format      string
	verbose     bool
	showVersion bool
	hostname    string
	username    string   // from --username
	positional  []string // non-flag arguments, the first of which is a username
}

// posUsername returns the username given positionally, if any.
func (c *config) posUsername() string {
	if len(c.positional) == 0 {
		return ""
	}
	return c.positional[0]
}

// subject returns the username the report is for, preferring the flag. Callers
// must run validate first, which rejects supplying both forms.
func (c *config) subject() string {
	if c.username != "" {
		return c.username
	}
	return c.posUsername()
}

// validate rejects argument combinations that cannot be honoured
// unambiguously. Reporting on the wrong account is worse than a clear error.
func (c *config) validate() error {
	if len(c.positional) > 1 {
		return fmt.Errorf("only one username may be given, got %d: %s",
			len(c.positional), strings.Join(c.positional, " "))
	}
	if c.username != "" && c.posUsername() != "" && c.username != c.posUsername() {
		return fmt.Errorf("cannot combine --username %q with the positional username %q; give one",
			c.username, c.posUsername())
	}
	return nil
}

func main() {
	handleMain(os.Args[1:])
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func logVerbose(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func parseFlags(args []string) *config {
	cfg := &config{}
	fs := flag.NewFlagSet("gh-history", flag.ExitOnError)
	fs.StringVar(&cfg.fromDate, "from", "", "Start date (YYYY-MM-DD)")
	fs.StringVar(&cfg.fromDate, "f", "", "Start date (YYYY-MM-DD)")
	fs.StringVar(&cfg.toDate, "to", "", "End date (YYYY-MM-DD)")
	fs.StringVar(&cfg.toDate, "t", "", "End date (YYYY-MM-DD)")
	fs.IntVar(&cfg.year, "year", 0, "Full year shorthand")
	fs.IntVar(&cfg.year, "y", 0, "Full year shorthand")
	fs.BoolVar(&cfg.lastMonth, "last-month", false, "Previous calendar month")
	fs.BoolVar(&cfg.last90, "last-90-days", false, "Last 90 days")
	fs.StringVar(&cfg.outputFile, "output", "", "Output file path")
	fs.StringVar(&cfg.outputFile, "o", "", "Output file path")
	fs.StringVar(&cfg.format, "format", "markdown", "Output format (text|json|markdown|html)")
	fs.StringVar(&cfg.username, "username", "", "GitHub username to report on")
	fs.StringVar(&cfg.username, "u", "", "GitHub username to report on")
	fs.StringVar(&cfg.hostname, "hostname", "", "GitHub host (for GitHub Enterprise; defaults to GH_HOST or github.com)")
	fs.BoolVar(&cfg.verbose, "verbose", false, "Verbose output")
	fs.BoolVar(&cfg.verbose, "v", false, "Verbose output")
	fs.BoolVar(&cfg.showVersion, "version", false, "Show version")

	// Go's flag package stops parsing at the first non-flag argument, so a plain
	// fs.Parse(args) would silently ignore every flag written after the
	// username — "gh history octocat --format json" would quietly produce the
	// default Markdown over the default date range. Consume positionals one at
	// a time and keep parsing what follows.
	fs.Parse(args)
	for fs.NArg() > 0 {
		cfg.positional = append(cfg.positional, fs.Arg(0))
		fs.Parse(fs.Args()[1:])
	}

	cfg.format = strings.ToLower(cfg.format)

	return cfg
}

const usageHint = "Usage: gh history <username> [options]\nOr authenticate with: gh auth login"

func resolveUser(cfg *config) string {
	if s := cfg.subject(); s != "" {
		return s
	}
	// Report what actually failed. Collapsing a network error, an expired token
	// and a missing token into one "username required" message sends users to
	// re-authenticate credentials that may be working fine.
	client, err := newAPIClient(cfg)
	if err != nil {
		fatal("no username given and the GitHub client could not be created: %v\n%s", err, usageHint)
	}
	username, err := client.GetAuthenticatedUser()
	if err != nil {
		fatal("no username given and the authenticated user could not be resolved: %v\n%s", err, usageHint)
	}
	if username == "" {
		fatal("username required. %s", usageHint)
	}
	logVerbose(cfg.verbose, "Using authenticated user: %s", username)
	return username
}

// fetchResult holds events and supplemental data from all fetch sources.
type fetchResult struct {
	Events        []models.Event
	CalendarDays  []models.ContributionDay
	Totals        models.ContributionTotals
	CommitsByRepo []models.RepoCount
	CalendarTotal int
}

func fetchEvents(cfg *config, client *api.Client, dr daterange.DateRange, username string) fetchResult {
	var allEvents []models.Event
	var allCalendarDays []models.ContributionDay
	var totals models.ContributionTotals
	var calendarTotal int
	commitsByRepo := map[string]int{}

	logVerbose(cfg.verbose, "Fetching %s to %s...",
		dr.Start.Format(ghutil.DateFormat), dr.End.Format(ghutil.DateFormat))

	for _, chunk := range splitIntoYearChunks(dr) {
		logVerbose(cfg.verbose, "  GraphQL chunk: %s to %s",
			chunk.Start.Format(ghutil.DateFormat), chunk.End.Format(ghutil.DateFormat))
		result, err := client.FetchContributions(username, chunk)
		if err != nil {
			fatal("fetching contributions: %v", err)
		}
		allEvents = append(allEvents, result.Events...)
		allCalendarDays = append(allCalendarDays, result.CalendarDays...)

		// GitHub reports these per query window, so multi-year ranges must
		// accumulate them rather than keep the last chunk's figures.
		totals.Commits += result.Totals.Commits
		totals.Issues += result.Totals.Issues
		totals.PullRequests += result.Totals.PullRequests
		totals.Reviews += result.Totals.Reviews
		totals.Repositories += result.Totals.Repositories
		calendarTotal += result.CalendarTotal
		for _, rc := range result.CommitsByRepo {
			commitsByRepo[rc.Repo] += rc.Count
		}
	}

	// Keep whatever was retrieved and warn regardless of verbosity: a report
	// that is silently missing comments looks complete but is not.
	comments, err := client.FetchIssueComments(username, dr)
	allEvents = append(allEvents, comments...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: issue comments incomplete: %v\n", err)
	}

	// Dedup by ID (year chunk boundaries may overlap)
	seen := make(map[string]bool, len(allEvents))
	deduped := make([]models.Event, 0, len(allEvents))
	for _, e := range allEvents {
		if !seen[e.ID] {
			seen[e.ID] = true
			deduped = append(deduped, e)
		}
	}

	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].CreatedAt.After(deduped[j].CreatedAt)
	})

	return fetchResult{
		Events:        deduped,
		CalendarDays:  allCalendarDays,
		Totals:        totals,
		CommitsByRepo: sortedRepoCounts(commitsByRepo),
		CalendarTotal: calendarTotal,
	}
}

// sortedRepoCounts converts the accumulated per-repository counts into a slice
// ordered by count descending, breaking ties on the repository name so the
// output does not vary between runs on Go's randomized map iteration order.
func sortedRepoCounts(counts map[string]int) []models.RepoCount {
	out := make([]models.RepoCount, 0, len(counts))
	for repo, count := range counts {
		out = append(out, models.RepoCount{Repo: repo, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Repo < out[j].Repo
	})
	return out
}

// writeToFileOrStdout writes data to a file, or to stdout when no file is given.
func writeToFileOrStdout(data []byte, outputFile string) {
	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0o644); err != nil {
			fatal("writing file: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Saved to: %s\n", outputFile)
		return
	}
	fmt.Println(string(data))
}

func writeOutput(cfg *config, stats models.Statistics) {
	switch cfg.format {
	case "text":
		if err := output.FormatText(stats); err != nil {
			fatal("%v", err)
		}
	case "json":
		// FormatJSON already returns indented JSON, so terminal rendering only
		// ever added colour — not worth the dependency it cost.
		data, err := output.FormatJSON(stats)
		if err != nil {
			fatal("%v", err)
		}
		writeToFileOrStdout(data, cfg.outputFile)
	case "markdown":
		writeToFileOrStdout([]byte(output.FormatMarkdown(stats)), cfg.outputFile)
	case "html":
		outPath := cfg.outputFile
		if outPath == "" {
			outPath = stats.Username + "-report.html"
		}
		if !strings.HasSuffix(outPath, ".html") {
			outPath += ".html"
		}
		if err := output.GenerateHTML(stats, outPath); err != nil {
			fatal("%v", err)
		}
		fmt.Fprintf(os.Stderr, "Report saved to: %s\n", outPath)
		if err := openInBrowser(outPath); err != nil {
			logVerbose(cfg.verbose, "could not open a browser: %v", err)
		}
	default:
		fatal("unknown format %q", cfg.format)
	}
}

func handleMain(args []string) {
	cfg := parseFlags(args)

	if cfg.showVersion {
		fmt.Printf("gh-history %s\n", version)
		return
	}

	if err := cfg.validate(); err != nil {
		fatal("%v", err)
	}

	username := resolveUser(cfg)

	dr, err := daterange.ParseDateRange(cfg.fromDate, cfg.toDate, cfg.year, cfg.lastMonth, cfg.last90)
	if err != nil {
		fatal("%v", err)
	}

	client, err := newAPIClient(cfg)
	if err != nil {
		fatal("%v", err)
	}
	client.Verbose = cfg.verbose

	exists, err := client.CheckUserExists(username)
	if err != nil {
		fatal("checking user: %v", err)
	}
	if !exists {
		fatal("user %q not found", username)
	}

	result := fetchEvents(cfg, client, dr, username)

	calc := &analysis.Calculator{
		Username:      username,
		DateRange:     dr,
		CalendarDays:  result.CalendarDays,
		Totals:        result.Totals,
		CommitsByRepo: result.CommitsByRepo,
		CalendarTotal: result.CalendarTotal,
	}
	stats := calc.Calculate(result.Events)

	writeOutput(cfg, stats)
}

// openInBrowser opens path in the user's browser, honouring $BROWSER the way
// the gh CLI does before falling back to the platform opener.
//
// The path is made absolute first: a relative path beginning with "-" would
// otherwise be read as a flag by the opener. Arguments are passed as a list, so
// nothing is interpreted by a shell.
func openInBrowser(path string) error {
	name, args, err := browserCommand(path, os.Getenv("BROWSER"), runtime.GOOS)
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

// browserCommand builds the opener invocation for path. Split out from
// openInBrowser so it can be tested without launching anything.
func browserCommand(path, launcher, goos string) (string, []string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	if launcher != "" {
		fields := strings.Fields(launcher)
		if len(fields) == 0 {
			return "", nil, fmt.Errorf("BROWSER is set but empty")
		}
		return fields[0], append(fields[1:], abs), nil
	}
	switch goos {
	case "darwin":
		return "open", []string{abs}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", abs}, nil
	default:
		return "xdg-open", []string{abs}, nil
	}
}

// newAPIClient creates an API client for the configured host.
//
// Token lookup is delegated to auth.TokenForHost, which already applies the
// documented order — GH_TOKEN, GITHUB_TOKEN, then the gh config — and, for a
// GitHub Enterprise host, prefers GH_ENTERPRISE_TOKEN / GITHUB_ENTERPRISE_TOKEN.
// Checking the env vars separately here would ignore that per-host distinction.
func newAPIClient(cfg *config) (*api.Client, error) {
	host := cfg.hostname
	if host == "" {
		host, _ = auth.DefaultHost()
	}
	token, _ := auth.TokenForHost(host)
	logVerbose(cfg.verbose, "Using host: %s", host)
	return api.NewClient(host, token)
}

// splitIntoYearChunks splits a date range into chunks of at most 1 year each,
// as required by GitHub's contributionsCollection API.
func splitIntoYearChunks(dr daterange.DateRange) []daterange.DateRange {
	var chunks []daterange.DateRange
	chunkStart := dr.Start
	for chunkStart.Before(dr.End) || chunkStart.Equal(dr.End) {
		chunkEnd := chunkStart.AddDate(1, 0, -1)
		if chunkEnd.After(dr.End) {
			chunkEnd = dr.End
		}
		chunk, _ := daterange.New(chunkStart, chunkEnd)
		chunks = append(chunks, chunk)
		chunkStart = chunkEnd.AddDate(0, 0, 1)
	}
	return chunks
}
