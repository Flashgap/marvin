package github

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/pkg/github"
)

var _ = Describe("nearestLayerReviewers", func() {
	candidates := map[string]struct{}{"maxime": {}, "clem": {}, "alice": {}, "bob": {}}

	DescribeTable("walks the stack from the PR's layer, down first then up",
		func(layers []github.StackLayer, prNumber int, expectedReviewers []string) {
			Expect(nearestLayerReviewers(layers, prNumber, candidates)).To(Equal(expectedReviewers))
		},
		Entry("takes the layer right below, not an older one",
			[]github.StackLayer{{Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2, Reviewers: []string{"clem"}}, {Number: 3, Position: 3}},
			3, []string{"clem"}),
		Entry("skips layers without candidate reviewers",
			[]github.StackLayer{{Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2, Reviewers: []string{"bot"}}, {Number: 3, Position: 3}},
			3, []string{"maxime"}),
		Entry("falls back to the nearest layer above when none below has reviewers",
			[]github.StackLayer{{Number: 1, Position: 1}, {Number: 2, Position: 2}, {Number: 3, Position: 3, Reviewers: []string{"alice"}}, {Number: 4, Position: 4, Reviewers: []string{"bob"}}},
			2, []string{"alice"}),
		Entry("prefers a layer below over a closer one above",
			[]github.StackLayer{{Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2}, {Number: 3, Position: 3}, {Number: 4, Position: 4, Reviewers: []string{"clem"}}},
			3, []string{"maxime"}),
		Entry("does not depend on the order layers are listed in",
			[]github.StackLayer{{Number: 3, Position: 3}, {Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2, Reviewers: []string{"clem"}}},
			3, []string{"clem"}),
		Entry("never returns the PR's own reviewers",
			[]github.StackLayer{{Number: 1, Position: 1}, {Number: 2, Position: 2, Reviewers: []string{"maxime"}}},
			2, nil),
		Entry("returns nothing when the PR is not in the stack",
			[]github.StackLayer{{Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2}},
			3, nil),
	)
})
