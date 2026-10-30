package github_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gogithub "github.com/google/go-github/v90/github"
	"go.uber.org/mock/gomock"

	svcgithub "github.com/Flashgap/marvin/internal/service/github"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
)

var _ = Describe("AddLabel", func() {
	const prNumber = 12

	var (
		mockCtrl   *gomock.Controller
		mockClient *mock_github.MockClient
		svc        svcgithub.Service
		event      *gogithub.PullRequestEvent
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mock_github.NewMockClient(mockCtrl)
		svc = svcgithub.NewService(mockClient, nil)
		event = &gogithub.PullRequestEvent{}
	})

	label := func(name string) *gogithub.Label {
		return &gogithub.Label{Name: name}
	}

	It("adds the first label starting with the given name when matching by prefix", func(ctx SpecContext) {
		mockClient.EXPECT().ListLabels(gomock.Any(), event, gomock.Any()).
			Return([]*gogithub.Label{label("good first issue"), label("go")}, nil, nil)
		mockClient.EXPECT().AddPRLabels(gomock.Any(), event, prNumber, []string{"good first issue"}).Return(nil, nil, nil)

		Expect(svc.AddLabel(ctx, event, prNumber, "go", svcgithub.LabelMatchPrefix)).To(Succeed())
	})

	It("only adds the label with exactly the given name, case-insensitively, when matching exactly", func(ctx SpecContext) {
		mockClient.EXPECT().ListLabels(gomock.Any(), event, gomock.Any()).
			Return([]*gogithub.Label{label("good first issue"), label("Go")}, nil, nil)
		mockClient.EXPECT().AddPRLabels(gomock.Any(), event, prNumber, []string{"Go"}).Return(nil, nil, nil)

		Expect(svc.AddLabel(ctx, event, prNumber, "go", svcgithub.LabelMatchExact)).To(Succeed())
	})

	It("looks for the label on every page of the repository labels", func(ctx SpecContext) {
		first := mockClient.EXPECT().ListLabels(gomock.Any(), event, &gogithub.ListOptions{Page: 1, PerPage: 100}).
			Return([]*gogithub.Label{label("good first issue")}, &gogithub.Response{NextPage: 2}, nil)
		mockClient.EXPECT().ListLabels(gomock.Any(), event, &gogithub.ListOptions{Page: 2, PerPage: 100}).
			Return([]*gogithub.Label{label("go")}, &gogithub.Response{}, nil).
			After(first)
		mockClient.EXPECT().AddPRLabels(gomock.Any(), event, prNumber, []string{"go"}).Return(nil, nil, nil)

		Expect(svc.AddLabel(ctx, event, prNumber, "go", svcgithub.LabelMatchExact)).To(Succeed())
	})

	It("returns ErrLabelNotFound when no label matches exactly", func(ctx SpecContext) {
		mockClient.EXPECT().ListLabels(gomock.Any(), event, gomock.Any()).
			Return([]*gogithub.Label{label("good first issue")}, nil, nil)

		Expect(svc.AddLabel(ctx, event, prNumber, "go", svcgithub.LabelMatchExact)).To(MatchError(svcgithub.ErrLabelNotFound))
	})
})
