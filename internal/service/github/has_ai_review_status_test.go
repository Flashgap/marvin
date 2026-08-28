package github_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gogithub "github.com/google/go-github/v90/github"
	"go.uber.org/mock/gomock"

	svcgithub "github.com/Flashgap/marvin/internal/service/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/utils"
)

var _ = Describe("HasAIReviewStatusSucceeded", func() {
	const (
		headSHA  = "b2112755"
		contexts = "CodeRabbit"
	)

	var (
		mockCtrl   *gomock.Controller
		mockClient *mock_github.MockClient
		svc        svcgithub.Service
		event      *gogithub.PullRequestEvent
	)

	status := func(context, state string) *gogithub.RepoStatus {
		return &gogithub.RepoStatus{Context: utils.Ptr(context), State: utils.Ptr(state)}
	}
	combined := func(statuses ...*gogithub.RepoStatus) *gogithub.CombinedStatus {
		return &gogithub.CombinedStatus{Statuses: statuses}
	}

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(mockCtrl)
		svc = svcgithub.NewService(mockClient)
		event = &gogithub.PullRequestEvent{
			Repo: &gogithub.Repository{Name: utils.Ptr("infra"), Owner: &gogithub.User{Login: utils.Ptr("hector-finance")}},
		}
	})

	AfterEach(func() { mockCtrl.Finish() })

	When("a matching context has a success status", func() {
		It("passes (case-insensitive context match)", func(ctx SpecContext) {
			mockClient.EXPECT().GetCombinedStatus(gomock.Any(), event, headSHA, gomock.Any()).
				Return(combined(status("scalr/hub/frontend", "success"), status("coderabbit", "success")), nil, nil)

			ok, err := svc.HasAIReviewStatusSucceeded(ctx, event, headSHA, []string{contexts})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})

	When("the matching context is present but not successful", func() {
		It("blocks", func(ctx SpecContext) {
			mockClient.EXPECT().GetCombinedStatus(gomock.Any(), event, headSHA, gomock.Any()).
				Return(combined(status("CodeRabbit", "pending")), nil, nil)

			ok, err := svc.HasAIReviewStatusSucceeded(ctx, event, headSHA, []string{contexts})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	When("no matching context is present", func() {
		It("blocks", func(ctx SpecContext) {
			mockClient.EXPECT().GetCombinedStatus(gomock.Any(), event, headSHA, gomock.Any()).
				Return(combined(status("scalr/hub/frontend", "success")), nil, nil)

			ok, err := svc.HasAIReviewStatusSucceeded(ctx, event, headSHA, []string{contexts})
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	When("no contexts are configured", func() {
		It("blocks without calling the API", func(ctx SpecContext) {
			// No GetCombinedStatus expectation: an empty context list must short-circuit.
			ok, err := svc.HasAIReviewStatusSucceeded(ctx, event, headSHA, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})
})
