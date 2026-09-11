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

	// Pull request merge states, as reported by the mergeable_state field. GitHub computes them
	// lazily and only populates them on a single-PR GET, never on a list or a webhook payload.
	MergeableStateBehind   = "behind"
	MergeableStateBlocked  = "blocked"
	MergeableStateClean    = "clean"
	MergeableStateDirty    = "dirty"
	MergeableStateDraft    = "draft"
	MergeableStateHasHooks = "has_hooks"
	MergeableStateUnknown  = "unknown"
	MergeableStateUnstable = "unstable"

	// MaxPerPage is the max number of results per page
	MaxPerPage = 100
)
