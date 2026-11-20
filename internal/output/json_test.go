package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/analysis"
	"github.com/ivuorinen/gh-history/internal/models"
	"github.com/ivuorinen/gh-history/internal/testutil"
)

func decodeJSON(t *testing.T, stats models.Statistics) map[string]any {
	t.Helper()
	data, err := FormatJSON(stats)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return out
}

// JSON is the full offering: it must carry the detail the human-readable
// formats omit, not just the shared summary.
func TestFormatJSON_CarriesFullDetail(t *testing.T) {
	out := decodeJSON(t, testutil.SampleStats())

	for _, key := range []string{
		"events", "contribution_totals", "commits_by_repo",
		"events_by_date", "calendar",
	} {
		if _, ok := out[key]; !ok {
			t.Errorf("JSON is missing %q", key)
		}
	}

	totals := out["contribution_totals"].(map[string]any)
	for key, want := range map[string]float64{
		"commits": 80, "issues": 15, "pull_requests": 10,
		"reviews": 10, "repositories": 3,
	} {
		if got := totals[key].(float64); got != want {
			t.Errorf("contribution_totals.%s = %v, want %v", key, got, want)
		}
	}

	// Both repo lists must use the same key casing: models.RepoCount carries no
	// JSON tags, so emitting it directly would produce {"Repo","Count"} beside
	// repoCounts' {"repo","count"} in the same document.
	for _, key := range []string{"top_repos", "commits_by_repo"} {
		entries := out[key].([]any)
		if len(entries) == 0 {
			t.Fatalf("%s is empty; the casing assertion needs data", key)
		}
		first := entries[0].(map[string]any)
		if _, ok := first["repo"]; !ok {
			t.Errorf("%s uses the wrong key casing: %v", key, first)
		}
		if _, ok := first["Repo"]; ok {
			t.Errorf("%s leaked Go field names: %v", key, first)
		}
	}

	byRepo := out["commits_by_repo"].([]any)
	if len(byRepo) != 2 {
		t.Fatalf("expected 2 commits_by_repo entries, got %d", len(byRepo))
	}
	first := byRepo[0].(map[string]any)
	if first["repo"] != "testuser/repo1" || first["count"].(float64) != 50 {
		t.Errorf("unexpected first commits_by_repo entry: %v", first)
	}

	cal := out["calendar"].(map[string]any)
	if cal["reported_total"].(float64) != 30 {
		t.Errorf("calendar.reported_total = %v, want 30", cal["reported_total"])
	}
	// Weekday must use this project's 0=Monday convention, matching
	// events_by_weekday — not GitHub's 0=Sunday.
	days := cal["days"].([]any)
	day0 := days[0].(map[string]any) // 2024-01-10 is a Wednesday
	if day0["weekday"].(float64) != 2 {
		t.Errorf("2024-01-10 weekday = %v, want 2 (Wednesday, Monday=0)", day0["weekday"])
	}
}

// JSON lists every repository, so a consumer gets each one's count and
// visibility without regrouping the events, and a repository missing from the
// list had no events. The other formats keep the top 15.
func TestFormatJSON_TopReposListsEveryRepository(t *testing.T) {
	at := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	var events []models.Event
	// u/repo00 has one event and u/repo19 twenty, so the counts fix the order;
	// the even-numbered repositories are private.
	for i := range 20 {
		for range i + 1 {
			events = append(events, models.Event{
				Type: "IssueCommentEvent", Repo: fmt.Sprintf("u/repo%02d", i),
				RepoPrivate: i%2 == 0, CreatedAt: at,
			})
		}
	}
	calc := analysis.Calculator{Username: "u", DateRange: testutil.SampleDateRange()}
	stats := calc.Calculate(events)
	out := decodeJSON(t, stats)

	entries := out["top_repos"].([]any)
	if len(entries) != 20 {
		t.Fatalf("top_repos has %d entries, want one per repository in events (20)", len(entries))
	}
	sum := 0
	for i, raw := range entries {
		e := raw.(map[string]any)
		n := 19 - i
		want := map[string]any{"repo": fmt.Sprintf("u/repo%02d", n), "count": float64(n + 1), "private": n%2 == 0}
		for k, v := range want {
			if e[k] != v {
				t.Errorf("top_repos[%d].%s = %v, want %v", i, k, e[k], v)
			}
		}
		count, _ := e["count"].(float64)
		sum += int(count)
	}
	if total := int(out["summary"].(map[string]any)["total_events"].(float64)); sum != total {
		t.Errorf("top_repos counts sum to %d, want summary.total_events (%d)", sum, total)
	}
	// events_by_repo carried the same counts without visibility.
	if _, ok := out["events_by_repo"]; ok {
		t.Error("JSON still carries events_by_repo")
	}

	md := FormatMarkdown(stats)
	if !strings.Contains(md, "| 15 | u/repo05 | public | 6 |") || strings.Contains(md, "u/repo04") {
		t.Errorf("markdown should list the top 15 repositories, u/repo19 down to u/repo05:\n%s", md)
	}
}

func TestFormatJSON_EventDetail(t *testing.T) {
	out := decodeJSON(t, testutil.SampleStats())
	events := out["events"].([]any)
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	byID := map[string]map[string]any{}
	for _, e := range events {
		m := e.(map[string]any)
		byID[m["id"].(string)] = m
	}

	// Merged PR carries number, title, action and the merged flag.
	pr := byID["3"]
	if pr["title"] != "Add widget" || pr["number"].(float64) != 7 {
		t.Errorf("PR event lost its detail: %v", pr)
	}
	if pr["action"] != "closed" || pr["merged"] != true {
		t.Errorf("expected closed+merged, got %v", pr)
	}

	// Review carries its state.
	if got := byID["5"]["review_state"]; got != "APPROVED" {
		t.Errorf("review_state = %v, want APPROVED", got)
	}

	// A comment event has no action, number, title or review state; those keys
	// must be absent rather than present as zero values.
	comment := byID["1"]
	for _, key := range []string{"action", "number", "title", "review_state", "merged", "description"} {
		if _, ok := comment[key]; ok {
			t.Errorf("comment event should omit %q, got %v", key, comment[key])
		}
	}

	// An opened PR is not closed, so it carries no merged flag.
	if _, ok := byID["2"]["merged"]; ok {
		t.Error("an opened PR should not carry a merged flag")
	}
}

func TestFormatJSON_SummaryMatchesOtherFormats(t *testing.T) {
	// JSON adds detail but must not disagree with the shared summary.
	stats := testutil.SampleStats()
	out := decodeJSON(t, stats)
	summary := out["summary"].(map[string]any)

	for key, want := range map[string]int{
		"total_events": stats.TotalEvents, "commits": stats.CommitCount,
		"prs_opened": stats.PROpened, "prs_merged": stats.PRMerged,
		"prs_closed": stats.PRClosed, "issues_opened": stats.IssuesOpened,
		"issues_closed": stats.IssuesClosed, "reviews": stats.ReviewsCount,
	} {
		if got := int(summary[key].(float64)); got != want {
			t.Errorf("summary.%s = %d, want %d", key, got, want)
		}
	}
}

func TestFormatJSON_NilCalendarAndEmptyEvents(t *testing.T) {
	out := decodeJSON(t, models.Statistics{Username: "u"})
	if out["calendar"] != nil {
		t.Errorf("expected null calendar, got %v", out["calendar"])
	}
	if events, ok := out["events"].([]any); !ok || len(events) != 0 {
		t.Errorf("expected an empty events array, got %v", out["events"])
	}
}
