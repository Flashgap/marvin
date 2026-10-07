package github

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	gogithub "github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shurcooL/graphql"
)

// This test lives in package github so it can point the GraphQL client at a fake server: decoding and
// mapping the response is what mocking the Client interface skips, and a mistake there fails silently.
var _ = Describe("ListStackLayers", func() {
	It("returns every layer with its deduped reviewers, merged layers included", func(ctx context.Context) {
		// Shape captured from the real stack #15 of Flashgap/marvin, plus a pending request, a team request
		// (no login) and a repeat reviewer
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"stack":{"entries":{"nodes":[
				{"position":1,"pullRequest":{"number":13,
					"reviews":{"nodes":[{"author":{"login":"Dal-Papa"}},{"author":{"login":"Dal-Papa"}}]},
					"reviewRequests":{"nodes":[]}}},
				{"position":2,"pullRequest":{"number":14,
					"reviews":{"nodes":[{"author":{"login":"Dal-Papa"}}]},
					"reviewRequests":{"nodes":[{"requestedReviewer":{"login":"clem"}},{"requestedReviewer":{}}]}}}
			]}}}}}}`))
		}))
		DeferCleanup(server.Close)

		c := &client{graphql: graphql.NewClient(server.URL, server.Client())}
		webhook := &gogithub.PullRequestEvent{Repo: &gogithub.Repository{
			Name:  gogithub.Ptr("marvin"),
			Owner: &gogithub.User{Login: gogithub.Ptr("Flashgap")},
		}}

		layers, err := c.ListStackLayers(ctx, webhook, 14)
		Expect(err).NotTo(HaveOccurred())
		Expect(layers).To(Equal([]StackLayer{
			{Number: 13, Position: 1, Reviewers: []string{"Dal-Papa"}},
			{Number: 14, Position: 2, Reviewers: []string{"Dal-Papa", "clem"}},
		}))
	})
})

var _ = Describe("ListOpenPRsWithReviewers", func() {
	It("only asks for reviews with a verdict and merges them with pending requests", func(ctx context.Context) {
		var body string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequests":{"nodes":[
				{"number":286,"additions":120,
					"reviews":{"nodes":[{"author":{"login":"Dal-Papa"}},{"author":{"login":"Dal-Papa"}}]},
					"reviewRequests":{"nodes":[{"requestedReviewer":{"login":"clem"}},{"requestedReviewer":{}}]}}
			]}}}}`))
		}))
		DeferCleanup(server.Close)

		c := &client{graphql: graphql.NewClient(server.URL, server.Client())}
		webhook := &gogithub.PullRequestEvent{Repo: &gogithub.Repository{
			Name:  gogithub.Ptr("backend"),
			Owner: &gogithub.User{Login: gogithub.Ptr("hector-finance")},
		}}

		prs, err := c.ListOpenPRsWithReviewers(ctx, webhook)
		Expect(err).NotTo(HaveOccurred())
		// COMMENTED reviews are filtered by GitHub: the author replying in a thread, or a drive-by comment
		Expect(body).To(ContainSubstring("reviews(first: 100, states: [APPROVED, CHANGES_REQUESTED, DISMISSED])"))
		Expect(prs).To(Equal([]OpenPRReviewLoad{
			{Number: 286, Additions: 120, Reviewers: map[string]struct{}{"Dal-Papa": {}, "clem": {}}},
		}))
	})
})
