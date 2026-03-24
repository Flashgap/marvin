package github

const (
	EventPullRequestActionClosed          = "closed"
	EventPullRequestActionEdited          = "edited"
	EventPullRequestActionLabeled         = "labeled"
	EventPullRequestActionOpened          = "opened"
	EventPullRequestActionReopened        = "reopened"
	EventPullRequestActionReviewRequested = "review_requested"
	EventPullRequestActionSynchronize     = "synchronize"
	EventPullRequestActionUnlabeled       = "unlabeled"
	PullRequestStateClosed                = "closed"

	EventCheckRunActionCompleted     = "completed"
	CheckRunStatusCompleted          = "completed"
	CheckRunConclusionActionRequired = "action_required"
	CheckRunConclusionSuccess        = "success"

	EventPullRequestReviewActionSubmitted  = "submitted"
	PullRequestReviewStateApproved         = "approved"
	PullRequestReviewStateChangesRequested = "changes_requested"

	// MaxPerPage is the max number of results per page
	MaxPerPage = 100
)
