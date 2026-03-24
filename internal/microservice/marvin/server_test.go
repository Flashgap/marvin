package marvin_test

import (
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/microservice/marvin"
	marvinroute "github.com/Flashgap/marvin/internal/route/marvin"
	"github.com/Flashgap/marvin/internal/service/github"
	mock_jira "github.com/Flashgap/marvin/internal/service/jira/mock"
	mock_marvin "github.com/Flashgap/marvin/internal/service/marvin/mock"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/testenv"
	"github.com/Flashgap/marvin/pkg/utils"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Marvin usvc test suite")
}

var (
	prHeaders = utils.KV{
		"Content-Type":   "application/json",
		"X-GitHub-Event": "pull_request",
	}
	checkHeaders = utils.KV{
		"Content-Type":   "application/json",
		"X-GitHub-Event": "check_run",
	}
)

const (
	// GH sdk doesn't validate payloads
	// it only needs to be valid json to be Unmarshaled
	prPayload    = `{}`
	checkPayload = `{}`
)

type marvinTest struct {
	httpEnv    *testenv.HTTPEnv
	marvinMock *mock_marvin.MockService
}

func initMarvinTest(ctx SpecContext) marvinTest {
	cfg := marvin.Config{}
	mockCtrl := gomock.NewController(GinkgoT())
	marvinMock := mock_marvin.NewMockService(mockCtrl)
	services := &marvin.Services{
		MarvinService: marvinMock,
		GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
		JiraService:   mock_jira.NewMockService(mockCtrl),
	}
	server, err := marvin.NewServer(ctx, &cfg, services)
	Expect(err).ToNot(HaveOccurred())

	return marvinTest{
		httpEnv:    testenv.NewHTTPEnv(server.Handler),
		marvinMock: marvinMock,
	}
}

var _ = Describe("Marvin api", func() {
	Context("endpoints", func() {
		Context("POST at /github/webhook", func() {
			Context("OnPullRequest", func() {
				It("should call marvin service for pull_request event", func(ctx SpecContext) {
					deps := initMarvinTest(ctx)
					deps.marvinMock.EXPECT().OnPullRequest(gomock.Any(), gomock.Any()).Return(nil).Times(1)
					deps.marvinMock.EXPECT().OnCheckRun(gomock.Any(), gomock.Any()).Return(nil).Times(0)

					deps.httpEnv.ServeHTTPRequest(
						http.MethodPost, marvinroute.Paths.WebHooks+"/github/webhook",
						prHeaders,
						prPayload,
						http.StatusOK)
				})

				It("it should not call marvin service for check_run event", func(ctx SpecContext) {
					deps := initMarvinTest(ctx)
					deps.marvinMock.EXPECT().OnPullRequest(gomock.Any(), gomock.Any()).Return(nil).Times(0)
					deps.marvinMock.EXPECT().OnCheckRun(gomock.Any(), gomock.Any()).AnyTimes()

					deps.httpEnv.ServeHTTPRequest(
						http.MethodPost, marvinroute.Paths.WebHooks+"/github/webhook",
						checkHeaders,
						checkPayload,
						http.StatusOK)
				})
			})

			Context("OnCheckRun", func() {
				It("should call marvin service for check_run event", func(ctx SpecContext) {
					deps := initMarvinTest(ctx)
					deps.marvinMock.EXPECT().OnCheckRun(gomock.Any(), gomock.Any()).Return(nil).Times(1)
					deps.marvinMock.EXPECT().OnPullRequest(gomock.Any(), gomock.Any()).Return(nil).Times(0)

					deps.httpEnv.ServeHTTPRequest(
						http.MethodPost, marvinroute.Paths.WebHooks+"/github/webhook",
						checkHeaders,
						prPayload,
						http.StatusOK)
				})

				It("it should not call marvin service for pull_request event", func(ctx SpecContext) {
					deps := initMarvinTest(ctx)
					deps.marvinMock.EXPECT().OnCheckRun(gomock.Any(), gomock.Any()).Return(nil).Times(0)
					deps.marvinMock.EXPECT().OnPullRequest(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

					deps.httpEnv.ServeHTTPRequest(
						http.MethodPost, marvinroute.Paths.WebHooks+"/github/webhook",
						prHeaders,
						prPayload,
						http.StatusOK)
				})
			})
		})
	})
})
