package marvin_test

import (
	"net/http"

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
)

func buildStandupTest(ctx SpecContext, cfg marvin.Config) *testenv.HTTPEnv {
	mockCtrl := gomock.NewController(GinkgoT())
	services := &marvin.Services{
		MarvinService: mock_marvin.NewMockService(mockCtrl),
		GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
		JiraService:   mock_jira.NewMockService(mockCtrl),
	}
	server, err := marvin.NewServer(ctx, &cfg, services)
	Expect(err).ToNot(HaveOccurred())
	return testenv.NewHTTPEnv(server.Handler)
}

var _ = Describe("POST /marvin/_task/standup/remind", func() {
	path := marvinroute.Paths.Tasks + "/standup/remind"

	It("answers 200 without doing anything yet", func(ctx SpecContext) {
		// Dev leaves task authentication off: the middleware's own tests cover it.
		cfg := marvin.Config{}
		cfg.IsDevEnv = true

		env := buildStandupTest(ctx, cfg)
		env.ServeHTTPRequest(http.MethodPost, path, nil, nil, http.StatusOK)
	})

	It("returns 401 without a task token", func(ctx SpecContext) {
		cfg := marvin.Config{}
		cfg.TasksAudience = "https://marvin.example.com"
		cfg.TasksServiceAccount = "marvin-trigger@project.iam.gserviceaccount.com"

		env := buildStandupTest(ctx, cfg)
		env.ServeHTTPRequest(http.MethodPost, path, nil, nil, http.StatusUnauthorized)
	})
})
