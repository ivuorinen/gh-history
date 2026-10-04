package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	hostGitHub    = "github.com"
	hostLocalhost = "github.localhost"
	// tenancySuffix marks a GitHub tenancy instance, which uses the github.com
	// style endpoint rather than the Enterprise Server one.
	tenancySuffix = "ghe.com"

	userAgent = "gh-history"
)

// NormalizeHost lowercases host and collapses github.com / github.localhost
// subdomains onto their canonical name.
func NormalizeHost(host string) string {
	h := strings.ToLower(host)
	if strings.HasSuffix(h, "."+hostGitHub) {
		return hostGitHub
	}
	if strings.HasSuffix(h, "."+hostLocalhost) {
		return hostLocalhost
	}
	return h
}

// IsEnterprise reports whether host is a GitHub Enterprise Server instance, as
// opposed to github.com, a local instance, or a tenancy instance. Enterprise
// Server uses a different API endpoint and different token environment
// variables.
func IsEnterprise(host string) bool {
	h := NormalizeHost(host)
	return h != hostGitHub && h != hostLocalhost && !strings.HasSuffix(h, "."+tenancySuffix)
}

// GraphQLEndpoint returns the GraphQL URL for host.
func GraphQLEndpoint(host string) string {
	h := NormalizeHost(host)
	if IsEnterprise(h) {
		return fmt.Sprintf("https://%s/api/graphql", h)
	}
	if h == hostLocalhost {
		return fmt.Sprintf("http://api.%s/graphql", h)
	}
	return fmt.Sprintf("https://api.%s/graphql", h)
}

// GraphQLError is an error response from the GraphQL API.
type GraphQLError struct {
	Errors []GraphQLErrorItem
}

// GraphQLErrorItem is a single entry of a GraphQL error response.
type GraphQLErrorItem struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Path    []any  `json:"path"`
}

func (e *GraphQLError) Error() string {
	msgs := make([]string, 0, len(e.Errors))
	for _, item := range e.Errors {
		msgs = append(msgs, item.Message)
	}
	return "GraphQL: " + strings.Join(msgs, ", ")
}

// HTTPError is a non-200 response from the API.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Status)
}

// graphQLClient posts GraphQL queries over HTTP. It replaces go-gh's
// api.GraphQLClient, of which this project used only the Do method.
type graphQLClient struct {
	endpoint string
	token    string
	timeZone string
	http     *http.Client
}

// LocalTimeZone returns the IANA name of the machine's time zone, or "" if it
// cannot be determined.
//
// This matters more than it looks: GitHub buckets contributionsCollection by
// the Time-Zone request header, so the same query returns different counts
// depending on it. Sending the local zone matches what the user sees on
// github.com, and is what go-gh did. Go's time.Local only reports "Local", so
// the name has to be recovered from the environment or the zoneinfo symlink.
func LocalTimeZone() string {
	if tz := strings.TrimPrefix(os.Getenv("TZ"), ":"); validZone(tz) {
		return tz
	}
	// On Unix /etc/localtime is a symlink into the zoneinfo database.
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if _, name, ok := strings.Cut(target, "zoneinfo/"); ok && validZone(name) {
			return name
		}
	}
	return ""
}

// validZone reports whether name is a loadable IANA zone. It rejects the POSIX
// TZ forms (such as "CET-1CEST,M3.5.0") that GitHub would not understand.
func validZone(name string) bool {
	if name == "" || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// maxErrorBody caps how much of an error response body is read into the
// message, so a proxy returning a large HTML page cannot flood the output.
const maxErrorBody = 2048

// retryDelays are the waits before each retry of a transient failure. Every
// query this tool sends is a read, so repeating one is safe; without retries a
// single gateway error on any page discarded the whole multi-request report.
// A variable so tests can run without sleeping.
var retryDelays = []time.Duration{time.Second, 2 * time.Second}

// Do executes query with variables and unmarshals the response's "data" object
// into response, retrying transient failures (see retryable).
func (c *graphQLClient) Do(query string, variables map[string]any, response any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return fmt.Errorf("encode GraphQL request: %w", err)
	}
	for attempt := 0; ; attempt++ {
		err = c.post(body, response)
		if attempt >= len(retryDelays) || !retryable(err) {
			return err
		}
		time.Sleep(retryDelays[attempt])
	}
}

// retryable reports whether err is a failure GitHub's GraphQL endpoint is known
// to produce under load and recover from: a 502, 503 or 504, or a timeout.
// Client errors (4xx) and GraphQL errors are not retried; repeating them
// returns the same answer.
func retryable(err error) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// post sends one request and decodes its response.
func (c *graphQLClient) post(body []byte, response any) error {
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build GraphQL request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "bearer "+c.token)
	}
	// Determines how GitHub buckets contributions into days.
	if c.timeZone != "" {
		req.Header.Set("Time-Zone", c.timeZone)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GraphQL request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(snippet)}
	}

	// Decode errors and data together: GitHub reports a missing user as a
	// NOT_FOUND error alongside a null data field.
	var envelope struct {
		Data   json.RawMessage    `json:"data"`
		Errors []GraphQLErrorItem `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode GraphQL response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return &GraphQLError{Errors: envelope.Errors}
	}
	if len(envelope.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, response); err != nil {
		return fmt.Errorf("decode GraphQL data: %w", err)
	}
	return nil
}
