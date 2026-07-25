package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeHost(t *testing.T) {
	tests := map[string]string{
		"github.com":           "github.com",
		"GitHub.com":           "github.com",
		"api.github.com":       "github.com",
		"github.localhost":     "github.localhost",
		"api.github.localhost": "github.localhost",
		"github.example.com":   "github.example.com",
		"GITHUB.EXAMPLE.COM":   "github.example.com",
		"acme.ghe.com":         "acme.ghe.com",
	}
	for in, want := range tests {
		if got := NormalizeHost(in); got != want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsEnterprise(t *testing.T) {
	tests := map[string]bool{
		"github.com":         false,
		"api.github.com":     false,
		"github.localhost":   false,
		"acme.ghe.com":       false, // tenancy, not Enterprise Server
		"github.example.com": true,
		"ghe.internal":       true,
	}
	for host, want := range tests {
		if got := IsEnterprise(host); got != want {
			t.Errorf("IsEnterprise(%q) = %v, want %v", host, got, want)
		}
	}
}

// The endpoint differs by host class; sending a query to the wrong URL is a
// silent failure mode for Enterprise users.
func TestGraphQLEndpoint(t *testing.T) {
	tests := map[string]string{
		"github.com":         "https://api.github.com/graphql",
		"api.github.com":     "https://api.github.com/graphql",
		"github.example.com": "https://github.example.com/api/graphql",
		"acme.ghe.com":       "https://api.acme.ghe.com/graphql",
		"github.localhost":   "http://api.github.localhost/graphql",
	}
	for host, want := range tests {
		if got := GraphQLEndpoint(host); got != want {
			t.Errorf("GraphQLEndpoint(%q) = %q, want %q", host, got, want)
		}
	}
}

// testClient points a graphQLClient at a stub server.
func testClient(t *testing.T, handler http.HandlerFunc) *graphQLClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &graphQLClient{endpoint: srv.URL, token: "tok", http: srv.Client()}
}

func TestGraphQLClientDo_SendsQueryAndAuth(t *testing.T) {
	var gotBody map[string]any
	var gotAuth, gotContentType, gotUserAgent, gotMethod string

	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotUserAgent = r.Header.Get("User-Agent")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"octocat"}}}`))
	})

	var resp struct {
		Viewer struct{ Login string }
	}
	if err := c.Do("query { viewer { login } }", map[string]any{"n": 1}, &resp); err != nil {
		t.Fatal(err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotAuth != "bearer tok" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "bearer tok")
	}
	if gotContentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if gotUserAgent == "" {
		t.Error("User-Agent must be set")
	}
	if gotBody["query"] != "query { viewer { login } }" {
		t.Errorf("query not sent verbatim: %v", gotBody["query"])
	}
	vars, ok := gotBody["variables"].(map[string]any)
	if !ok || vars["n"] != float64(1) {
		t.Errorf("variables not sent: %v", gotBody["variables"])
	}
	if resp.Viewer.Login != "octocat" {
		t.Errorf("data not unmarshalled into response: %+v", resp)
	}
}

// The response structs use Go field names against camelCase JSON and typed
// times; this is the decoding the whole tool depends on.
func TestGraphQLClientDo_DecodesNestedCamelCaseAndTimes(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"user":{"contributionsCollection":{
			"totalCommitContributions": 7,
			"contributionCalendar":{"totalContributions":9,"weeks":[{"contributionDays":[{"date":"2024-01-02","contributionCount":3}]}]},
			"pullRequestContributions":{"nodes":[{"occurredAt":"2024-01-10T10:00:00Z","pullRequest":{"number":42,"title":"T","state":"OPEN","createdAt":"2024-01-09T08:00:00Z","repository":{"nameWithOwner":"a/b"}}}],"pageInfo":{"endCursor":"c1","hasNextPage":true}}
		}}}}`))
	})

	var resp contributionsResponse
	if err := c.Do("q", nil, &resp); err != nil {
		t.Fatal(err)
	}

	cc := resp.User.ContributionsCollection
	if cc.TotalCommitContributions != 7 {
		t.Errorf("totalCommitContributions = %d, want 7", cc.TotalCommitContributions)
	}
	if cc.ContributionCalendar.TotalContributions != 9 {
		t.Errorf("calendar total = %d, want 9", cc.ContributionCalendar.TotalContributions)
	}
	days := cc.ContributionCalendar.Weeks[0].ContributionDays
	if len(days) != 1 || days[0].Date != "2024-01-02" || days[0].ContributionCount != 3 {
		t.Errorf("calendar days = %+v", days)
	}
	nodes := cc.PullRequestContributions.Nodes
	if len(nodes) != 1 {
		t.Fatalf("expected 1 PR node, got %d", len(nodes))
	}
	if !nodes[0].OccurredAt.Equal(time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("occurredAt = %v", nodes[0].OccurredAt)
	}
	if !nodes[0].PullRequest.CreatedAt.Equal(time.Date(2024, 1, 9, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("pullRequest.createdAt = %v", nodes[0].PullRequest.CreatedAt)
	}
	if nodes[0].PullRequest.Number != 42 || nodes[0].PullRequest.Repository.NameWithOwner != "a/b" {
		t.Errorf("pr = %+v", nodes[0].PullRequest)
	}
	pi := cc.PullRequestContributions.PageInfo
	if pi.EndCursor == nil || *pi.EndCursor != "c1" || !pi.HasNextPage {
		t.Errorf("pageInfo = %+v", pi)
	}
}

// A missing user comes back as HTTP 200 with an errors array, which is what
// CheckUserExists depends on.
func TestGraphQLClientDo_GraphQLErrorsBecomeGraphQLError(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"user":null},"errors":[{"type":"NOT_FOUND","message":"Could not resolve to a User","path":["user"]}]}`))
	})

	var resp struct{ User *struct{ Login string } }
	err := c.Do("q", nil, &resp)
	if err == nil {
		t.Fatal("expected an error")
	}
	var gqlErr *GraphQLError
	if !errors.As(err, &gqlErr) {
		t.Fatalf("expected *GraphQLError, got %T: %v", err, err)
	}
	if len(gqlErr.Errors) != 1 || gqlErr.Errors[0].Type != "NOT_FOUND" {
		t.Errorf("errors = %+v", gqlErr.Errors)
	}
	if !isNotFound(err) {
		t.Error("isNotFound should recognise this error")
	}
}

func TestGraphQLClientDo_HTTPErrorStatus(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	})

	err := c.Do("q", nil, &struct{}{})
	if err == nil {
		t.Fatal("expected an error")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", httpErr.StatusCode)
	}
	// A 401 must not be mistaken for "user does not exist".
	if isNotFound(err) {
		t.Error("an HTTP error must not be treated as NOT_FOUND")
	}
}

// GitHub buckets contributionsCollection by the Time-Zone header, so dropping
// it silently changes every count in the report. Measured against the live API:
// the same one-day query returns 1 review with no header and 2 with the local
// zone.
func TestGraphQLClientDo_SendsTimeZoneHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Time-Zone")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	c := &graphQLClient{endpoint: srv.URL, timeZone: "Europe/Berlin", http: srv.Client()}
	if err := c.Do("q", nil, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	if got != "Europe/Berlin" {
		t.Errorf("Time-Zone = %q, want Europe/Berlin", got)
	}
}

func TestGraphQLClientDo_OmitsEmptyTimeZone(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Time-Zone"]
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	c := &graphQLClient{endpoint: srv.URL, http: srv.Client()}
	if err := c.Do("q", nil, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Error("an undeterminable zone must omit the header, not send an empty one")
	}
}

func TestLocalTimeZone(t *testing.T) {
	t.Run("TZ is used", func(t *testing.T) {
		t.Setenv("TZ", "Europe/Berlin")
		if got := LocalTimeZone(); got != "Europe/Berlin" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("leading colon is stripped", func(t *testing.T) {
		t.Setenv("TZ", ":Europe/Berlin")
		if got := LocalTimeZone(); got != "Europe/Berlin" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("POSIX TZ is rejected, not forwarded", func(t *testing.T) {
		// GitHub would not understand this form; falling through to the
		// zoneinfo symlink (or to no header) is correct.
		t.Setenv("TZ", "CET-1CEST,M3.5.0,M10.5.0/3")
		if got := LocalTimeZone(); got == "CET-1CEST,M3.5.0,M10.5.0/3" {
			t.Error("a POSIX TZ spec must not be sent as an IANA zone")
		}
	})
}

func TestValidZone(t *testing.T) {
	for _, name := range []string{"Europe/Berlin", "UTC", "America/Los_Angeles"} {
		if !validZone(name) {
			t.Errorf("validZone(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"", "Local", "CET-1CEST,M3.5.0", "Not/AZone"} {
		if validZone(name) {
			t.Errorf("validZone(%q) = true, want false", name)
		}
	}
}

func TestGraphQLClientDo_NoTokenOmitsAuthHeader(t *testing.T) {
	var hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	c := &graphQLClient{endpoint: srv.URL, http: srv.Client()}
	if err := c.Do("q", nil, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	if hadAuth {
		t.Error("no Authorization header should be sent when there is no token")
	}
}

func TestGraphQLClientDo_MalformedJSON(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	})
	if err := c.Do("q", nil, &struct{}{}); err == nil {
		t.Error("expected a decode error")
	}
}

func TestNewClient_UsesHostEndpoint(t *testing.T) {
	c, err := NewClient("github.example.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	gql, ok := c.gqlClient.(*graphQLClient)
	if !ok {
		t.Fatalf("expected *graphQLClient, got %T", c.gqlClient)
	}
	if gql.endpoint != "https://github.example.com/api/graphql" {
		t.Errorf("endpoint = %q", gql.endpoint)
	}
	if gql.http.Timeout != RequestTimeout {
		t.Errorf("timeout = %v, want %v", gql.http.Timeout, RequestTimeout)
	}
}

func TestNewClient_EmptyHostDefaultsToGitHub(t *testing.T) {
	c, _ := NewClient("", "")
	gql := c.gqlClient.(*graphQLClient)
	if gql.endpoint != "https://api.github.com/graphql" {
		t.Errorf("endpoint = %q", gql.endpoint)
	}
}
