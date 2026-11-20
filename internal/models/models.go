package models

import (
	"sort"
	"time"

	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/ghutil"
)

// Category represents an event category.
type Category string

// The categories every event falls into. The values are the keys of
// events_by_category in JSON; CategoryOther holds any event type
// analysis.EventCategories does not list.
const (
	CategoryPullRequests Category = "pull_requests"
	CategoryIssues       Category = "issues"
	CategoryReviews      Category = "reviews"
	CategoryComments     Category = "comments"
	CategoryRepos        Category = "repos"
	CategoryOther        Category = "other"
)

// Event action values. Empty means the event type carries no action.
const (
	ActionOpened = "opened"
	ActionClosed = "closed"
)

// Event represents a GitHub contribution, synthesized from the GraphQL
// contributionsCollection. Fields are typed rather than carried in an untyped
// payload map: every producer is in-process, so there is no serialization
// boundary that would require one.
type Event struct {
	ID   string
	Type string
	Repo string
	// RepoPrivate is Repo's visibility as GitHub reported it at fetch time.
	// Internal repositories on GitHub Enterprise count as private.
	RepoPrivate bool
	RepoID      int64  // Repo's GitHub database ID; see RepoCount.ID
	Action      string // ActionOpened, ActionClosed, or "" when not applicable
	Merged      bool   // Only meaningful for a closed PullRequestEvent
	CreatedAt   time.Time

	// Detail surfaced by the JSON format. Empty when the event type does not
	// carry the field.
	Number           int       // Pull request or issue number
	Title            string    // Pull request or issue title
	Description      string    // Repository description, for CreateEvent
	ReviewState      string    // APPROVED, CHANGES_REQUESTED, COMMENTED, …
	SubjectCreatedAt time.Time // When the PR or issue itself was created
	// SubjectState is the PR's or issue's state when it was fetched (OPEN,
	// CLOSED, or MERGED for a PR), not when the event happened. The merge and
	// close rates count opened items by it.
	SubjectState string
}

// Date returns the day the event falls on in loc, the report's zone.
func (e Event) Date(loc *time.Location) time.Time {
	return ghutil.TruncateToDay(e.CreatedAt, loc)
}

// StreakInfo holds information about activity streaks.
type StreakInfo struct {
	LongestStreak      int
	LongestStreakStart *time.Time
	LongestStreakEnd   *time.Time
	CurrentStreak      int
	CurrentStreakStart *time.Time
	ActiveDays         int
	TotalDays          int
}

// ActivityRate returns the percentage of days with activity.
func (s StreakInfo) ActivityRate() float64 {
	return ghutil.SafeDiv(s.ActiveDays, s.TotalDays) * 100
}

// ContributionTotals holds GitHub's own contribution counts for the period.
// The event-derived counters are computed from the contributions the token can
// list, so the two agree unless the token cannot see some repositories or a
// collection was truncated. Commits exist only here: no event carries them.
type ContributionTotals struct {
	Commits      int
	Issues       int
	PullRequests int
	Reviews      int
	Repositories int
}

// Statistics holds calculated statistics from GitHub events.
type Statistics struct {
	Username         string // Report subject
	DateRange        daterange.DateRange
	TotalEvents      int
	EventsByCategory map[Category]int
	EventsByType     map[string]int
	EventsByRepo     map[string]int
	// RepoPrivate and RepoID map every repository the report names to its
	// visibility and database ID. EventsByRepo keys carry neither, so TopRepos
	// reads them from here.
	RepoPrivate     map[string]bool
	RepoID          map[string]int64
	EventsByDate    map[string]int // Keys are "2006-01-02" formatted date strings
	EventsByWeekday map[int]int    // 0=Monday, 6=Sunday
	EventsByHour    map[int]int
	Streaks         *StreakInfo
	Calendar        *ContributionCalendar
	CommitCount     int
	PROpened        int
	PRMerged        int // Mutually exclusive with PRClosed (merged is not also counted as closed)
	PRClosed        int // Closed without merge
	IssuesOpened    int
	IssuesClosed    int
	ReviewsCount    int
	// PROpenedMerged and IssuesOpenedClosed count, among the PRs and issues
	// opened in the range, those merged or closed by fetch time. They are the
	// rates' numerators because they count the opened items themselves, so a
	// rate cannot exceed 100% whatever events Calculate is given. PRMerged and
	// IssuesClosed count close events inside the range instead, which leaves
	// out an item merged or closed after the range ends.
	PROpenedMerged     int
	IssuesOpenedClosed int

	// Detail surfaced by the JSON format only.
	Events        []Event            // The events the statistics were computed from
	Totals        ContributionTotals // GitHub's own totals, private repos included
	CommitsByRepo []RepoCount        // Commit counts per repository, private repos included
	// GeneratedAt is when the report was made, just after its data was fetched.
	// PROpenedMerged and IssuesOpenedClosed, and so the rates, count each item by
	// its state when its chunk was fetched, which is no later than GeneratedAt:
	// an item merged or closed after GeneratedAt is not counted as such.
	GeneratedAt time.Time
}

// TopRepos returns the top n repositories by event count. A non-positive n
// returns nothing rather than panicking on the slice bound.
//
// Ties break on the repository name: the slice is built from map iteration, so
// without a tie-break the order, and which tied repositories make the cut,
// changed between identical runs.
//
// Each entry's Private comes from RepoPrivate and its ID from RepoID; a
// repository missing from them reads as public, with ID 0.
func (s Statistics) TopRepos(n int) []RepoCount {
	if n <= 0 {
		return nil
	}
	repos := make([]RepoCount, 0, len(s.EventsByRepo))
	for repo, count := range s.EventsByRepo {
		repos = append(repos, RepoCount{Repo: repo, Count: count, Private: s.RepoPrivate[repo], ID: s.RepoID[repo]})
	}
	sort.Slice(repos, func(i, j int) bool {
		if repos[i].Count != repos[j].Count {
			return repos[i].Count > repos[j].Count
		}
		return repos[i].Repo < repos[j].Repo
	})
	if len(repos) > n {
		repos = repos[:n]
	}
	return repos
}

// PRMergeRate returns the percentage of PRs opened in the range that were merged
// by fetch time. It divides PROpenedMerged, not PRMerged: see PROpenedMerged.
func (s Statistics) PRMergeRate() float64 {
	return ghutil.SafeDiv(s.PROpenedMerged, s.PROpened) * 100
}

// IssueCloseRate returns the percentage of issues opened in the range that were
// closed by fetch time, for the same reason dividing IssuesOpenedClosed rather
// than IssuesClosed.
func (s Statistics) IssueCloseRate() float64 {
	return ghutil.SafeDiv(s.IssuesOpenedClosed, s.IssuesOpened) * 100
}

// RepoCount pairs a repository name with its event count.
type RepoCount struct {
	Repo  string
	Count int
	// Private is the repository's visibility, so every table of repositories
	// can mark the private ones.
	Private bool
	// ID is the repository's GitHub database ID. Unlike the name it stays the
	// same through renames and reveals nothing about a private repository, so
	// a consumer can label one by it across reports.
	ID int64
}

// ContributionDay represents a single day from GitHub's contributionCalendar.
type ContributionDay struct {
	Date              time.Time
	ContributionCount int
}

// Weekday returns the day index using this project's convention (0=Monday),
// matching Statistics.EventsByWeekday. GitHub's own contributionDays.weekday
// uses 0=Sunday, so it is derived from Date here rather than carried through —
// two conflicting weekday conventions in one document is a footgun.
func (d ContributionDay) Weekday() int {
	return (int(d.Date.Weekday()) + 6) % 7
}

// ContributionCalendar holds the full contribution calendar from GraphQL.
type ContributionCalendar struct {
	// TotalContributions is the sum over Days, which are filtered to the
	// requested range.
	TotalContributions int
	// ReportedTotal is GitHub's own figure, summed over the query windows. It
	// equals TotalContributions when the windows match the requested range, and
	// is kept separate so a disagreement shows up instead of being hidden.
	ReportedTotal int
	Days          []ContributionDay
}
