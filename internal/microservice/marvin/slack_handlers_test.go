package marvin_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	gogithub "github.com/google/go-github/v90/github"
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
	"github.com/Flashgap/marvin/internal/service/reviewload"
	mock_reviewload "github.com/Flashgap/marvin/internal/service/reviewload/mock"
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
			Return(&slack.Msg{ResponseType: slack.ResponseTypeEphemeral, Text: "ok"}, nil)

		env := buildLockTest(ctx, mockLock)
		rec := env.ServeHTTPRequest(http.MethodPost, path, slackFormHeaders,
			form(map[string]string{"user_id": "UVICTIM", "user_name": "victim", "text": "<@UFINDER|finder>"}),
			http.StatusOK)

		var body slack.Msg
		Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
		Expect(body.ResponseType).To(Equal(slack.ResponseTypeEphemeral))
		Expect(body.Text).To(Equal("ok"))
	})

	It("dispatches to Leaderboard when text is empty", func(ctx SpecContext) {
		mockCtrl := gomock.NewController(GinkgoT())
		mockLock := mock_lock.NewMockService(mockCtrl)
		mockLock.EXPECT().Leaderboard(gomock.Any()).Return(&slack.Msg{ResponseType: slack.ResponseTypeEphemeral, Text: "top..."}, nil)

		env := buildLockTest(ctx, mockLock)
		rec := env.ServeHTTPRequest(http.MethodPost, path, slackFormHeaders,
			form(map[string]string{"user_id": "UVICTIM", "user_name": "victim", "text": ""}),
			http.StatusOK)

		var body slack.Msg
		Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Text).To(Equal("top..."))
	})
})

var _ = Describe("POST /marvin/_webhook/slack/review-load", func() {
	path := marvinroute.Paths.WebHooks + "/slack/review-load"

	var (
		mockReviewLoad *mock_reviewload.MockService
		env            *testenv.HTTPEnv
	)

	BeforeEach(func(ctx SpecContext) {
		mockCtrl := gomock.NewController(GinkgoT())
		mockReviewLoad = mock_reviewload.NewMockService(mockCtrl)

		cfg := marvin.Config{}
		cfg.IsDevEnv = true // bypass Slack signing in tests
		server, err := marvin.NewServer(ctx, &cfg, &marvin.Services{
			MarvinService: mock_marvin.NewMockService(mockCtrl),
			GithubService: github.NewService(mock_github.NewMockClient(mockCtrl)),
			JiraService:   mock_jira.NewMockService(mockCtrl),
			ReviewLoad:    mockReviewLoad,
		})
		Expect(err).ToNot(HaveOccurred())
		env = testenv.NewHTTPEnv(server.Handler)
	})

	respond := func(text string, status int) slack.Msg {
		rec := env.ServeHTTPRequest(http.MethodPost, path, slackFormHeaders,
			form(map[string]string{"user_id": "U1", "text": text}), status)

		var body slack.Msg
		if status == http.StatusOK {
			Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
			Expect(body.ResponseType).To(Equal(slack.ResponseTypeEphemeral))
		}
		return body
	}

	backend := &gogithub.Repository{
		FullName: gogithub.Ptr("hector-finance/backend"),
		HTMLURL:  gogithub.Ptr("https://github.com/hector-finance/backend"),
	}

	It("lists reviewers with links to their PRs", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "backend").Return(backend, []reviewload.Reviewer{
			{Login: "0rax", Score: 90, PRs: []int{320}},
			{Login: "lebascou", Score: 154, PRs: []int{371, 373}},
		}, nil)

		Expect(respond(" backend ", http.StatusOK).Text).To(Equal(
			"*Review load of hector-finance/backend* (additions of the open PRs each person reviews, lowest is picked first)\n" +
				"• *0rax* — 90 — <https://github.com/hector-finance/backend/pull/320|#320>\n" +
				"• *lebascou* — 154 — <https://github.com/hector-finance/backend/pull/371|#371>, <https://github.com/hector-finance/backend/pull/373|#373>\n"))
	})

	It("says when nobody is reviewing", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "backend").Return(backend, nil, nil)

		Expect(respond("backend", http.StatusOK).Text).To(Equal("Nobody is reviewing an open PR of *hector-finance/backend*."))
	})

	It("shows the usage when no repository is given", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "").
			Return(nil, nil, &reviewload.UnknownRepositoryError{Repositories: []string{"backend", "marvin"}})

		Expect(respond("", http.StatusOK).Text).To(Equal("Usage: `/review-load <repository>`. Repositories: `backend`, `marvin`"))
	})

	It("rejects an unknown repository", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "frontend").
			Return(nil, nil, &reviewload.UnknownRepositoryError{Name: "frontend", Repositories: []string{"backend", "marvin"}})

		Expect(respond("frontend", http.StatusOK).Text).To(Equal("Unknown repository `frontend`. Repositories: `backend`, `marvin`"))
	})

	It("fails on GitHub errors", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "backend").Return(nil, nil, errors.New("boom"))

		respond("backend", http.StatusInternalServerError)
	})
})
