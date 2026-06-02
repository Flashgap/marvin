package database

import (
	"fmt"
	"strings"
)

// Dialect exposes the driver-specific bits of SQL composition.
type Dialect interface {
	// Placeholder returns the parameter placeholder for the given 1-indexed
	// position. "$1", "$2", ... for Postgres; "?" for MySQL.
	Placeholder(n int) string

	// Upsert returns an INSERT statement that updates the listed columns
	// when a row with the given unique-key column already exists.
	// insertCols is the full list of columns provided in the INSERT (including
	// keyCol); updateCols is the subset of columns to overwrite on conflict.
	// The returned SQL uses dialect placeholders in insertCols order.
	Upsert(table, keyCol string, insertCols, updateCols []string) string
}

func dialectFor(d Driver) Dialect {
	switch d {
	case DriverMySQL:
		return mysqlDialect{}
	default:
		return postgresDialect{}
	}
}

type postgresDialect struct{}

func (postgresDialect) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }

func (d postgresDialect) Upsert(table, keyCol string, insertCols, updateCols []string) string {
	placeholders := make([]string, len(insertCols))
	for i := range insertCols {
		placeholders[i] = d.Placeholder(i + 1)
	}
	setClauses := make([]string, len(updateCols))
	for i, c := range updateCols {
		setClauses[i] = fmt.Sprintf("%s = EXCLUDED.%s", c, c)
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s",
		table,
		strings.Join(insertCols, ", "),
		strings.Join(placeholders, ", "),
		keyCol,
		strings.Join(setClauses, ", "),
	)
}

type mysqlDialect struct{}

func (mysqlDialect) Placeholder(_ int) string { return "?" }

func (d mysqlDialect) Upsert(table, _ string, insertCols, updateCols []string) string {
	placeholders := make([]string, len(insertCols))
	for i := range insertCols {
		placeholders[i] = "?"
	}
	setClauses := make([]string, len(updateCols))
	for i, c := range updateCols {
		setClauses[i] = fmt.Sprintf("%s = VALUES(%s)", c, c)
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		table,
		strings.Join(insertCols, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(setClauses, ", "),
	)
}
