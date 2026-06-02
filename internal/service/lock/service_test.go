package lock_test

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/Flashgap/marvin/internal/service/lock"
	mock_lock_slack "github.com/Flashgap/marvin/internal/service/slack/mock"
	slacksvc "github.com/Flashgap/marvin/internal/service/slack"
	"github.com/Flashgap/marvin/pkg/database"
)

func TestLockService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lock service suite")
}

// newLockService constructs a lock service backed by sqlmock, skipping the
// migration step (we don't want to test the database package here).
func newLockService(t GinkgoTInterface) (lock.Service, sqlmock.Sqlmock, *mock_lock_slack.MockService, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())

	dbc := database.NewTestClient(db, database.DriverPostgres)
	ctrl := gomock.NewController(t)
	mslack := mock_lock_slack.NewMockService(ctrl)

	// The real constructor calls db.Migrate first. For unit tests we bypass
	// that by using the test-only constructor exposed below.
	svc := lock.NewTestService(dbc, mslack)

	cleanup := func() { db.Close() }
	return svc, mock, mslack, cleanup
}

var _ = Describe("Lock", func() {
	It("returns a usage ephemeral when text is not a valid mention", func(ctx SpecContext) {
		svc, _, _, cleanup := newLockService(GinkgoT())
		defer cleanup()

		resp, err := svc.Lock(ctx, lock.SlashPayload{UserID: "UVICTIM", Text: "alice"})
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Type).To(Equal(lock.ResponseEphemeral))
		Expect(resp.Text).To(ContainSubstring("Usage"))
	})

	It("rejects self-lock", func(ctx SpecContext) {
		svc, _, _, cleanup := newLockService(GinkgoT())
		defer cleanup()

		resp, err := svc.Lock(ctx, lock.SlashPayload{UserID: "USELF", Text: "<@USELF|me>"})
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Text).To(ContainSubstring("can't lock yourself"))
	})

	It("rejects bot targets", func(ctx SpecContext) {
		svc, _, mslack, cleanup := newLockService(GinkgoT())
		defer cleanup()
		mslack.EXPECT().GetUser(gomock.Any(), "UBOT").Return(&slacksvc.User{ID: "UBOT", Name: "marvin", IsBot: true}, nil)

		resp, err := svc.Lock(ctx, lock.SlashPayload{UserID: "UVICTIM", UserName: "victim", Text: "<@UBOT|marvin>"})
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Text).To(ContainSubstring("Bots"))
	})

	It("applies points + DMs the victim on the happy path", func(ctx SpecContext) {
		svc, mock, mslack, cleanup := newLockService(GinkgoT())
		defer cleanup()

		mslack.EXPECT().GetUser(gomock.Any(), "UFINDER").Return(&slacksvc.User{ID: "UFINDER", Name: "finder"}, nil)

		// Cooldown query: no recent event.
		mock.ExpectQuery("SELECT 1 FROM lock_events").
			WithArgs("UVICTIM", "UFINDER", sqlmock.AnyArg()).
			WillReturnError(sql.ErrNoRows)

		// Transaction: upsert victim, upsert finder, insert event, commit.
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO lock_users").
			WithArgs("UVICTIM", "victim", -1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO lock_users").
			WithArgs("UFINDER", "finder", 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO lock_events").
			WithArgs("UVICTIM", "UFINDER").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		// Read back tallies.
		mock.ExpectQuery("SELECT points FROM lock_users").
			WithArgs("UVICTIM").
			WillReturnRows(sqlmock.NewRows([]string{"points"}).AddRow(-3))
		mock.ExpectQuery("SELECT points FROM lock_users").
			WithArgs("UFINDER").
			WillReturnRows(sqlmock.NewRows([]string{"points"}).AddRow(7))

		// Fire-and-forget DM — async goroutine; allow up to once.
		mslack.EXPECT().SendDM(gomock.Any(), "UVICTIM", gomock.Any()).Return(nil).MaxTimes(1)

		resp, err := svc.Lock(ctx, lock.SlashPayload{UserID: "UVICTIM", UserName: "victim", Text: "<@UFINDER|finder>"})
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Type).To(Equal(lock.ResponseEphemeral))
		Expect(resp.Text).To(ContainSubstring("you: -3"))
		Expect(resp.Text).To(ContainSubstring("<@UFINDER>: 7"))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("reports a cooldown when a recent event exists", func(ctx SpecContext) {
		svc, mock, mslack, cleanup := newLockService(GinkgoT())
		defer cleanup()

		mslack.EXPECT().GetUser(gomock.Any(), "UFINDER").Return(&slacksvc.User{ID: "UFINDER", Name: "finder"}, nil)
		mock.ExpectQuery("SELECT 1 FROM lock_events").
			WithArgs("UVICTIM", "UFINDER", sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))

		resp, err := svc.Lock(ctx, lock.SlashPayload{UserID: "UVICTIM", UserName: "victim", Text: "<@UFINDER|finder>"})
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Text).To(ContainSubstring("Give it a moment"))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})
})

var _ = Describe("Leaderboard", func() {
	It("renders top + bottom 3", func(ctx SpecContext) {
		svc, mock, _, cleanup := newLockService(GinkgoT())
		defer cleanup()

		mock.ExpectQuery("ORDER BY points DESC").
			WillReturnRows(sqlmock.NewRows([]string{"slack_user_id", "slack_user_name", "points"}).
				AddRow("U1", "alice", 5).
				AddRow("U2", "bob", 3))
		mock.ExpectQuery("ORDER BY points ASC").
			WillReturnRows(sqlmock.NewRows([]string{"slack_user_id", "slack_user_name", "points"}).
				AddRow("U3", "carol", -2))

		resp, err := svc.Leaderboard(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Type).To(Equal(lock.ResponseEphemeral))
		Expect(resp.Text).To(ContainSubstring("*Top 3*"))
		Expect(resp.Text).To(ContainSubstring("@alice — 5"))
		Expect(resp.Text).To(ContainSubstring("*Bottom 3*"))
		Expect(resp.Text).To(ContainSubstring("@carol — -2"))
	})

	It("returns a friendly empty message", func(ctx SpecContext) {
		svc, mock, _, cleanup := newLockService(GinkgoT())
		defer cleanup()

		empty := sqlmock.NewRows([]string{"slack_user_id", "slack_user_name", "points"})
		mock.ExpectQuery("ORDER BY points DESC").WillReturnRows(empty)
		mock.ExpectQuery("ORDER BY points ASC").WillReturnRows(empty)

		resp, err := svc.Leaderboard(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Text).To(ContainSubstring("No locks yet"))
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

		_, err = lock.NewService(ctx, dbc, nil, mfs)
		Expect(err).To(HaveOccurred())
	})
})
