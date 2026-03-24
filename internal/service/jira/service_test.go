package jira_test

import (
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/service/jira"
	pkgjira "github.com/Flashgap/marvin/pkg/jira"
	mock_jira "github.com/Flashgap/marvin/pkg/jira/mock"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jira service test suite")
}

var _ = Describe("Service tests", func() {
	var mockCtrl *gomock.Controller
	var jiraService jira.Service
	var mockables *mockableServices

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		jiraService, mockables = newService(mockCtrl)
	})

	When("issue already exist in Jira", func() {
		It("should error if we have multiple matches for the issue", func(ctx SpecContext) {
			mockables.jiraClient.EXPECT().SearchIssue(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pkgjira.SearchResponse{
				Total: 2,
				Issues: []pkgjira.Issue{
					{ID: uuid.NewString()},
					{ID: uuid.NewString()},
				},
			}, nil).Times(1)

			Expect(jiraService.DoCapReportWorkflow(ctx, &jira.Issue{}, 1)).ToNot(Succeed())
		})

		It("should update the time spent and return", func(ctx SpecContext) {
			mockables.jiraClient.EXPECT().SearchIssue(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pkgjira.SearchResponse{
				Total: 1,
				Issues: []pkgjira.Issue{
					{ID: uuid.NewString()},
				},
			}, nil).Times(1)
			mockables.jiraClient.EXPECT().AddIssueWorklog(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

			Expect(jiraService.DoCapReportWorkflow(ctx, &jira.Issue{}, 1)).To(Succeed())
		})
	})

	When("issue does not exist in Jira", func() {
		When("epic exists in Jira", func() {
			It("should create the issue", func(ctx SpecContext) {
				mockables.jiraClient.EXPECT().SearchIssue(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pkgjira.SearchResponse{
					Total:  0,
					Issues: nil,
				}, nil).Times(1) // Issue search
				mockables.jiraClient.EXPECT().SearchIssue(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pkgjira.SearchResponse{
					Total: 1,
					Issues: []pkgjira.Issue{
						{ID: uuid.NewString()},
					},
				}, nil).Times(1) // Epic search
				mockables.jiraClient.EXPECT().CreateIssue(gomock.Any(), gomock.Any()).Return(uuid.NewString(), nil).Times(1)
				mockables.jiraClient.EXPECT().TransitionIssue(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
				mockables.jiraClient.EXPECT().AddIssueWorklog(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				Expect(jiraService.DoCapReportWorkflow(ctx, &jira.Issue{}, 1)).To(Succeed())
			})
		})

		When("epic does not exist in Jira", func() {
			It("should create the epic and issue", func(ctx SpecContext) {
				mockables.jiraClient.EXPECT().SearchIssue(gomock.Any(), gomock.Any(), gomock.Any()).Return(&pkgjira.SearchResponse{
					Total:  0,
					Issues: nil,
				}, nil).Times(2) // Issue and Epic search
				mockables.jiraClient.EXPECT().CreateIssue(gomock.Any(), gomock.Any()).Return(uuid.NewString(), nil).Times(2)
				mockables.jiraClient.EXPECT().TransitionIssue(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
				mockables.jiraClient.EXPECT().AddIssueWorklog(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

				Expect(jiraService.DoCapReportWorkflow(ctx, &jira.Issue{}, 2)).To(Succeed())
			})
		})
	})
})

type mockableServices struct {
	jiraClient *mock_jira.MockClient
}

func newService(mockCtrl *gomock.Controller) (jira.Service, *mockableServices) {
	jiraClient := mock_jira.NewMockClient(mockCtrl)
	jiraService, err := jira.NewService(&config.Jira{
		JiraFields: map[string]string{
			"ProjectKey":              "",
			"ProjectID":               "",
			"TaskIssueTypeID":         "",
			"EpicIssueTypeID":         "",
			"StartDateCustomFieldKey": "",
			"InProgressTransitionID":  "",
			"DoneTransitionID":        "",
		},
	}, jiraClient)
	Expect(err).ToNot(HaveOccurred())

	return jiraService, &mockableServices{jiraClient: jiraClient}
}
