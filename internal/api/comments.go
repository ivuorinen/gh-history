package api

import (
	"fmt"
	"time"

	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/models"
)

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
	startDT := dr.StartTime()
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
