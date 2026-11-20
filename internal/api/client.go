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
// API, which reports the reason better than a local guess would.
func NewClient(host, token string) (*Client, error) {
	if host == "" {
		host = hostGitHub
	}
	return &Client{gqlClient: &graphQLClient{
		endpoint: GraphQLEndpoint(host),
		token:    token,
		timeZone: LocalTimeZone(),
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
	// included. Callers querying multiple windows must merge by repository.
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
            repository { ...repoRef }
          }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
  }
}` + repoRefFragment

const paginateIssuesQuery = `
query($login: String!, $from: DateTime!, $to: DateTime!, $after: String!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      issueContributions(first: 100, after: $after) {
        nodes {
          occurredAt
          issue {
            number title createdAt closedAt state
            repository { ...repoRef }
          }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
  }
}` + repoRefFragment

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
              repository { ...repoRef }
            }
          }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
  }
}` + repoRefFragment

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
          repository { ...repoRef description }
        }
        pageInfo { endCursor hasNextPage }
      }
    }
  }
}` + repoRefFragment

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
//
// report is the whole range the caller is reporting on, of which dr is one
// chunk. A PR or issue is a contribution only in the chunk where it was opened,
// so its close is matched against report rather than dr: otherwise one opened
// late in a chunk and closed in the next is never counted as closed or merged.
func (c *Client) FetchContributions(username string, dr, report daterange.DateRange) (ContributionResult, error) {
	from := dr.Start.Format(time.RFC3339)
	// GitHub treats `to` as inclusive at day granularity: sending the next
	// midnight counts the whole following day in every total and node list
	// (measured: 41 commits for a one-day range whose day held 25). The last
	// second of End is the boundary that matches the range.
	to := dr.EndDateTime().Add(-time.Second).Format(time.RFC3339)

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

	// Synthesize events.
	var events []models.Event

	for _, n := range allPRs {
		repo := n.PullRequest.Repository.NameWithOwner
		private := n.PullRequest.Repository.IsPrivate
		repoID := n.PullRequest.Repository.DatabaseID
		events = append(events, models.Event{
			ID:               fmt.Sprintf("gql-pr-opened-%d-%s", n.PullRequest.Number, repo),
			Type:             "PullRequestEvent",
			Repo:             repo,
			RepoPrivate:      private,
			RepoID:           repoID,
			Action:           models.ActionOpened,
			CreatedAt:        n.OccurredAt,
			Number:           n.PullRequest.Number,
			Title:            n.PullRequest.Title,
			SubjectCreatedAt: n.PullRequest.CreatedAt,
			SubjectState:     n.PullRequest.State,
		})

		if (n.PullRequest.State == "CLOSED" || n.PullRequest.State == "MERGED") && n.PullRequest.ClosedAt != nil {
			closedAt := *n.PullRequest.ClosedAt
			if !closedAt.Before(report.Start) && closedAt.Before(report.EndDateTime()) {
				events = append(events, models.Event{
					ID:               fmt.Sprintf("gql-pr-closed-%d-%s", n.PullRequest.Number, repo),
					Type:             "PullRequestEvent",
					Repo:             repo,
					RepoPrivate:      private,
					RepoID:           repoID,
					Action:           models.ActionClosed,
					Merged:           n.PullRequest.MergedAt != nil,
					CreatedAt:        closedAt,
					Number:           n.PullRequest.Number,
					Title:            n.PullRequest.Title,
					SubjectCreatedAt: n.PullRequest.CreatedAt,
					SubjectState:     n.PullRequest.State,
				})
			}
		}
	}

	for _, n := range allIssues {
		repo := n.Issue.Repository.NameWithOwner
		private := n.Issue.Repository.IsPrivate
		repoID := n.Issue.Repository.DatabaseID
		events = append(events, models.Event{
			ID:               fmt.Sprintf("gql-issue-opened-%d-%s", n.Issue.Number, repo),
			Type:             "IssuesEvent",
			Repo:             repo,
			RepoPrivate:      private,
			RepoID:           repoID,
			Action:           models.ActionOpened,
			CreatedAt:        n.OccurredAt,
			Number:           n.Issue.Number,
			Title:            n.Issue.Title,
			SubjectCreatedAt: n.Issue.CreatedAt,
			SubjectState:     n.Issue.State,
		})

		if n.Issue.State == "CLOSED" && n.Issue.ClosedAt != nil {
			closedAt := *n.Issue.ClosedAt
			if !closedAt.Before(report.Start) && closedAt.Before(report.EndDateTime()) {
				events = append(events, models.Event{
					ID:               fmt.Sprintf("gql-issue-closed-%d-%s", n.Issue.Number, repo),
					Type:             "IssuesEvent",
					Repo:             repo,
					RepoPrivate:      private,
					RepoID:           repoID,
					Action:           models.ActionClosed,
					CreatedAt:        closedAt,
					Number:           n.Issue.Number,
					Title:            n.Issue.Title,
					SubjectCreatedAt: n.Issue.CreatedAt,
					SubjectState:     n.Issue.State,
				})
			}
		}
	}

	for _, n := range allReviews {
		repo := n.PullRequestReview.PullRequest.Repository.NameWithOwner
		private := n.PullRequestReview.PullRequest.Repository.IsPrivate
		repoID := n.PullRequestReview.PullRequest.Repository.DatabaseID
		submittedAt := n.PullRequestReview.SubmittedAt.Format(time.RFC3339)
		events = append(events, models.Event{
			ID:          fmt.Sprintf("gql-review-%s-%d-%s", submittedAt, n.PullRequestReview.PullRequest.Number, repo),
			Type:        "PullRequestReviewEvent",
			Repo:        repo,
			RepoPrivate: private,
			RepoID:      repoID,
			CreatedAt:   n.OccurredAt,
			Number:      n.PullRequestReview.PullRequest.Number,
			Title:       n.PullRequestReview.PullRequest.Title,
			ReviewState: n.PullRequestReview.State,
		})
	}

	for _, n := range allRepos {
		repo := n.Repository.NameWithOwner
		private := n.Repository.IsPrivate
		repoID := n.Repository.DatabaseID
		events = append(events, models.Event{
			ID:          fmt.Sprintf("gql-repo-created-%s", repo),
			Type:        "CreateEvent",
			Repo:        repo,
			RepoPrivate: private,
			RepoID:      repoID,
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
			Repo:    r.Repository.NameWithOwner,
			Count:   r.Contributions.TotalCount,
			Private: r.Repository.IsPrivate,
			ID:      r.Repository.DatabaseID,
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

// paginateGQL is a generic paginator for GraphQL contribution sub-collections.
// It fetches every page after pi, until GitHub reports no next page, the way
// `gh api --paginate` does. doPage executes the query and returns the new nodes
// and pageInfo.
//
// There is deliberately no page limit. GitHub ends the walk itself, and
// contributionsCollection spans at most a year. A 10-page limit cut a year's
// reviews at 1,100 while GitHub returned all 2,594 across 26 pages. What the
// limit also did was stop a walk whose pages never end, so a cursor GitHub
// returns a second time is now an error instead of a request.
//
// A page failure is returned as an error: a partial result reported as
// complete silently understates every statistic derived from it.
func paginateGQL[T any](
	login, from, to string,
	pi pageInfo,
	doPage func(vars map[string]any) ([]T, pageInfo, error),
) ([]T, error) {
	var all []T
	requested := map[string]bool{}
	for page := 1; pi.HasNextPage && pi.EndCursor != nil; page++ {
		if requested[*pi.EndCursor] {
			return all, fmt.Errorf("page %d: cursor %q repeated", page, *pi.EndCursor)
		}
		requested[*pi.EndCursor] = true
		vars := map[string]any{
			"login": login, "from": from, "to": to, "after": *pi.EndCursor,
		}
		nodes, nextPI, err := doPage(vars)
		if err != nil {
			return all, fmt.Errorf("page %d: %w", page, err)
		}
		all = append(all, nodes...)
		pi = nextPI
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
        id
        createdAt
        updatedAt
        repository { ...repoRef }
      }
      pageInfo { endCursor hasNextPage }
    }
  }
}` + repoRefFragment

// issueCommentNode is one comment. ID is GitHub's node id, the only field that
// is unique per comment: creation time and repository collide for two comments
// posted in the same second.
type issueCommentNode struct {
	ID         string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Repository repoRef
}

type issueCommentsResponse struct {
	User struct {
		IssueComments struct {
			Nodes    []issueCommentNode
			PageInfo pageInfo
		}
	}
}

// FetchIssueComments fetches issue comments via GraphQL, filtered to the given
// date range. It returns any events gathered so far alongside an error, so a
// caller can keep partial data.
//
// The walk runs until it provably passes the start of the range or the
// comments run out, however many pages that takes: a range is reached only
// after every comment updated since it. A 10-page limit stopped the walk at
// 1,000 comments, so older ranges came back short or empty. A cursor GitHub
// returns a second time ends the walk with an error, as in paginateGQL.
func (c *Client) FetchIssueComments(username string, dr daterange.DateRange) ([]models.Event, error) {
	var events []models.Event
	startDT := dr.Start
	endDT := dr.EndDateTime()

	var cursor *string
	requested := map[string]bool{}
	for {
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
			private := n.Repository.IsPrivate
			repoID := n.Repository.DatabaseID
			events = append(events, models.Event{
				ID:          "gql-comment-" + n.ID,
				Type:        "IssueCommentEvent",
				Repo:        repo,
				RepoPrivate: private,
				RepoID:      repoID,
				CreatedAt:   n.CreatedAt,
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
		if requested[*pi.EndCursor] {
			return events, fmt.Errorf("issue comments: cursor %q repeated", *pi.EndCursor)
		}
		requested[*pi.EndCursor] = true
		cursor = pi.EndCursor
	}
}
