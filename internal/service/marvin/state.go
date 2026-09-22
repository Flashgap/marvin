package marvin

import (
	"context"
	"errors"

	"github.com/Flashgap/marvin/internal/service/github"
	"github.com/Flashgap/marvin/internal/service/marvin/prstate"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
)

// stateLabels maps each review-lifecycle state to its GitHub label.
// prstate.None has no label: it's the absence of any lifecycle label.
var stateLabels = map[prstate.State]string{
	prstate.WorkInProgress:  github.LabelWorkInProgress,
	prstate.ReadyForReview:  github.LabelReadyForReview,
	prstate.ChangesRequired: github.LabelChangesRequired,
	prstate.Approved:        github.LabelApproved,
}

// enterState makes to's label present and removes each label in clear, unconditionally.
//
// This never inspects the PR's actual current labels: webhook payloads (especially the PR
// embedded in a pull_request_review event) aren't reliably populated with the full label list,
// so callers pass the specific labels the prior state could plausibly be under per
// prstate.Transition's event table. AddLabel/RemoveLabel are idempotent (a redundant add or an
// already-absent remove is a no-op against GitHub's API), so calling this blind is safe.
func (s *service) enterState(ctx context.Context, webhook pkggithub.RepoSenderGetter, prNumber int, to prstate.State, clear ...prstate.State) error {
	var errs error
	for _, from := range clear {
		if label, ok := stateLabels[from]; ok {
			if err := s.githubService.RemoveLabel(ctx, webhook, prNumber, label); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	if label, ok := stateLabels[to]; ok {
		if err := s.githubService.AddLabel(ctx, webhook, prNumber, label); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}
