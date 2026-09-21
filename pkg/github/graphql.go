package github

import (
	"context"
	"fmt"

	"github.com/shurcooL/graphql"
)

// OpenPRReviewLoad holds, for one open pull request, its size and the logins of everyone who has
// either reviewed it or is currently requested to review it.
type OpenPRReviewLoad struct {
	Number    int
	Additions int
	Reviewers map[string]struct{}
}

// userLogin is embedded (not named) in query structs so the shurcooL/graphql decoder recognizes it
// as an inline fragment's field set rather than a regular named field: a struct field tagged
// `graphql:"... on X"` is deliberately unmatchable by name during decoding, its fields are matched
// instead. See shurcooL/graphql internal/jsonutil.hasGraphQLName.
type userLogin struct {
	Login graphql.String
}

type openPRsWithReviewersQuery struct {
	Repository struct {
		PullRequests struct {
			Nodes []struct {
				Number    graphql.Int
				Additions graphql.Int
				Reviews   struct {
					Nodes []struct {
						Author struct {
							Login graphql.String
						}
					}
				} `graphql:"reviews(first: 100)"`
				ReviewRequests struct {
					Nodes []struct {
						RequestedReviewer struct {
							userLogin `graphql:"... on User"`
						}
					}
				} `graphql:"reviewRequests(first: 100)"`
			}
		} `graphql:"pullRequests(states: OPEN, first: 100)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

// ListOpenPRsWithReviewers returns, for every open PR of the repository, its
// size (additions) and the set of logins who have reviewed it or are requested to review it.
func (h *client) ListOpenPRsWithReviewers(ctx context.Context, webhook RepoSenderGetter) ([]OpenPRReviewLoad, error) {
	var query openPRsWithReviewersQuery
	variables := map[string]any{
		"owner": graphql.String(webhook.GetRepo().GetOwner().GetLogin()),
		"name":  graphql.String(webhook.GetRepo().GetName()),
	}

	if err := h.graphql.Query(ctx, &query, variables); err != nil {
		return nil, fmt.Errorf("error performing graphQL query: %w", err)
	}

	nodes := query.Repository.PullRequests.Nodes
	prs := make([]OpenPRReviewLoad, 0, len(nodes))
	for _, node := range nodes {
		reviewers := make(map[string]struct{})
		for _, review := range node.Reviews.Nodes {
			if login := string(review.Author.Login); login != "" {
				reviewers[login] = struct{}{}
			}
		}
		for _, reviewRequest := range node.ReviewRequests.Nodes {
			if login := string(reviewRequest.RequestedReviewer.Login); login != "" {
				reviewers[login] = struct{}{}
			}
		}

		prs = append(prs, OpenPRReviewLoad{
			Number:    int(node.Number),
			Additions: int(node.Additions),
			Reviewers: reviewers,
		})
	}

	return prs, nil
}
