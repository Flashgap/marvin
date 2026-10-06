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

// nearestLayerReviewers walks the stack away from prNumber's layer, first down towards the bottom then up,
// and returns the reviewers of the first layer that has any among candidates. It returns nil when prNumber
// isn't part of layers or no other layer has a candidate reviewer.
func nearestLayerReviewers(layers []github.StackLayer, prNumber int, candidates map[string]struct{}) []string {
	byPosition := make(map[int]github.StackLayer, len(layers))
	current, top := 0, 0
	for _, layer := range layers {
		byPosition[layer.Position] = layer
		top = max(top, layer.Position)
		if layer.Number == prNumber {
			current = layer.Position
		}
	}
	if current == 0 {
		return nil
	}

	order := make([]int, 0, len(layers))
	for position := current - 1; position >= 1; position-- {
		order = append(order, position)
	}
	for position := current + 1; position <= top; position++ {
		order = append(order, position)
	}

	for _, position := range order {
		layer := byPosition[position]
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
