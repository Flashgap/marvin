package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"ariga.io/atlas/sql/migrate"
	"github.com/doug-martin/goqu/v9"
)

// migrationsTable is the bookkeeping table that records which Atlas migration
// files have already been applied. Schema is kept driver-agnostic.
const migrationsTable = "marvin_schema_migrations"

// applyMigrations applies pending Atlas-formatted .sql migrations from the
// driver-specific subdirectory (`<driver>/`) of mfs. Each statement in each
// pending file runs inside a single per-file transaction. Applied versions are
// recorded in marvin_schema_migrations so re-runs are no-ops.
//
// Non-`.sql` files (e.g. atlas.sum) are ignored — they are used by the Atlas
// CLI for authoring and integrity checking, not at runtime.
func applyMigrations(ctx context.Context, db *sql.DB, driver Driver, mfs fs.FS) error {
	subdir := string(driver)
	entries, err := fs.ReadDir(mfs, subdir)
	if err != nil {
		return fmt.Errorf("migrate: reading %s: %w", subdir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	if err := ensureMigrationsTable(ctx, db, driver); err != nil {
		return err
	}

	applied, err := loadAppliedVersions(ctx, db, driver)
	if err != nil {
		return err
	}

	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		if _, ok := applied[version]; ok {
			continue
		}
		body, err := fs.ReadFile(mfs, path.Join(subdir, name))
		if err != nil {
			return fmt.Errorf("migrate: reading %s: %w", name, err)
		}
		stmts, err := migrate.Stmts(string(body))
		if err != nil {
			return fmt.Errorf("migrate: parsing %s: %w", name, err)
		}
		if err := applyFile(ctx, db, driver, version, stmts); err != nil {
			return fmt.Errorf("migrate: applying %s: %w", name, err)
		}
	}
	return nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB, driver Driver) error {
	tsType := "TIMESTAMP"
	if driver == DriverMySQL {
		tsType = "DATETIME"
	}
	q := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (version VARCHAR(255) PRIMARY KEY, applied_at %s NOT NULL)",
		migrationsTable, tsType,
	)
	if _, err := db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("migrate: creating %s: %w", migrationsTable, err)
	}
	return nil
}

func loadAppliedVersions(ctx context.Context, db *sql.DB, driver Driver) (map[string]struct{}, error) {
	q, _, err := goqu.Dialect(string(driver)).
		From(migrationsTable).
		Select("version").
		Prepared(true).
		ToSQL()
	if err != nil {
		return nil, fmt.Errorf("migrate: building select: %w", err)
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("migrate: reading applied versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := make(map[string]struct{})
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("migrate: scanning version: %w", err)
		}
		applied[v] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applied, nil
}

func applyFile(ctx context.Context, db *sql.DB, driver Driver, version string, stmts []*migrate.Stmt) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, s := range stmts {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, text); err != nil {
			return fmt.Errorf("statement at pos %d: %w", s.Pos, err)
		}
	}

	insert, args, err := goqu.Dialect(string(driver)).
		Insert(migrationsTable).
		Prepared(true).
		Rows(goqu.Record{
			"version":    version,
			"applied_at": goqu.L("NOW()"),
		}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("building insert: %w", err)
	}
	if _, err := tx.ExecContext(ctx, insert, args...); err != nil {
		return fmt.Errorf("recording version: %w", err)
	}

	return tx.Commit()
}
