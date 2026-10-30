package github

const (
	LabelApproved        = "action:Approved"
	LabelMerge           = "action:Merge"
	LabelReadyForReview  = "action:Ready for review"
	LabelWorkInProgress  = "action:Work in progress"
	LabelDependencies    = "action:dependencies"
	LabelHotfix          = "action:Hotfix"
	LabelChangesRequired = "action:Changes required"
	LabelDoNotMerge      = "action:DO NOT MERGE"
	LabelExtraCareful    = "action:Extra careful"
)

// LabelMatch tells AddLabel how to find the repository label matching the given name.
type LabelMatch int

const (
	// LabelMatchPrefix matches the first repository label starting with the given name,
	// case-insensitively. Used for Marvin's own "action:*" labels.
	LabelMatchPrefix LabelMatch = iota
	// LabelMatchExact only matches a repository label with exactly the given name, case-insensitively.
	LabelMatchExact
)
