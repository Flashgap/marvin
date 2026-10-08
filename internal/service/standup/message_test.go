package standup_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
	"github.com/Flashgap/marvin/internal/service/standup"
)

var _ = Describe("isStandupMessage", func() {
	DescribeTable("keeps only people's top-level posts",
		func(m slacksvc.Message, want bool) {
			Expect(standup.IsStandupMessage(m)).To(Equal(want))
		},
		Entry("plain post", slacksvc.Message{UserID: "U1", Text: "Today: x"}, true),
		Entry("file_share with text", slacksvc.Message{UserID: "U1", Text: "Today: x", SubType: "file_share"}, true),
		Entry("file_share without text", slacksvc.Message{UserID: "U1", SubType: "file_share"}, false),
		Entry("thread_broadcast", slacksvc.Message{UserID: "U1", Text: "x", SubType: "thread_broadcast"}, false),
		Entry("channel_join", slacksvc.Message{UserID: "U1", Text: "x", SubType: "channel_join"}, false),
		Entry("channel_topic", slacksvc.Message{UserID: "U1", Text: "x", SubType: "channel_topic"}, false),
		Entry("bot_message", slacksvc.Message{Text: "x", SubType: "bot_message", BotID: "B1"}, false),
		Entry("me_message", slacksvc.Message{UserID: "U1", Text: "x", SubType: "me_message"}, false),
		Entry("app post with a bot_id", slacksvc.Message{UserID: "U1", Text: "x", BotID: "B1"}, false),
		Entry("no user", slacksvc.Message{Text: "x"}, false),
		Entry("whitespace only", slacksvc.Message{UserID: "U1", Text: " \n\t"}, false),
	)
})

var _ = Describe("latestUpdates", func() {
	It("keeps each user's newest standup message", func() {
		newest := slacksvc.Message{UserID: "U1", TS: "3", Text: "new"}
		got := standup.LatestUpdates([]slacksvc.Message{
			{UserID: "U1", TS: "4", SubType: "channel_join", Text: "joined"},
			newest,
			{UserID: "U1", TS: "2", Text: "old"},
			{UserID: "U2", TS: "1", Text: "other"},
		})
		Expect(got).To(HaveLen(2))
		Expect(got["U1"]).To(Equal(newest))
		Expect(got["U2"].Text).To(Equal("other"))
	})
})

var _ = Describe("quotableText", func() {
	DescribeTable("extracts the Today section",
		func(text, want string) {
			Expect(standup.QuotableText(text)).To(Equal(want))
		},
		Entry("plain headers", "Yesterday: a\nToday: b\nBlockers: c", "b"),
		Entry("bold headers", "*Yesterday:* a\n*Today:* b\n*Blockers:* c", "b"),
		Entry("italic header", "_Today_: b\n_Blockers_: c", "b"),
		Entry("lowercase", "today: b", "b"),
		Entry("quoted header", "> Today: b\n> Blockers: c", "b"),
		Entry("quoted header as Slack sends it", "&gt; Yesterday: a\n&gt; Today: b\n&gt; Blockers: c", "b"),
		Entry("section ending at a quoted header as Slack sends it", "Today: b\n&gt;Blockers: c", "b"),
		Entry("multi-line section", "Today:\n- b1\n- b2\nBlockers: none", "- b1\n- b2"),
		Entry("ends at a singular Blocker header", "Today: b\nBlocker: c", "b"),
		Entry("Today as the last section", "Yesterday: a\nToday: b\nand more", "b\nand more"),
		Entry("empty Today section falls back to the full text", "Yesterday: a\nToday:\nBlockers: c", "Yesterday: a\nToday:\nBlockers: c"),
		Entry("no header gives the full text", "  shipped the thing  \n", "shipped the thing"),
		Entry("a Note: line doesn't end the section", "Today: b\nNote: c\nBlockers: d", "b\nNote: c"),
	)
})

var _ = Describe("formatReminder", func() {
	const template = "\nCopy, fill in and post in <#C1>:\nYesterday: …\nToday: …\nBlockers: …"

	It("sends only the template without a stored update", func() {
		Expect(standup.FormatReminder("C1", "", "")).To(Equal(":wave: Time for your standup in <#C1>!\n" + template))
	})

	It("quotes every line of the stored update with its date", func() {
		Expect(standup.FormatReminder("C1", "1791274320.000100", "Today: did the thing\n\nand another thing")).To(Equal(
			":wave: Time for your standup in <#C1>!\n" +
				"On <!date^1791274320^{date_long}|Tue, 06 Oct 2026> you said:\n" +
				"> did the thing\n" +
				"> \n" +
				"> and another thing\n" +
				template))
	})

	It("cuts a long update at the last newline before the cap", func() {
		text := strings.Repeat("a", 2000) + "\n" + strings.Repeat("b", 2000)
		Expect(standup.FormatReminder("C1", "1791274320.000100", text)).To(ContainSubstring(
			fmt.Sprintf("> %s…\n%s", strings.Repeat("a", 2000), template)))
	})

	It("hard-cuts a long update without newlines", func() {
		text := strings.Repeat("é", 4000)
		Expect(standup.FormatReminder("C1", "1791274320.000100", text)).To(ContainSubstring(
			fmt.Sprintf("> %s…\n%s", strings.Repeat("é", 3000), template)))
	})
})
