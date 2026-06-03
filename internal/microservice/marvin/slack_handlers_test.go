package marvin_test

import (
	"encoding/json"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/microservice/marvin"
	marvinroute "github.com/Flashgap/marvin/internal/route/marvin"
	"github.com/Flashgap/marvin/internal/service/github"
	mock_jira "github.com/Flashgap/marvin/internal/service/jira/mock"
	"github.com/Flashgap/marvin/internal/service/lock"
	mock_lock "github.com/Flashgap/marvin/internal/service/lock/mock"
	mock_marvin "github.com/Flashgap/marvin/internal/service/marvin/mock"
	mock_github "github.com/Flashgap/marvin/pkg/github/mock"
	"github.com/Flashgap/marvin/pkg/testenv"
)

func buildLockTest(ctx SpecContext, lockSvc lock.Service) *testenv.HTTPEnv {
	cfg := marvin.Config{}
	cfg.IsDevEnv = true // bypass Slack signing in tests

	mockCtrl := gomock.NewController(GinkgoT())
	services := &marvin.Services{
		MarvinService: mock_marvin.NewMockService(mockCtrl),
		GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
		JiraService:   mock_jira.NewMockService(mockCtrl),
		LockService:   lockSvc,
	}
	server, err := marvin.NewServer(ctx, &cfg, services)
	Expect(err).ToNot(HaveOccurred())
	return testenv.NewHTTPEnv(server.Handler)
}

func form(values map[string]string) string {
	v := url.Values{}
	for k, val := range values {
		v.Set(k, val)
	}
	return v.Encode()
}

var slackFormHeaders = map[string]any{"Content-Type": "application/x-www-form-urlencoded"}

var _ = Describe("POST /marvin/_webhook/slack/lock", func() {
	path := marvinroute.Paths.WebHooks + "/slack/lock"

	It("returns 501 when the lock service isn't wired in", func(ctx SpecContext) {
		env := buildLockTest(ctx, nil)
		env.ServeHTTPRequest(http.MethodPost, path, slackFormHeaders,
			form(map[string]string{"user_id": "U1", "text": "<@U2|x>"}),
			http.StatusNotImplemented)
	})

	It("dispatches to Lock when text is a mention", func(ctx SpecContext) {
		mockCtrl := gomock.NewController(GinkgoT())
		mockLock := mock_lock.NewMockService(mockCtrl)
		mockLock.EXPECT().
			Lock(gomock.Any(), slack.SlashCommand{UserID: "UVICTIM", UserName: "victim", Text: "<@UFINDER|finder>"}).
			Return(&lock.Response{Type: slack.ResponseTypeEphemeral, Text: "ok"}, nil)

		env := buildLockTest(ctx, mockLock)
		rec := env.ServeHTTPRequest(http.MethodPost, path, slackFormHeaders,
			form(map[string]string{"user_id": "UVICTIM", "user_name": "victim", "text": "<@UFINDER|finder>"}),
			http.StatusOK)

		var body map[string]string
		Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
		Expect(body["response_type"]).To(Equal("ephemeral"))
		Expect(body["text"]).To(Equal("ok"))
	})

	It("dispatches to Leaderboard when text is empty", func(ctx SpecContext) {
		mockCtrl := gomock.NewController(GinkgoT())
		mockLock := mock_lock.NewMockService(mockCtrl)
		mockLock.EXPECT().Leaderboard(gomock.Any()).Return(&lock.Response{Type: slack.ResponseTypeEphemeral, Text: "top..."}, nil)

		env := buildLockTest(ctx, mockLock)
		rec := env.ServeHTTPRequest(http.MethodPost, path, slackFormHeaders,
			form(map[string]string{"user_id": "UVICTIM", "user_name": "victim", "text": ""}),
			http.StatusOK)

		var body map[string]string
		Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
		Expect(body["text"]).To(Equal("top..."))
	})
})
