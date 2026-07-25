package api

import (
	"errors"
	"fmt"
	"os"
	"time"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// RequestTimeout bounds every individual API request. Without it go-gh builds
// an http.Client with a zero Timeout, which never gives up on a stalled
// connection.
const RequestTimeout = 30 * time.Second

// gqlDoer abstracts go-gh's GraphQLClient.Do for testability.
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

// NewClient creates a Client using go-gh's default authentication (reads gh CLI config and env vars).
func NewClient() (*Client, error) {
	gqlClient, err := ghAPI.NewGraphQLClient(ghAPI.ClientOptions{Timeout: RequestTimeout})
	if err != nil {
		return nil, fmt.Errorf("create GraphQL client: %w", err)
	}
	return &Client{gqlClient: gqlClient}, nil
}

// NewClientWithToken creates a Client with an explicit auth token.
func NewClientWithToken(token string) (*Client, error) {
	gqlClient, err := ghAPI.NewGraphQLClient(ghAPI.ClientOptions{
		AuthToken: token,
		Timeout:   RequestTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create GraphQL client: %w", err)
	}
	return &Client{gqlClient: gqlClient}, nil
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

// CheckUserExists checks if a GitHub user exists.
func (c *Client) CheckUserExists(username string) (bool, error) {
	var resp struct {
		User *struct{ Login string }
	}
	err := c.gqlClient.Do(`query($login: String!) { user(login: $login) { login } }`,
		map[string]any{"login": username}, &resp)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("GraphQL user check: %w", err)
	}
	return resp.User != nil, nil
}

// isNotFound reports whether err is GitHub's structured NOT_FOUND response.
// Matching the error type rather than its English prose keeps user lookups
// working if GitHub rewords the message.
func isNotFound(err error) bool {
	var gqlErr *ghAPI.GraphQLError
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
	// included. Callers querying multiple windows must merge by repository.
	CommitsByRepo []models.RepoCount
	// CalendarTotal is GitHub's reported total for the (week-aligned) window.
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
				Repository    struct{ NameWithOwner string }
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
		Repository struct{ NameWithOwner string }
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
		Repository struct{ NameWithOwner string }
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
			Repository struct{ NameWithOwner string }
		}
	}
}

type repoContributionNode struct {
	OccurredAt time.Time
	Repository struct {
		NameWithOwner string
		Description   string
	}
}

type pageInfo struct {
	EndCursor   *string
	HasNextPage bool
}

const contributionsQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!) {
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

      commitContributionsByRepository(maxRepositories: 100) {
        repository { nameWithOwner }
        contributions { totalCount }
      }

      pullRequestContributions(first: 100) {
        nodes {
          occurredAt
          pullRequest {
            number title state createdAt closedAt mergedAt
            repository { nameWithOwner }
          }
        }
        pageInfo { endCursor hasNextPage }
      }

      issueContributions(first: 100) {
        nodes {
          occurredAt
          issue {
            number title createdAt closedAt state
            repository { nameWithOwner }
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
              repository { nameWithOwner }
            }
          }
        }
        pageInfo { endCursor hasNextPage }
      }

      repositoryContributions(first: 100) {
        nodes {
          occurredAt
          repository { nameWithOwner description }
        }
        pageInfo { endCursor hasNextPage }
      }

}
  }
}`

// Pagination queries for individual sub-collections.

const paginatePRsQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!, $after: String!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      pullRequestContributions(first: 100, after: $after) {
        nodes {
          occurredAt
          pullRequest {
            number title state createdAt closedAt mergedAt
            repository { nameWithOwner }
          }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
  }
}`

const paginateIssuesQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!, $after: String!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      issueContributions(first: 100, after: $after) {
        nodes {
          occurredAt
          issue {
            number title createdAt closedAt state
            repository { nameWithOwner }
          }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
  }
}`

const paginateReviewsQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!, $after: String!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      pullRequestReviewContributions(first: 100, after: $after) {
        nodes {
          occurredAt
          pullRequestReview {
            state submittedAt
            pullRequest {
              number title
              repository { nameWithOwner }
            }
          }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
  }
}`

// Pagination response types (only the relevant sub-collection).

type paginatePRsResponse struct {
	User struct {
		ContributionsCollection struct {
			PullRequestContributions struct {
				Nodes    []prContributionNode
				PageInfo pageInfo
			}
		}
	}
}

type paginateIssuesResponse struct {
	User struct {
		ContributionsCollection struct {
			IssueContributions struct {
				Nodes    []issueContributionNode
				PageInfo pageInfo
			}
		}
	}
}

type paginateReviewsResponse struct {
	User struct {
		ContributionsCollection struct {
			PullRequestReviewContributions struct {
				Nodes    []reviewContributionNode
				PageInfo pageInfo
			}
		}
	}
}

const paginateReposQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!, $after: String!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      repositoryContributions(first: 100, after: $after) {
        nodes {
          occurredAt
          repository { nameWithOwner description }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
  }
}`

type paginateReposResponse struct {
	User struct {
		ContributionsCollection struct {
			RepositoryContributions struct {
				Nodes    []repoContributionNode
				PageInfo pageInfo
			}
		}
	}
}

// FetchContributions fetches PRs, issues, reviews, and calendar data via GraphQL contributionsCollection.
// The date range must be at most 1 year; callers should split larger ranges into yearly chunks.
func (c *Client) FetchContributions(username string, dr daterange.DateRange) (ContributionResult, error) {
	from := dr.Start.Format(time.RFC3339)
	to := dr.EndDateTime().Format(time.RFC3339)

	vars := map[string]any{
		"login": username,
		"from":  from,
		"to":    to,
	}

	var resp contributionsResponse
	if err := c.gqlClient.Do(contributionsQuery, vars, &resp); err != nil {
		return ContributionResult{}, fmt.Errorf("GraphQL query: %w", err)
	}

	cc := resp.User.ContributionsCollection

	// Collect all nodes, paginating each sub-collection as needed. A pagination
	// failure is fatal: silently returning a partial page would understate every
	// statistic derived from it while looking like a complete result.
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

	// Synthesize events.
	var events []models.Event

	for _, n := range allPRs {
		repo := n.PullRequest.Repository.NameWithOwner
		events = append(events, models.Event{
			ID:               fmt.Sprintf("gql-pr-opened-%d-%s", n.PullRequest.Number, repo),
			Type:             "PullRequestEvent",
			Repo:             repo,
			Action:           models.ActionOpened,
			CreatedAt:        n.OccurredAt,
			Number:           n.PullRequest.Number,
			Title:            n.PullRequest.Title,
			SubjectCreatedAt: n.PullRequest.CreatedAt,
		})

		if (n.PullRequest.State == "CLOSED" || n.PullRequest.State == "MERGED") && n.PullRequest.ClosedAt != nil {
			closedAt := *n.PullRequest.ClosedAt
			if !closedAt.Before(dr.Start) && closedAt.Before(dr.EndDateTime()) {
				events = append(events, models.Event{
					ID:               fmt.Sprintf("gql-pr-closed-%d-%s", n.PullRequest.Number, repo),
					Type:             "PullRequestEvent",
					Repo:             repo,
					Action:           models.ActionClosed,
					Merged:           n.PullRequest.MergedAt != nil,
					CreatedAt:        closedAt,
					Number:           n.PullRequest.Number,
					Title:            n.PullRequest.Title,
					SubjectCreatedAt: n.PullRequest.CreatedAt,
				})
			}
		}
	}

	for _, n := range allIssues {
		repo := n.Issue.Repository.NameWithOwner
		events = append(events, models.Event{
			ID:               fmt.Sprintf("gql-issue-opened-%d-%s", n.Issue.Number, repo),
			Type:             "IssuesEvent",
			Repo:             repo,
			Action:           models.ActionOpened,
			CreatedAt:        n.OccurredAt,
			Number:           n.Issue.Number,
			Title:            n.Issue.Title,
			SubjectCreatedAt: n.Issue.CreatedAt,
		})

		if n.Issue.State == "CLOSED" && n.Issue.ClosedAt != nil {
			closedAt := *n.Issue.ClosedAt
			if !closedAt.Before(dr.Start) && closedAt.Before(dr.EndDateTime()) {
				events = append(events, models.Event{
					ID:               fmt.Sprintf("gql-issue-closed-%d-%s", n.Issue.Number, repo),
					Type:             "IssuesEvent",
					Repo:             repo,
					Action:           models.ActionClosed,
					CreatedAt:        closedAt,
					Number:           n.Issue.Number,
					Title:            n.Issue.Title,
					SubjectCreatedAt: n.Issue.CreatedAt,
				})
			}
		}
	}

	for _, n := range allReviews {
		repo := n.PullRequestReview.PullRequest.Repository.NameWithOwner
		submittedAt := n.PullRequestReview.SubmittedAt.Format(time.RFC3339)
		events = append(events, models.Event{
			ID:          fmt.Sprintf("gql-review-%s-%d-%s", submittedAt, n.PullRequestReview.PullRequest.Number, repo),
			Type:        "PullRequestReviewEvent",
			Repo:        repo,
			CreatedAt:   n.OccurredAt,
			Number:      n.PullRequestReview.PullRequest.Number,
			Title:       n.PullRequestReview.PullRequest.Title,
			ReviewState: n.PullRequestReview.State,
		})
	}

	for _, n := range allRepos {
		repo := n.Repository.NameWithOwner
		events = append(events, models.Event{
			ID:          fmt.Sprintf("gql-repo-created-%s", repo),
			Type:        "CreateEvent",
			Repo:        repo,
			CreatedAt:   n.OccurredAt,
			Description: n.Repository.Description,
		})
	}

	// Parse contribution calendar days.
	var calendarDays []models.ContributionDay
	for _, week := range cc.ContributionCalendar.Weeks {
		for _, day := range week.ContributionDays {
			parsed, err := time.Parse(time.DateOnly, day.Date)
			if err != nil {
				continue
			}
			calendarDays = append(calendarDays, models.ContributionDay{
				Date:              parsed,
				ContributionCount: day.ContributionCount,
			})
		}
	}

	commitsByRepo := make([]models.RepoCount, 0, len(cc.CommitContributionsByRepository))
	for _, r := range cc.CommitContributionsByRepository {
		commitsByRepo = append(commitsByRepo, models.RepoCount{
			Repo:  r.Repository.NameWithOwner,
			Count: r.Contributions.TotalCount,
		})
	}

	c.logf("  GraphQL: %d PRs, %d issues, %d reviews, %d repos, %d calendar days\n",
		len(allPRs), len(allIssues), len(allReviews), len(allRepos), len(calendarDays))

	return ContributionResult{
		Events:       events,
		CalendarDays: calendarDays,
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

// ErrTruncated reports that a collection had more pages than MaxPaginationPages
// allows, so the returned data is incomplete.
var ErrTruncated = errors.New("results truncated: more pages available than the pagination limit allows")

// paginateGQL is a generic paginator for GraphQL contribution sub-collections.
// It fetches up to MaxPaginationPages additional pages using the provided query.
// doPage executes the query and returns the new nodes and pageInfo.
//
// Both a page failure and hitting the page limit are returned as errors: a
// partial result reported as complete silently understates every statistic
// derived from it.
func paginateGQL[T any](
	login, from, to string,
	pi pageInfo,
	doPage func(vars map[string]any) ([]T, pageInfo, error),
) ([]T, error) {
	var all []T
	for i := 0; i < ghutil.MaxPaginationPages && pi.HasNextPage && pi.EndCursor != nil; i++ {
		vars := map[string]any{
			"login": login, "from": from, "to": to, "after": *pi.EndCursor,
		}
		nodes, nextPI, err := doPage(vars)
		if err != nil {
			return all, fmt.Errorf("page %d: %w", i+1, err)
		}
		all = append(all, nodes...)
		pi = nextPI
	}
	if pi.HasNextPage && pi.EndCursor != nil {
		return all, fmt.Errorf("%w (limit %d pages)", ErrTruncated, ghutil.MaxPaginationPages)
	}
	return all, nil
}

func (c *Client) paginatePRs(login, from, to string, pi pageInfo) ([]prContributionNode, error) {
	return paginateGQL(login, from, to, pi,
		func(vars map[string]any) ([]prContributionNode, pageInfo, error) {
			var resp paginatePRsResponse
			err := c.gqlClient.Do(paginatePRsQuery, vars, &resp)
			cc := resp.User.ContributionsCollection.PullRequestContributions
			return cc.Nodes, cc.PageInfo, err
		})
}

func (c *Client) paginateIssues(login, from, to string, pi pageInfo) ([]issueContributionNode, error) {
	return paginateGQL(login, from, to, pi,
		func(vars map[string]any) ([]issueContributionNode, pageInfo, error) {
			var resp paginateIssuesResponse
			err := c.gqlClient.Do(paginateIssuesQuery, vars, &resp)
			cc := resp.User.ContributionsCollection.IssueContributions
			return cc.Nodes, cc.PageInfo, err
		})
}

func (c *Client) paginateReviews(login, from, to string, pi pageInfo) ([]reviewContributionNode, error) {
	return paginateGQL(login, from, to, pi,
		func(vars map[string]any) ([]reviewContributionNode, pageInfo, error) {
			var resp paginateReviewsResponse
			err := c.gqlClient.Do(paginateReviewsQuery, vars, &resp)
			cc := resp.User.ContributionsCollection.PullRequestReviewContributions
			return cc.Nodes, cc.PageInfo, err
		})
}

func (c *Client) paginateRepos(login, from, to string, pi pageInfo) ([]repoContributionNode, error) {
	return paginateGQL(login, from, to, pi,
		func(vars map[string]any) ([]repoContributionNode, pageInfo, error) {
			var resp paginateReposResponse
			err := c.gqlClient.Do(paginateReposQuery, vars, &resp)
			cc := resp.User.ContributionsCollection.RepositoryContributions
			return cc.Nodes, cc.PageInfo, err
		})
}

// issueCommentsQuery walks the user's comments newest-updated first. GitHub's
// IssueCommentOrderField only offers UPDATED_AT, so updatedAt is selected
// alongside createdAt purely to make termination decidable: createdAt <=
// updatedAt always holds, so once a node's updatedAt predates the range start,
// no later node in this descending walk can fall inside the range.
const issueCommentsQuery = `
query($login: String!, $after: String) {
  user(login: $login) {
    issueComments(first: 100, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes {
        createdAt
        updatedAt
        repository { nameWithOwner }
      }
      pageInfo { endCursor hasNextPage }
    }
  }
}`

type issueCommentsResponse struct {
	User struct {
		IssueComments struct {
			Nodes []struct {
				CreatedAt  time.Time
				UpdatedAt  time.Time
				Repository struct{ NameWithOwner string }
			}
			PageInfo pageInfo
		}
	}
}

// FetchIssueComments fetches issue comments via GraphQL, filtered to the given
// date range. It returns any events gathered so far alongside an error, so a
// caller can keep partial data; an ErrTruncated error means the page limit was
// reached before the walk provably passed the start of the range.
func (c *Client) FetchIssueComments(username string, dr daterange.DateRange) ([]models.Event, error) {
	var events []models.Event
	startDT := dr.Start
	endDT := dr.EndDateTime()

	var cursor *string
	for range ghutil.MaxPaginationPages {
		vars := map[string]any{
			"login": username,
		}
		if cursor != nil {
			vars["after"] = *cursor
		}

		var resp issueCommentsResponse
		if err := c.gqlClient.Do(issueCommentsQuery, vars, &resp); err != nil {
			return events, fmt.Errorf("GraphQL issue comments: %w", err)
		}

		nodes := resp.User.IssueComments.Nodes
		exhausted := false
		for _, n := range nodes {
			// Descending by updatedAt: everything from here on is older still.
			if n.UpdatedAt.Before(startDT) {
				exhausted = true
				break
			}
			if n.CreatedAt.Before(startDT) || !n.CreatedAt.Before(endDT) {
				continue
			}
			repo := n.Repository.NameWithOwner
			events = append(events, models.Event{
				ID:        fmt.Sprintf("gql-comment-%s-%s", n.CreatedAt.Format(time.RFC3339), repo),
				Type:      "IssueCommentEvent",
				Repo:      repo,
				CreatedAt: n.CreatedAt,
			})
		}
		if exhausted {
			c.logf("  GraphQL comments: %d\n", len(events))
			return events, nil
		}

		pi := resp.User.IssueComments.PageInfo
		if !pi.HasNextPage || pi.EndCursor == nil {
			c.logf("  GraphQL comments: %d\n", len(events))
			return events, nil
		}
		cursor = pi.EndCursor
	}

	// Ran out of page budget without reaching comments older than the range.
	c.logf("  GraphQL comments: %d (truncated)\n", len(events))
	return events, fmt.Errorf("issue comments: %w (limit %d pages)", ErrTruncated, ghutil.MaxPaginationPages)
}
