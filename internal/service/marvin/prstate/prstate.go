// Package prstate implements the pull request review-lifecycle state machine.
//
// A PR's lifecycle label (Work in progress / Ready for review / Changes
// required / Approved) is exactly one of a small set of mutually exclusive
// states. Transition is the single source of truth for how webhook events
// move that state, so every caller applies the same rules instead of each
// hand-rolling its own add/remove label pair.
//
// Labels that aren't part of the review lifecycle (Hotfix, DO NOT MERGE,
// dependencies, Merge) are not modeled here: they're independent flags a
// caller reads as a guard, not states a PR moves through.
package prstate

// State is a PR's position in the review lifecycle.
type State int

const (
	None State = iota
	WorkInProgress
	ReadyForReview
	ChangesRequired
	Approved
)

// Event is something that happened to a PR that may move its State.
type Event int

const (
	// OpenedDraft fires when a PR is opened or reopened as a draft.
	OpenedDraft Event = iota
	// ConvertedToDraft fires when a PR is converted back to draft.
	ConvertedToDraft
	// ReadyRequested fires both when GitHub's native "ready for review"
	// action happens and when the Ready for review label is added manually.
	ReadyRequested
	// ReviewChangesRequested fires when a review requests changes.
	ReviewChangesRequested
	// ReviewApproved fires when a review approves the PR.
	ReviewApproved
)

// Guards carries the single precondition each event needs checked before its
// transition is allowed to fire (a repo config flag, the AI review gate,
// enough approvals, ...). It's meaningless for ungated rules.
type Guards struct {
	Allowed bool
}

type rule struct {
	to State // target state when unconditional, or when gated and Guards.Allowed is true

	gated   bool
	onBlock State // target state when gated and Guards.Allowed is false
	blockOK bool  // whether landing on onBlock counts as a transition (ok=true) or a no-op (ok=false)
}

var table = map[State]map[Event]rule{
	None: {
		OpenedDraft:      {to: WorkInProgress},
		ConvertedToDraft: {to: WorkInProgress},
		ReadyRequested:   {gated: true, to: ReadyForReview, onBlock: WorkInProgress, blockOK: true},
	},
	WorkInProgress: {
		ReadyRequested: {gated: true, to: ReadyForReview, onBlock: WorkInProgress, blockOK: true},
	},
	ReadyForReview: {
		ConvertedToDraft:       {to: WorkInProgress},
		ReviewChangesRequested: {to: ChangesRequired},
		ReviewApproved:         {gated: true, to: Approved, blockOK: false},
	},
	ChangesRequired: {
		ConvertedToDraft: {to: WorkInProgress},
		ReadyRequested:   {gated: true, to: ReadyForReview, onBlock: WorkInProgress, blockOK: true},
		ReviewApproved:   {gated: true, to: Approved, blockOK: false},
	},
	Approved: {
		ConvertedToDraft:       {to: WorkInProgress},
		ReviewChangesRequested: {to: ChangesRequired},
	},
}

// Transition returns the state current moves to when event fires, and
// whether a transition actually happened. For a gated rule whose guard
// isn't satisfied, the result is either the rule's declared onBlock state
// (ok=true — something did happen, e.g. reverting to WorkInProgress) or a
// no-op returning current unchanged (ok=false), per blockOK.
func Transition(current State, event Event, guards Guards) (State, bool) {
	events, known := table[current]
	if !known {
		return current, false
	}

	r, known := events[event]
	if !known {
		return current, false
	}

	if !r.gated {
		return r.to, true
	}

	if guards.Allowed {
		return r.to, true
	}

	if r.blockOK {
		return r.onBlock, true
	}

	return current, false
}
