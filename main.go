package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

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
		// Incomplete but usable: warn regardless of verbosity rather than
		// reporting understated figures as if they were complete.
		for _, what := range result.Truncated {
			fmt.Fprintf(os.Stderr,
				"Warning: %s for %s to %s hit the pagination limit; counts are understated\n",
				what, chunk.Start.Format(ghutil.DateFormat), chunk.End.Format(ghutil.DateFormat))
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
func newAPIClient(cfg *config) (*api.Client, error) {
	host := resolveHost(cfg.hostname)
	token := resolveToken(host)
	logVerbose(cfg.verbose, "Using host: %s", host)
	if token == "" {
		logVerbose(cfg.verbose, "No token found; requests will be unauthenticated")
	}
	return api.NewClient(host, token)
}

// resolveHost picks the GitHub host: the --hostname flag, then GH_HOST, then the
// sole host in the gh CLI config, then github.com.
func resolveHost(flagHost string) string {
	if flagHost != "" {
		return flagHost
	}
	if h := os.Getenv("GH_HOST"); h != "" {
		return h
	}
	if hosts := configuredHosts(); len(hosts) == 1 {
		// Matching gh's own rule: a single configured host is unambiguous, more
		// than one is not, so anything else falls through to the default.
		return hosts[0]
	}
	return "github.com"
}

// tokenEnvVars lists the environment variables holding a token for host, in
// precedence order. Enterprise Server uses a separate pair so that a github.com
// token is never sent to an internal instance, or vice versa.
func tokenEnvVars(host string) []string {
	if api.IsEnterprise(host) {
		return []string{"GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}
	}
	return []string{"GH_TOKEN", "GITHUB_TOKEN"}
}

// resolveToken finds a token for host: the environment first, then the gh CLI,
// which owns the config file and any system keyring entry.
func resolveToken(host string) string {
	for _, name := range tokenEnvVars(host) {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ghAuthToken(host)
}

// ghAuthTimeout bounds the gh subprocess. It is a local credential lookup, so
// it should be near-instant; without a deadline a wedged gh (an unresponsive
// keyring or credential helper, say) would hang gh-history indefinitely.
const ghAuthTimeout = 10 * time.Second

// ghAuthToken asks the gh CLI for a token. gh-history runs as a gh extension, so
// gh is normally on PATH; when it is not, the caller proceeds unauthenticated.
func ghAuthToken(host string) string {
	ctx, cancel := context.WithTimeout(context.Background(), ghAuthTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", host).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// configuredHosts returns the hosts present in the gh CLI's hosts.yml.
//
// The file is generated by gh and its top-level keys are the hostnames, so they
// are read directly rather than adding a YAML dependency for one lookup.
func configuredHosts() []string {
	path := filepath.Join(ghConfigDir(), "hosts.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var hosts []string
	for _, line := range strings.Split(string(data), "\n") {
		// A top-level key: no indentation, not a comment, ending in a colon.
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		if name, ok := strings.CutSuffix(strings.TrimRight(line, " \r"), ":"); ok && name != "" {
			hosts = append(hosts, name)
		}
	}
	return hosts
}

// ghConfigDir mirrors gh's own config location rules.
func ghConfigDir() string {
	if dir := os.Getenv("GH_CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "gh")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gh")
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
