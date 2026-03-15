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
	CategoryCommits      Category = "commits"
	CategoryPullRequests Category = "pull_requests"
	CategoryIssues       Category = "issues"
	CategoryReviews      Category = "reviews"
	CategoryComments     Category = "comments"
	CategoryRepos        Category = "repos"
	CategoryReleases     Category = "releases"
	CategoryOther        Category = "other"
)

// Event represents a GitHub event.
type Event struct {
	ID        string
	Type      string
	Actor     string         // GitHub login who performed the event (not necessarily the report subject)
	Repo      string
	Payload   map[string]any
	CreatedAt time.Time
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

// Statistics holds calculated statistics from GitHub events.
type Statistics struct {
	Username         string              // Report subject
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

// ContributionCalendar holds the full contribution calendar from GraphQL.
type ContributionCalendar struct {
	TotalContributions int
	Days               []ContributionDay
}
