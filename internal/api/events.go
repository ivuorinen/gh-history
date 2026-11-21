package api

import (
	"fmt"
	"time"

	"github.com/ivuorinen/gh-history/internal/daterange"
	"github.com/ivuorinen/gh-history/internal/models"
)

// The functions here turn each kind of contributionsCollection node into the
// events a report counts. An event's ID is built from what it describes, so a
// year chunk that returns the same contribution again produces the same ID and
// main.fetchEvents drops the repeat.

// prEvents returns an opened event for every pull request and, for one closed
// or merged inside report, a closed event. The close is matched against the
// whole report, not the chunk the pull request was fetched in: a pull request
// is a contribution only in the chunk where it was opened, so one opened late
// in a chunk and closed in the next would otherwise never count as closed.
func prEvents(nodes []prContributionNode, report daterange.DateRange) []models.Event {
	var events []models.Event
	for _, n := range nodes {
		pr, repo := n.PullRequest, n.PullRequest.Repository
		events = append(events, models.Event{
			ID:               fmt.Sprintf("gql-pr-opened-%d-%s", pr.Number, repo.NameWithOwner),
			Type:             "PullRequestEvent",
			Repo:             repo.NameWithOwner,
			RepoPrivate:      repo.IsPrivate,
			RepoID:           repo.DatabaseID,
			Action:           models.ActionOpened,
			CreatedAt:        n.OccurredAt,
			Number:           pr.Number,
			Title:            pr.Title,
			SubjectCreatedAt: pr.CreatedAt,
			SubjectState:     pr.State,
		})
		if (pr.State == "CLOSED" || pr.State == "MERGED") && pr.ClosedAt != nil && inReport(*pr.ClosedAt, report) {
			events = append(events, models.Event{
				ID:               fmt.Sprintf("gql-pr-closed-%d-%s", pr.Number, repo.NameWithOwner),
				Type:             "PullRequestEvent",
				Repo:             repo.NameWithOwner,
				RepoPrivate:      repo.IsPrivate,
				RepoID:           repo.DatabaseID,
				Action:           models.ActionClosed,
				Merged:           pr.MergedAt != nil,
				CreatedAt:        *pr.ClosedAt,
				Number:           pr.Number,
				Title:            pr.Title,
				SubjectCreatedAt: pr.CreatedAt,
				SubjectState:     pr.State,
			})
		}
	}
	return events
}

// issueEvents returns an opened event for every issue and, for one closed
// inside report, a closed event, matching the close against the whole report
// for the reason prEvents gives.
func issueEvents(nodes []issueContributionNode, report daterange.DateRange) []models.Event {
	var events []models.Event
	for _, n := range nodes {
		issue, repo := n.Issue, n.Issue.Repository
		events = append(events, models.Event{
			ID:               fmt.Sprintf("gql-issue-opened-%d-%s", issue.Number, repo.NameWithOwner),
			Type:             "IssuesEvent",
			Repo:             repo.NameWithOwner,
			RepoPrivate:      repo.IsPrivate,
			RepoID:           repo.DatabaseID,
			Action:           models.ActionOpened,
			CreatedAt:        n.OccurredAt,
			Number:           issue.Number,
			Title:            issue.Title,
			SubjectCreatedAt: issue.CreatedAt,
			SubjectState:     issue.State,
		})
		if issue.State == "CLOSED" && issue.ClosedAt != nil && inReport(*issue.ClosedAt, report) {
			events = append(events, models.Event{
				ID:               fmt.Sprintf("gql-issue-closed-%d-%s", issue.Number, repo.NameWithOwner),
				Type:             "IssuesEvent",
				Repo:             repo.NameWithOwner,
				RepoPrivate:      repo.IsPrivate,
				RepoID:           repo.DatabaseID,
				Action:           models.ActionClosed,
				CreatedAt:        *issue.ClosedAt,
				Number:           issue.Number,
				Title:            issue.Title,
				SubjectCreatedAt: issue.CreatedAt,
				SubjectState:     issue.State,
			})
		}
	}
	return events
}

// reviewEvents returns one event per submitted review. The submission time is
// part of the ID because one pull request can carry several reviews.
func reviewEvents(nodes []reviewContributionNode) []models.Event {
	var events []models.Event
	for _, n := range nodes {
		review, pr := n.PullRequestReview, n.PullRequestReview.PullRequest
		events = append(events, models.Event{
			ID:          fmt.Sprintf("gql-review-%s-%d-%s", review.SubmittedAt.Format(time.RFC3339), pr.Number, pr.Repository.NameWithOwner),
			Type:        "PullRequestReviewEvent",
			Repo:        pr.Repository.NameWithOwner,
			RepoPrivate: pr.Repository.IsPrivate,
			RepoID:      pr.Repository.DatabaseID,
			CreatedAt:   n.OccurredAt,
			Number:      pr.Number,
			Title:       pr.Title,
			ReviewState: review.State,
		})
	}
	return events
}

// repoEvents returns one event per repository created.
func repoEvents(nodes []repoContributionNode) []models.Event {
	var events []models.Event
	for _, n := range nodes {
		events = append(events, models.Event{
			ID:          fmt.Sprintf("gql-repo-created-%s", n.Repository.NameWithOwner),
			Type:        "CreateEvent",
			Repo:        n.Repository.NameWithOwner,
			RepoPrivate: n.Repository.IsPrivate,
			RepoID:      n.Repository.DatabaseID,
			CreatedAt:   n.OccurredAt,
			Description: n.Repository.Description,
		})
	}
	return events
}

// inReport reports whether t falls on one of report's days, in report's zone.
func inReport(t time.Time, report daterange.DateRange) bool {
	return !t.Before(report.StartTime()) && t.Before(report.EndDateTime())
}

// calendarDays returns the contribution calendar's days from resp. A day whose
// date does not parse is skipped rather than failing the whole fetch.
func calendarDays(resp contributionsResponse) []models.ContributionDay {
	var days []models.ContributionDay
	for _, week := range resp.User.ContributionsCollection.ContributionCalendar.Weeks {
		for _, day := range week.ContributionDays {
			parsed, err := time.Parse(time.DateOnly, day.Date)
			if err != nil {
				continue
			}
			days = append(days, models.ContributionDay{
				Date:              parsed,
				ContributionCount: day.ContributionCount,
			})
		}
	}
	return days
}
