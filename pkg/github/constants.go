package github

const (
	EventPullRequestActionClosed           = "closed"
	EventPullRequestActionConvertedToDraft = "converted_to_draft"
	EventPullRequestActionEdited           = "edited"
	EventPullRequestActionLabeled          = "labeled"
	EventPullRequestActionOpened           = "opened"
	EventPullRequestActionReadyForReview   = "ready_for_review"
	EventPullRequestActionReopened         = "reopened"
	EventPullRequestActionReviewRequested  = "review_requested"
	EventPullRequestActionSynchronize      = "synchronize"
	EventPullRequestActionUnlabeled        = "unlabeled"
	PullRequestStateClosed                 = "closed"

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
