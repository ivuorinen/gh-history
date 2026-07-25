package analysis

import "github.com/ivuorinen/gh-history/internal/models"

// EventCategories maps GitHub event types to categories.
//
// This lists exactly the event types api.FetchContributions and
// api.FetchIssueComments synthesize. The GraphQL contributionsCollection is the
// only data source, so entries for REST Events API types (PushEvent, ForkEvent,
// ReleaseEvent, WatchEvent, …) would be unreachable — CategorizeEvent maps
// anything unlisted to CategoryOther.
var EventCategories = map[string]models.Category{
	"PullRequestEvent":       models.CategoryPullRequests,
	"PullRequestReviewEvent": models.CategoryReviews,
	"IssuesEvent":            models.CategoryIssues,
	"IssueCommentEvent":      models.CategoryComments,
	"CreateEvent":            models.CategoryRepos,
}

// CategoryLabels provides human-readable labels for categories.
var CategoryLabels = map[models.Category]string{
	models.CategoryPullRequests: "Pull Requests",
	models.CategoryIssues:       "Issues",
	models.CategoryReviews:      "Code Reviews",
	models.CategoryComments:     "Comments",
	models.CategoryRepos:        "Repository Actions",
	models.CategoryOther:        "Other",
}

// CategorizeEvent returns the category for an event type.
func CategorizeEvent(eventType string) models.Category {
	if cat, ok := EventCategories[eventType]; ok {
		return cat
	}
	return models.CategoryOther
}
