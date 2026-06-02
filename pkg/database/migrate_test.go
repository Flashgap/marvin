package database

import (
	"context"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("applyMigrations", func() {
	It("creates the bookkeeping table, applies pending files in order, and skips applied ones", func(ctx SpecContext) {
		db, mock, err := sqlmock.New()
		Expect(err).ToNot(HaveOccurred())
		defer db.Close()

		mfs := fstest.MapFS{
			"postgres/0001_init.sql":     {Data: []byte("CREATE TABLE a (id INT);")},
			"postgres/0002_more.sql":     {Data: []byte("CREATE TABLE b (id INT);\nCREATE TABLE c (id INT);")},
			"postgres/atlas.sum":         {Data: []byte("# atlas sum file — ignored at runtime\n")},
			"postgres/notes.txt":         {Data: []byte("ignored")},
		}

		mock.ExpectExec("CREATE TABLE IF NOT EXISTS marvin_schema_migrations").
			WillReturnResult(sqlmock.NewResult(0, 0))

		// 0001 is already applied.
		mock.ExpectQuery("SELECT version FROM marvin_schema_migrations").
			WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("0001_init"))

		// 0002 applies in a single tx with both statements.
		mock.ExpectBegin()
		mock.ExpectExec("CREATE TABLE b").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE TABLE c").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("INSERT INTO marvin_schema_migrations").
			WithArgs("0002_more").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		Expect(applyMigrations(ctx, db, DriverPostgres, mfs)).To(Succeed())
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("is a no-op when no files are pending", func(ctx SpecContext) {
		db, mock, err := sqlmock.New()
		Expect(err).ToNot(HaveOccurred())
		defer db.Close()

		mfs := fstest.MapFS{
			"postgres/0001_init.sql": {Data: []byte("CREATE TABLE a (id INT);")},
		}

		mock.ExpectExec("CREATE TABLE IF NOT EXISTS marvin_schema_migrations").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("SELECT version FROM marvin_schema_migrations").
			WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("0001_init"))

		Expect(applyMigrations(ctx, db, DriverPostgres, mfs)).To(Succeed())
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("returns an error when the driver subdirectory is missing", func() {
		db, _, err := sqlmock.New()
		Expect(err).ToNot(HaveOccurred())
		defer db.Close()

		err = applyMigrations(context.Background(), db, DriverMySQL, fstest.MapFS{
			"postgres/0001_init.sql": {Data: []byte("")},
		})
		Expect(err).To(MatchError(ContainSubstring("mysql")))
	})
})
