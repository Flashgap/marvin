package slack_test

import (
	"testing"

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
	})
})
