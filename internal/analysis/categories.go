package analysis

import "github.com/ivuorinen/gh-history/internal/models"

// EventCategories maps GitHub event types to categories.
var EventCategories = map[string]models.Category{
	"PushEvent":                      models.CategoryCommits,
	"PullRequestEvent":               models.CategoryPullRequests,
	"PullRequestReviewEvent":         models.CategoryReviews,
	"PullRequestReviewCommentEvent":  models.CategoryComments,
	"IssuesEvent":                    models.CategoryIssues,
	"IssueCommentEvent":              models.CategoryComments,
	"CreateEvent":                    models.CategoryRepos,
	"DeleteEvent":                    models.CategoryRepos,
	"ForkEvent":                      models.CategoryRepos,
	"WatchEvent":                     models.CategoryRepos,
	"ReleaseEvent":                   models.CategoryReleases,
	"CommitCommentEvent":             models.CategoryComments,
	"GollumEvent":                    models.CategoryOther,
	"MemberEvent":                    models.CategoryOther,
	"PublicEvent":                    models.CategoryOther,
	"SponsorshipEvent":               models.CategoryOther,
	"PullRequestReviewThreadEvent":   models.CategoryReviews,
	"RepositoryEvent":                models.CategoryRepos,
	"StarEvent":                      models.CategoryRepos,
	"TeamAddEvent":                   models.CategoryOther,
	"OrgBlockEvent":                  models.CategoryOther,
	"ProjectCardEvent":               models.CategoryOther,
	"ProjectColumnEvent":             models.CategoryOther,
	"ProjectEvent":                   models.CategoryOther,
}

// CategoryLabels provides human-readable labels for categories.
var CategoryLabels = map[models.Category]string{
	models.CategoryCommits:      "Commits",
	models.CategoryPullRequests: "Pull Requests",
	models.CategoryIssues:       "Issues",
	models.CategoryReviews:      "Code Reviews",
	models.CategoryComments:     "Comments",
	models.CategoryRepos:        "Repository Actions",
	models.CategoryReleases:     "Releases",
	models.CategoryOther:        "Other",
}

// CategorizeEvent returns the category for an event type.
func CategorizeEvent(eventType string) models.Category {
	if cat, ok := EventCategories[eventType]; ok {
		return cat
	}
	return models.CategoryOther
}
