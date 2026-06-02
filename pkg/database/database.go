//go:generate mockgen --source=$GOFILE --destination=mock/mock.go --package mock_database
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	// Driver registrations. Both drivers are always linked in; the runtime
	// configuration selects which one is used.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Driver identifies a supported SQL driver.
type Driver string

const (
	DriverPostgres Driver = "postgres"
	DriverMySQL    Driver = "mysql"

	defaultPostgresPort = 5432
	defaultMySQLPort    = 3306
)

// Client wraps a *sql.DB managed by this package.
type Client interface {
	// DB returns the underlying *sql.DB handle for callers that need it.
	DB() *sql.DB
	// Driver returns the configured driver.
	Driver() Driver
	// Dialect returns the driver-specific SQL dialect (placeholders, UPSERTs).
	Dialect() Dialect
	// Ping verifies the connection is alive.
	Ping(ctx context.Context) error
	// Migrate applies any pending Atlas-formatted SQL migrations embedded under
	// the driver-specific subdirectory of fs (i.e. fs/<driver>/*.sql).
	Migrate(ctx context.Context, fs fs.FS) error
	// Close releases the connection pool.
	Close() error
}

// Config holds connection settings used to build a Client.
type Config struct {
	Driver   Driver
	Host     string
	Port     int
	User     string
	Password string
	Database string
	// Params is a map of driver-specific connection parameters.
	Params map[string]string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type client struct {
	db     *sql.DB
	driver Driver
}

// NewClient validates the config, opens the connection pool, applies pool
// tuning, and pings the database. The returned Client owns the *sql.DB and is
// responsible for closing it.
func NewClient(ctx context.Context, cfg Config) (Client, error) {
	driverName, dsn, err := buildDSN(cfg)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening %s database: %w", cfg.Driver, err)
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging %s database: %w", cfg.Driver, err)
	}

	return &client{db: db, driver: cfg.Driver}, nil
}

// NewTestClient wraps an existing *sql.DB without opening a connection or
// applying pool settings. Intended for tests that drive the Client via
// sqlmock; not for production use.
func NewTestClient(db *sql.DB, driver Driver) Client {
	return &client{db: db, driver: driver}
}

func (c *client) DB() *sql.DB                    { return c.db }
func (c *client) Driver() Driver                 { return c.driver }
func (c *client) Dialect() Dialect               { return dialectFor(c.driver) }
func (c *client) Ping(ctx context.Context) error { return c.db.PingContext(ctx) }
func (c *client) Close() error                   { return c.db.Close() }

func (c *client) Migrate(ctx context.Context, mfs fs.FS) error {
	return applyMigrations(ctx, c.db, c.driver, mfs)
}

// buildDSN validates cfg and returns the database/sql driver name and DSN.
// The password is never included in error messages.
func buildDSN(cfg Config) (string, string, error) {
	if cfg.Host == "" {
		return "", "", fmt.Errorf("database: Host is required")
	}
	if cfg.User == "" {
		return "", "", fmt.Errorf("database: User is required")
	}
	if cfg.Database == "" {
		return "", "", fmt.Errorf("database: Database is required")
	}

	switch cfg.Driver {
	case DriverPostgres:
		port := cfg.Port
		if port == 0 {
			port = defaultPostgresPort
		}
		return "pgx", postgresDSN(cfg, port), nil
	case DriverMySQL:
		port := cfg.Port
		if port == 0 {
			port = defaultMySQLPort
		}
		return "mysql", mysqlDSN(cfg, port), nil
	case "":
		return "", "", fmt.Errorf("database: Driver is required (supported: %s, %s)", DriverPostgres, DriverMySQL)
	default:
		return "", "", fmt.Errorf("database: unsupported driver %q (supported: %s, %s)", cfg.Driver, DriverPostgres, DriverMySQL)
	}
}

// postgresDSN builds a libpq-style key/value DSN. Key/value avoids
// URL-encoding pitfalls for passwords with special characters.
func postgresDSN(cfg Config, port int) string {
	parts := []string{
		"host=" + pgQuote(cfg.Host),
		"port=" + strconv.Itoa(port),
		"user=" + pgQuote(cfg.User),
		"dbname=" + pgQuote(cfg.Database),
	}
	if cfg.Password != "" {
		parts = append(parts, "password="+pgQuote(cfg.Password))
	}
	for _, k := range sortedKeys(cfg.Params) {
		parts = append(parts, k+"="+pgQuote(cfg.Params[k]))
	}
	return strings.Join(parts, " ")
}

// pgQuote escapes a libpq key/value value: wraps it in single quotes when it
// contains whitespace, a single quote, or a backslash.
func pgQuote(v string) string {
	if v == "" {
		return "''"
	}
	if !strings.ContainsAny(v, " \t'\\") {
		return v
	}
	escaped := strings.ReplaceAll(v, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}

// mysqlDSN builds a go-sql-driver/mysql DSN:
//
//	user:password@tcp(host:port)/dbname?k=v&...
func mysqlDSN(cfg Config, port int) string {
	var b strings.Builder
	b.WriteString(cfg.User)
	if cfg.Password != "" {
		b.WriteByte(':')
		b.WriteString(cfg.Password)
	}
	b.WriteString("@tcp(")
	b.WriteString(cfg.Host)
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(port))
	b.WriteString(")/")
	b.WriteString(cfg.Database)

	keys := sortedKeys(cfg.Params)
	if len(keys) > 0 {
		b.WriteByte('?')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte('&')
			}
			b.WriteString(url.QueryEscape(k))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(cfg.Params[k]))
		}
	}
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
