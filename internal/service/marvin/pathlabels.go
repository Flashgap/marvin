package marvin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	gogithub "github.com/google/go-github/v90/github"

	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/internal/service/github"
	pkggithub "github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/utils"
)

const commentMissingPathLabels = "Hey, I couldn't add the %s label(s) matching the files changed in this PR: they don't exist in this repository. Please create them, or fix the `reviewers.rules` labels in %s."

// applyPathLabels adds to the PR the label of every reviewers rule matching a changed file, or
// reviewers.default_label when none does. Labels missing from the repository are reported in a PR
// comment unless silent. Labels are only ever added: one stays when a later push stops touching its
// path, since Marvin can't tell it apart from a label added by hand.
func (s *service) applyPathLabels(ctx context.Context, webhook pkggithub.RepoSenderGetter, pr *gogithub.PullRequest, config *GitHubRepositoryConfiguration, silent bool) error {
	log := middlewares.LoggerFromGHContext(ctx, "marvin.applyPathLabels")

	labels, err := s.resolvePathLabels(ctx, webhook, pr.GetNumber(), config)
	if err != nil {
		return fmt.Errorf("cannot resolve path labels: %w", err)
	}

	var missing []string
	var errs error
	for _, label := range labels {
		if slices.ContainsFunc(pr.Labels, func(prLabel *gogithub.Label) bool { return strings.EqualFold(prLabel.GetName(), label) }) {
			continue
		}

		err := s.githubService.AddLabel(ctx, webhook, pr.GetNumber(), label, github.LabelMatchExact)
		if errors.Is(err, github.ErrLabelNotFound) {
			missing = append(missing, fmt.Sprintf("%#q", label))
			continue
		}
		errs = errors.Join(errs, err)
	}

	if len(missing) > 0 {
		log.Warnf("path labels missing from the repository: %v", missing)
		if !silent {
			if _, _, err := s.githubService.CreatePRComment(ctx, webhook, pr.GetNumber(), &gogithub.IssueComment{
				Body: utils.Ptr(fmt.Sprintf(commentMissingPathLabels, strings.Join(missing, ", "), RepoConfigFileName)),
			}); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}

	return errs
}
