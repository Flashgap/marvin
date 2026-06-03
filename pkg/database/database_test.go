package database

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	gomysql "github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDatabase(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Database suite")
}

var _ = Describe("buildDSN", func() {
	baseValid := Config{
		Driver:   DriverPostgres,
		Host:     "db.example.com",
		User:     "marvin",
		Database: "marvin",
	}

	Context("validation", func() {
		It("rejects missing Driver", func() {
			cfg := baseValid
			cfg.Driver = ""
			_, _, err := buildDSN(cfg)
			Expect(err).To(MatchError(ContainSubstring("Driver is required")))
		})

		It("rejects unsupported Driver", func() {
			cfg := baseValid
			cfg.Driver = "oracle"
			_, _, err := buildDSN(cfg)
			Expect(err).To(MatchError(ContainSubstring("unsupported driver")))
		})

		It("rejects missing Host", func() {
			cfg := baseValid
			cfg.Host = ""
			_, _, err := buildDSN(cfg)
			Expect(err).To(MatchError(ContainSubstring("Host is required")))
		})

		It("rejects missing User", func() {
			cfg := baseValid
			cfg.User = ""
			_, _, err := buildDSN(cfg)
			Expect(err).To(MatchError(ContainSubstring("User is required")))
		})

		It("rejects missing Database", func() {
			cfg := baseValid
			cfg.Database = ""
			_, _, err := buildDSN(cfg)
			Expect(err).To(MatchError(ContainSubstring("Database is required")))
		})

		It("never leaks the password in error messages", func() {
			cfg := baseValid
			cfg.Driver = "oracle"
			cfg.Password = "super-secret-token"
			_, _, err := buildDSN(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).ToNot(ContainSubstring("super-secret-token"))
		})
	})

	Context("postgres", func() {
		It("builds a key/value DSN with the default port and sorted params", func() {
			cfg := Config{
				Driver:   DriverPostgres,
				Host:     "db.example.com",
				User:     "marvin",
				Password: "pw",
				Database: "marvin",
				Params:   map[string]string{"sslmode": "disable", "application_name": "marvin"},
			}
			driver, dsn, err := buildDSN(cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(driver).To(Equal("pgx"))
			Expect(dsn).To(Equal("host=db.example.com port=5432 user=marvin dbname=marvin password=pw application_name=marvin sslmode=disable"))
		})

		It("uses the explicit port when set", func() {
			cfg := baseValid
			cfg.Port = 6543
			_, dsn, err := buildDSN(cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(dsn).To(ContainSubstring("port=6543"))
		})

		It("quotes values containing spaces or quotes", func() {
			cfg := baseValid
			cfg.Password = `p w'd`
			_, dsn, err := buildDSN(cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(dsn).To(ContainSubstring(`password='p w\'d'`))
		})
	})

	Context("mysql", func() {
		It("builds a DSN with the default port and sorted params", func() {
			cfg := Config{
				Driver:   DriverMySQL,
				Host:     "db.example.com",
				User:     "marvin",
				Password: "pw",
				Database: "marvin",
				Params:   map[string]string{"parseTime": "true", "charset": "utf8mb4"},
			}
			driver, dsn, err := buildDSN(cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(driver).To(Equal("mysql"))
			Expect(dsn).To(Equal("marvin:pw@tcp(db.example.com:3306)/marvin?charset=utf8mb4&parseTime=true"))
		})

		It("omits the password when empty", func() {
			cfg := Config{
				Driver:   DriverMySQL,
				Host:     "db.example.com",
				User:     "marvin",
				Database: "marvin",
			}
			_, dsn, err := buildDSN(cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(dsn).To(Equal("marvin@tcp(db.example.com:3306)/marvin"))
		})

		It("escapes a password containing DSN delimiter characters", func() {
			cfg := Config{
				Driver:   DriverMySQL,
				Host:     "db.example.com",
				User:     "marvin",
				Password: "p@ss:wo/rd?&!",
				Database: "marvin",
			}
			_, dsn, err := buildDSN(cfg)
			Expect(err).ToNot(HaveOccurred())
			// The driver's own parser must round-trip the same fields back.
			parsed, perr := gomysql.ParseDSN(dsn)
			Expect(perr).ToNot(HaveOccurred())
			Expect(parsed.User).To(Equal("marvin"))
			Expect(parsed.Passwd).To(Equal("p@ss:wo/rd?&!"))
			Expect(parsed.Addr).To(Equal("db.example.com:3306"))
			Expect(parsed.DBName).To(Equal("marvin"))
		})
	})
})

var _ = Describe("Client (sqlmock)", func() {
	It("forwards Ping to the underlying *sql.DB", func(ctx SpecContext) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		Expect(err).ToNot(HaveOccurred())
		mock.ExpectPing()

		c := NewTestClient(db, DriverPostgres)
		Expect(c.Ping(ctx)).To(Succeed())
		Expect(c.DB()).To(BeIdenticalTo(db))
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})

	It("closes the underlying *sql.DB", func() {
		db, mock, err := sqlmock.New()
		Expect(err).ToNot(HaveOccurred())
		mock.ExpectClose()

		c := NewTestClient(db, DriverPostgres)
		Expect(c.Close()).To(Succeed())
		Expect(mock.ExpectationsWereMet()).To(Succeed())
	})
})

var _ = Describe("NewClient", func() {
	It("returns the validation error without opening a connection", func() {
		_, err := NewClient(context.Background(), Config{Driver: "oracle", Host: "h", User: "u", Database: "d"})
		Expect(err).To(MatchError(ContainSubstring("unsupported driver")))
	})
})
