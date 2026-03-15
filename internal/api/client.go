package api

import (
	"fmt"
	"strings"
	"time"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/ghutil"
	"github.com/ivuorinen/gh-history/internal/models"
)

// gqlDoer abstracts go-gh's GraphQLClient.Do for testability.
type gqlDoer interface {
	Do(query string, variables map[string]any, response any) error
}

// Client wraps GraphQL calls to the GitHub API.
type Client struct {
	gqlClient gqlDoer
	Verbose   bool
}

// NewClient creates a Client using go-gh's default authentication (reads gh CLI config and env vars).
func NewClient() (*Client, error) {
	gqlClient, err := ghAPI.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("create GraphQL client: %w", err)
	}
	return &Client{gqlClient: gqlClient}, nil
}

// NewClientWithToken creates a Client with an explicit auth token.
func NewClientWithToken(token string) (*Client, error) {
	gqlClient, err := ghAPI.NewGraphQLClient(ghAPI.ClientOptions{AuthToken: token})
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
		if strings.Contains(err.Error(), "Could not resolve") {
			return false, nil
		}
		return false, fmt.Errorf("GraphQL user check: %w", err)
	}
	return resp.User != nil, nil
}

// GraphQL response types for contributionsCollection.

// ContributionResult holds events plus calendar data from a GraphQL contributionsCollection query.
type ContributionResult struct {
	Events                   []models.Event
	CalendarDays             []models.ContributionDay
	TotalCommitContributions int
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
						Weekday           int
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
            weekday
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

	// Collect all nodes, paginating each sub-collection as needed.
	allPRs := cc.PullRequestContributions.Nodes
	allPRs = append(allPRs, c.paginatePRs(username, from, to, cc.PullRequestContributions.PageInfo)...)

	allIssues := cc.IssueContributions.Nodes
	allIssues = append(allIssues, c.paginateIssues(username, from, to, cc.IssueContributions.PageInfo)...)

	allReviews := cc.PullRequestReviewContributions.Nodes
	allReviews = append(allReviews, c.paginateReviews(username, from, to, cc.PullRequestReviewContributions.PageInfo)...)

	allRepos := cc.RepositoryContributions.Nodes
	allRepos = append(allRepos, c.paginateRepos(username, from, to, cc.RepositoryContributions.PageInfo)...)

	// Synthesize events.
	var events []models.Event

	for _, n := range allPRs {
		repo := n.PullRequest.Repository.NameWithOwner
		events = append(events, models.Event{
			ID:    fmt.Sprintf("gql-pr-opened-%d-%s", n.PullRequest.Number, repo),
			Type:  "PullRequestEvent",
			Actor: username,
			Repo:  repo,
			Payload: map[string]any{
				"action":       "opened",
				"pull_request": map[string]any{"number": n.PullRequest.Number, "title": n.PullRequest.Title},
			},
			CreatedAt: n.OccurredAt,
		})

		if (n.PullRequest.State == "CLOSED" || n.PullRequest.State == "MERGED") && n.PullRequest.ClosedAt != nil {
			closedAt := *n.PullRequest.ClosedAt
			if !closedAt.Before(dr.StartDateTime()) && closedAt.Before(dr.EndDateTime()) {
				merged := n.PullRequest.MergedAt != nil
				events = append(events, models.Event{
					ID:    fmt.Sprintf("gql-pr-closed-%d-%s", n.PullRequest.Number, repo),
					Type:  "PullRequestEvent",
					Actor: username,
					Repo:  repo,
					Payload: map[string]any{
						"action":       "closed",
						"pull_request": map[string]any{"number": n.PullRequest.Number, "title": n.PullRequest.Title, "merged": merged},
					},
					CreatedAt: closedAt,
				})
			}
		}
	}

	for _, n := range allIssues {
		repo := n.Issue.Repository.NameWithOwner
		events = append(events, models.Event{
			ID:    fmt.Sprintf("gql-issue-opened-%d-%s", n.Issue.Number, repo),
			Type:  "IssuesEvent",
			Actor: username,
			Repo:  repo,
			Payload: map[string]any{
				"action": "opened",
			},
			CreatedAt: n.OccurredAt,
		})

		if n.Issue.State == "CLOSED" && n.Issue.ClosedAt != nil {
			closedAt := *n.Issue.ClosedAt
			if !closedAt.Before(dr.StartDateTime()) && closedAt.Before(dr.EndDateTime()) {
				events = append(events, models.Event{
					ID:    fmt.Sprintf("gql-issue-closed-%d-%s", n.Issue.Number, repo),
					Type:  "IssuesEvent",
					Actor: username,
					Repo:  repo,
					Payload: map[string]any{
						"action": "closed",
					},
					CreatedAt: closedAt,
				})
			}
		}
	}

	for _, n := range allReviews {
		repo := n.PullRequestReview.PullRequest.Repository.NameWithOwner
		submittedAt := n.PullRequestReview.SubmittedAt.Format(time.RFC3339)
		events = append(events, models.Event{
			ID:    fmt.Sprintf("gql-review-%s-%d-%s", submittedAt, n.PullRequestReview.PullRequest.Number, repo),
			Type:  "PullRequestReviewEvent",
			Actor: username,
			Repo:  repo,
			Payload: map[string]any{
				"review": map[string]any{"state": n.PullRequestReview.State},
			},
			CreatedAt: n.OccurredAt,
		})
	}

	for _, n := range allRepos {
		repo := n.Repository.NameWithOwner
		events = append(events, models.Event{
			ID:    fmt.Sprintf("gql-repo-created-%s", repo),
			Type:  "CreateEvent",
			Actor: username,
			Repo:  repo,
			Payload: map[string]any{
				"ref_type":    "repository",
				"description": n.Repository.Description,
			},
			CreatedAt: n.OccurredAt,
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

	if c.Verbose {
		fmt.Printf("  GraphQL: %d PRs, %d issues, %d reviews, %d repos, %d calendar days\n",
			len(allPRs), len(allIssues), len(allReviews), len(allRepos), len(calendarDays))
	}

	return ContributionResult{
		Events:                   events,
		CalendarDays:             calendarDays,
		TotalCommitContributions: cc.TotalCommitContributions,
	}, nil
}

// paginateGQL is a generic paginator for GraphQL contribution sub-collections.
// It fetches up to MaxPaginationPages additional pages using the provided query.
// doPage executes the query and returns the new nodes and pageInfo.
func paginateGQL[T any](
	c *Client,
	login, from, to string,
	pi pageInfo,
	doPage func(vars map[string]any) ([]T, pageInfo, error),
) []T {
	var all []T
	for i := 0; i < ghutil.MaxPaginationPages && pi.HasNextPage && pi.EndCursor != nil; i++ {
		vars := map[string]any{
			"login": login, "from": from, "to": to, "after": *pi.EndCursor,
		}
		nodes, nextPI, err := doPage(vars)
		if err != nil {
			break
		}
		all = append(all, nodes...)
		pi = nextPI
	}
	return all
}

func (c *Client) paginatePRs(login, from, to string, pi pageInfo) []prContributionNode {
	return paginateGQL(c, login, from, to, pi,
		func(vars map[string]any) ([]prContributionNode, pageInfo, error) {
			var resp paginatePRsResponse
			err := c.gqlClient.Do(paginatePRsQuery, vars, &resp)
			cc := resp.User.ContributionsCollection.PullRequestContributions
			return cc.Nodes, cc.PageInfo, err
		})
}

func (c *Client) paginateIssues(login, from, to string, pi pageInfo) []issueContributionNode {
	return paginateGQL(c, login, from, to, pi,
		func(vars map[string]any) ([]issueContributionNode, pageInfo, error) {
			var resp paginateIssuesResponse
			err := c.gqlClient.Do(paginateIssuesQuery, vars, &resp)
			cc := resp.User.ContributionsCollection.IssueContributions
			return cc.Nodes, cc.PageInfo, err
		})
}

func (c *Client) paginateReviews(login, from, to string, pi pageInfo) []reviewContributionNode {
	return paginateGQL(c, login, from, to, pi,
		func(vars map[string]any) ([]reviewContributionNode, pageInfo, error) {
			var resp paginateReviewsResponse
			err := c.gqlClient.Do(paginateReviewsQuery, vars, &resp)
			cc := resp.User.ContributionsCollection.PullRequestReviewContributions
			return cc.Nodes, cc.PageInfo, err
		})
}

func (c *Client) paginateRepos(login, from, to string, pi pageInfo) []repoContributionNode {
	return paginateGQL(c, login, from, to, pi,
		func(vars map[string]any) ([]repoContributionNode, pageInfo, error) {
			var resp paginateReposResponse
			err := c.gqlClient.Do(paginateReposQuery, vars, &resp)
			cc := resp.User.ContributionsCollection.RepositoryContributions
			return cc.Nodes, cc.PageInfo, err
		})
}

const issueCommentsQuery = `
query($login: String!, $after: String) {
  user(login: $login) {
    issueComments(first: 100, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes {
        createdAt
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
				Repository struct{ NameWithOwner string }
			}
			PageInfo pageInfo
		}
	}
}

// FetchIssueComments fetches issue comments via GraphQL, filtered to the given date range.
func (c *Client) FetchIssueComments(username string, dr daterange.DateRange) ([]models.Event, error) {
	var events []models.Event
	startDT := dr.StartDateTime()
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
		for _, n := range nodes {
			if n.CreatedAt.Before(startDT) || !n.CreatedAt.Before(endDT) {
				continue
			}
			repo := n.Repository.NameWithOwner
			events = append(events, models.Event{
				ID:        fmt.Sprintf("gql-comment-%s-%s", n.CreatedAt.Format(time.RFC3339), repo),
				Type:      "IssueCommentEvent",
				Actor:     username,
				Repo:      repo,
				Payload:   map[string]any{},
				CreatedAt: n.CreatedAt,
			})
		}

		pi := resp.User.IssueComments.PageInfo
		if !pi.HasNextPage || pi.EndCursor == nil {
			break
		}
		cursor = pi.EndCursor
	}

	if c.Verbose {
		fmt.Printf("  GraphQL comments: %d\n", len(events))
	}

	return events, nil
}
