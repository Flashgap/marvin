package standup_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
	mock_slack "github.com/Flashgap/marvin/internal/service/slack/mock"
	"github.com/Flashgap/marvin/internal/service/standup"
	"github.com/Flashgap/marvin/pkg/database"
	pkgslack "github.com/Flashgap/marvin/pkg/slack"
)

func TestStandupService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Standup service suite")
}

var (
	now        = time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC) // a Wednesday
	todayStart = time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	yesterday  = time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	lastWeek   = time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
)

const standupText = "Yesterday: reviews\nToday: did the thing\nBlockers: none"

func ts(t time.Time) string { return fmt.Sprintf("%d.000100", t.Unix()) }

func post(userID string, at time.Time, text string) slacksvc.Message {
	return slacksvc.Message{UserID: userID, TS: ts(at), PostedAt: at, Text: text}
}

// newStandupService constructs a standup service backed by sqlmock, skipping
// the migration step, with its clock frozen at now.
func newStandupService(t GinkgoTInterface) (standup.Service, sqlmock.Sqlmock, *mock_slack.MockService) {
	t.Helper()
	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() { _ = db.Close() })

	ctrl := gomock.NewController(t)
	mslack := mock_slack.NewMockService(ctrl)
	svc := standup.NewTestService(database.NewTestClient(db, database.DriverPostgres), mslack, "C1", func() time.Time { return now })
	return svc, mock, mslack
}

func expectChannel(mslack *mock_slack.MockService, members []string, history ...slacksvc.Message) {
	mslack.EXPECT().ChannelMembers(gomock.Any(), "C1").Return(members, nil)
	mslack.EXPECT().ChannelHistory(gomock.Any(), "C1", now.Add(-14*24*time.Hour)).Return(history, nil)
}

// expectStates expects the standup_users read; each row is
// (slack_user_id, last_update_ts, last_update_text, reminded_today).
func expectStates(mock sqlmock.Sqlmock, rows ...[]driver.Value) {
	r := sqlmock.NewRows([]string{"slack_user_id", "last_update_ts", "last_update_text", "reminded_today"})
	for _, row := range rows {
		r.AddRow(row...)
	}
	mock.ExpectQuery(`SELECT .* FROM .*standup_users`).WithArgs(todayStart).WillReturnRows(r)
}

func expectSaveLastUpdate(mock sqlmock.Sqlmock) *sqlmock.ExpectedExec {
	return mock.ExpectExec(`INSERT INTO .*standup_users.*last_update_ts`)
}

func expectMarkReminded(mock sqlmock.Sqlmock) *sqlmock.ExpectedExec {
	return mock.ExpectExec(`INSERT INTO .*standup_users.*last_reminded_at`)
}

// captureDM expects one DM to userID and returns a pointer to its text.
func captureDM(mslack *mock_slack.MockService, userID string) *string {
	var text string
	mslack.EXPECT().SendDM(gomock.Any(), userID, gomock.Any()).DoAndReturn(func(_ context.Context, _, msg string) error {
		text = msg
		return nil
	})
	return &text
}

func human(userID string) *slacksvc.User { return &slacksvc.User{ID: userID} }

var _ = Describe("Remind", func() {
	It("skips a member already reminded today", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"}, post("U1", yesterday, standupText))
		expectStates(mock, []driver.Value{"U1", nil, nil, 1})

		report, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(report).To(Equal(&standup.Report{Members: 1, AlreadyReminded: 1}))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("stores today's post without sending a DM", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"}, post("U1", todayStart.Add(time.Hour), standupText))
		expectStates(mock)
		expectSaveLastUpdate(mock).WillReturnResult(sqlmock.NewResult(1, 1))

		report, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(report).To(Equal(&standup.Report{Members: 1, AlreadyPosted: 1}))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("stores yesterday's post, DMs a quote of it and marks the member reminded", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"}, post("U1", yesterday, standupText))
		expectStates(mock)
		expectSaveLastUpdate(mock).WithArgs(standupText, ts(yesterday), "U1", now, standupText, ts(yesterday), now).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		dm := captureDM(mslack, "U1")
		expectMarkReminded(mock).WithArgs(now, "U1", now, now, now).WillReturnResult(sqlmock.NewResult(1, 1))

		report, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(report).To(Equal(&standup.Report{Members: 1, Reminded: 1}))
		Expect(*dm).To(ContainSubstring(fmt.Sprintf("On <!date^%d^{date_long}|Tue, 06 Oct 2026> you said:\n> did the thing\n", yesterday.Unix())))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("quotes the stored update when the member hasn't posted in the window", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"})
		expectStates(mock, []driver.Value{"U1", ts(lastWeek), "Today: old news", 0})
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		dm := captureDM(mslack, "U1")
		expectMarkReminded(mock).WillReturnResult(sqlmock.NewResult(1, 1))

		report, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Reminded).To(Equal(1))
		Expect(*dm).To(ContainSubstring("|Wed, 30 Sep 2026> you said:\n> old news\n"))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("sends only the template to a brand-new member", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"})
		expectStates(mock)
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		dm := captureDM(mslack, "U1")
		expectMarkReminded(mock).WillReturnResult(sqlmock.NewResult(1, 1))

		report, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Reminded).To(Equal(1))
		Expect(*dm).ToNot(ContainSubstring("you said"))
		Expect(*dm).To(ContainSubstring("Yesterday: …\nToday: …\nBlockers: …"))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("ignores bots and deactivated users", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"UBOT", "UGONE"})
		expectStates(mock)
		mslack.EXPECT().GetUser(gomock.Any(), "UBOT").Return(&slacksvc.User{ID: "UBOT", IsBot: true}, nil)
		mslack.EXPECT().GetUser(gomock.Any(), "UGONE").Return(&slacksvc.User{ID: "UGONE", Deleted: true}, nil)

		report, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(report).To(Equal(&standup.Report{Members: 2, Ignored: 2}))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("keeps going when a DM fails, returns an error and leaves that member unmarked", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1", "U2"})
		expectStates(mock)
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		mslack.EXPECT().SendDM(gomock.Any(), "U1", gomock.Any()).Return(errors.New("timeout"))
		mslack.EXPECT().GetUser(gomock.Any(), "U2").Return(human("U2"), nil)
		captureDM(mslack, "U2")
		expectMarkReminded(mock).WithArgs(now, "U2", now, now, now).WillReturnResult(sqlmock.NewResult(1, 1))

		report, err := svc.Remind(ctx)
		Expect(err).To(MatchError(ContainSubstring("timeout")))
		Expect(report).To(Equal(&standup.Report{Members: 2, Reminded: 1}))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("returns an error when a member can't be looked up", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"})
		expectStates(mock)
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(nil, errors.New("ratelimited"))

		report, err := svc.Remind(ctx)
		Expect(err).To(MatchError(ContainSubstring("ratelimited")))
		Expect(report).To(Equal(&standup.Report{Members: 1}))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("returns an error when marking a member reminded fails", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"})
		expectStates(mock)
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		captureDM(mslack, "U1")
		expectMarkReminded(mock).WillReturnError(errors.New("db down"))

		report, err := svc.Remind(ctx)
		Expect(err).To(MatchError(ContainSubstring("db down")))
		Expect(report).To(Equal(&standup.Report{Members: 1}))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	Context("before the per-member loop", func() {
		It("fails when the members can't be listed", func(ctx SpecContext) {
			svc, _, mslack := newStandupService(GinkgoT())
			mslack.EXPECT().ChannelMembers(gomock.Any(), "C1").Return(nil, errors.New("not_in_channel"))

			_, err := svc.Remind(ctx)
			Expect(err).To(MatchError(ContainSubstring("not_in_channel")))
		})

		It("fails when the history can't be read", func(ctx SpecContext) {
			svc, _, mslack := newStandupService(GinkgoT())
			mslack.EXPECT().ChannelMembers(gomock.Any(), "C1").Return([]string{"U1"}, nil)
			mslack.EXPECT().ChannelHistory(gomock.Any(), "C1", gomock.Any()).Return(nil, errors.New("channel_not_found"))

			_, err := svc.Remind(ctx)
			Expect(err).To(MatchError(ContainSubstring("channel_not_found")))
		})

		It("fails when the stored members can't be read", func(ctx SpecContext) {
			svc, mock, mslack := newStandupService(GinkgoT())
			expectChannel(mslack, []string{"U1"})
			mock.ExpectQuery(`SELECT .* FROM .*standup_users`).WillReturnError(errors.New("db down"))

			_, err := svc.Remind(ctx)
			Expect(err).To(MatchError(ContainSubstring("db down")))
			Expect(mock.ExpectationsWereMet()).To(Succeed())
		})
	})

	It("reminds again the day after a late answer, quoting it", func(ctx SpecContext) {
		// Reminded yesterday at 09:00, answered at 10:00: today's run must not
		// mistake that answer for today's standup.
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"}, post("U1", yesterday, "Today: late answer"))
		expectStates(mock, []driver.Value{"U1", ts(lastWeek), "Today: old news", 0})
		expectSaveLastUpdate(mock).WillReturnResult(sqlmock.NewResult(1, 1))
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		dm := captureDM(mslack, "U1")
		expectMarkReminded(mock).WillReturnResult(sqlmock.NewResult(1, 1))

		report, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Reminded).To(Equal(1))
		Expect(*dm).To(ContainSubstring("> late answer\n"))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("doesn't rewrite an unchanged update", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"}, post("U1", yesterday, standupText))
		expectStates(mock, []driver.Value{"U1", ts(yesterday), standupText, 0})
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		captureDM(mslack, "U1")
		expectMarkReminded(mock).WillReturnResult(sqlmock.NewResult(1, 1))

		_, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("stores an edited update and quotes the new text", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"}, post("U1", yesterday, "Today: edited"))
		expectStates(mock, []driver.Value{"U1", ts(yesterday), "Today: original", 0})
		expectSaveLastUpdate(mock).WillReturnResult(sqlmock.NewResult(1, 1))
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		dm := captureDM(mslack, "U1")
		expectMarkReminded(mock).WillReturnResult(sqlmock.NewResult(1, 1))

		_, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(*dm).To(ContainSubstring("> edited\n"))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("still sends the DM when storing the update fails, and returns an error", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"}, post("U1", yesterday, standupText))
		expectStates(mock)
		expectSaveLastUpdate(mock).WillReturnError(errors.New("db hiccup"))
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		dm := captureDM(mslack, "U1")
		expectMarkReminded(mock).WillReturnResult(sqlmock.NewResult(1, 1))

		report, err := svc.Remind(ctx)
		Expect(err).To(MatchError(ContainSubstring("db hiccup")))
		Expect(report.Reminded).To(Equal(1))
		Expect(*dm).To(ContainSubstring("> did the thing\n"))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("ignores a member who can never be DMed, without marking them", func(ctx SpecContext) {
		svc, mock, mslack := newStandupService(GinkgoT())
		expectChannel(mslack, []string{"U1"})
		expectStates(mock)
		mslack.EXPECT().GetUser(gomock.Any(), "U1").Return(human("U1"), nil)
		mslack.EXPECT().SendDM(gomock.Any(), "U1", gomock.Any()).
			Return(fmt.Errorf("%w: %w", pkgslack.ErrOpenConversation, slack.SlackErrorResponse{Err: "cannot_dm_bot"}))

		report, err := svc.Remind(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(report).To(Equal(&standup.Report{Members: 1, Ignored: 1}))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})
})

var _ = Describe("NewService", func() {
	It("propagates migration errors", func(ctx SpecContext) {
		db, mock, err := sqlmock.New()
		Expect(err).ToNot(HaveOccurred())
		defer db.Close()
		dbc := database.NewTestClient(db, database.DriverPostgres)

		mfs := fstest.MapFS{
			"postgres/0001_init.sql": {Data: []byte("CREATE TABLE x (id INT);")},
		}
		mock.ExpectExec("CREATE TABLE IF NOT EXISTS marvin_schema_migrations").
			WillReturnError(context.Canceled)

		_, err = standup.NewService(ctx, dbc, mock_slack.NewMockService(gomock.NewController(GinkgoT())), "C1", mfs)
		Expect(err).To(HaveOccurred())
	})
})
