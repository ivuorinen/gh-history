package output

import (
	"testing"
	"time"

	"github.com/ivuorinen/gh-history/internal/models"
)

// rateStats has 3 of 4 opened PRs merged and 1 of 4 opened issues closed. Its
// PRMerged and IssuesClosed differ from both, so a rate computed from them
// shows up as a wrong value.
func rateStats() models.Statistics {
	return models.Statistics{
		PROpened: 4, PROpenedMerged: 3, PRMerged: 7,
		IssuesOpened: 4, IssuesOpenedClosed: 1, IssuesClosed: 9,
	}
}

// The summary rows are what text, Markdown and HTML render
// (TestSummaryParityAcrossFormats holds that), so the rates reach all three
// through here.
func TestBuildSummary_IncludesRates(t *testing.T) {
	got := map[string]string{}
	for _, r := range BuildSummary(rateStats()) {
		got[r.Label] = r.Value
	}
	for label, want := range map[string]string{
		"PR Merge Rate":    "75.0%",
		"Issue Close Rate": "25.0%",
	} {
		if got[label] != want {
			t.Errorf("summary row %q = %q, want %q", label, got[label], want)
		}
	}
}

func TestFormatJSON_SummaryIncludesRates(t *testing.T) {
	summary := decodeJSON(t, rateStats())["summary"].(map[string]any)
	for key, want := range map[string]float64{"pr_merge_rate": 75, "issue_close_rate": 25} {
		if got, ok := summary[key].(float64); !ok || got != want {
			t.Errorf("summary.%s = %v, want %v", key, summary[key], want)
		}
	}
}

// subject_state appears on PR and issue events and is omitted where an event
// type has no subject, like every other per-type detail field.
func TestFormatJSON_EventSubjectState(t *testing.T) {
	at := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	out := decodeJSON(t, models.Statistics{Events: []models.Event{
		{ID: "pr", Type: "PullRequestEvent", Action: models.ActionOpened, SubjectState: "MERGED", CreatedAt: at},
		{ID: "c", Type: "IssueCommentEvent", CreatedAt: at},
	}})
	events := out["events"].([]any)
	if got := events[0].(map[string]any)["subject_state"]; got != "MERGED" {
		t.Errorf("PR event subject_state = %v, want MERGED", got)
	}
	if got, ok := events[1].(map[string]any)["subject_state"]; ok {
		t.Errorf("comment event must omit subject_state, got %v", got)
	}
}
