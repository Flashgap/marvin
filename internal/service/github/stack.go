package github

import (
	"context"
	"slices"

	gogithub "github.com/google/go-github/v90/github"

	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/pkg/github"
)

// reviewersFromStack returns the candidates reviewing the layer of pr's stack closest to it, so a stacked PR
// keeps the reviewers who already have the context of the layers around it. Fetching the stack is best
// effort: stacked PRs are a GitHub preview feature, and failing to read one must never block assignment.
func (s *service) reviewersFromStack(ctx context.Context, webhook github.RepoSenderGetter, pr *gogithub.PullRequest, candidates map[string]struct{}) []string {
	log := middlewares.LoggerFromGHContext(ctx, "github.reviewersFromStack")

	layers, err := s.ListStackLayers(ctx, webhook, pr.GetNumber())
	if err != nil {
		log.Warnf("cannot list the layers of stack #%d, falling back to review load: %v", pr.GetStack().GetNumber(), err)
		return nil
	}

	reviewers := nearestLayerReviewers(layers, pr.GetNumber(), candidates)
	if len(reviewers) > 0 {
		log.Infof("PR is in stack #%d, keeping the reviewers of its nearest layer: %v", pr.GetStack().GetNumber(), reviewers)
	}

	return reviewers
}

// nearestLayerReviewers returns the candidates reviewing the layer closest to prNumber's in its stack: the
// layers below first, nearest first, then the layers above. It returns nil when prNumber isn't part of
// layers or no other layer has a candidate reviewer.
func nearestLayerReviewers(layers []github.StackLayer, prNumber int, candidates map[string]struct{}) []string {
	layers = slices.SortedFunc(slices.Values(layers), func(a, b github.StackLayer) int { return a.Position - b.Position })
	current := slices.IndexFunc(layers, func(layer github.StackLayer) bool { return layer.Number == prNumber })
	if current < 0 {
		return nil
	}

	below := layers[:current]
	slices.Reverse(below) // layers is our own sorted copy, the caller's slice is untouched
	for _, layer := range slices.Concat(below, layers[current+1:]) {
		var reviewers []string
		for _, login := range layer.Reviewers {
			if _, ok := candidates[login]; ok {
				reviewers = append(reviewers, login)
			}
		}
		if len(reviewers) > 0 {
			return reviewers
		}
	}

	return nil
}

// preferredFirst moves preferred logins to the front of ranked, keeping the relative order of both groups.
// Ranked by review load, this picks the least loaded preferred reviewers first.
func preferredFirst(ranked, preferred []string) []string {
	result := make([]string, 0, len(ranked))
	for _, login := range ranked {
		if slices.Contains(preferred, login) {
			result = append(result, login)
		}
	}
	for _, login := range ranked {
		if !slices.Contains(preferred, login) {
			result = append(result, login)
		}
	}

	return result
}
