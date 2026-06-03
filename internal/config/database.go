package config

import (
	"time"

	"github.com/Flashgap/marvin/pkg/database"
)

// Database configuration. The database client is optional: when DBHost is empty,
// no database client is initialized.
type Database struct {
	// DBHost is the database host. When empty, the database feature is disabled.
	DBHost string `envconfig:"DB_HOST"`

	// DBDriver selects the database driver. Required when DBHost is set.
	// Supported values: "postgres", "mysql".
	DBDriver string `envconfig:"DB_DRIVER"`

	// DBPort is the database port. Defaults to the driver-standard port if zero.
	DBPort int `envconfig:"DB_PORT"`

	// DBUser is the database user.
	DBUser string `envconfig:"DB_USER"`

	// DBPassword is the database password.
	DBPassword string `envconfig:"DB_PASSWORD" secret:"true"`

	// DBName is the database name to connect to.
	DBName string `envconfig:"DB_NAME"`

	// DBParams is an optional map of driver-specific connection parameters,
	// e.g. "sslmode:disable,connect_timeout:5".
	DBParams map[string]string `envconfig:"DB_PARAMS"`

	// DBMaxOpenConns caps the number of open connections to the database.
	DBMaxOpenConns int `envconfig:"DB_MAX_OPEN_CONNS" default:"25"`

	// DBMaxIdleConns caps the number of idle connections kept in the pool.
	DBMaxIdleConns int `envconfig:"DB_MAX_IDLE_CONNS" default:"5"`

	// DBConnMaxLifetime caps how long a connection may be reused.
	DBConnMaxLifetime time.Duration `envconfig:"DB_CONN_MAX_LIFETIME" default:"30m"`

	// DBConnMaxIdleTime caps how long a connection may remain idle.
	DBConnMaxIdleTime time.Duration `envconfig:"DB_CONN_MAX_IDLE_TIME" default:"5m"`
}

// DBEnabled reports whether the database client should be initialized.
func (c Database) DBEnabled() bool {
	return c.DBHost != ""
}

// DatabaseConfig converts the env-bound configuration into a package-level
// database.Config consumed by pkg/database.NewClient.
func (c Database) DatabaseConfig() database.Config {
	return database.Config{
		Driver:          database.Driver(c.DBDriver),
		Host:            c.DBHost,
		Port:            c.DBPort,
		User:            c.DBUser,
		Password:        c.DBPassword,
		Database:        c.DBName,
		Params:          c.DBParams,
		MaxOpenConns:    c.DBMaxOpenConns,
		MaxIdleConns:    c.DBMaxIdleConns,
		ConnMaxLifetime: c.DBConnMaxLifetime,
		ConnMaxIdleTime: c.DBConnMaxIdleTime,
	}
}
