package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	gogithub "github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/pkg/github"
)

var _ = Describe("MergeAsync", func() {
	var (
		server  *httptest.Server
		client  github.Client
		webhook *gogithub.PullRequestEvent
		status  int
		body    string
		gotPath string
	)

	BeforeEach(func() {
		webhook = &gogithub.PullRequestEvent{
			Repo: &gogithub.Repository{
				Name:  gogithub.Ptr("backend"),
				Owner: &gogithub.User{Login: gogithub.Ptr("hector-finance")},
			},
		}

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))

		baseURL := server.URL + "/"
		ghClient, err := gogithub.NewClient(gogithub.WithURLs(&baseURL, nil))
		Expect(err).NotTo(HaveOccurred())
		client = github.NewClient(ghClient)
	})

	AfterEach(func() {
		server.Close()
	})

	It("should hit the merge-async endpoint", func() {
		status = http.StatusOK
		body = `{"status":"merged","details":{"sha":"f8f19c5"}}`

		_, _, err := client.MergePRAsync(context.Background(), webhook, 187, github.PullRequestMergeAsyncRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(gotPath).To(Equal("/repos/hector-finance/backend/pulls/187/merge-async"))
	})

	It("should return the merged status on a 200", func() {
		status = http.StatusOK
		body = `{"status":"merged","details":{"sha":"f8f19c5","message":"merged"}}`

		res, _, err := client.MergePRAsync(context.Background(), webhook, 187, github.PullRequestMergeAsyncRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.GetStatus()).To(Equal(github.MergeAsyncStatusMerged))
		Expect(res.GetDetails().GetSHA()).To(Equal("f8f19c5"))
	})

	// GitHub answers 202 Accepted while it merges in the background. go-github maps every 202 to
	// an AcceptedError, which used to surface as a merge failure even though the merge succeeded.
	It("should return the pending status instead of an error on a 202", func() {
		status = http.StatusAccepted
		body = `{"status":"pending","details":{"uuid":"c0ffee","message":"merge in progress"}}`

		res, _, err := client.MergePRAsync(context.Background(), webhook, 187, github.PullRequestMergeAsyncRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.GetStatus()).To(Equal(github.MergeAsyncStatusPending))
		Expect(res.GetDetails().GetUUID()).To(Equal("c0ffee"))
		Expect(res.GetDetails().GetMessage()).To(Equal("merge in progress"))
	})

	It("should error when a 202 body cannot be parsed", func() {
		status = http.StatusAccepted
		body = `not json`

		res, _, err := client.MergePRAsync(context.Background(), webhook, 187, github.PullRequestMergeAsyncRequest{})
		Expect(err).To(HaveOccurred())
		Expect(res).To(BeNil())
	})

	It("should error on an API failure", func() {
		status = http.StatusConflict
		body = `{"message":"head branch was modified"}`

		res, _, err := client.MergePRAsync(context.Background(), webhook, 187, github.PullRequestMergeAsyncRequest{})
		Expect(err).To(HaveOccurred())
		Expect(res).To(BeNil())
	})
})
