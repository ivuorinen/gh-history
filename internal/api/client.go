package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/models"
)

// RequestTimeout bounds every individual API request. http.Client's zero
// Timeout never gives up on a stalled connection, which would hang the CLI.
const RequestTimeout = 30 * time.Second

// gqlDoer abstracts graphQLClient.Do so tests can substitute canned responses.
type gqlDoer interface {
	Do(query string, variables map[string]any, response any) error
}

// Client wraps GraphQL calls to the GitHub API.
type Client struct {
	gqlClient gqlDoer
	Verbose   bool
}

// logf writes a diagnostic line to stderr. Diagnostics must never go to stdout:
// that is where the JSON and Markdown reports are written.
func (c *Client) logf(format string, args ...any) {
	if c.Verbose {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// NewClient creates a Client for the given host and token. An empty host means
// github.com. An empty token is allowed — unauthenticated requests fail at the
// API, which reports the reason better than a local guess would. timeZone is
// the IANA name sent as every request's Time-Zone header: the zone the report's
// DateRange counts days in.
func NewClient(host, token, timeZone string) (*Client, error) {
	if host == "" {
		host = hostGitHub
	}
	return &Client{gqlClient: &graphQLClient{
		endpoint: GraphQLEndpoint(host),
		token:    token,
		timeZone: timeZone,
		http:     &http.Client{Timeout: RequestTimeout},
	}}, nil
}

// GetAuthenticatedUser returns the login of the currently authenticated user.
func (c *Client) GetAuthenticatedUser() (string, error) {
	var resp struct {
		Viewer struct{ Login string }
	}
	if err := c.gqlClient.Do("query { viewer { login } }", nil, &resp); err != nil {
		return "", fmt.Errorf("GraphQL viewer query: %w", err)
	}
	return resp.Viewer.Login, nil
}

// LookupUser reports whether a GitHub user exists and returns the account's own
// spelling of its login. GitHub matches logins case-insensitively, so the
// returned login can differ from username in case; callers report under the
// returned one, or a report for "OCTOCAT" is labelled with the caller's typing.
func (c *Client) LookupUser(username string) (login string, exists bool, err error) {
	var resp struct {
		User *struct{ Login string }
	}
	err = c.gqlClient.Do(`query($login: String!) { user(login: $login) { login } }`,
		map[string]any{"login": username}, &resp)
	if err != nil {
		if isNotFound(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("GraphQL user check: %w", err)
	}
	if resp.User == nil {
		return "", false, nil
	}
	return resp.User.Login, true, nil
}

// isNotFound reports whether err is GitHub's structured NOT_FOUND response.
// Matching the error type rather than its English prose keeps user lookups
// working if GitHub rewords the message.
func isNotFound(err error) bool {
	var gqlErr *GraphQLError
	if !errors.As(err, &gqlErr) {
		return false
	}
	for _, e := range gqlErr.Errors {
		if e.Type == "NOT_FOUND" {
			return true
		}
	}
	return false
}

// GraphQL response types for contributionsCollection.

// ContributionResult holds events plus calendar data from a GraphQL contributionsCollection query.
type ContributionResult struct {
	Events       []models.Event
	CalendarDays []models.ContributionDay
	// Totals are GitHub's own contribution counts for the window, which include
	// private repositories. Callers querying multiple windows must sum them.
	Totals models.ContributionTotals
	// CommitsByRepo is GitHub's per-repository commit breakdown, private repos
	// included. A window past GitHub's repository limit is fetched in shorter
	// windows, and a repository is listed once per window it has commits in, so
	// callers merge by repository, as they do across their own windows.
	CommitsByRepo []models.RepoCount
	// CalendarTotal is GitHub's reported calendar total for the query window.
	CalendarTotal int
}

type contributionsResponse struct {
	User struct {
		ContributionsCollection struct {
			TotalCommitContributions            int
			TotalIssueContributions             int
			TotalPullRequestContributions       int
			TotalPullRequestReviewContributions int
			TotalRepositoryContributions        int

			ContributionCalendar struct {
				TotalContributions int
				Weeks              []struct {
					ContributionDays []struct {
						Date              string
						ContributionCount int
					}
				}
			}

			CommitContributionsByRepository []struct {
				Repository    repoRef
				Contributions struct{ TotalCount int }
			}

			PullRequestContributions struct {
				Nodes    []prContributionNode
				PageInfo pageInfo
			}

			IssueContributions struct {
				Nodes    []issueContributionNode
				PageInfo pageInfo
			}

			PullRequestReviewContributions struct {
				Nodes    []reviewContributionNode
				PageInfo pageInfo
			}

			RepositoryContributions struct {
				Nodes    []repoContributionNode
				PageInfo pageInfo
			}
		}
	}
}

type prContributionNode struct {
	OccurredAt  time.Time
	PullRequest struct {
		Number     int
		Title      string
		State      string
		CreatedAt  time.Time
		ClosedAt   *time.Time
		MergedAt   *time.Time
		Repository repoRef
	}
}

type issueContributionNode struct {
	OccurredAt time.Time
	Issue      struct {
		Number     int
		Title      string
		CreatedAt  time.Time
		ClosedAt   *time.Time
		State      string
		Repository repoRef
	}
}

type reviewContributionNode struct {
	OccurredAt        time.Time
	PullRequestReview struct {
		State       string
		SubmittedAt time.Time
		PullRequest struct {
			Number     int
			Title      string
			Repository repoRef
		}
	}
}

type repoContributionNode struct {
	OccurredAt time.Time
	Repository struct {
		repoRef
		Description string
	}
}

// repoRef is the repository a contribution node points at. It is one named
// type rather than an anonymous struct per node so that a field added here
// reaches every node at once, instead of reaching some node types and being
// missed on others. repoRefFragment requests the same fields in every query.
type repoRef struct {
	NameWithOwner string
	// IsPrivate is GitHub's isPrivate, which is also true for internal
	// repositories on GitHub Enterprise.
	IsPrivate bool
	// DatabaseID is GitHub's fullDatabaseId: the value databaseId holds, as a
	// BigInt sent in a JSON string. databaseId is a 32-bit Int, and repository
	// IDs already pass two-thirds of its range.
	DatabaseID int64 `json:"fullDatabaseId,string"`
}

// repoRefFragment selects the fields repoRef decodes. Every query selects its
// repositories as `repository { ...repoRef }` and ends with this fragment, so a
// field added here reaches every query. Written into each query by hand, a
// field could be missed in some, and GitHub reports no error for a field that
// was never requested. A fragment is GraphQL's own way to share a selection. A
// query that selects no repository must not append it: GitHub rejects a
// fragment the query does not use.
const repoRefFragment = `
fragment repoRef on Repository { nameWithOwner isPrivate fullDatabaseId }`

type pageInfo struct {
	EndCursor   *string
	HasNextPage bool
}

// maxCommitRepositories is the most repositories commitContributionsByRepository
// lists, GitHub's own maximum. The list has no pagination.
const maxCommitRepositories = 100

const contributionsQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!, $maxRepositories: Int!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      totalCommitContributions
      totalIssueContributions
      totalPullRequestContributions
      totalPullRequestReviewContributions
      totalRepositoryContributions

      contributionCalendar {
        totalContributions
        weeks {
          contributionDays {
            date
            contributionCount
          }
        }
      }

      commitContributionsByRepository(maxRepositories: $maxRepositories) {
        repository { ...repoRef }
        contributions { totalCount }
      }

      pullRequestContributions(first: 100) {
        nodes {
          occurredAt
          pullRequest {
            number title state createdAt closedAt mergedAt
            repository { ...repoRef }
          }
        }
        pageInfo { endCursor hasNextPage }
      }

      issueContributions(first: 100) {
        nodes {
          occurredAt
          issue {
            number title createdAt closedAt state
            repository { ...repoRef }
          }
        }
        pageInfo { endCursor hasNextPage }
      }

      pullRequestReviewContributions(first: 100) {
        nodes {
          occurredAt
          pullRequestReview {
            state submittedAt
            pullRequest {
              number title
              repository { ...repoRef }
            }
          }
        }
        pageInfo { endCursor hasNextPage }
      }

      repositoryContributions(first: 100) {
        nodes {
          occurredAt
          repository { ...repoRef description }
        }
        pageInfo { endCursor hasNextPage }
      }

}
  }
}` + repoRefFragment

// commitsByRepoQuery is contributionsQuery's commit breakdown on its own, for
// the shorter windows repoCommitCounts fetches.
const commitsByRepoQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!, $maxRepositories: Int!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      totalCommitContributions
      commitContributionsByRepository(maxRepositories: $maxRepositories) {
        repository { ...repoRef }
        contributions { totalCount }
      }
    }
  }
}` + repoRefFragment

// FetchContributions fetches PRs, issues, reviews, and calendar data via GraphQL contributionsCollection.
// The date range must be at most 1 year; callers should split larger ranges into yearly chunks.
//
// report is the whole range the caller is reporting on, of which dr is one
// chunk. A PR or issue is a contribution only in the chunk where it was opened,
// so its close is matched against report rather than dr: otherwise one opened
// late in a chunk and closed in the next is never counted as closed or merged.
func (c *Client) FetchContributions(username string, dr, report daterange.DateRange) (ContributionResult, error) {
	from, to := queryWindow(dr)
	vars := map[string]any{
		"login":           username,
		"from":            from,
		"to":              to,
		"maxRepositories": maxCommitRepositories,
	}

	var resp contributionsResponse
	if err := c.gqlClient.Do(contributionsQuery, vars, &resp); err != nil {
		return ContributionResult{}, fmt.Errorf("GraphQL query: %w", err)
	}

	cc := resp.User.ContributionsCollection

	// Collect all nodes, paginating each sub-collection to its last page. A
	// pagination failure aborts the fetch: silently returning a partial page
	// would understate every statistic derived from it while looking like a
	// complete result.
	allPRs, err := c.paginatePRs(username, from, to, cc.PullRequestContributions.PageInfo)
	if err != nil {
		return ContributionResult{}, fmt.Errorf("pull request contributions: %w", err)
	}
	allPRs = append(cc.PullRequestContributions.Nodes, allPRs...)

	allIssues, err := c.paginateIssues(username, from, to, cc.IssueContributions.PageInfo)
	if err != nil {
		return ContributionResult{}, fmt.Errorf("issue contributions: %w", err)
	}
	allIssues = append(cc.IssueContributions.Nodes, allIssues...)

	allReviews, err := c.paginateReviews(username, from, to, cc.PullRequestReviewContributions.PageInfo)
	if err != nil {
		return ContributionResult{}, fmt.Errorf("review contributions: %w", err)
	}
	allReviews = append(cc.PullRequestReviewContributions.Nodes, allReviews...)

	allRepos, err := c.paginateRepos(username, from, to, cc.RepositoryContributions.PageInfo)
	if err != nil {
		return ContributionResult{}, fmt.Errorf("repository contributions: %w", err)
	}
	allRepos = append(cc.RepositoryContributions.Nodes, allRepos...)

	events := prEvents(allPRs, report)
	events = append(events, issueEvents(allIssues, report)...)
	events = append(events, reviewEvents(allReviews)...)
	events = append(events, repoEvents(allRepos)...)
	days := calendarDays(resp)

	commitsByRepo, err := c.repoCommitCounts(username, dr, resp)
	if err != nil {
		return ContributionResult{}, fmt.Errorf("commit contributions by repository: %w", err)
	}

	c.logf("  GraphQL: %d PRs, %d issues, %d reviews, %d repos, %d calendar days\n",
		len(allPRs), len(allIssues), len(allReviews), len(allRepos), len(days))

	return ContributionResult{
		Events:       events,
		CalendarDays: days,
		Totals: models.ContributionTotals{
			Commits:      cc.TotalCommitContributions,
			Issues:       cc.TotalIssueContributions,
			PullRequests: cc.TotalPullRequestContributions,
			Reviews:      cc.TotalPullRequestReviewContributions,
			Repositories: cc.TotalRepositoryContributions,
		},
		CommitsByRepo: commitsByRepo,
		CalendarTotal: cc.ContributionCalendar.TotalContributions,
	}, nil
}

// queryWindow returns contributionsCollection's `from` and `to` for dr.
func queryWindow(dr daterange.DateRange) (from, to string) {
	// The range's own midnights, in the zone the Time-Zone header names, so the
	// fetched days are the days GitHub buckets the calendar into.
	from = dr.StartTime().Format(time.RFC3339)
	// GitHub treats `to` as inclusive at day granularity: sending the next
	// midnight counts the whole following day in every total and node list
	// (measured: 41 commits for a one-day range whose day held 25). The last
	// second of End is the boundary that matches the range.
	to = dr.EndDateTime().Add(-time.Second).Format(time.RFC3339)
	return from, to
}

// repoCommitCounts returns dr's per-repository commit counts from resp, GitHub's
// answer for dr. A window that lists maxCommitRepositories repositories and
// falls short of its commit total was cut off by the limit, so its two halves
// are fetched instead, each splitting again while it is cut off the same way,
// down to single days. A window counts only its own days, so the windows'
// counts add up to dr's, and a repository with commits in several windows is
// listed once for each. A shortfall that is left, from a single day past the
// limit or a shorter list, is fetchEvents' to report.
func (c *Client) repoCommitCounts(username string, dr daterange.DateRange, resp contributionsResponse) ([]models.RepoCount, error) {
	cc := resp.User.ContributionsCollection
	counts := make([]models.RepoCount, 0, len(cc.CommitContributionsByRepository))
	listed := 0
	for _, r := range cc.CommitContributionsByRepository {
		counts = append(counts, models.RepoCount{
			Repo:    r.Repository.NameWithOwner,
			Count:   r.Contributions.TotalCount,
			Private: r.Repository.IsPrivate,
			ID:      r.Repository.DatabaseID,
		})
		listed += r.Contributions.TotalCount
	}
	// A shorter list falling short is not the limit: splitting it would query
	// every day of the window without finding the missing commits.
	cut := len(counts) == maxCommitRepositories && listed < cc.TotalCommitContributions
	if !cut || !dr.Start.Before(dr.End) {
		return counts, nil
	}
	c.logf("  GraphQL: %s to %s lists %d repositories with %d of %d commits; fetching each half\n",
		dr.Start.Format(time.DateOnly), dr.End.Format(time.DateOnly),
		len(counts), listed, cc.TotalCommitContributions)

	mid := dr.Start.AddDate(0, 0, int(dr.End.Sub(dr.Start)/(24*time.Hour))/2)
	counts = nil
	for _, half := range []daterange.DateRange{
		{Start: dr.Start, End: mid, Loc: dr.Loc},
		{Start: mid.AddDate(0, 0, 1), End: dr.End, Loc: dr.Loc},
	} {
		from, to := queryWindow(half)
		vars := map[string]any{
			"login":           username,
			"from":            from,
			"to":              to,
			"maxRepositories": maxCommitRepositories,
		}
		var halfResp contributionsResponse
		if err := c.gqlClient.Do(commitsByRepoQuery, vars, &halfResp); err != nil {
			return nil, fmt.Errorf("%s to %s: %w",
				half.Start.Format(time.DateOnly), half.End.Format(time.DateOnly), err)
		}
		halfCounts, err := c.repoCommitCounts(username, half, halfResp)
		if err != nil {
			return nil, err
		}
		counts = append(counts, halfCounts...)
	}
	return counts, nil
}
