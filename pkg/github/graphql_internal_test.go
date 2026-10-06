package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	gogithub "github.com/google/go-github/v90/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shurcooL/graphql"
)

// This test lives in package github so it can point the GraphQL client at a fake server: it exercises
// the real query building and response decoding, which mocking the Client interface would skip.
var _ = Describe("ListStackLayers", func() {
	var (
		server   *httptest.Server
		c        *client
		webhook  *gogithub.PullRequestEvent
		response string
		gotQuery string
		gotVars  map[string]any
	)

	BeforeEach(func() {
		webhook = &gogithub.PullRequestEvent{
			Repo: &gogithub.Repository{
				Name:  gogithub.Ptr("marvin"),
				Owner: &gogithub.User{Login: gogithub.Ptr("Flashgap")},
			},
		}

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
			gotQuery, gotVars = body.Query, body.Variables

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(response))
		}))

		c = &client{graphql: graphql.NewClient(server.URL, server.Client())}
	})

	AfterEach(func() {
		server.Close()
	})

	It("queries the stack of the given PR, and returns nil when it isn't stacked", func(ctx context.Context) {
		response = `{"data":{"repository":{"pullRequest":{"stack":null}}}}`

		layers, err := c.ListStackLayers(ctx, webhook, 12)
		Expect(err).NotTo(HaveOccurred())
		Expect(layers).To(BeNil())
		Expect(gotQuery).To(ContainSubstring("pullRequest(number: $number){stack{entries(first: 100)"))
		Expect(gotVars).To(Equal(map[string]any{"owner": "Flashgap", "name": "marvin", "number": float64(12)}))
	})

	It("returns every layer with its deduped reviewers, merged layers included", func(ctx context.Context) {
		// Shape captured from the real stack #15 of Flashgap/marvin, plus a pending request and a repeat reviewer
		response = `{"data":{"repository":{"pullRequest":{"stack":{"entries":{"nodes":[
			{"position":1,"pullRequest":{"number":13,
				"reviews":{"nodes":[{"author":{"login":"Dal-Papa"}},{"author":{"login":"Dal-Papa"}}]},
				"reviewRequests":{"nodes":[]}}},
			{"position":2,"pullRequest":{"number":14,
				"reviews":{"nodes":[{"author":{"login":"Dal-Papa"}}]},
				"reviewRequests":{"nodes":[{"requestedReviewer":{"login":"clem"}},{"requestedReviewer":{}}]}}}
		]}}}}}}`

		layers, err := c.ListStackLayers(ctx, webhook, 14)
		Expect(err).NotTo(HaveOccurred())
		Expect(layers).To(Equal([]StackLayer{
			{Number: 13, Position: 1, Reviewers: []string{"Dal-Papa"}},
			{Number: 14, Position: 2, Reviewers: []string{"Dal-Papa", "clem"}},
		}))
	})

	It("returns an error when the query fails", func(ctx context.Context) {
		response = `{"errors":[{"message":"Could not resolve to a PullRequest with the number of 999."}]}`

		_, err := c.ListStackLayers(ctx, webhook, 999)
		Expect(err).To(MatchError(ContainSubstring("Could not resolve to a PullRequest")))
	})
})
