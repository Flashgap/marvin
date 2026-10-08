package slack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/slack-go/slack"
)

func TestSlack(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Slack pkg test suite")
}

// newTestClient points a slackClient at a fake Slack API that answers each call with the next of
// pages, and sends the form of every request it receives on the returned channel.
func newTestClient(pages ...string) (*slackClient, <-chan url.Values) {
	requests := make(chan url.Values, len(pages))
	var served atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(served.Add(1)) - 1
		if i >= len(pages) || r.ParseForm() != nil {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		r.PostForm.Set("path", r.URL.Path)
		requests <- r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(pages[i]))
	}))
	DeferCleanup(server.Close)
	return &slackClient{slack.New("xoxb-test", slack.OptionAPIURL(server.URL+"/"))}, requests
}

var _ = Describe("GetChannelMembers", func() {
	It("follows the cursor across pages", func(ctx context.Context) {
		c, requests := newTestClient(
			`{"ok":true,"members":["U1","U2"],"response_metadata":{"next_cursor":"page2"}}`,
			`{"ok":true,"members":["U3"],"response_metadata":{"next_cursor":""}}`,
		)

		members, err := c.GetChannelMembers(ctx, "C1")
		Expect(err).ToNot(HaveOccurred())
		Expect(members).To(Equal([]string{"U1", "U2", "U3"}))
		var first, second url.Values
		Expect(requests).To(Receive(&first))
		Expect(first.Get("path")).To(Equal("/conversations.members"))
		Expect(first.Get("channel")).To(Equal("C1"))
		Expect(first.Get("limit")).To(Equal("200"))
		Expect(first.Has("cursor")).To(BeFalse())
		Expect(requests).To(Receive(&second))
		Expect(second.Get("cursor")).To(Equal("page2"))
	})

	It("wraps Slack errors", func(ctx context.Context) {
		c, _ := newTestClient(`{"ok":false,"error":"not_in_channel"}`)

		_, err := c.GetChannelMembers(ctx, "C1")
		Expect(err).To(MatchError(ErrGetChannelMembers))
		Expect(err).To(MatchError(ContainSubstring("not_in_channel")))
	})
})

var _ = Describe("GetChannelHistory", func() {
	oldest := time.Unix(1790000000, 0)

	It("follows has_more across pages", func(ctx context.Context) {
		c, requests := newTestClient(
			`{"ok":true,"has_more":true,"messages":[{"user":"U1","ts":"1790000300.000200","text":"b"}],"response_metadata":{"next_cursor":"page2"}}`,
			`{"ok":true,"has_more":false,"messages":[{"user":"U2","ts":"1790000100.000100","text":"a"}]}`,
		)

		msgs, err := c.GetChannelHistory(ctx, "C1", oldest)
		Expect(err).ToNot(HaveOccurred())
		Expect(msgs).To(HaveLen(2))
		Expect(msgs[0].User).To(Equal("U1"))
		Expect(msgs[0].Timestamp).To(Equal("1790000300.000200"))
		Expect(msgs[1].User).To(Equal("U2"))
		var first, second url.Values
		Expect(requests).To(Receive(&first))
		Expect(first.Get("path")).To(Equal("/conversations.history"))
		Expect(first.Get("channel")).To(Equal("C1"))
		Expect(first.Get("oldest")).To(Equal("1790000000"))
		Expect(first.Get("limit")).To(Equal("200"))
		Expect(requests).To(Receive(&second))
		Expect(second.Get("cursor")).To(Equal("page2"))
	})

	It("stops when has_more comes without a cursor", func(ctx context.Context) {
		c, requests := newTestClient(`{"ok":true,"has_more":true,"messages":[{"user":"U1","ts":"1790000100.000100","text":"a"}]}`)

		msgs, err := c.GetChannelHistory(ctx, "C1", oldest)
		Expect(err).ToNot(HaveOccurred())
		Expect(msgs).To(HaveLen(1))
		Expect(requests).To(HaveLen(1))
	})

	It("wraps Slack errors", func(ctx context.Context) {
		c, _ := newTestClient(`{"ok":false,"error":"channel_not_found"}`)

		_, err := c.GetChannelHistory(ctx, "C1", oldest)
		Expect(err).To(MatchError(ErrGetChannelHistory))
		Expect(err).To(MatchError(ContainSubstring("channel_not_found")))
	})
})

var _ = Describe("IsPermanentDMError", func() {
	slackErr := func(code string) error {
		return fmt.Errorf("%w: %w", ErrOpenConversation, slack.SlackErrorResponse{Err: code})
	}

	DescribeTable("classifies DM errors",
		func(err error, want bool) {
			Expect(IsPermanentDMError(err)).To(Equal(want))
		},
		Entry("cannot_dm_bot", slackErr("cannot_dm_bot"), true),
		Entry("user_not_found", slackErr("user_not_found"), true),
		Entry("user_not_visible", slackErr("user_not_visible"), true),
		Entry("user_disabled", slackErr("user_disabled"), true),
		Entry("another Slack error", slackErr("ratelimited"), false),
		Entry("a non-Slack error", errors.New("connection reset"), false),
		Entry("nil", nil, false),
	)

	It("recognizes the error SendMessage returns", func(ctx context.Context) {
		c, _ := newTestClient(`{"ok":false,"error":"cannot_dm_bot"}`)

		err := c.SendMessage(ctx, "UBOT", "hi")
		Expect(err).To(HaveOccurred())
		Expect(IsPermanentDMError(err)).To(BeTrue())
	})
})
