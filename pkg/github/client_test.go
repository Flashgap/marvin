package github_test

import (
	"testing"

	gogithub "github.com/google/go-github/v63/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/pkg/github"
	"github.com/Flashgap/marvin/pkg/utils"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Github pkg test suite")
}

var _ = Describe("Github", func() {
	Context("IsLabelInList", func() {
		var labels = []*gogithub.Label{
			{
				Name: utils.Ptr("Merge 🚀"),
			},
			{
				Name: utils.Ptr("Ready for review 👌"),
			},
			{
				Name: utils.Ptr("Work in progress ⏳"),
			},
		}
		DescribeTable("table", func(label string, labels []*gogithub.Label, isIn bool) {
			output := github.IsLabelInList(labels, label)
			Expect(output).To(Equal(isIn))
		},
			Entry("it should have the label", "merge", labels, true),
			Entry("it should not have the label", "in progress", labels, false),
		)
	})
})
