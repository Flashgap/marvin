package slack_test

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
	mock_slack "github.com/Flashgap/marvin/pkg/slack/mock"
)

func TestSlackService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Slack service suite")
}

var _ = Describe("Service", func() {
	var (
		ctrl   *gomock.Controller
		client *mock_slack.MockClient
		svc    slacksvc.Service
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client = mock_slack.NewMockClient(ctrl)
		svc = slacksvc.NewService(client)
	})

	Context("SendDM", func() {
		It("delegates to the underlying client", func(ctx SpecContext) {
			client.EXPECT().SendMessage(gomock.Any(), "U123", "hello").Return(nil)
			Expect(svc.SendDM(ctx, "U123", "hello")).To(Succeed())
		})
	})

	Context("GetUser", func() {
		It("prefers DisplayName for the user name", func(ctx SpecContext) {
			u := &slack.User{ID: "U1", RealName: "Alice Real", Name: "alice_login"}
			u.Profile.DisplayName = "alice"
			client.EXPECT().GetUser(gomock.Any(), "U1").Return(u, nil)

			got, err := svc.GetUser(ctx, "U1")
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal(&slacksvc.User{ID: "U1", Name: "alice", IsBot: false}))
		})

		It("falls back to RealName then Name", func(ctx SpecContext) {
			u := &slack.User{ID: "U2", RealName: "Bob Real", Name: "bob_login"}
			client.EXPECT().GetUser(gomock.Any(), "U2").Return(u, nil)
			got, err := svc.GetUser(ctx, "U2")
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Name).To(Equal("Bob Real"))

			u2 := &slack.User{ID: "U3", Name: "carol_login"}
			client.EXPECT().GetUser(gomock.Any(), "U3").Return(u2, nil)
			got, err = svc.GetUser(ctx, "U3")
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Name).To(Equal("carol_login"))
		})

		It("forwards IsBot", func(ctx SpecContext) {
			u := &slack.User{ID: "Ubot", IsBot: true}
			u.Profile.DisplayName = "marvin-bot"
			client.EXPECT().GetUser(gomock.Any(), "Ubot").Return(u, nil)
			got, err := svc.GetUser(ctx, "Ubot")
			Expect(err).ToNot(HaveOccurred())
			Expect(got.IsBot).To(BeTrue())
		})

		It("forwards Deleted", func(ctx SpecContext) {
			client.EXPECT().GetUser(gomock.Any(), "Ugone").Return(&slack.User{ID: "Ugone", Deleted: true}, nil)
			got, err := svc.GetUser(ctx, "Ugone")
			Expect(err).ToNot(HaveOccurred())
			Expect(got.Deleted).To(BeTrue())
		})
	})

	Context("ChannelMembers", func() {
		It("delegates to the underlying client", func(ctx SpecContext) {
			client.EXPECT().GetChannelMembers(gomock.Any(), "C1").Return([]string{"U1", "U2"}, nil)
			Expect(svc.ChannelMembers(ctx, "C1")).To(Equal([]string{"U1", "U2"}))
		})
	})

	Context("ChannelHistory", func() {
		oldest := time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)

		It("maps messages and parses PostedAt from the ts", func(ctx SpecContext) {
			msg := slack.Message{}
			msg.User = "U1"
			msg.Timestamp = "1791274320.123456"
			msg.Text = "Today: ship it"
			msg.SubType = "file_share"
			bot := slack.Message{}
			bot.BotID = "B1"
			bot.Timestamp = "1791274000.000100"
			client.EXPECT().GetChannelHistory(gomock.Any(), "C1", oldest).Return([]slack.Message{msg, bot}, nil)

			got, err := svc.ChannelHistory(ctx, "C1", oldest)
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal([]slacksvc.Message{
				{UserID: "U1", TS: "1791274320.123456", PostedAt: time.Unix(1791274320, 0).UTC(), Text: "Today: ship it", SubType: "file_share"},
				{TS: "1791274000.000100", PostedAt: time.Unix(1791274000, 0).UTC(), BotID: "B1"},
			}))
		})

		It("returns the client error", func(ctx SpecContext) {
			client.EXPECT().GetChannelHistory(gomock.Any(), "C1", oldest).Return(nil, errors.New("boom"))
			_, err := svc.ChannelHistory(ctx, "C1", oldest)
			Expect(err).To(MatchError("boom"))
		})
	})

	DescribeTable("ParseTS",
		func(ts string, want time.Time) {
			Expect(slacksvc.ParseTS(ts)).To(Equal(want))
		},
		Entry("seconds and micros", "1791274320.123456", time.Unix(1791274320, 0).UTC()),
		Entry("seconds only", "1791274320", time.Unix(1791274320, 0).UTC()),
		Entry("malformed", "nope", time.Time{}),
		Entry("empty", "", time.Time{}),
	)
})
