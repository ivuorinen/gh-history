package api

import (
	"fmt"
)

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
