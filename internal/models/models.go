package models

import (
	"sort"
	"time"

	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/ghutil"
)

// Category represents an event category.
type Category string

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
	ID        string
	Type      string
	Repo      string
	Action    string // ActionOpened, ActionClosed, or "" when not applicable
	Merged    bool   // Only meaningful for a closed PullRequestEvent
	CreatedAt time.Time

	// Detail surfaced by the JSON format. Empty when the event type does not
	// carry the field.
	Number           int       // Pull request or issue number
	Title            string    // Pull request or issue title
	Description      string    // Repository description, for CreateEvent
	ReviewState      string    // APPROVED, CHANGES_REQUESTED, COMMENTED, …
	SubjectCreatedAt time.Time // When the PR or issue itself was created
}

// Date returns the event date (without time).
func (e Event) Date() time.Time {
	return ghutil.TruncateToDay(e.CreatedAt)
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
// Unlike the event-derived counters these include private repositories, so they
// are generally higher than what the public event list can account for.
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
	EventsByDate     map[string]int // Keys are "2006-01-02" formatted date strings
	EventsByWeekday  map[int]int    // 0=Monday, 6=Sunday
	EventsByHour     map[int]int
	Streaks          *StreakInfo
	Calendar         *ContributionCalendar
	CommitCount      int
	PROpened         int
	PRMerged         int // Mutually exclusive with PRClosed (merged is not also counted as closed)
	PRClosed         int // Closed without merge
	IssuesOpened     int
	IssuesClosed     int
	ReviewsCount     int

	// Detail surfaced by the JSON format only.
	Events        []Event            // The events the statistics were computed from
	Totals        ContributionTotals // GitHub's own totals, private repos included
	CommitsByRepo []RepoCount        // Commit counts per repository, private repos included
}

// TopRepos returns the top n repositories by event count.
func (s Statistics) TopRepos(n int) []RepoCount {
	repos := make([]RepoCount, 0, len(s.EventsByRepo))
	for repo, count := range s.EventsByRepo {
		repos = append(repos, RepoCount{Repo: repo, Count: count})
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Count > repos[j].Count
	})
	if len(repos) > n {
		repos = repos[:n]
	}
	return repos
}

// PRMergeRate returns the pull request merge rate as a percentage.
func (s Statistics) PRMergeRate() float64 {
	return ghutil.SafeDiv(s.PRMerged, s.PROpened) * 100
}

// IssueCloseRate returns the issue close rate as a percentage.
func (s Statistics) IssueCloseRate() float64 {
	return ghutil.SafeDiv(s.IssuesClosed, s.IssuesOpened) * 100
}

// RepoCount pairs a repository name with its event count.
type RepoCount struct {
	Repo  string
	Count int
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
	// ReportedTotal is GitHub's own figure for the query window. That window is
	// week-aligned and so can be wider than the requested range, which is why
	// it is reported separately rather than replacing TotalContributions.
	ReportedTotal int
	Days          []ContributionDay
}
