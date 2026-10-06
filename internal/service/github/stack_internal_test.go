package github

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/pkg/github"
)

var _ = Describe("nearestLayerReviewers", func() {
	everyone := func(string) bool { return true }

	DescribeTable("walks the stack from the PR's layer, down first then up",
		func(layers []github.StackLayer, prNumber int, eligible func(string) bool, expectedReviewers []string, expectedPR int) {
			reviewers, fromPR := nearestLayerReviewers(layers, prNumber, eligible)
			Expect(reviewers).To(Equal(expectedReviewers))
			Expect(fromPR).To(Equal(expectedPR))
		},
		Entry("takes the layer right below, not an older one",
			[]github.StackLayer{{Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2, Reviewers: []string{"clem"}}, {Number: 3, Position: 3}},
			3, everyone, []string{"clem"}, 2),
		Entry("skips layers without eligible reviewers",
			[]github.StackLayer{{Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2, Reviewers: []string{"bot"}}, {Number: 3, Position: 3}},
			3, func(login string) bool { return login != "bot" }, []string{"maxime"}, 1),
		Entry("falls back to the nearest layer above when none below has reviewers",
			[]github.StackLayer{{Number: 1, Position: 1}, {Number: 2, Position: 2}, {Number: 3, Position: 3, Reviewers: []string{"alice"}}, {Number: 4, Position: 4, Reviewers: []string{"bob"}}},
			2, everyone, []string{"alice"}, 3),
		Entry("prefers a layer below over a closer one above",
			[]github.StackLayer{{Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2}, {Number: 3, Position: 3}, {Number: 4, Position: 4, Reviewers: []string{"clem"}}},
			3, everyone, []string{"maxime"}, 1),
		Entry("does not depend on the order layers are listed in",
			[]github.StackLayer{{Number: 3, Position: 3}, {Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2, Reviewers: []string{"clem"}}},
			3, everyone, []string{"clem"}, 2),
		Entry("never returns the PR's own reviewers",
			[]github.StackLayer{{Number: 1, Position: 1}, {Number: 2, Position: 2, Reviewers: []string{"maxime"}}},
			2, everyone, nil, 0),
		Entry("returns nothing when the PR is not in the stack",
			[]github.StackLayer{{Number: 1, Position: 1, Reviewers: []string{"maxime"}}, {Number: 2, Position: 2}},
			3, everyone, nil, 0),
	)
})
