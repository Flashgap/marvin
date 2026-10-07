package marvin_test

import (
	"encoding/json"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/microservice/marvin"
	marvinroute "github.com/Flashgap/marvin/internal/route/marvin"
	"github.com/Flashgap/marvin/internal/service/github"
	mock_jira "github.com/Flashgap/marvin/internal/service/jira/mock"
	mock_marvin "github.com/Flashgap/marvin/internal/service/marvin/mock"
	"github.com/Flashgap/marvin/internal/service/standup"
	mock_standup "github.com/Flashgap/marvin/internal/service/standup/mock"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/testenv"
)

func buildStandupTest(ctx SpecContext, cfg marvin.Config, standupSvc standup.Service) *testenv.HTTPEnv {
	mockCtrl := gomock.NewController(GinkgoT())
	services := &marvin.Services{
		MarvinService:  mock_marvin.NewMockService(mockCtrl),
		GithubService:  github.NewService(mock_github.NewMockClient(mockCtrl)),
		JiraService:    mock_jira.NewMockService(mockCtrl),
		StandupService: standupSvc,
	}
	server, err := marvin.NewServer(ctx, &cfg, services)
	Expect(err).ToNot(HaveOccurred())
	return testenv.NewHTTPEnv(server.Handler)
}

// devConfig bypasses the task secret check in tests.
func devConfig() marvin.Config {
	cfg := marvin.Config{}
	cfg.IsDevEnv = true
	return cfg
}

var _ = Describe("POST /marvin/_task/standup/remind", func() {
	path := marvinroute.Paths.Tasks + "/standup/remind"

	It("returns 501 when the standup service isn't wired in", func(ctx SpecContext) {
		env := buildStandupTest(ctx, devConfig(), nil)
		env.ServeHTTPRequest(http.MethodPost, path, nil, nil, http.StatusNotImplemented)
	})

	It("returns the report of the run", func(ctx SpecContext) {
		mockStandup := mock_standup.NewMockService(gomock.NewController(GinkgoT()))
		mockStandup.EXPECT().Remind(gomock.Any()).Return(&standup.Report{Members: 12, Reminded: 7, AlreadyPosted: 3, Ignored: 2}, nil)

		env := buildStandupTest(ctx, devConfig(), mockStandup)
		rec := env.ServeHTTPRequest(http.MethodPost, path, nil, nil, http.StatusOK)

		var body map[string]int
		Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
		Expect(body).To(Equal(map[string]int{"members": 12, "reminded": 7, "already_posted": 3, "already_reminded": 0, "ignored": 2}))
	})

	It("returns 500 when the run fails, so the cron retries", func(ctx SpecContext) {
		mockStandup := mock_standup.NewMockService(gomock.NewController(GinkgoT()))
		mockStandup.EXPECT().Remind(gomock.Any()).Return(&standup.Report{Members: 1}, errors.New("DM U1: timeout"))

		env := buildStandupTest(ctx, devConfig(), mockStandup)
		env.ServeHTTPRequest(http.MethodPost, path, nil, nil, http.StatusInternalServerError)
	})

	It("returns 401 without the task secret outside dev", func(ctx SpecContext) {
		cfg := marvin.Config{}
		cfg.TasksSecret = "test-secret"
		mockStandup := mock_standup.NewMockService(gomock.NewController(GinkgoT()))

		env := buildStandupTest(ctx, cfg, mockStandup)
		env.ServeHTTPRequest(http.MethodPost, path, nil, nil, http.StatusUnauthorized)
	})
})
