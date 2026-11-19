package api

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

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
			return &GraphQLError{
				Errors: []GraphQLErrorItem{{
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
						Repository repoRef
					}{
						Number: 1, Title: "Open PR", State: "OPEN",
						CreatedAt:  time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
						Repository: repoRef{NameWithOwner: "user/repo1"},
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
						Repository repoRef
					}{
						Number: 2, Title: "Merged PR", State: "MERGED",
						CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
						ClosedAt:  &closedAt, MergedAt: &mergedAt,
						Repository: repoRef{NameWithOwner: "user/repo2"},
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
						Repository repoRef
					}{
						Number: 5, Title: "Open issue", State: "OPEN",
						CreatedAt:  time.Date(2024, 1, 12, 10, 0, 0, 0, time.UTC),
						Repository: repoRef{NameWithOwner: "user/repo1"},
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
						Repository repoRef
					}{
						Number: 6, Title: "Closed issue", State: "CLOSED",
						CreatedAt:  time.Date(2024, 1, 18, 10, 0, 0, 0, time.UTC),
						ClosedAt:   &issueClosedAt,
						Repository: repoRef{NameWithOwner: "user/repo2"},
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
							Repository repoRef
						}
					}{
						State:       "APPROVED",
						SubmittedAt: time.Date(2024, 1, 14, 10, 0, 0, 0, time.UTC),
						PullRequest: struct {
							Number     int
							Title      string
							Repository repoRef
						}{
							Number: 3, Title: "Some PR",
							Repository: repoRef{NameWithOwner: "user/repo1"},
						},
					},
				},
			}
			resp.User.ContributionsCollection.RepositoryContributions.Nodes = []repoContributionNode{
				{
					OccurredAt: time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC),
					Repository: struct {
						repoRef
						Description string
					}{
						repoRef:     repoRef{NameWithOwner: "user/new-repo"},
						Description: "A new repository",
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

	result, err := c.FetchContributions("user", dr, dr)
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

// Everything the GraphQL query asks for must reach the event or the result —
// fetching a field and then dropping it is what made these dead in the first
// place.
func TestFetchContributions_WiresQueriedDetail(t *testing.T) {
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			resp := response.(*contributionsResponse)
			cc := &resp.User.ContributionsCollection
			cc.TotalCommitContributions = 11
			cc.TotalIssueContributions = 22
			cc.TotalPullRequestContributions = 33
			cc.TotalPullRequestReviewContributions = 44
			cc.TotalRepositoryContributions = 55
			cc.ContributionCalendar.TotalContributions = 66
			cc.CommitContributionsByRepository = []struct {
				Repository    repoRef
				Contributions struct{ TotalCount int }
			}{
				{
					Repository:    repoRef{NameWithOwner: "user/private"},
					Contributions: struct{ TotalCount int }{TotalCount: 99},
				},
			}
			cc.PullRequestContributions.Nodes = []prContributionNode{{
				OccurredAt: time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
				PullRequest: struct {
					Number     int
					Title      string
					State      string
					CreatedAt  time.Time
					ClosedAt   *time.Time
					MergedAt   *time.Time
					Repository repoRef
				}{
					Number: 42, Title: "Fix the thing", State: "OPEN",
					CreatedAt:  time.Date(2024, 1, 9, 8, 0, 0, 0, time.UTC),
					Repository: repoRef{NameWithOwner: "user/repo1"},
				},
			}}
			cc.PullRequestReviewContributions.Nodes = []reviewContributionNode{{
				OccurredAt: time.Date(2024, 1, 14, 10, 0, 0, 0, time.UTC),
				PullRequestReview: struct {
					State       string
					SubmittedAt time.Time
					PullRequest struct {
						Number     int
						Title      string
						Repository repoRef
					}
				}{
					State:       "CHANGES_REQUESTED",
					SubmittedAt: time.Date(2024, 1, 14, 10, 0, 0, 0, time.UTC),
					PullRequest: struct {
						Number     int
						Title      string
						Repository repoRef
					}{
						Number: 7, Title: "Someone else's PR",
						Repository: repoRef{NameWithOwner: "user/repo2"},
					},
				},
			}}
			cc.RepositoryContributions.Nodes = []repoContributionNode{{
				OccurredAt: time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC),
				Repository: struct {
					repoRef
					Description string
				}{repoRef: repoRef{NameWithOwner: "user/new-repo"}, Description: "A brand new repo"},
			}}
			return nil
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	result, err := newTestClient(mock).FetchContributions("user", dr, dr)
	if err != nil {
		t.Fatal(err)
	}

	want := models.ContributionTotals{
		Commits: 11, Issues: 22, PullRequests: 33, Reviews: 44, Repositories: 55,
	}
	if result.Totals != want {
		t.Errorf("Totals = %+v, want %+v", result.Totals, want)
	}
	if result.CalendarTotal != 66 {
		t.Errorf("CalendarTotal = %d, want 66", result.CalendarTotal)
	}
	if len(result.CommitsByRepo) != 1 ||
		result.CommitsByRepo[0].Repo != "user/private" ||
		result.CommitsByRepo[0].Count != 99 {
		t.Errorf("CommitsByRepo = %+v", result.CommitsByRepo)
	}

	byType := map[string]models.Event{}
	for _, e := range result.Events {
		byType[e.Type] = e
	}
	if pr := byType["PullRequestEvent"]; pr.Number != 42 ||
		pr.Title != "Fix the thing" ||
		!pr.SubjectCreatedAt.Equal(time.Date(2024, 1, 9, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("PR detail not wired: %+v", pr)
	}
	if rv := byType["PullRequestReviewEvent"]; rv.ReviewState != "CHANGES_REQUESTED" ||
		rv.Number != 7 || rv.Title != "Someone else's PR" {
		t.Errorf("review detail not wired: %+v", rv)
	}
	if cr := byType["CreateEvent"]; cr.Description != "A brand new repo" {
		t.Errorf("repo description not wired: %+v", cr)
	}
}

// Every node type carries its repository's visibility into the events it
// produces, and the per-repository commit counts carry it too, so a report can
// mark private repositories. Each node type appears once on a private and once
// on a public repository: a mapping that drops the flag fails the private one,
// and one that hard-codes it fails the public one.
func TestFetchContributions_WiresRepoVisibility(t *testing.T) {
	priv := repoRef{NameWithOwner: "user/private", IsPrivate: true}
	pub := repoRef{NameWithOwner: "user/public"}
	at := time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC)
	closed := time.Date(2024, 1, 12, 10, 0, 0, 0, time.UTC)

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			cc := &response.(*contributionsResponse).User.ContributionsCollection
			for i, repo := range []repoRef{priv, pub} {
				var commits struct {
					Repository    repoRef
					Contributions struct{ TotalCount int }
				}
				commits.Repository = repo
				commits.Contributions.TotalCount = 5
				cc.CommitContributionsByRepository = append(cc.CommitContributionsByRepository, commits)

				var pr prContributionNode
				pr.OccurredAt = at
				pr.PullRequest.Number = i + 1
				pr.PullRequest.State = "MERGED"
				pr.PullRequest.ClosedAt = &closed
				pr.PullRequest.MergedAt = &closed
				pr.PullRequest.Repository = repo
				cc.PullRequestContributions.Nodes = append(cc.PullRequestContributions.Nodes, pr)

				var issue issueContributionNode
				issue.OccurredAt = at
				issue.Issue.Number = i + 1
				issue.Issue.State = "CLOSED"
				issue.Issue.ClosedAt = &closed
				issue.Issue.Repository = repo
				cc.IssueContributions.Nodes = append(cc.IssueContributions.Nodes, issue)

				var review reviewContributionNode
				review.OccurredAt = at
				review.PullRequestReview.SubmittedAt = at
				review.PullRequestReview.PullRequest.Number = i + 1
				review.PullRequestReview.PullRequest.Repository = repo
				cc.PullRequestReviewContributions.Nodes = append(cc.PullRequestReviewContributions.Nodes, review)

				var created repoContributionNode
				created.OccurredAt = at
				created.Repository.repoRef = repo
				cc.RepositoryContributions.Nodes = append(cc.RepositoryContributions.Nodes, created)
			}
			return nil
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	result, err := newTestClient(mock).FetchContributions("user", dr, dr)
	if err != nil {
		t.Fatal(err)
	}

	// Per repository: PR opened + closed, issue opened + closed, review, created.
	if len(result.Events) != 12 {
		t.Fatalf("expected 12 events, got %d", len(result.Events))
	}
	for _, e := range result.Events {
		if want := e.Repo == "user/private"; e.RepoPrivate != want {
			t.Errorf("%s %s on %s: RepoPrivate = %v, want %v",
				e.Type, e.Action, e.Repo, e.RepoPrivate, want)
		}
	}
	wantCommits := []models.RepoCount{
		{Repo: "user/private", Count: 5, Private: true},
		{Repo: "user/public", Count: 5},
	}
	if !slices.Equal(result.CommitsByRepo, wantCommits) {
		t.Errorf("CommitsByRepo = %+v, want %+v", result.CommitsByRepo, wantCommits)
	}
}

// Issue comments carry their repository's visibility the same way the
// contribution events do.
func TestFetchIssueComments_WiresRepoVisibility(t *testing.T) {
	at := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []issueCommentNode{
				{ID: "c1", CreatedAt: at, UpdatedAt: at,
					Repository: repoRef{NameWithOwner: "user/private", IsPrivate: true}},
				{ID: "c2", CreatedAt: at, UpdatedAt: at,
					Repository: repoRef{NameWithOwner: "user/public"}},
			}
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
	if len(events) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(events))
	}
	for _, e := range events {
		if want := e.Repo == "user/private"; e.RepoPrivate != want {
			t.Errorf("comment on %s: RepoPrivate = %v, want %v", e.Repo, e.RepoPrivate, want)
		}
	}
}

func TestCheckUserExists_NotFoundUsesStructuredError(t *testing.T) {
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			return &GraphQLError{
				Errors: []GraphQLErrorItem{{
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
			return &GraphQLError{
				Errors: []GraphQLErrorItem{{
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
			return &GraphQLError{
				Errors: []GraphQLErrorItem{{Type: "RATE_LIMITED", Message: "slow down"}},
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
	_, err := newTestClient(mock).FetchContributions("user", dr, dr)
	if err == nil {
		t.Fatal("a failed pagination page must not be reported as a complete result")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected the underlying error to be wrapped, got: %v", err)
	}
}

// Every page GitHub offers is fetched. A 10-page limit cut a year's reviews at
// 1,100 while GitHub returned all 2,594 across 26 pages.
func TestFetchContributions_FollowsEveryPage(t *testing.T) {
	const extraPages = 15
	prNode := func(number int) prContributionNode {
		n := prContributionNode{OccurredAt: time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC)}
		n.PullRequest.Number = number
		n.PullRequest.State = "OPEN"
		n.PullRequest.Repository.NameWithOwner = "user/repo"
		return n
	}
	page := 0
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			if strings.Contains(query, "pullRequestContributions(first: 100)") {
				cursor := "c0"
				resp := response.(*contributionsResponse)
				resp.User.ContributionsCollection.PullRequestContributions.Nodes = []prContributionNode{prNode(0)}
				resp.User.ContributionsCollection.PullRequestContributions.PageInfo = pageInfo{
					EndCursor: &cursor, HasNextPage: true,
				}
				return nil
			}
			if !strings.Contains(query, "pullRequestContributions(first: 100, after: $after)") {
				return nil
			}
			// Each request must continue from the cursor the previous page returned.
			if want := fmt.Sprintf("c%d", page); variables["after"] != want {
				return fmt.Errorf("request %d sent after=%v, want %q", page+1, variables["after"], want)
			}
			page++
			cursor := fmt.Sprintf("c%d", page)
			resp := response.(*paginatePRsResponse)
			resp.User.ContributionsCollection.PullRequestContributions.Nodes = []prContributionNode{prNode(page)}
			resp.User.ContributionsCollection.PullRequestContributions.PageInfo = pageInfo{
				EndCursor: &cursor, HasNextPage: page < extraPages,
			}
			return nil
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	result, err := newTestClient(mock).FetchContributions("user", dr, dr)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 16 {
		t.Errorf("got %d PR events, want 16: the first page plus all 15 that follow", len(result.Events))
	}
}

// With no page limit, a cursor that comes back again is what stops a walk that
// would otherwise never end. The mock fails on its fourth request, so a
// missing check surfaces as a call count rather than a hung test.
func TestFetchContributions_RepeatedCursorIsAnError(t *testing.T) {
	cursor := "A"
	calls := 0
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			if strings.Contains(query, "pullRequestContributions(first: 100)") {
				resp := response.(*contributionsResponse)
				resp.User.ContributionsCollection.PullRequestContributions.PageInfo = pageInfo{
					EndCursor: &cursor, HasNextPage: true,
				}
				return nil
			}
			if !strings.Contains(query, "pullRequestContributions(first: 100, after: $after)") {
				return nil
			}
			calls++
			if calls > 3 {
				return fmt.Errorf("request %d: the walk did not stop", calls)
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
	if _, err := newTestClient(mock).FetchContributions("user", dr, dr); err == nil {
		t.Fatal("a page that repeats its cursor must not be reported as complete")
	}
	if calls != 1 {
		t.Errorf("sent %d follow-up requests, want 1: the repeated cursor must not be requested again", calls)
	}
}

func TestFetchIssueComments_FiltersToRange(t *testing.T) {
	inRange := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	beforeRange := time.Date(2023, 12, 20, 10, 0, 0, 0, time.UTC)
	afterRange := time.Date(2024, 2, 5, 10, 0, 0, 0, time.UTC)

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []issueCommentNode{
				// Edited recently, created after the range → excluded.
				{CreatedAt: afterRange, UpdatedAt: afterRange,
					Repository: repoRef{NameWithOwner: "user/repo1"}},
				// In range → included.
				{CreatedAt: inRange, UpdatedAt: inRange,
					Repository: repoRef{NameWithOwner: "user/repo2"}},
				// Created before the range but edited inside it → excluded by createdAt.
				{CreatedAt: beforeRange, UpdatedAt: inRange,
					Repository: repoRef{NameWithOwner: "user/repo3"}},
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

// Two comments in one repository in the same second must stay two events; the
// old time+repo ID made them collide and dedup dropped one.
func TestFetchIssueComments_SameSecondCommentsHaveDistinctIDs(t *testing.T) {
	at := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	repo := repoRef{NameWithOwner: "user/repo"}
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []issueCommentNode{
				{ID: "IC_a", CreatedAt: at, UpdatedAt: at, Repository: repo},
				{ID: "IC_b", CreatedAt: at, UpdatedAt: at, Repository: repo},
			}
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
	if len(events) != 2 || events[0].ID == events[1].ID {
		t.Errorf("expected two distinct comment events, got %+v", events)
	}
}

func TestFetchIssueComments_StopsOnceUpdatedAtPredatesRange(t *testing.T) {
	// updatedAt descending: the first node older than the range start proves no
	// later node can be in range (createdAt <= updatedAt), so the walk stops
	// there instead of paging through every older comment.
	calls := 0
	cursor := "next"
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			calls++
			// No page limit backs the date stop up any more: fail at once if it
			// lets a second request through.
			if calls > 1 {
				return fmt.Errorf("request %d: the walk did not stop at the range start", calls)
			}
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []issueCommentNode{
				{CreatedAt: old, UpdatedAt: old,
					Repository: repoRef{NameWithOwner: "user/repo"}},
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
			resp.User.IssueComments.Nodes = []issueCommentNode{
				{CreatedAt: inRange, UpdatedAt: inRange,
					Repository: repoRef{NameWithOwner: "user/repo"}},
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

// The walk runs until it passes the range start or the comments run out. A
// 10-page limit stopped it at 1,000 comments, so any range older than the
// user's 1,000 most recently updated comments came back short or empty.
func TestFetchIssueComments_FollowsEveryPage(t *testing.T) {
	const pages = 12
	inRange := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	calls := 0

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			calls++
			// The first request starts at the top; each later one continues from
			// the cursor the previous page returned.
			after, sent := variables["after"]
			if want := fmt.Sprintf("c%d", calls-1); (calls == 1 && sent) || (calls > 1 && after != want) {
				return fmt.Errorf("request %d sent after=%v (sent: %t)", calls, after, sent)
			}
			cursor := fmt.Sprintf("c%d", calls)
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []issueCommentNode{
				{ID: cursor, CreatedAt: inRange, UpdatedAt: inRange,
					Repository: repoRef{NameWithOwner: "user/repo"}},
			}
			resp.User.IssueComments.PageInfo = pageInfo{EndCursor: &cursor, HasNextPage: calls < pages}
			return nil
		},
	}

	dr := daterange.DateRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
	}
	events, err := newTestClient(mock).FetchIssueComments("user", dr)
	if err != nil {
		t.Fatalf("a walk that reaches the last page is complete: %v", err)
	}
	if len(events) != 12 {
		t.Errorf("got %d comments, want 12: one from every page", len(events))
	}
}

// A cursor that comes back again ends the comment walk with an error, keeping
// what was fetched. The mock fails on its fourth request, so a missing check
// surfaces as a call count rather than a hung test.
func TestFetchIssueComments_RepeatedCursorIsAnError(t *testing.T) {
	inRange := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	cursor := "A"
	calls := 0

	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			calls++
			if calls > 3 {
				return fmt.Errorf("request %d: the walk did not stop", calls)
			}
			resp := response.(*issueCommentsResponse)
			resp.User.IssueComments.Nodes = []issueCommentNode{
				{ID: fmt.Sprintf("IC_%d", calls), CreatedAt: inRange, UpdatedAt: inRange,
					Repository: repoRef{NameWithOwner: "user/repo"}},
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
		t.Fatal("a page that repeats its cursor must be reported, not treated as the end")
	}
	if calls != 2 {
		t.Errorf("sent %d requests, want 2: the repeated cursor must not be requested again", calls)
	}
	if len(events) != 2 {
		t.Errorf("the comments fetched before the repeat must be kept, got %d", len(events))
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
							Repository repoRef
						}{
							Number: 1, Title: "PR 1", State: "OPEN",
							Repository: repoRef{NameWithOwner: "user/repo"},
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
						Repository repoRef
					}{
						Number: 2, Title: "PR 2", State: "OPEN",
						Repository: repoRef{NameWithOwner: "user/repo"},
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

	result, err := c.FetchContributions("user", dr, dr)
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

// GitHub counts the whole day `to` falls on, so `to` must be the last second of
// End. Sending the next midnight added a day to every total.
func TestFetchContributions_ToIsLastSecondOfEnd(t *testing.T) {
	var gotFrom, gotTo any
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			gotFrom, gotTo = variables["from"], variables["to"]
			return nil
		},
	}
	day := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	dr := daterange.DateRange{Start: day, End: day}
	if _, err := newTestClient(mock).FetchContributions("user", dr, dr); err != nil {
		t.Fatal(err)
	}
	if gotFrom != "2025-01-01T00:00:00Z" {
		t.Errorf("from = %v, want 2025-01-01T00:00:00Z", gotFrom)
	}
	if gotTo != "2025-01-01T23:59:59Z" {
		t.Errorf("to = %v, want 2025-01-01T23:59:59Z", gotTo)
	}
}

// A PR opened in one year chunk and merged in the next is a contribution of the
// first chunk only, so its merge must be matched against the report range.
func TestFetchContributions_CloseMatchedAgainstReportRange(t *testing.T) {
	mergedAt := time.Date(2024, 1, 5, 9, 0, 0, 0, time.UTC)
	mock := &mockGQLClient{
		doFunc: func(query string, variables map[string]any, response any) error {
			resp := response.(*contributionsResponse)
			resp.User.ContributionsCollection.PullRequestContributions.Nodes = []prContributionNode{{
				OccurredAt: time.Date(2023, 12, 20, 9, 0, 0, 0, time.UTC),
				PullRequest: struct {
					Number     int
					Title      string
					State      string
					CreatedAt  time.Time
					ClosedAt   *time.Time
					MergedAt   *time.Time
					Repository repoRef
				}{
					Number: 9, State: "MERGED", ClosedAt: &mergedAt, MergedAt: &mergedAt,
					Repository: repoRef{NameWithOwner: "user/repo"},
				},
			}}
			return nil
		},
	}
	chunk := daterange.DateRange{
		Start: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	report := daterange.DateRange{Start: chunk.Start, End: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)}

	result, err := newTestClient(mock).FetchContributions("user", chunk, report)
	if err != nil {
		t.Fatal(err)
	}
	merged := false
	for _, e := range result.Events {
		if e.Action == models.ActionClosed && e.Merged {
			merged = true
		}
	}
	if !merged {
		t.Error("a merge in the next chunk of the report range must produce a merged event")
	}

	// The same PR reported on 2023 alone: the merge is outside the range.
	result, err = newTestClient(mock).FetchContributions("user", chunk, chunk)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range result.Events {
		if e.Action == models.ActionClosed {
			t.Error("a merge after the report range must not be counted")
		}
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

	_, err := c.FetchContributions("user", dr, dr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "GraphQL") {
		t.Errorf("expected GraphQL error, got: %v", err)
	}
}
