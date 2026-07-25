package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/models"
)

// mockGQLClient implements gqlDoer for testing.
type mockGQLClient struct {
	doFunc func(query string, variables map[string]any, response any) error
}

func (m *mockGQLClient) Do(query string, variables map[string]any, response any) error {
	return m.doFunc(query, variables, response)
}

// newTestClient creates a Client with a mock GraphQL client.
func newTestClient(gql gqlDoer) *Client {
	return &Client{gqlClient: gql}
}

func TestGetAuthenticatedUserGraphQL(t *testing.T) {
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			resp := response.(*struct {
				Viewer struct{ Login string }
			})
			resp.Viewer.Login = "testuser"
			return nil
		},
	}

	c := newTestClient(mock)
	login, err := c.GetAuthenticatedUser()
	if err != nil {
		t.Fatal(err)
	}
	if login != "testuser" {
		t.Errorf("expected testuser, got %s", login)
	}
}

func TestCheckUserExistsGraphQL(t *testing.T) {
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			login := variables["login"].(string)
			if login == "testuser" {
				resp := response.(*struct {
					User *struct{ Login string }
				})
				user := struct{ Login string }{Login: "testuser"}
				resp.User = &user
				return nil
			}
			// What GitHub actually returns for an unknown login: a structured
			// GraphQL error of type NOT_FOUND.
			return &ghAPI.GraphQLError{
				Errors: []ghAPI.GraphQLErrorItem{{
					Type:    "NOT_FOUND",
					Message: fmt.Sprintf("Could not resolve to a User with the login of '%s'", login),
				}},
			}
		},
	}

	c := newTestClient(mock)

	exists, err := c.CheckUserExists("testuser")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected user to exist")
	}

	exists, err = c.CheckUserExists("nobody")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected user not to exist")
	}
}

func TestCheckUserExistsGraphQLError(t *testing.T) {
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			return fmt.Errorf("network timeout")
		},
	}

	c := newTestClient(mock)
	_, err := c.CheckUserExists("testuser")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "GraphQL user check") {
		t.Errorf("expected GraphQL user check error, got: %v", err)
	}
}

func TestFetchContributions(t *testing.T) {
	closedAt := time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC)
	mergedAt := time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC)
	issueClosedAt := time.Date(2024, 1, 25, 10, 0, 0, 0, time.UTC)

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			resp := response.(*contributionsResponse)
			resp.User.ContributionsCollection.PullRequestContributions.Nodes = []prContributionNode{
				{
					OccurredAt: time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
					PullRequest: struct {
						Number     int
						Title      string
						State      string
						CreatedAt  time.Time
						ClosedAt   *time.Time
						MergedAt   *time.Time
						Repository struct{ NameWithOwner string }
					}{
						Number: 1, Title: "Open PR", State: "OPEN",
						CreatedAt:  time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
						Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo1"},
					},
				},
				{
					OccurredAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
					PullRequest: struct {
						Number     int
						Title      string
						State      string
						CreatedAt  time.Time
						ClosedAt   *time.Time
						MergedAt   *time.Time
						Repository struct{ NameWithOwner string }
					}{
						Number: 2, Title: "Merged PR", State: "MERGED",
						CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
						ClosedAt:  &closedAt, MergedAt: &mergedAt,
						Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo2"},
					},
				},
			}
			resp.User.ContributionsCollection.IssueContributions.Nodes = []issueContributionNode{
				{
					OccurredAt: time.Date(2024, 1, 12, 10, 0, 0, 0, time.UTC),
					Issue: struct {
						Number     int
						Title      string
						CreatedAt  time.Time
						ClosedAt   *time.Time
						State      string
						Repository struct{ NameWithOwner string }
					}{
						Number: 5, Title: "Open issue", State: "OPEN",
						CreatedAt:  time.Date(2024, 1, 12, 10, 0, 0, 0, time.UTC),
						Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo1"},
					},
				},
				{
					OccurredAt: time.Date(2024, 1, 18, 10, 0, 0, 0, time.UTC),
					Issue: struct {
						Number     int
						Title      string
						CreatedAt  time.Time
						ClosedAt   *time.Time
						State      string
						Repository struct{ NameWithOwner string }
					}{
						Number: 6, Title: "Closed issue", State: "CLOSED",
						CreatedAt:  time.Date(2024, 1, 18, 10, 0, 0, 0, time.UTC),
						ClosedAt:   &issueClosedAt,
						Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo2"},
					},
				},
			}
			resp.User.ContributionsCollection.PullRequestReviewContributions.Nodes = []reviewContributionNode{
				{
					OccurredAt: time.Date(2024, 1, 14, 10, 0, 0, 0, time.UTC),
					PullRequestReview: struct {
						State       string
						SubmittedAt time.Time
						PullRequest struct {
							Number     int
							Title      string
							Repository struct{ NameWithOwner string }
						}
					}{
						State:       "APPROVED",
						SubmittedAt: time.Date(2024, 1, 14, 10, 0, 0, 0, time.UTC),
						PullRequest: struct {
							Number     int
							Title      string
							Repository struct{ NameWithOwner string }
						}{
							Number: 3, Title: "Some PR",
							Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo1"},
						},
					},
				},
			}
			resp.User.ContributionsCollection.RepositoryContributions.Nodes = []repoContributionNode{
				{
					OccurredAt: time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC),
					Repository: struct {
						NameWithOwner string
						Description   string
					}{
						NameWithOwner: "user/new-repo",
						Description:   "A new repository",
					},
				},
			}
			return nil
		},
	}

	c := newTestClient(mock)
	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	result, err := c.FetchContributions("user", dr)
	if err != nil {
		t.Fatal(err)
	}

	events := result.Events

	// 2 PR opened + 1 PR closed + 2 issue opened + 1 issue closed + 1 review + 1 repo created = 8
	if len(events) != 8 {
		t.Fatalf("expected 8 events, got %d", len(events))
	}

	// Check event types
	typeCounts := map[string]int{}
	for _, e := range events {
		typeCounts[e.Type]++
	}
	if typeCounts["PullRequestEvent"] != 3 {
		t.Errorf("expected 3 PullRequestEvents, got %d", typeCounts["PullRequestEvent"])
	}
	if typeCounts["IssuesEvent"] != 3 {
		t.Errorf("expected 3 IssuesEvents, got %d", typeCounts["IssuesEvent"])
	}
	if typeCounts["PullRequestReviewEvent"] != 1 {
		t.Errorf("expected 1 PullRequestReviewEvent, got %d", typeCounts["PullRequestReviewEvent"])
	}
	if typeCounts["CreateEvent"] != 1 {
		t.Errorf("expected 1 CreateEvent, got %d", typeCounts["CreateEvent"])
	}

	// Check a specific ID
	found := false
	for _, e := range events {
		if e.ID == "gql-pr-opened-1-user/repo1" {
			found = true
			if e.Repo != "user/repo1" {
				t.Errorf("expected repo user/repo1, got %s", e.Repo)
			}
		}
	}
	if !found {
		t.Error("expected to find event gql-pr-opened-1-user/repo1")
	}

	// Check merged PR is flagged as merged, and opened PRs carry the open action
	for _, e := range events {
		switch e.ID {
		case "gql-pr-closed-2-user/repo2":
			if e.Action != models.ActionClosed {
				t.Errorf("expected closed action, got %q", e.Action)
			}
			if !e.Merged {
				t.Error("expected Merged=true for merged PR")
			}
		case "gql-pr-opened-1-user/repo1":
			if e.Action != models.ActionOpened {
				t.Errorf("expected opened action, got %q", e.Action)
			}
			if e.Merged {
				t.Error("expected Merged=false for opened PR")
			}
		}
	}
}

func TestCheckUserExists_NotFoundUsesStructuredError(t *testing.T) {
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			return &ghAPI.GraphQLError{
				Errors: []ghAPI.GraphQLErrorItem{{
					Type:    "NOT_FOUND",
					Message: "Could not resolve to a User with the login of 'nobody'",
				}},
			}
		},
	}
	exists, err := newTestClient(mock).CheckUserExists("nobody")
	if err != nil {
		t.Fatalf("NOT_FOUND should not be an error, got %v", err)
	}
	if exists {
		t.Error("expected user not to exist")
	}
}

func TestCheckUserExists_NotFoundIndependentOfWording(t *testing.T) {
	// Detection must survive GitHub rewording its message.
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			return &ghAPI.GraphQLError{
				Errors: []ghAPI.GraphQLErrorItem{{
					Type:    "NOT_FOUND",
					Message: "totally different wording",
				}},
			}
		},
	}
	exists, err := newTestClient(mock).CheckUserExists("nobody")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if exists {
		t.Error("expected user not to exist")
	}
}

func TestCheckUserExists_OtherGraphQLErrorIsReturned(t *testing.T) {
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			return &ghAPI.GraphQLError{
				Errors: []ghAPI.GraphQLErrorItem{{Type: "RATE_LIMITED", Message: "slow down"}},
			}
		},
	}
	if _, err := newTestClient(mock).CheckUserExists("someone"); err == nil {
		t.Fatal("expected a non-NOT_FOUND GraphQL error to be returned")
	}
}

func TestFetchContributions_PaginationErrorIsReturned(t *testing.T) {
	cursor := "cursor123"
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			if strings.Contains(query, "pullRequestContributions(first: 100)") {
				resp := response.(*contributionsResponse)
				resp.User.ContributionsCollection.PullRequestContributions.PageInfo = pageInfo{
					EndCursor: &cursor, HasNextPage: true,
				}
				return nil
			}
			return fmt.Errorf("rate limited")
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	_, err := newTestClient(mock).FetchContributions("user", dr)
	if err == nil {
		t.Fatal("a failed pagination page must not be reported as a complete result")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected the underlying error to be wrapped, got: %v", err)
	}
}

func TestFetchContributions_PageLimitReportsTruncation(t *testing.T) {
	cursor := "cursor123"
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			// Every page claims another page follows, so the limit is reached.
			if strings.Contains(query, "pullRequestContributions(first: 100)") {
				resp := response.(*contributionsResponse)
				resp.User.ContributionsCollection.PullRequestContributions.PageInfo = pageInfo{
					EndCursor: &cursor, HasNextPage: true,
				}
				return nil
			}
			resp := response.(*paginatePRsResponse)
			resp.User.ContributionsCollection.PullRequestContributions.PageInfo = pageInfo{
				EndCursor: &cursor, HasNextPage: true,
			}
			return nil
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	_, err := newTestClient(mock).FetchContributions("user", dr)
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("expected ErrTruncated when the page limit is hit, got: %v", err)
	}
}

func TestFetchIssueComments_FiltersToRange(t *testing.T) {
	inRange := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	beforeRange := time.Date(2023, 12, 20, 10, 0, 0, 0, time.UTC)
	afterRange := time.Date(2024, 2, 5, 10, 0, 0, 0, time.UTC)

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []struct {
				CreatedAt  time.Time
				UpdatedAt  time.Time
				Repository struct{ NameWithOwner string }
			}{
				// Edited recently, created after the range → excluded.
				{CreatedAt: afterRange, UpdatedAt: afterRange,
					Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo1"}},
				// In range → included.
				{CreatedAt: inRange, UpdatedAt: inRange,
					Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo2"}},
				// Created before the range but edited inside it → excluded by createdAt.
				{CreatedAt: beforeRange, UpdatedAt: inRange,
					Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo3"}},
			}
			resp.User.IssueComments.PageInfo = pageInfo{HasNextPage: false}
			return nil
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	events, err := newTestClient(mock).FetchIssueComments("user", dr)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 in-range comment, got %d", len(events))
	}
	if events[0].Repo != "user/repo2" {
		t.Errorf("expected user/repo2, got %s", events[0].Repo)
	}
	if events[0].Type != "IssueCommentEvent" {
		t.Errorf("expected IssueCommentEvent, got %s", events[0].Type)
	}
}

func TestFetchIssueComments_StopsOnceUpdatedAtPredatesRange(t *testing.T) {
	// updatedAt descending: the first node older than the range start proves no
	// later node can be in range (createdAt <= updatedAt), so the walk stops
	// without burning the remaining page budget.
	calls := 0
	cursor := "next"
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			calls++
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []struct {
				CreatedAt  time.Time
				UpdatedAt  time.Time
				Repository struct{ NameWithOwner string }
			}{
				{CreatedAt: old, UpdatedAt: old,
					Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo"}},
			}
			resp.User.IssueComments.PageInfo = pageInfo{EndCursor: &cursor, HasNextPage: true}
			return nil
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	events, err := newTestClient(mock).FetchIssueComments("user", dr)
	if err != nil {
		t.Fatalf("reaching the end of the relevant window is not an error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
	if calls != 1 {
		t.Errorf("expected the walk to stop after 1 page, got %d calls", calls)
	}
}

func TestFetchIssueComments_ReturnsPartialResultsOnError(t *testing.T) {
	inRange := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	cursor := "next"
	calls := 0

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			calls++
			if calls > 1 {
				return fmt.Errorf("network blip")
			}
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []struct {
				CreatedAt  time.Time
				UpdatedAt  time.Time
				Repository struct{ NameWithOwner string }
			}{
				{CreatedAt: inRange, UpdatedAt: inRange,
					Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo"}},
			}
			resp.User.IssueComments.PageInfo = pageInfo{EndCursor: &cursor, HasNextPage: true}
			return nil
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	events, err := newTestClient(mock).FetchIssueComments("user", dr)
	if err == nil {
		t.Fatal("expected the page error to be reported")
	}
	if len(events) != 1 {
		t.Errorf("successfully fetched comments must be returned alongside the error, got %d", len(events))
	}
}

func TestFetchContributionsPagination(t *testing.T) {
	cursor := "cursor123"
	callCount := 0

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			callCount++
			if strings.Contains(query, "pullRequestContributions(first: 100)") {
				// Initial query — return 1 PR with hasNextPage
				resp := response.(*contributionsResponse)
				resp.User.ContributionsCollection.PullRequestContributions.Nodes = []prContributionNode{
					{
						OccurredAt: time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
						PullRequest: struct {
							Number     int
							Title      string
							State      string
							CreatedAt  time.Time
							ClosedAt   *time.Time
							MergedAt   *time.Time
							Repository struct{ NameWithOwner string }
						}{
							Number: 1, Title: "PR 1", State: "OPEN",
							Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo"},
						},
					},
				}
				resp.User.ContributionsCollection.PullRequestContributions.PageInfo = pageInfo{
					EndCursor:   &cursor,
					HasNextPage: true,
				}
				return nil
			}
			// Pagination query
			resp := response.(*paginatePRsResponse)
			resp.User.ContributionsCollection.PullRequestContributions.Nodes = []prContributionNode{
				{
					OccurredAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
					PullRequest: struct {
						Number     int
						Title      string
						State      string
						CreatedAt  time.Time
						ClosedAt   *time.Time
						MergedAt   *time.Time
						Repository struct{ NameWithOwner string }
					}{
						Number: 2, Title: "PR 2", State: "OPEN",
						Repository: struct{ NameWithOwner string }{NameWithOwner: "user/repo"},
					},
				},
			}
			resp.User.ContributionsCollection.PullRequestContributions.PageInfo = pageInfo{
				HasNextPage: false,
			}
			return nil
		},
	}

	c := newTestClient(mock)
	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	result, err := c.FetchContributions("user", dr)
	if err != nil {
		t.Fatal(err)
	}

	// 2 PR opened events (1 from initial + 1 from pagination)
	prCount := 0
	for _, e := range result.Events {
		if e.Type == "PullRequestEvent" {
			prCount++
		}
	}
	if prCount != 2 {
		t.Errorf("expected 2 PullRequestEvents, got %d", prCount)
	}
	if callCount != 2 {
		t.Errorf("expected 2 GraphQL calls, got %d", callCount)
	}
}

func TestFetchContributionsGraphQLError(t *testing.T) {
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			return fmt.Errorf("GraphQL error: rate limited")
		},
	}

	c := newTestClient(mock)
	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	_, err := c.FetchContributions("user", dr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "GraphQL") {
		t.Errorf("expected GraphQL error, got: %v", err)
	}
}
