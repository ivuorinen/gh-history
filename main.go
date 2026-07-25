package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/cli/go-gh/v2/pkg/browser"
	"github.com/cli/go-gh/v2/pkg/jsonpretty"
	"github.com/cli/go-gh/v2/pkg/markdown"
	"github.com/cli/go-gh/v2/pkg/term"
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
	username    string
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
	fs.BoolVar(&cfg.verbose, "verbose", false, "Verbose output")
	fs.BoolVar(&cfg.verbose, "v", false, "Verbose output")
	fs.BoolVar(&cfg.showVersion, "version", false, "Show version")

	fs.Parse(args)

	cfg.format = strings.ToLower(cfg.format)

	if fs.NArg() > 0 {
		cfg.username = fs.Arg(0)
	}
	return cfg
}

const usageHint = "Usage: gh history <username> [options]\nOr authenticate with: gh auth login"

func resolveUser(cfg *config) string {
	if cfg.username != "" {
		return cfg.username
	}
	// Report what actually failed. Collapsing a network error, an expired token
	// and a missing token into one "username required" message sends users to
	// re-authenticate credentials that may be working fine.
	client, err := newAPIClient()
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

// writeToFileOrStdout writes data to a file or stdout with optional terminal rendering.
func writeToFileOrStdout(data []byte, outputFile string, renderForTerminal func([]byte) string) {
	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0o644); err != nil {
			fatal("writing file: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Saved to: %s\n", outputFile)
		return
	}
	if renderForTerminal != nil {
		t := term.FromEnv()
		if t.IsTerminalOutput() {
			fmt.Print(renderForTerminal(data))
			return
		}
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
		data, err := output.FormatJSON(stats)
		if err != nil {
			fatal("%v", err)
		}
		writeToFileOrStdout(data, cfg.outputFile, func(data []byte) string {
			t := term.FromEnv()
			var buf bytes.Buffer
			_ = jsonpretty.Format(&buf, bytes.NewReader(data), "  ", t.IsColorEnabled())
			return buf.String()
		})
	case "markdown":
		md := output.FormatMarkdown(stats)
		writeToFileOrStdout([]byte(md), cfg.outputFile, func(data []byte) string {
			t := term.FromEnv()
			rendered, err := markdown.Render(string(data), markdown.WithTheme(t.Theme()))
			if err != nil {
				return string(data)
			}
			return rendered
		})
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
		b := browser.New("", os.Stdout, os.Stderr)
		_ = b.Browse(outPath)
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

	username := resolveUser(cfg)

	dr, err := daterange.ParseDateRange(cfg.fromDate, cfg.toDate, cfg.year, cfg.lastMonth, cfg.last90)
	if err != nil {
		fatal("%v", err)
	}

	client, err := newAPIClient()
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

// newAPIClient creates an API client using go-gh's auth.
func newAPIClient() (*api.Client, error) {
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return api.NewClientWithToken(token)
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return api.NewClientWithToken(token)
	}

	host, _ := auth.DefaultHost()
	token, _ := auth.TokenForHost(host)
	if token != "" {
		return api.NewClientWithToken(token)
	}

	return api.NewClient()
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
