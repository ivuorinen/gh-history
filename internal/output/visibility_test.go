package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/models"
	"github.com/ivuorinen/gh-history/internal/testutil"
)

// visibilityStats is a report naming one private and one public repository in
// every place a repository appears: events, top repositories and commit counts.
func visibilityStats() models.Statistics {
	at := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	return models.Statistics{
		Username:     "testuser",
		DateRange:    testutil.SampleDateRange(),
		EventsByRepo: map[string]int{"u/priv": 3, "u/pub": 2},
		RepoPrivate:  map[string]bool{"u/priv": true, "u/pub": false},
		CommitsByRepo: []models.RepoCount{
			{Repo: "u/priv", Count: 4, Private: true},
			{Repo: "u/pub", Count: 1},
		},
		Events: []models.Event{
			{ID: "1", Type: "IssueCommentEvent", Repo: "u/priv", RepoPrivate: true, CreatedAt: at},
			{ID: "2", Type: "IssueCommentEvent", Repo: "u/pub", CreatedAt: at},
		},
	}
}

func TestFormatText_MarksRepoVisibility(t *testing.T) {
	var buf bytes.Buffer
	if err := FormatTextTo(&buf, false, 80, visibilityStats()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, row := range []string{"1.\tu/priv\tprivate\t3 events", "2.\tu/pub\tpublic\t2 events"} {
		if !strings.Contains(out, row) {
			t.Errorf("text report is missing top-repository row %q:\n%s", row, out)
		}
	}
}

func TestFormatMarkdown_MarksRepoVisibility(t *testing.T) {
	md := FormatMarkdown(visibilityStats())
	for _, line := range []string{
		"| # | Repository | Visibility | Events |",
		"| 1 | u/priv | private | 3 |",
		"| 2 | u/pub | public | 2 |",
	} {
		if !strings.Contains(md, line) {
			t.Errorf("markdown report is missing %q:\n%s", line, md)
		}
	}
}

func TestGenerateHTML_MarksRepoVisibility(t *testing.T) {
	html, err := buildHTML(visibilityStats())
	if err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{
		`<th scope="col">Visibility</th>`,
		`<td>u/priv</td><td>private</td><td>3</td>`,
		`<td>u/pub</td><td>public</td><td>2</td>`,
	} {
		if !strings.Contains(html, frag) {
			t.Errorf("HTML repositories table is missing %q", frag)
		}
	}
}

func TestFormatJSON_MarksRepoVisibility(t *testing.T) {
	out := decodeJSON(t, visibilityStats())

	for key, wantCounts := range map[string][2]float64{
		"top_repos":       {3, 2},
		"commits_by_repo": {4, 1},
	} {
		entries := out[key].([]any)
		if len(entries) != 2 {
			t.Fatalf("%s: expected 2 entries, got %d", key, len(entries))
		}
		for i, want := range []map[string]any{
			{"repo": "u/priv", "count": wantCounts[0], "private": true},
			{"repo": "u/pub", "count": wantCounts[1], "private": false},
		} {
			got := entries[i].(map[string]any)
			for k, v := range want {
				if got[k] != v {
					t.Errorf("%s[%d].%s = %v, want %v", key, i, k, got[k], v)
				}
			}
		}
	}

	for _, raw := range out["events"].([]any) {
		e := raw.(map[string]any)
		want := e["repo"] == "u/priv"
		if got, ok := e["repo_private"].(bool); !ok || got != want {
			t.Errorf("event on %v: repo_private = %v, want %v", e["repo"], e["repo_private"], want)
		}
	}
}
