package marvin_test

import (
	"encoding/json"
	"errors"
	"fmt"
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
		Name:     gogithub.Ptr("backend"),
		FullName: gogithub.Ptr("hector-finance/backend"),
		HTMLURL:  gogithub.Ptr("https://github.com/hector-finance/backend"),
	}
	frontend := &gogithub.Repository{Name: gogithub.Ptr("frontend"), FullName: gogithub.Ptr("hector-finance/frontend")}

	pr := func(repo string, number int) reviewload.PullRequest {
		return reviewload.PullRequest{Number: number, URL: fmt.Sprintf("https://github.com/hector-finance/%s/pull/%d", repo, number)}
	}

	It("lists reviewers in a table with links to their PRs", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "backend").Return(backend, []reviewload.Reviewer{
			{Login: "0rax", Score: 90, PRs: []reviewload.PullRequest{pr("backend", 320)}},
			{Login: "lebascou", Score: 154, PRs: []reviewload.PullRequest{pr("backend", 371), pr("backend", 373)}},
		}, nil)

		msg := respond(" backend ", http.StatusOK)
		Expect(msg.Text).To(Equal("*Review load of hector-finance/backend*"))
		Expect(msg.Blocks.BlockSet).To(HaveLen(2))

		table, err := json.Marshal(msg.Blocks.BlockSet[1])
		Expect(err).NotTo(HaveOccurred())
		Expect(table).To(MatchJSON(`{
			"type": "table",
			"column_settings": [
				{"align": "left", "is_wrapped": false},
				{"align": "right", "is_wrapped": false},
				{"align": "left", "is_wrapped": true}
			],
			"rows": [
				[
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [{"type": "text", "text": "Reviewer", "style": {"bold": true}}]}]},
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [{"type": "text", "text": "Score", "style": {"bold": true}}]}]},
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [{"type": "text", "text": "PRs", "style": {"bold": true}}]}]}
				],
				[
					{"type": "raw_text", "text": "0rax"},
					{"type": "raw_text", "text": "90"},
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [
						{"type": "link", "url": "https://github.com/hector-finance/backend/pull/320", "text": "#320"}
					]}]}
				],
				[
					{"type": "raw_text", "text": "lebascou"},
					{"type": "raw_text", "text": "154"},
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [
						{"type": "link", "url": "https://github.com/hector-finance/backend/pull/371", "text": "#371"},
						{"type": "text", "text": ", "},
						{"type": "link", "url": "https://github.com/hector-finance/backend/pull/373", "text": "#373"}
					]}]}
				]
			]
		}`))
	})

	It("says when nobody is reviewing", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "backend").Return(backend, nil, nil)

		Expect(respond("backend", http.StatusOK).Text).To(Equal("Nobody is reviewing an open PR of *hector-finance/backend*."))
	})

	It("ranks every repository in one table, each repository's reviewers after a row naming it", func() {
		mockReviewLoad.EXPECT().ReviewLoad(gomock.Any()).Return([]reviewload.RepositoryReviewers{
			{Repo: backend, Reviewers: []reviewload.Reviewer{{Login: "0rax", Score: 90, PRs: []reviewload.PullRequest{pr("backend", 320)}}}},
			{Repo: frontend, Reviewers: []reviewload.Reviewer{{Login: "Jane", Score: 307, PRs: []reviewload.PullRequest{pr("frontend", 211)}}}},
		}, nil)

		msg := respond(" ", http.StatusOK)
		Expect(msg.Text).To(Equal("*Review load of every repository*"))
		Expect(msg.Blocks.BlockSet).To(HaveLen(2))

		table, err := json.Marshal(msg.Blocks.BlockSet[1])
		Expect(err).NotTo(HaveOccurred())
		Expect(table).To(MatchJSON(`{
			"type": "table",
			"column_settings": [
				{"align": "left", "is_wrapped": false},
				{"align": "right", "is_wrapped": false},
				{"align": "left", "is_wrapped": true}
			],
			"rows": [
				[
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [{"type": "text", "text": "Reviewer", "style": {"bold": true}}]}]},
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [{"type": "text", "text": "Score", "style": {"bold": true}}]}]},
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [{"type": "text", "text": "PRs", "style": {"bold": true}}]}]}
				],
				[
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [{"type": "text", "text": "backend", "style": {"bold": true}}]}]},
					{"type": "raw_text", "text": ""},
					{"type": "raw_text", "text": ""}
				],
				[
					{"type": "raw_text", "text": "0rax"},
					{"type": "raw_text", "text": "90"},
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [
						{"type": "link", "url": "https://github.com/hector-finance/backend/pull/320", "text": "#320"}
					]}]}
				],
				[
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [{"type": "text", "text": "frontend", "style": {"bold": true}}]}]},
					{"type": "raw_text", "text": ""},
					{"type": "raw_text", "text": ""}
				],
				[
					{"type": "raw_text", "text": "Jane"},
					{"type": "raw_text", "text": "307"},
					{"type": "rich_text", "elements": [{"type": "rich_text_section", "elements": [
						{"type": "link", "url": "https://github.com/hector-finance/frontend/pull/211", "text": "#211"}
					]}]}
				]
			]
		}`))
	})

	It("says when nobody is reviewing in any repository", func() {
		mockReviewLoad.EXPECT().ReviewLoad(gomock.Any()).Return(nil, nil)

		Expect(respond("", http.StatusOK).Text).To(Equal("Nobody is reviewing an open PR of *every repository*."))
	})

	It("rejects an unknown repository", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "frontend").
			Return(nil, nil, &reviewload.UnknownRepositoryError{Name: "frontend", Repositories: []string{"backend", "marvin"}})

		Expect(respond("frontend", http.StatusOK).Text).To(Equal("Unknown repository `frontend`. Usage: `/review-load [repository]`, all repositories when omitted. Repositories: `backend`, `marvin`"))
	})

	It("fails on GitHub errors", func() {
		mockReviewLoad.EXPECT().RepoReviewLoad(gomock.Any(), "backend").Return(nil, nil, errors.New("boom"))

		respond("backend", http.StatusInternalServerError)
	})

	It("fails on GitHub errors across repositories", func() {
		mockReviewLoad.EXPECT().ReviewLoad(gomock.Any()).Return(nil, errors.New("boom"))

		respond("", http.StatusInternalServerError)
	})
})
